package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// RecentTerminalByMonitorID reads up to limit newest terminal CheckRuns for one
// Monitor without mutating execution state, newest first, so index 0 matches
// LatestTerminalByMonitorID.
func (repository *Repository) RecentTerminalByMonitorID(
	ctx context.Context,
	monitorID domain.MonitorID,
	limit int,
) ([]ports.LatestCheckResultRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rows, err := repository.pool.Query(
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
				LIMIT $2
			) AS latest ON true
			WHERE monitor.id = $1::uuid
		`,
		monitorID.String(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent terminal check results: %w", err)
	}
	defer rows.Close()

	records := make([]ports.LatestCheckResultRecord, 0, limit)
	for rows.Next() {
		var (
			rawCheckID  *string
			rawKind     *string
			httpStatus  *int
			durationMS  *int64
			completedAt *time.Time
		)
		if err := rows.Scan(&rawCheckID, &rawKind, &httpStatus, &durationMS, &completedAt); err != nil {
			return nil, fmt.Errorf("scan recent terminal check result: %w", err)
		}
		if rawCheckID == nil &&
			rawKind == nil &&
			httpStatus == nil &&
			durationMS == nil &&
			completedAt == nil {
			return []ports.LatestCheckResultRecord{}, nil
		}
		if rawCheckID == nil || rawKind == nil || completedAt == nil {
			return nil, errors.New("map recent terminal check result: incomplete terminal row")
		}
		checkID, err := domain.ParseCheckID(*rawCheckID)
		if err != nil {
			return nil, fmt.Errorf("map recent terminal check id: %w", err)
		}
		records = append(records, ports.LatestCheckResultRecord{
			CheckID:     checkID,
			ResultKind:  domain.CheckResultKind(*rawKind),
			HTTPStatus:  httpStatus,
			DurationMS:  durationMS,
			CompletedAt: *completedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent terminal check results: %w", err)
	}
	if len(records) == 0 {
		return nil, ports.ErrMonitorNotFound
	}
	return records, nil
}
