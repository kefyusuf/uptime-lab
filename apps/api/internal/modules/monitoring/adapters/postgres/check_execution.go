package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.CheckExecutionRepository = (*Repository)(nil)

// ClaimDueCheck atomically reconciles expired work, claims one due Monitor, and
// creates its durable pending CheckRun.
func (repository *Repository) ClaimDueCheck(
	ctx context.Context,
	input ports.ClaimDueCheckInput,
) (ports.ClaimedCheck, error) {
	if err := ctx.Err(); err != nil {
		return ports.ClaimedCheck{}, err
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	rows, err := tx.Query(
		ctx,
		`
			UPDATE monitoring.check_runs
			SET completed_at = deadline_at,
			    result_kind = 'worker_timeout'
			WHERE completed_at IS NULL
			  AND deadline_at <= $1
			RETURNING monitor_id::text
		`,
		input.Now,
	)
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("reconcile expired check runs: %w", err)
	}
	reconciled, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("collect reconciled monitors: %w", err)
	}
	if err := resetReconciledIncidentState(ctx, tx, reconciled); err != nil {
		return ports.ClaimedCheck{}, err
	}

	if err := repository.retainHistoryWindow(ctx, tx); err != nil {
		return ports.ClaimedCheck{}, err
	}

	var (
		rawMonitorID string
		rawTargetURL string
	)
	err = tx.QueryRow(
		ctx,
		`
			SELECT
				monitor.id::text,
				monitor.target_url
			FROM monitoring.monitors AS monitor
			LEFT JOIN LATERAL (
				SELECT run.completed_at AS latest_completed_at
				FROM monitoring.check_runs AS run
				WHERE run.monitor_id = monitor.id
				  AND run.completed_at IS NOT NULL
				ORDER BY run.completed_at DESC
				LIMIT 1
			) AS terminal ON true
			WHERE monitor.paused = false AND NOT EXISTS (
				SELECT 1
				FROM monitoring.check_runs AS pending
				WHERE pending.monitor_id = monitor.id
				  AND pending.completed_at IS NULL
			)
			  AND (
				terminal.latest_completed_at IS NULL
				OR terminal.latest_completed_at <= $1
			  )
			ORDER BY
				COALESCE(terminal.latest_completed_at, monitor.created_at) ASC,
				monitor.id ASC
			FOR UPDATE OF monitor SKIP LOCKED
			LIMIT 1
		`,
		input.DueBefore,
	).Scan(&rawMonitorID, &rawTargetURL)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return ports.ClaimedCheck{}, fmt.Errorf("commit empty claim transaction: %w", err)
		}
		return ports.ClaimedCheck{}, ports.ErrNoDueCheck
	}
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("select due monitor: %w", err)
	}

	// A separate Read Committed statement observes the locked candidate's current state.
	var paused bool
	if err := tx.QueryRow(ctx, `SELECT paused FROM monitoring.monitors WHERE id=$1::uuid`, rawMonitorID).Scan(&paused); err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("recheck locked monitor scheduling: %w", err)
	}
	if paused {
		if err := tx.Commit(ctx); err != nil {
			return ports.ClaimedCheck{}, fmt.Errorf("commit paused candidate: %w", err)
		}
		return ports.ClaimedCheck{}, ports.ErrNoDueCheck
	}

	if _, err := tx.Exec(
		ctx,
		`
			INSERT INTO monitoring.check_runs (
				id,
				monitor_id,
				issued_at,
				deadline_at
			)
			VALUES ($1::uuid, $2::uuid, $3, $4)
		`,
		input.CheckID.String(),
		rawMonitorID,
		input.Now,
		input.Deadline,
	); err != nil {
		if isPendingClaimConflict(err) {
			return ports.ClaimedCheck{}, ports.ErrNoDueCheck
		}
		return ports.ClaimedCheck{}, fmt.Errorf("insert pending check run: %w", err)
	}

	monitorID, err := domain.ParseMonitorID(rawMonitorID)
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("map claimed monitor id: %w", err)
	}
	targetURL, err := domain.NewTargetURL(rawTargetURL)
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("map claimed target URL: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("commit claim transaction: %w", err)
	}

	return ports.ClaimedCheck{
		MonitorID: monitorID,
		TargetURL: targetURL,
	}, nil
}

