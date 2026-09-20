package application

import (
	"context"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// IDGenerator provides monitor identities to the application layer.
type IDGenerator func() domain.MonitorID

// Clock provides the current time to the application layer.
type Clock func() time.Time

// RegisterMonitor orchestrates monitor registration.
type RegisterMonitor struct {
	repository  ports.MonitorRepository
	idGenerator IDGenerator
	clock       Clock
}

// NewRegisterMonitor constructs the registration use case.
func NewRegisterMonitor(
	repository ports.MonitorRepository,
	idGenerator IDGenerator,
	clock Clock,
) RegisterMonitor {
	return RegisterMonitor{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

// Execute validates, creates, and persists one monitor.
func (useCase RegisterMonitor) Execute(ctx context.Context, rawTarget string) (domain.Monitor, error) {
	target, err := domain.NewTargetURL(rawTarget)
	if err != nil {
		return domain.Monitor{}, err
	}

	id := useCase.idGenerator()
	createdAt := useCase.clock()

	monitor, err := domain.NewMonitor(id, target, createdAt)
	if err != nil {
		return domain.Monitor{}, err
	}

	if err := useCase.repository.Create(ctx, monitor); err != nil {
		return domain.Monitor{}, ErrPersistence
	}

	return monitor, nil
}
