package ports

import (
	"context"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// InventoryAnchor is a persisted ordering position, not an identity lookup.
type InventoryAnchor struct {
	CreatedAt time.Time
	ID        domain.MonitorID
}

// InventoryCandidate preserves ordering metadata when a target cannot be projected.
type InventoryCandidate struct {
	ID          domain.MonitorID
	CreatedAt   time.Time
	TargetURL   string
	TargetBytes int64
	Oversized   bool
}

// MonitorInventoryRepository provides a bounded, strictly descending candidate read.
type MonitorInventoryRepository interface {
	Candidates(context.Context, *InventoryAnchor, int, int) ([]InventoryCandidate, error)
}