// CompleteCheck atomically terminalizes one pending CheckRun or verifies an
// exact idempotent duplicate without mutating an existing terminal row.
func (repository *Repository) CompleteCheck(
	ctx context.Context,
	input ports.CompleteCheckInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin check completion transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	var (
		rawMonitorID     string
		deadlineAt       time.Time
		completedAt      *time.Time
		resultKind       *string
		storedHTTPStatus *int
		storedDurationMS *int64
	)
	err = tx.QueryRow(
		ctx,
		`
			SELECT monitor_id::text, deadline_at, completed_at, result_kind, http_status, duration_ms
			FROM monitoring.check_runs
			WHERE id = $1::uuid
			FOR UPDATE
		`,
		input.CheckID.String(),
	).Scan(
		&rawMonitorID,
		&deadlineAt,
		&completedAt,
		&resultKind,
		&storedHTTPStatus,
		&storedDurationMS,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit missing check transaction: %w", err)
		}
		return ports.ErrCheckRunNotFound
	}
	if err != nil {
		return fmt.Errorf("lock check run: %w", err)
	}
	monitorID, err := domain.ParseMonitorID(rawMonitorID)
	if err != nil {
		return fmt.Errorf("map completion monitor id: %w", err)
	}

	if completedAt != nil {
		if terminalResultMatches(input.Result, resultKind, storedHTTPStatus, storedDurationMS) {
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit duplicate check result: %w", err)
			}
			return nil
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit conflicting check result: %w", err)
		}
		return ports.ErrCheckRunConflict
	}

	if !input.CompletedAt.Before(deadlineAt) {
		if _, err := tx.Exec(
			ctx,
			`
				UPDATE monitoring.check_runs
				SET completed_at = deadline_at,
				    result_kind = 'worker_timeout'
				WHERE id = $1::uuid
				  AND completed_at IS NULL
			`,
			input.CheckID.String(),
		); err != nil {
			return fmt.Errorf("terminalize late check run: %w", err)
		}

		if err := repository.foldIncidentOutcome(ctx, tx, monitorID, domain.TerminalInconclusive, "", deadlineAt, input.CheckID, input.IncidentID); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit late check timeout: %w", err)
		}
		return ports.ErrCheckRunConflict
	}

	httpStatus, hasHTTPStatus := input.Result.HTTPStatus()
	var persistedHTTPStatus *int
	if hasHTTPStatus {
		persistedHTTPStatus = &httpStatus
	}
	durationMS := input.Result.DurationMS()

	if _, err := tx.Exec(
		ctx,
		`
			UPDATE monitoring.check_runs
			SET completed_at = $2,
			    result_kind = $3,
			    http_status = $4,
			    duration_ms = $5
			WHERE id = $1::uuid
			  AND completed_at IS NULL
		`,
		input.CheckID.String(),
		input.CompletedAt,
		string(input.Result.Kind()),
		persistedHTTPStatus,
		durationMS,
	); err != nil {
		return fmt.Errorf("complete check run: %w", err)
	}

	outcome, reason, err := domain.ClassifyTerminalOutcome(input.Result.Kind(), persistedHTTPStatus)
	if err != nil {
		return fmt.Errorf("classify terminal outcome: %w", err)
	}
	if err := repository.foldIncidentOutcome(ctx, tx, monitorID, outcome, reason, input.CompletedAt, input.CheckID, input.IncidentID); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit check result: %w", err)
	}
	return nil
}

func terminalResultMatches(
	result domain.CheckResult,
	storedKind *string,
	storedHTTPStatus *int,
	storedDurationMS *int64,
) bool {
	if storedKind == nil || storedDurationMS == nil {
		return false
	}
	if *storedKind != string(result.Kind()) || *storedDurationMS != result.DurationMS() {
		return false
	}

	httpStatus, hasHTTPStatus := result.HTTPStatus()
	if !hasHTTPStatus {
		return storedHTTPStatus == nil
	}
	return storedHTTPStatus != nil && *storedHTTPStatus == httpStatus
}

func isPendingClaimConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "check_runs_one_pending_per_monitor_idx"
}

