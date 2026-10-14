package ports

import (
	"context"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// MonitorSchedulingRepository reads and atomically sets desired claim eligibility.
type MonitorSchedulingRepository interface {
	GetScheduling(context.Context, domain.MonitorID) (domain.SchedulingState, error)
	SetScheduling(context.Context, domain.MonitorID, domain.SchedulingState) (domain.SchedulingState, error)
}
