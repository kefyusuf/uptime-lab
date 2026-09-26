package ports

import (
	"context"
	"errors"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

var (
	// ErrNoDueCheck signals that no Monitor is currently eligible for a new CheckRun.
	ErrNoDueCheck = errors.New("no due check")

	// ErrCheckRunNotFound signals that the supplied CheckID does not identify a durable run.
	ErrCheckRunNotFound = errors.New("check run not found")

	// ErrCheckRunConflict signals that a result cannot terminalize the identified run.
	ErrCheckRunConflict = errors.New("check run conflict")
)

// ClaimDueCheckInput carries Go-owned scheduling facts into the persistence adapter.
type ClaimDueCheckInput struct {
	CheckID   domain.CheckID
	Now       time.Time
	DueBefore time.Time
	Deadline  time.Time
}

// ClaimedCheck is the execution-relevant Monitor data returned by an atomic claim.
type ClaimedCheck struct {
	MonitorID domain.MonitorID
	TargetURL domain.TargetURL
}

// CompleteCheckInput carries one validated result and the Go-owned completion instant.
type CompleteCheckInput struct {
	CheckID     domain.CheckID
	CompletedAt time.Time
	Result      domain.CheckResult
}

// CheckExecutionRepository defines only the persistence capabilities required by execution coordination.
type CheckExecutionRepository interface {
	ClaimDueCheck(context.Context, ClaimDueCheckInput) (ClaimedCheck, error)
	CompleteCheck(context.Context, CompleteCheckInput) error
}
