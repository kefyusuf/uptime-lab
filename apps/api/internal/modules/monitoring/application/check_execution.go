package application

import (
	"context"
	"errors"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const (
	checkCadence      = 60 * time.Second
	checkTimeout      = 10 * time.Second
	checkWindow       = 20 * time.Second
	checkMaxRedirects = 3
)

// CheckIDGenerator provides durable check identities to the application layer.
type CheckIDGenerator func() (domain.CheckID, error)

// CheckWork is the exact execution description exposed to the Checker adapter.
type CheckWork struct {
	CheckID      domain.CheckID
	MonitorID    domain.MonitorID
	TargetURL    domain.TargetURL
	Timeout      time.Duration
	MaxRedirects int
}

// ClaimDueCheck coordinates one atomic due-work claim.
type ClaimDueCheck struct {
	repository  ports.CheckExecutionRepository
	idGenerator CheckIDGenerator
	clock       Clock
}

// NewClaimDueCheck constructs the due-work claim use case.
func NewClaimDueCheck(
	repository ports.CheckExecutionRepository,
	idGenerator CheckIDGenerator,
	clock Clock,
) ClaimDueCheck {
	return ClaimDueCheck{
		repository:  repository,
		idGenerator: idGenerator,
		clock:       clock,
	}
}

// Execute claims at most one due Monitor and returns its execution work.
func (useCase ClaimDueCheck) Execute(ctx context.Context) (CheckWork, error) {
	checkID, err := useCase.idGenerator()
	if err != nil {
		return CheckWork{}, err
	}
	if _, err := domain.NewCheckID(checkID.UUID()); err != nil {
		return CheckWork{}, err
	}

	now := useCase.clock()
	claimed, err := useCase.repository.ClaimDueCheck(ctx, ports.ClaimDueCheckInput{
		CheckID:   checkID,
		Now:       now,
		DueBefore: now.Add(-checkCadence),
		Deadline:  now.Add(checkWindow),
	})
	if err != nil {
		if errors.Is(err, ports.ErrNoDueCheck) {
			return CheckWork{}, ErrNoDueCheck
		}
		return CheckWork{}, ErrPersistence
	}

	return CheckWork{
		CheckID:      checkID,
		MonitorID:    claimed.MonitorID,
		TargetURL:    claimed.TargetURL,
		Timeout:      checkTimeout,
		MaxRedirects: checkMaxRedirects,
	}, nil
}

// SubmitCheckResultInput is the transport-neutral normalized result input.
type SubmitCheckResultInput struct {
	Kind       domain.CheckResultKind
	DurationMS int64
	HTTPStatus *int
}

// SubmitCheckResult coordinates one idempotent result completion.
type SubmitCheckResult struct {
	repository ports.CheckExecutionRepository
	clock      Clock
}

// NewSubmitCheckResult constructs the result-submission use case.
func NewSubmitCheckResult(repository ports.CheckExecutionRepository, clock Clock) SubmitCheckResult {
	return SubmitCheckResult{
		repository: repository,
		clock:      clock,
	}
}

// Execute validates the submitted result before asking persistence to terminalize the CheckRun.
func (useCase SubmitCheckResult) Execute(
	ctx context.Context,
	rawCheckID string,
	input SubmitCheckResultInput,
) error {
	checkID, err := domain.ParseCheckID(rawCheckID)
	if err != nil {
		return err
	}

	result, err := newSubmittedCheckResult(input)
	if err != nil {
		return err
	}

	completedAt := useCase.clock()
	err = useCase.repository.CompleteCheck(ctx, ports.CompleteCheckInput{
		CheckID:     checkID,
		CompletedAt: completedAt,
		Result:      result,
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, ports.ErrCheckRunNotFound) {
		return ErrCheckRunNotFound
	}
	if errors.Is(err, ports.ErrCheckRunConflict) {
		return ErrCheckRunConflict
	}
	return ErrPersistence
}

func newSubmittedCheckResult(input SubmitCheckResultInput) (domain.CheckResult, error) {
	if input.Kind == domain.CheckResultHTTPResponse {
		if input.HTTPStatus == nil {
			return domain.CheckResult{}, domain.ErrInvalidCheckResult
		}
		return domain.NewHTTPResponseResult(input.DurationMS, *input.HTTPStatus)
	}

	if input.HTTPStatus != nil {
		return domain.CheckResult{}, domain.ErrInvalidCheckResult
	}
	return domain.NewCheckFailureResult(input.Kind, input.DurationMS)
}
