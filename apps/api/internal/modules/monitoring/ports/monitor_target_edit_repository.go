package ports

import (
	"context"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// MonitorTargetEditRepository defines the durable target-edit capability for
// one Monitor's existing identity.
type MonitorTargetEditRepository interface {
	EditTarget(context.Context, domain.MonitorID, domain.TargetURL) (domain.Monitor, error)
}
