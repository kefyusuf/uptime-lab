package ports

import (
	"context"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// IncidentAnchor is a persisted ordering position, not an identity lookup.
type IncidentAnchor struct {
	OpenedAt   time.Time
	IncidentID domain.IncidentID
}

// IncidentRecord is the persistence-neutral shape of one durable incident window read.
type IncidentRecord struct {
	IncidentID        domain.IncidentID
	OpenedAt          time.Time
	ResolvedAt        *time.Time
	OpeningReason     domain.IncidentReason
	OpenedByCheckID   *domain.CheckID
	ResolvedByCheckID *domain.CheckID
}

// MonitorIncidentRepository defines the bounded durable incident-history read capability.
// The method is named PageIncidents rather than Page because the same pool-only Repository
// already implements the terminal-history Page read; Go has no method overloading.
type MonitorIncidentRepository interface {
	PageIncidents(context.Context, domain.MonitorID, *IncidentAnchor, int) ([]IncidentRecord, error)
}
