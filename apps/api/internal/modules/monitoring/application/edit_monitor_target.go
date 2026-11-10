package application

import (
	"context"
	"errors"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// EditMonitorTarget replaces one Monitor's target URL on its existing
// immutable identity through the same syntactic validation registration uses.
type EditMonitorTarget struct {
	repository ports.MonitorTargetEditRepository
}

// NewEditMonitorTarget constructs the target edit use case.
func NewEditMonitorTarget(repository ports.MonitorTargetEditRepository) EditMonitorTarget {
	return EditMonitorTarget{repository: repository}
}

// Execute validates the inputs, delegates the durable edit and maps
// repository failures into application errors.
func (useCase EditMonitorTarget) Execute(
	ctx context.Context,
	monitorID domain.MonitorID,
	rawTargetURL string,
) (domain.Monitor, error) {
	if _, err := domain.NewMonitorID(monitorID.UUID()); err != nil {
		return domain.Monitor{}, err
	}

	newTarget, err := domain.NewTargetURL(rawTargetURL)
	if err != nil {
		return domain.Monitor{}, err
	}

	monitor, err := useCase.repository.EditTarget(ctx, monitorID, newTarget)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return domain.Monitor{}, context.Canceled
		case errors.Is(err, context.DeadlineExceeded):
			return domain.Monitor{}, context.DeadlineExceeded
		case errors.Is(err, ports.ErrMonitorNotFound):
			return domain.Monitor{}, ErrMonitorNotFound
		default:
			return domain.Monitor{}, ErrPersistence
		}
	}
	return monitor, nil
}
