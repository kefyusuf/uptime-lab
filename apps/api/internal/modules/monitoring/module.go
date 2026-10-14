package monitoring

import (
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// Module groups the Monitoring application use cases without owning infrastructure.
type Module struct {
	RegisterMonitor        application.RegisterMonitor
	GetMonitor             application.GetMonitor
	GetLatestCheckResult   application.GetLatestCheckResult
	GetMonitorAvailability application.GetMonitorAvailability
	ListMonitors           application.ListMonitors
	GetMonitorScheduling   application.GetMonitorScheduling
	SetMonitorScheduling   application.SetMonitorScheduling
}

// NewModule composes Monitoring use cases from their inward-facing dependencies.
func NewModule(
	repository ports.MonitorRepository,
	latestResultRepository ports.LatestCheckResultRepository,
	idGenerator application.IDGenerator,
	clock application.Clock,
) Module {
	return Module{
		RegisterMonitor:        application.NewRegisterMonitor(repository, idGenerator, clock),
		GetMonitor:             application.NewGetMonitor(repository),
		GetLatestCheckResult:   application.NewGetLatestCheckResult(latestResultRepository),
		GetMonitorAvailability: application.NewGetMonitorAvailability(latestResultRepository, clock),
	}
}

// NewModuleWithInventory composes the bounded read without changing legacy construction.
func NewModuleWithInventory(repository ports.MonitorRepository, latest ports.LatestCheckResultRepository, inventory ports.MonitorInventoryRepository, idGenerator application.IDGenerator, clock application.Clock) Module {
	module := NewModule(repository, latest, idGenerator, clock)
	module.ListMonitors = application.NewListMonitors(inventory)
	return module
}

// NewModuleWithScheduling adds durable scheduling to the public use cases.
func NewModuleWithScheduling(repository ports.MonitorRepository, latest ports.LatestCheckResultRepository, inventory ports.MonitorInventoryRepository, idGenerator application.IDGenerator, clock application.Clock, scheduling ports.MonitorSchedulingRepository) Module {
	module := NewModuleWithInventory(repository, latest, inventory, idGenerator, clock)
	module.GetMonitorScheduling = application.NewGetMonitorScheduling(scheduling)
	module.SetMonitorScheduling = application.NewSetMonitorScheduling(scheduling)
	return module
}

// ExecutionModule groups the internal Checker execution use cases without owning infrastructure.
type ExecutionModule struct {
	ClaimDueCheck     application.ClaimDueCheck
	SubmitCheckResult application.SubmitCheckResult
}

// NewExecutionModule composes execution use cases from their inward-facing dependencies.
func NewExecutionModule(
	repository ports.CheckExecutionRepository,
	checkIDGenerator application.CheckIDGenerator,
	clock application.Clock,
) ExecutionModule {
	return ExecutionModule{
		ClaimDueCheck:     application.NewClaimDueCheck(repository, checkIDGenerator, clock),
		SubmitCheckResult: application.NewSubmitCheckResult(repository, clock),
	}
}
