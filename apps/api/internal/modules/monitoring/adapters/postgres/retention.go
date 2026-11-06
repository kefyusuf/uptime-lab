package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// historyRetentionCap keeps the newest historyRetentionCap terminal rows per
// Monitor and is a repository-owned local convention, not a configurable
// retention policy.
const historyRetentionCap = 50

// retainHistoryWindow removes terminal CheckRuns ranking above the
// historyRetentionCap within each Monitor's partition, keeping the newest
// rows ordered by terminal completion then CheckRun identity. Pending rows
// are never ranked and never removed — the pending immunity is structural,
// enforced by the completed_at filters. It must run inside the caller's
// claim transaction, after the expired-pending reconciliation and before the
// candidate selection, so the retained window is enforced by the single
// Go-owned deletion path.
func (repository *Repository) retainHistoryWindow(ctx context.Context, tx pgx.Tx) error {
	const retainHistoryWindow = `
		DELETE FROM monitoring.check_runs
		WHERE completed_at IS NOT NULL
		  AND id IN (
		    SELECT id FROM (
		      SELECT id,
		             row_number() OVER (PARTITION BY monitor_id
		                                ORDER BY completed_at DESC, id DESC) AS rank
		      FROM monitoring.check_runs
		      WHERE completed_at IS NOT NULL
		    ) ranked
		    WHERE rank > $1
		  )
	`
	batch := &pgx.Batch{}
	batch.Queue(retainHistoryWindow, historyRetentionCap)
	results := tx.SendBatch(ctx, batch)
	if _, err := results.Exec(); err != nil {
		_ = results.Close()
		return fmt.Errorf("retain history window: %w", err)
	}
	return results.Close()
}
