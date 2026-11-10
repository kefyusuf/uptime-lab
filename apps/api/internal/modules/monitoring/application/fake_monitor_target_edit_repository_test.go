package application

import (
	"context"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// fakeMonitorTargetEditRepository records edit calls for the edit use-case tests.
type fakeMonitorTargetEditRepository struct {
	editCalls  int
	gotCtx     context.Context
	gotID      domain.MonitorID
	gotTarget  domain.TargetURL
	editResult domain.Monitor
	editErr    error
}

func (repository *fakeMonitorTargetEditRepository) EditTarget(ctx context.Context, monitorID domain.MonitorID, targetURL domain.TargetURL) (domain.Monitor, error) {
	repository.editCalls++
	repository.gotCtx = ctx
	repository.gotID = monitorID
	repository.gotTarget = targetURL
	return repository.editResult, repository.editErr
}
