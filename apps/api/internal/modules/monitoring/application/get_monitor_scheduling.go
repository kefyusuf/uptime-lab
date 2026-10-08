package application

import (
	"context"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type GetMonitorScheduling struct {
	repository ports.MonitorSchedulingRepository
}

func NewGetMonitorScheduling(repository ports.MonitorSchedulingRepository) GetMonitorScheduling {
	return GetMonitorScheduling{repository: repository}
}

func (useCase GetMonitorScheduling) Execute(ctx context.Context, id domain.MonitorID) (domain.SchedulingState, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := domain.NewMonitorID(id.UUID()); err != nil {
		return "", err
	}
	state, err := useCase.repository.GetScheduling(ctx, id)
	if err != nil {
		return "", schedulingPersistenceError(err)
	}
	if !state.Valid() {
		return "", ErrPersistence
	}
	return state, nil
}

func schedulingPersistenceError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, ports.ErrMonitorNotFound):
		return ErrMonitorNotFound
	default:
		return ErrPersistence
	}
}
