package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	if _, err := tx.Exec(
		ctx,
		`
			UPDATE monitoring.check_runs
			SET completed_at = deadline_at,
			    result_kind = 'worker_timeout'
			WHERE completed_at IS NULL
			  AND deadline_at <= $1
		`,
		input.Now,
	); err != nil {
		return ports.ClaimedCheck{}, fmt.Errorf("reconcile expired check runs: %w", err)
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
			WHERE NOT EXISTS (
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
		deadlineAt       time.Time
		completedAt      *time.Time
		resultKind       *string
		storedHTTPStatus *int
		storedDurationMS *int64
	)
	err = tx.QueryRow(
		ctx,
		`
			SELECT deadline_at, completed_at, result_kind, http_status, duration_ms
			FROM monitoring.check_runs
			WHERE id = $1::uuid
			FOR UPDATE
		`,
		input.CheckID.String(),
	).Scan(
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
