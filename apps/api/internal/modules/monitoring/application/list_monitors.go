package application

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const inventoryTargetLimit = 245760

// InventoryInput supplies a validated page size and optional ordering position.
type InventoryInput struct {
	Limit  int
	Cursor string
}

// InventoryCandidates includes one lookahead candidate for HTTP byte budgeting.
type InventoryCandidates struct {
	Items []ports.InventoryCandidate
	Limit int
}

// ListMonitors retrieves and validates a bounded inventory candidate window.
type ListMonitors struct {
	repository ports.MonitorInventoryRepository
}

// NewListMonitors constructs the bounded inventory read use case.
func NewListMonitors(repository ports.MonitorInventoryRepository) ListMonitors {
	return ListMonitors{repository: repository}
}

// Execute validates input before storage and sanitizes invalid storage results.
func (useCase ListMonitors) Execute(ctx context.Context, input InventoryInput) (InventoryCandidates, error) {
	if input.Limit < 1 || input.Limit > 50 {
		return InventoryCandidates{}, ErrInvalidInventoryQuery
	}
	var anchor *ports.InventoryAnchor
	if input.Cursor != "" {
		decoded, err := DecodeInventoryCursor(input.Cursor)
		if err != nil {
			return InventoryCandidates{}, ErrInvalidInventoryQuery
		}
		anchor = &decoded
	}
	if err := ctx.Err(); err != nil {
		return InventoryCandidates{}, err
	}
	items, err := useCase.repository.Candidates(ctx, anchor, input.Limit+1, inventoryTargetLimit)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return InventoryCandidates{}, context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return InventoryCandidates{}, context.DeadlineExceeded
		}
		return InventoryCandidates{}, ErrPersistence
	}
	if err := ctx.Err(); err != nil {
		return InventoryCandidates{}, err
	}
	if len(items) > input.Limit+1 {
		return InventoryCandidates{}, ErrPersistence
	}
	previous := anchor
	for _, item := range items {
		key := ports.InventoryAnchor{ID: item.ID, CreatedAt: item.CreatedAt}
		if _, err := EncodeInventoryCursor(key); err != nil {
			return InventoryCandidates{}, ErrPersistence
		}
		if previous != nil && !(key.CreatedAt.Before(previous.CreatedAt) || (key.CreatedAt.Equal(previous.CreatedAt) && key.ID.String() < previous.ID.String())) {
			return InventoryCandidates{}, ErrPersistence
		}
		if item.Oversized {
			if item.TargetBytes <= inventoryTargetLimit || item.TargetURL != "" {
				return InventoryCandidates{}, ErrPersistence
			}
		} else {
			if item.TargetBytes < 1 || item.TargetBytes > inventoryTargetLimit || int64(len(item.TargetURL)) != item.TargetBytes || !utf8.ValidString(item.TargetURL) {
				return InventoryCandidates{}, ErrPersistence
			}
			if _, err := domain.NewTargetURL(item.TargetURL); err != nil {
				return InventoryCandidates{}, ErrPersistence
			}
		}
		previous = &key
	}
	return InventoryCandidates{Items: items, Limit: input.Limit}, nil
}
