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

var _ ports.MonitorIncidentRepository = (*Repository)(nil)

// PageIncidents reads one bounded durable incident-history window inside one Read
// Committed transaction: the Monitor existence probe and the keyset page query
// pipeline through one batch round trip. Under READ COMMITTED each statement observes
// its own snapshot, and because the probe executes first the page statement's snapshot
// is never older: a concurrent deletion resolves as a consistent pre-delete page or a
// 404, never rows for a vanished Monitor. Identifiers bind as binary uuid parameters,
// matching the history adapter's discipline.
func (repository *Repository) PageIncidents(ctx context.Context, monitorID domain.MonitorID, anchor *ports.IncidentAnchor, maxRows int) ([]ports.IncidentRecord, error) {
	if maxRows < 1 || maxRows > 51 {
		return nil, errors.New("invalid incident repository bounds")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bind := pgtype.UUID{Bytes: monitorID.UUID(), Valid: true}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin incidents transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	const existsProbe = "SELECT EXISTS (SELECT 1 FROM monitoring.monitors WHERE id = $1)"
	const firstPage = "SELECT id::text, opened_at, resolved_at, opening_reason, opened_by_check_id::text, resolved_by_check_id::text FROM monitoring.incidents WHERE monitor_id = $1 ORDER BY opened_at DESC, id DESC LIMIT $2"
	const anchoredPage = "SELECT id::text, opened_at, resolved_at, opening_reason, opened_by_check_id::text, resolved_by_check_id::text FROM monitoring.incidents WHERE monitor_id = $1 AND (opened_at, id) < ($2, $3) ORDER BY opened_at DESC, id DESC LIMIT $4"

	batch := &pgx.Batch{}
	batch.Queue(existsProbe, bind)
	if anchor != nil {
		batch.Queue(anchoredPage, bind, anchor.OpenedAt, pgtype.UUID{Bytes: anchor.IncidentID.UUID(), Valid: true}, maxRows)
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
		return nil, fmt.Errorf("query monitor incidents: %w", err)
	}
	defer rows.Close()

	entries := make([]ports.IncidentRecord, 0, maxRows)
	for rows.Next() {
		var rawIncidentID string
		var openedAt time.Time
		var resolvedAt *time.Time
		var reason string
		var rawOpenedBy, rawResolvedBy *string
		if err := rows.Scan(&rawIncidentID, &openedAt, &resolvedAt, &reason, &rawOpenedBy, &rawResolvedBy); err != nil {
			return nil, fmt.Errorf("scan incident row: %w", err)
		}
		incidentID, err := domain.ParseIncidentID(rawIncidentID)
		if err != nil {
			return nil, fmt.Errorf("map incident id: %w", err)
		}
		record := ports.IncidentRecord{
			IncidentID:    incidentID,
			OpenedAt:      openedAt.UTC(),
			OpeningReason: domain.IncidentReason(reason),
		}
		if resolvedAt != nil {
			utc := resolvedAt.UTC()
			record.ResolvedAt = &utc
		}
		if rawOpenedBy != nil {
			checkID, err := domain.ParseCheckID(*rawOpenedBy)
			if err != nil {
				return nil, fmt.Errorf("map incident opening evidence: %w", err)
			}
			record.OpenedByCheckID = &checkID
		}
		if rawResolvedBy != nil {
			checkID, err := domain.ParseCheckID(*rawResolvedBy)
			if err != nil {
				return nil, fmt.Errorf("map incident resolving evidence: %w", err)
			}
			record.ResolvedByCheckID = &checkID
		}
		entries = append(entries, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident rows: %w", err)
	}
	if err := results.Close(); err != nil {
		return nil, fmt.Errorf("finish incidents batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit incidents transaction: %w", err)
	}
	return entries, nil
}
