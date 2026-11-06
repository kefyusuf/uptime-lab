package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorHistoryRepository = (*Repository)(nil)

// Page reads one bounded terminal-history window inside one Read Committed
// transaction: the Monitor existence probe and the keyset page query pipeline
// through one batch round trip and share a snapshot, so a concurrent deletion
// resolves as a consistent page or a 404. Identifiers bind as binary uuid
// parameters, matching the deletion adapter's discipline.
func (repository *Repository) Page(ctx context.Context, monitorID domain.MonitorID, anchor *ports.HistoryAnchor, maxRows int) ([]ports.LatestCheckResultRecord, error) {
	if maxRows < 1 || maxRows > 51 {
		return nil, errors.New("invalid history repository bounds")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bind := pgtype.UUID{Bytes: monitorID.UUID(), Valid: true}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin history transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	const existsProbe = "SELECT EXISTS (SELECT 1 FROM monitoring.monitors WHERE id = $1)"
	const firstPage = "SELECT id::text, result_kind, http_status, duration_ms, completed_at FROM monitoring.check_runs WHERE monitor_id = $1 AND completed_at IS NOT NULL ORDER BY completed_at DESC, id DESC LIMIT $2"
	const anchoredPage = "SELECT id::text, result_kind, http_status, duration_ms, completed_at FROM monitoring.check_runs WHERE monitor_id = $1 AND completed_at IS NOT NULL AND (completed_at, id) < ($2, $3) ORDER BY completed_at DESC, id DESC LIMIT $4"

	batch := &pgx.Batch{}
	batch.Queue(existsProbe, bind)
	if anchor != nil {
		batch.Queue(anchoredPage, bind, anchor.CompletedAt, pgtype.UUID{Bytes: anchor.CheckID.UUID(), Valid: true}, maxRows)
	} else {
		batch.Queue(firstPage, bind, maxRows)
	}
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	var exists bool
	if err := results.QueryRow().Scan(&exists); err != nil {
		return nil, fmt.Errorf("probe monitor existence: %w", err)
	}
	if !exists {
		return nil, ports.ErrMonitorNotFound
	}

	rows, err := results.Query()
	if err != nil {
		return nil, fmt.Errorf("query monitor history: %w", err)
	}
	defer rows.Close()

	entries := make([]ports.LatestCheckResultRecord, 0, maxRows)
	for rows.Next() {
		var rawCheckID string
		var kind string
		var httpStatus *int
		var durationMS *int64
		var completedAt time.Time
		if err := rows.Scan(&rawCheckID, &kind, &httpStatus, &durationMS, &completedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		checkID, err := domain.ParseCheckID(rawCheckID)
		if err != nil {
			return nil, fmt.Errorf("map history check id: %w", err)
		}
		entries = append(entries, ports.LatestCheckResultRecord{
			CheckID:     checkID,
			ResultKind:  domain.CheckResultKind(kind),
			HTTPStatus:  httpStatus,
			DurationMS:  durationMS,
			CompletedAt: completedAt.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history rows: %w", err)
	}
	if err := results.Close(); err != nil {
		return nil, fmt.Errorf("finish history batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit history transaction: %w", err)
	}
	return entries, nil
}
