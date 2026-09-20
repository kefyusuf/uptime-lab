package application

import (
	"context"
	"errors"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// GetMonitor retrieves one monitor by identity.
type GetMonitor struct {
	repository ports.MonitorRepository
}

// NewGetMonitor constructs the monitor lookup use case.
func NewGetMonitor(repository ports.MonitorRepository) GetMonitor {
	return GetMonitor{repository: repository}
}

// Execute retrieves a monitor and maps repository failures into application errors.
func (useCase GetMonitor) Execute(ctx context.Context, id domain.MonitorID) (domain.Monitor, error) {
	if _, err := domain.NewMonitorID(id.UUID()); err != nil {
		return domain.Monitor{}, err
	}

	monitor, err := useCase.repository.ByID(ctx, id)
	if err == nil {
		return monitor, nil
	}
	if errors.Is(err, ports.ErrMonitorNotFound) {
		return domain.Monitor{}, ErrMonitorNotFound
	}
	return domain.Monitor{}, ErrPersistence
}
