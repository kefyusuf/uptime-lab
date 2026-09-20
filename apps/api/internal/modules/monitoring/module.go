package monitoring

import (
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// Module groups the Monitoring application use cases without owning infrastructure.
type Module struct {
	RegisterMonitor application.RegisterMonitor
	GetMonitor      application.GetMonitor
}

// NewModule composes Monitoring use cases from their inward-facing dependencies.
func NewModule(
	repository ports.MonitorRepository,
	idGenerator application.IDGenerator,
	clock application.Clock,
) Module {
	return Module{
		RegisterMonitor: application.NewRegisterMonitor(repository, idGenerator, clock),
		GetMonitor:      application.NewGetMonitor(repository),
	}
}
