package ports

import (
	"context"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// HistoryAnchor is a persisted ordering position, not an identity lookup.
type HistoryAnchor struct {
	CompletedAt time.Time
	CheckID     domain.CheckID
}

// MonitorHistoryRepository defines the bounded terminal-history read capability.
type MonitorHistoryRepository interface {
	Page(context.Context, domain.MonitorID, *HistoryAnchor, int) ([]LatestCheckResultRecord, error)
}
