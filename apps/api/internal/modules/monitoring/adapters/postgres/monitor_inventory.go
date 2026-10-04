package postgres

import (
	"context"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// Candidates is a temporary TDD stub for real PostgreSQL RED evidence.
func (repository *Repository) Candidates(context.Context, *ports.InventoryAnchor, int, int) ([]ports.InventoryCandidate, error) {
	return []ports.InventoryCandidate{}, nil
}
