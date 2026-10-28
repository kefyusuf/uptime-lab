package application

import (
	"context"
	"errors"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// DeleteMonitor permanently removes one monitor together with all of its check evidence.
type DeleteMonitor struct {
	repository ports.MonitorDeletionRepository
}

// NewDeleteMonitor constructs the monitor deletion use case.
func NewDeleteMonitor(repository ports.MonitorDeletionRepository) DeleteMonitor {
	return DeleteMonitor{repository: repository}
}

// Execute deletes a monitor and maps repository failures into application errors.
func (useCase DeleteMonitor) Execute(ctx context.Context, id domain.MonitorID) error {
	if _, err := domain.NewMonitorID(id.UUID()); err != nil {
		return err
	}

	err := useCase.repository.Delete(ctx, id)
	if err == nil {
		return nil
	}
	if errors.Is(err, ports.ErrMonitorNotFound) {
		return ErrMonitorNotFound
	}
	return ErrPersistence
}
