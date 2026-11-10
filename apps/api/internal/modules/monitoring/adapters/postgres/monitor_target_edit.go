package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorTargetEditRepository = (*Repository)(nil)

// EditTarget replaces one Monitor's target URL inside one Read Committed
// transaction: the existence probe and the update pipeline through one batch
// round trip, the returned row maps through the domain constructor, and the
// commit happens before success. Identifiers bind as binary uuid parameters,
// matching the deletion and history adapters' discipline.
func (repository *Repository) EditTarget(ctx context.Context, monitorID domain.MonitorID, targetURL domain.TargetURL) (domain.Monitor, error) {
	if err := ctx.Err(); err != nil {
		return domain.Monitor{}, err
	}

	bind := pgtype.UUID{Bytes: monitorID.UUID(), Valid: true}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("begin target edit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	const existsProbe = "SELECT EXISTS (SELECT 1 FROM monitoring.monitors WHERE id = $1)"
	const editTarget = "UPDATE monitoring.monitors SET target_url = $2 WHERE id = $1 RETURNING id::text, target_url, created_at"

	batch := &pgx.Batch{}
	batch.Queue(existsProbe, bind)
	batch.Queue(editTarget, bind, targetURL.String())
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	var exists bool
	if err := results.QueryRow().Scan(&exists); err != nil {
		return domain.Monitor{}, fmt.Errorf("probe monitor existence: %w", err)
	}
	if !exists {
		return domain.Monitor{}, ports.ErrMonitorNotFound
	}

	var (
		rawID        string
		rawTargetURL string
		createdAt    time.Time
	)
	if err := results.QueryRow().Scan(&rawID, &rawTargetURL, &createdAt); err != nil {
		return domain.Monitor{}, fmt.Errorf("read edited monitor row: %w", err)
	}
	if err := results.Close(); err != nil {
		return domain.Monitor{}, fmt.Errorf("finish target edit batch: %w", err)
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map edited monitor id: %w", err)
	}
	parsedTarget, err := domain.NewTargetURL(rawTargetURL)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map edited target: %w", err)
	}
	monitor, err := domain.NewMonitor(id, parsedTarget, createdAt.UTC())
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map edited monitor: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Monitor{}, fmt.Errorf("commit target edit transaction: %w", err)
	}
	return monitor, nil
}