// foldIncidentOutcome advances the durable per-Monitor incident fold inside the
// completion transaction, still holding the CheckRun row lock. The incidents row
// is written before incident_state so the open_incident_id foreign key always
// references an existing window; the steady-state upsert of an existing row whose
// monitor_id is unchanged takes no monitors lock.
func (repository *Repository) foldIncidentOutcome(
	ctx context.Context,
	tx pgx.Tx,
	monitorID domain.MonitorID,
	outcome domain.TerminalOutcome,
	reason domain.IncidentReason,
	at time.Time,
	checkID domain.CheckID,
	candidateID domain.IncidentID,
) error {
	state, err := readIncidentState(ctx, tx, monitorID)
	if err != nil {
		return err
	}
	next, action := domain.ApplyIncidentOutcome(state, outcome, reason, at, checkID, candidateID)

	switch action.Kind {
	case domain.IncidentActionOpen:
		if _, err := tx.Exec(ctx,
			`
				INSERT INTO monitoring.incidents (
					id, monitor_id, opened_at, opening_reason, opened_by_check_id
				)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)
			`,
			action.IncidentID.String(),
			monitorID.String(),
			action.At,
			string(action.Reason),
			action.CheckID.String(),
		); err != nil {
			return fmt.Errorf("open durable incident: %w", err)
		}
	case domain.IncidentActionResolve:
		if _, err := tx.Exec(ctx,
			`
				UPDATE monitoring.incidents
				SET resolved_at = $2,
				    resolved_by_check_id = $3::uuid
				WHERE id = $1::uuid
			`,
			action.IncidentID.String(),
			action.At,
			action.CheckID.String(),
		); err != nil {
			return fmt.Errorf("resolve durable incident: %w", err)
		}
	}

	var openPointer *string
	if next.OpenIncidentID != nil {
		raw := next.OpenIncidentID.String()
		openPointer = &raw
	}
	if _, err := tx.Exec(ctx,
		`
			INSERT INTO monitoring.incident_state (
				monitor_id, failure_streak, success_streak, open_incident_id
			)
			VALUES ($1::uuid, $2, $3, $4::uuid)
			ON CONFLICT (monitor_id) DO UPDATE SET
				failure_streak = EXCLUDED.failure_streak,
				success_streak = EXCLUDED.success_streak,
				open_incident_id = EXCLUDED.open_incident_id
		`,
		monitorID.String(),
		next.FailureStreak,
		next.SuccessStreak,
		openPointer,
	); err != nil {
		return fmt.Errorf("upsert incident state: %w", err)
	}
	return nil
}

func readIncidentState(ctx context.Context, tx pgx.Tx, monitorID domain.MonitorID) (domain.IncidentState, error) {
	var (
		failure int
		success int
		rawOpen *string
	)
	err := tx.QueryRow(ctx,
		`
			SELECT failure_streak, success_streak, open_incident_id::text
			FROM monitoring.incident_state
			WHERE monitor_id = $1::uuid
		`,
		monitorID.String(),
	).Scan(&failure, &success, &rawOpen)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IncidentState{}, nil
	}
	if err != nil {
		return domain.IncidentState{}, fmt.Errorf("read incident state: %w", err)
	}
	state := domain.IncidentState{FailureStreak: failure, SuccessStreak: success}
	if rawOpen != nil {
		openID, err := domain.ParseIncidentID(*rawOpen)
		if err != nil {
			return domain.IncidentState{}, fmt.Errorf("map open incident id: %w", err)
		}
		state.OpenIncidentID = &openID
	}
	return state, nil
}

// resetReconciledIncidentState zeroes both streaks for exactly the monitors whose
// expired pending runs were just terminalized, leaving any open pointer untouched
// (an inconclusive reconciliation never resolves). It pipelines through one batch.
func resetReconciledIncidentState(ctx context.Context, tx pgx.Tx, monitors []string) error {
	if len(monitors) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, rawMonitor := range monitors {
		batch.Queue(
			`UPDATE monitoring.incident_state SET failure_streak = 0, success_streak = 0 WHERE monitor_id = $1::uuid`,
			rawMonitor,
		)
	}
	results := tx.SendBatch(ctx, batch)
	for range monitors {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("reset reconciled incident state: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("finish reconciliation reset batch: %w", err)
	}
	return nil
}
