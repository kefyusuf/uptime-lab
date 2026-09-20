package ports

import (
	"context"
	"errors"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// ErrMonitorNotFound is the infrastructure-neutral repository not-found signal.
var ErrMonitorNotFound = errors.New("monitor not found")

// MonitorRepository defines the persistence capabilities needed by Monitoring.
type MonitorRepository interface {
	Create(context.Context, domain.Monitor) error
	ByID(context.Context, domain.MonitorID) (domain.Monitor, error)
}
