package application

import (
	"context"
	"errors"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// LatestCheckResult is the application-owned public-safe view of one terminal CheckRun.
type LatestCheckResult struct {
	checkID       domain.CheckID
	resultKind    domain.CheckResultKind
	httpStatus    int
	hasHTTPStatus bool
	durationMS    int64
	hasDurationMS bool
	completedAt   time.Time
}

// CheckID returns the durable CheckRun identity.
func (result LatestCheckResult) CheckID() domain.CheckID {
	return result.checkID
}

// ResultKind returns the normalized terminal execution outcome.
func (result LatestCheckResult) ResultKind() domain.CheckResultKind {
	return result.resultKind
}

// HTTPStatus returns the final HTTP status only for http_response results.
func (result LatestCheckResult) HTTPStatus() (int, bool) {
	if !result.hasHTTPStatus {
		return 0, false
	}
	return result.httpStatus, true
}

// DurationMS returns probe duration when the terminal result has one.
func (result LatestCheckResult) DurationMS() (int64, bool) {
	if !result.hasDurationMS {
		return 0, false
	}
	return result.durationMS, true
}

// CompletedAt returns the terminal completion instant normalized to UTC.
func (result LatestCheckResult) CompletedAt() time.Time {
	return result.completedAt
}

// GetLatestCheckResult retrieves one Monitor's latest terminal execution fact.
type GetLatestCheckResult struct {
	repository ports.LatestCheckResultRepository
}

// NewGetLatestCheckResult constructs the latest-terminal-result read use case.
func NewGetLatestCheckResult(repository ports.LatestCheckResultRepository) GetLatestCheckResult {
	return GetLatestCheckResult{repository: repository}
}

// Execute retrieves and validates one public-safe latest terminal result.
func (useCase GetLatestCheckResult) Execute(
	ctx context.Context,
	monitorID domain.MonitorID,
) (LatestCheckResult, error) {
	if _, err := domain.NewMonitorID(monitorID.UUID()); err != nil {
		return LatestCheckResult{}, err
	}

	record, err := useCase.repository.LatestTerminalByMonitorID(ctx, monitorID)
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrMonitorNotFound):
			return LatestCheckResult{}, ErrMonitorNotFound
		case errors.Is(err, ports.ErrNoTerminalCheckResult):
			return LatestCheckResult{}, ErrNoTerminalCheckResult
		default:
			return LatestCheckResult{}, ErrPersistence
		}
	}

	result, err := latestCheckResultFromRecord(record)
	if err != nil {
		return LatestCheckResult{}, ErrPersistence
	}
	return result, nil
}

func latestCheckResultFromRecord(record ports.LatestCheckResultRecord) (LatestCheckResult, error) {
	if _, err := domain.NewCheckID(record.CheckID.UUID()); err != nil {
		return LatestCheckResult{}, domain.ErrInvalidCheckResult
	}
	if record.CompletedAt.IsZero() {
		return LatestCheckResult{}, domain.ErrInvalidCheckResult
	}

	switch record.ResultKind {
	case domain.CheckResultHTTPResponse:
		if record.HTTPStatus == nil || record.DurationMS == nil {
			return LatestCheckResult{}, domain.ErrInvalidCheckResult
		}
		if _, err := domain.NewHTTPResponseResult(*record.DurationMS, *record.HTTPStatus); err != nil {
			return LatestCheckResult{}, err
		}
		return LatestCheckResult{
			checkID:       record.CheckID,
			resultKind:    record.ResultKind,
			httpStatus:    *record.HTTPStatus,
			hasHTTPStatus: true,
			durationMS:    *record.DurationMS,
			hasDurationMS: true,
			completedAt:   record.CompletedAt.UTC(),
		}, nil

	case domain.CheckResultWorkerTimeout:
		if record.HTTPStatus != nil || record.DurationMS != nil {
			return LatestCheckResult{}, domain.ErrInvalidCheckResult
		}
		return LatestCheckResult{
			checkID:     record.CheckID,
			resultKind:  record.ResultKind,
			completedAt: record.CompletedAt.UTC(),
		}, nil

	default:
		if record.HTTPStatus != nil || record.DurationMS == nil {
			return LatestCheckResult{}, domain.ErrInvalidCheckResult
		}
		if _, err := domain.NewCheckFailureResult(record.ResultKind, *record.DurationMS); err != nil {
			return LatestCheckResult{}, err
		}
		return LatestCheckResult{
			checkID:       record.CheckID,
			resultKind:    record.ResultKind,
			durationMS:    *record.DurationMS,
			hasDurationMS: true,
			completedAt:   record.CompletedAt.UTC(),
		}, nil
	}
}
