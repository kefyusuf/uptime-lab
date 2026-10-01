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

var _ ports.LatestCheckResultRepository = (*Repository)(nil)

// LatestTerminalByMonitorID reads one Monitor's latest terminal CheckRun without mutating execution state.
func (repository *Repository) LatestTerminalByMonitorID(
	ctx context.Context,
	monitorID domain.MonitorID,
) (ports.LatestCheckResultRecord, error) {
	if err := ctx.Err(); err != nil {
		return ports.LatestCheckResultRecord{}, err
	}

	var (
		rawCheckID  *string
		rawKind     *string
		httpStatus  *int
		durationMS  *int64
		completedAt *time.Time
	)

	err := repository.pool.QueryRow(
		ctx,
		`
			SELECT
				latest.id::text,
				latest.result_kind,
				latest.http_status,
				latest.duration_ms,
				latest.completed_at
			FROM monitoring.monitors AS monitor
			LEFT JOIN LATERAL (
				SELECT
					run.id,
					run.result_kind,
					run.http_status,
					run.duration_ms,
					run.completed_at
				FROM monitoring.check_runs AS run
				WHERE run.monitor_id = monitor.id
				  AND run.completed_at IS NOT NULL
				ORDER BY
					run.completed_at DESC,
					run.issued_at DESC,
					run.id DESC
				LIMIT 1
			) AS latest ON true
			WHERE monitor.id = $1::uuid
		`,
		monitorID.String(),
	).Scan(
		&rawCheckID,
		&rawKind,
		&httpStatus,
		&durationMS,
		&completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.LatestCheckResultRecord{}, ports.ErrMonitorNotFound
	}
	if err != nil {
		return ports.LatestCheckResultRecord{}, fmt.Errorf("get latest terminal check result: %w", err)
	}

	if rawCheckID == nil &&
		rawKind == nil &&
		httpStatus == nil &&
		durationMS == nil &&
		completedAt == nil {
		return ports.LatestCheckResultRecord{}, ports.ErrNoTerminalCheckResult
	}
	if rawCheckID == nil || rawKind == nil || completedAt == nil {
		return ports.LatestCheckResultRecord{}, errors.New("map latest terminal check result: incomplete terminal row")
	}

	checkID, err := domain.ParseCheckID(*rawCheckID)
	if err != nil {
		return ports.LatestCheckResultRecord{}, fmt.Errorf("map latest terminal check id: %w", err)
	}

	return ports.LatestCheckResultRecord{
		CheckID:     checkID,
		ResultKind:  domain.CheckResultKind(*rawKind),
		HTTPStatus:  httpStatus,
		DurationMS:  durationMS,
		CompletedAt: *completedAt,
	}, nil
}
