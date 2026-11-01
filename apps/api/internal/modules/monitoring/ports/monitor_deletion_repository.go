package ports

import (
	"context"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// MonitorDeletionRepository defines the durable removal capability for Monitors.
type MonitorDeletionRepository interface {
	Delete(context.Context, domain.MonitorID) error
}
