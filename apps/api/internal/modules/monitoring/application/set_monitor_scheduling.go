package application

import (
	"context"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type SetMonitorScheduling struct {
	repository ports.MonitorSchedulingRepository
}

func NewSetMonitorScheduling(repository ports.MonitorSchedulingRepository) SetMonitorScheduling {
	return SetMonitorScheduling{repository: repository}
}

func (useCase SetMonitorScheduling) Execute(ctx context.Context, id domain.MonitorID, state domain.SchedulingState) (domain.SchedulingState, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := domain.NewMonitorID(id.UUID()); err != nil {
		return "", err
	}
	if !state.Valid() {
		return "", domain.ErrInvalidSchedulingState
	}
	persisted, err := useCase.repository.SetScheduling(ctx, id, state)
	if err != nil {
		return "", schedulingPersistenceError(err)
	}
	if !persisted.Valid() || persisted != state {
		return "", ErrPersistence
	}
	return persisted, nil
}
