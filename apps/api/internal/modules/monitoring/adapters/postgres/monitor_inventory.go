package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorInventoryRepository = (*Repository)(nil)

// Candidates reads strict descending keys and projects oversized targets as markers.
func (repository *Repository) Candidates(ctx context.Context, anchor *ports.InventoryAnchor, maxRows, maxTargetBytes int) ([]ports.InventoryCandidate, error) {
	if maxRows < 1 || maxRows > 51 || maxTargetBytes != 245760 {
		return nil, errors.New("invalid inventory repository bounds")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := `SELECT id::text, created_at, octet_length(target_url),
 CASE WHEN octet_length(target_url)<=$1 THEN target_url ELSE NULL END
 FROM monitoring.monitors ORDER BY created_at DESC,id DESC LIMIT $2`
	args := []any{maxTargetBytes, maxRows}
	if anchor != nil {
		query = `SELECT id::text, created_at, octet_length(target_url),
  CASE WHEN octet_length(target_url)<=$1 THEN target_url ELSE NULL END
  FROM monitoring.monitors WHERE (created_at,id)<($3::timestamptz,$4::uuid)
  ORDER BY created_at DESC,id DESC LIMIT $2`
		args = append(args, anchor.CreatedAt, anchor.ID.String())
	}
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ports.InventoryCandidate, 0, maxRows)
	for rows.Next() {
		var rawID string
		var at pgtype.Timestamptz
		var target *string
		var bytes int64
		if err := rows.Scan(&rawID, &at, &bytes, &target); err != nil {
			return nil, err
		}
		id, err := domain.ParseMonitorID(rawID)
		if err != nil || id.String() != rawID || !at.Valid || at.InfinityModifier != pgtype.Finite || at.Time.IsZero() || at.Time.Year() < 1 || at.Time.Year() > 9999 || at.Time.Nanosecond()%int(time.Microsecond) != 0 {
			return nil, errors.New("invalid inventory ordering key")
		}
		item := ports.InventoryCandidate{ID: id, CreatedAt: at.Time.UTC(), TargetBytes: bytes, Oversized: target == nil}
		if target != nil {
			item.TargetURL = *target
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
