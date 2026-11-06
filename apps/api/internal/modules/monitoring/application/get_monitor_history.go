package application

import (
	"context"
	"errors"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// HistoryInput supplies a validated page size and optional ordering position.
type HistoryInput struct {
	Limit  int
	Cursor string
}

// HistoryCandidates includes one lookahead entry for HTTP byte budgeting.
type HistoryCandidates struct {
	Items []LatestCheckResult
	Limit int
}

// GetMonitorHistory retrieves and validates a bounded terminal-history window.
type GetMonitorHistory struct {
	repository ports.MonitorHistoryRepository
}

// NewGetMonitorHistory constructs the bounded history read use case.
func NewGetMonitorHistory(repository ports.MonitorHistoryRepository) GetMonitorHistory {
	return GetMonitorHistory{repository: repository}
}

// Execute validates input before storage and maps durable entries through the
// existing latest-result shape, sanitizing invalid storage results.
func (useCase GetMonitorHistory) Execute(
	ctx context.Context,
	monitorID domain.MonitorID,
	input HistoryInput,
) (HistoryCandidates, error) {
	if _, err := domain.NewMonitorID(monitorID.UUID()); err != nil {
		return HistoryCandidates{}, err
	}
	if input.Limit < 1 || input.Limit > 50 {
		return HistoryCandidates{}, ErrInvalidHistoryQuery
	}
	var anchor *ports.HistoryAnchor
	if input.Cursor != "" {
		decoded, err := DecodeHistoryCursor(input.Cursor)
		if err != nil {
			return HistoryCandidates{}, ErrInvalidHistoryQuery
		}
		anchor = &decoded
	}
	if err := ctx.Err(); err != nil {
		return HistoryCandidates{}, err
	}

	entries, err := useCase.repository.Page(ctx, monitorID, anchor, input.Limit+1)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return HistoryCandidates{}, context.Canceled
		case errors.Is(err, context.DeadlineExceeded):
			return HistoryCandidates{}, context.DeadlineExceeded
		case errors.Is(err, ports.ErrMonitorNotFound):
			return HistoryCandidates{}, ErrMonitorNotFound
		default:
			return HistoryCandidates{}, ErrPersistence
		}
	}

	results := make([]LatestCheckResult, 0, len(entries))
	for _, entry := range entries {
		result, err := latestCheckResultFromRecord(entry)
		if err != nil {
			return HistoryCandidates{}, ErrPersistence
		}
		results = append(results, result)
	}
	return HistoryCandidates{Items: results, Limit: input.Limit}, nil
}
