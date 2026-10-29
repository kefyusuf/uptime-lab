package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorDeletionRepository = (*Repository)(nil)

// Delete permanently removes one Monitor and every CheckRun referencing it in
// one Read Committed transaction. The four statements pipeline through one
// batch round trip: children are swept, the Monitor row is locked with
// not-found detection, children are swept again under the lock, the parent
// row is removed, and the commit happens before success. The identifier binds
// as a binary uuid parameter.
func (repository *Repository) Delete(ctx context.Context, id domain.MonitorID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	bind := pgtype.UUID{Bytes: id.UUID(), Valid: true}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin monitor deletion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	const sweepRuns = "DELETE FROM monitoring.check_runs WHERE monitor_id = $1"
	const lockMonitor = "SELECT id::text FROM monitoring.monitors WHERE id = $1 FOR UPDATE"
	const removeMonitor = "DELETE FROM monitoring.monitors WHERE id = $1"

	batch := &pgx.Batch{}
	batch.Queue(sweepRuns, bind)
	batch.Queue(lockMonitor, bind)
	batch.Queue(sweepRuns, bind)
	batch.Queue(removeMonitor, bind)
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	if _, err := results.Exec(); err != nil {
		return fmt.Errorf("sweep monitor check runs: %w", err)
	}

	var lockedID string
	if err := results.QueryRow().Scan(&lockedID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrMonitorNotFound
		}
		return fmt.Errorf("lock monitor for deletion: %w", err)
	}

	if _, err := results.Exec(); err != nil {
		return fmt.Errorf("resweep monitor check runs: %w", err)
	}

	if _, err := results.Exec(); err != nil {
		return fmt.Errorf("delete monitor row: %w", err)
	}

	if err := results.Close(); err != nil {
		return fmt.Errorf("finish deletion batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit monitor deletion transaction: %w", err)
	}
	return nil
}
