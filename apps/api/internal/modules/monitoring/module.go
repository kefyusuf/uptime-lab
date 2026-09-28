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
