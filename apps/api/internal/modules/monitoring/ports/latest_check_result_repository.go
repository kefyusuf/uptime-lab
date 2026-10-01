package ports

import (
	"context"
	"errors"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// ErrNoTerminalCheckResult signals that the Monitor exists but has no terminal CheckRun.
var ErrNoTerminalCheckResult = errors.New("no terminal check result")

// LatestCheckResultRecord is the persistence-neutral shape of one terminal CheckRun read.
type LatestCheckResultRecord struct {
	CheckID     domain.CheckID
	ResultKind  domain.CheckResultKind
	HTTPStatus  *int
	DurationMS  *int64
	CompletedAt time.Time
}

// LatestCheckResultRepository defines only the persistence capability required by the public latest-result read.
type LatestCheckResultRepository interface {
	LatestTerminalByMonitorID(context.Context, domain.MonitorID) (LatestCheckResultRecord, error)
}
