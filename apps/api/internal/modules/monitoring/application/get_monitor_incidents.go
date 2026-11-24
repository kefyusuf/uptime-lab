package application

import (
	"context"
	"errors"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var errInvalidIncidentRecord = errors.New("invalid incident record")

// IncidentInput supplies a validated page size and optional ordering position.
type IncidentInput struct {
	Limit  int
	Cursor string
}

// IncidentCandidates includes one lookahead entry for HTTP byte budgeting.
type IncidentCandidates struct {
	Items []Incident
	Limit int
}

// Incident is the application-owned public-safe view of one durable incident window.
type Incident struct {
	incidentID        domain.IncidentID
	openedAt          time.Time
	resolvedAt        time.Time
	hasResolvedAt     bool
	openingReason     domain.IncidentReason
	openedByCheckID   domain.CheckID
	hasOpenedBy       bool
	resolvedByCheckID domain.CheckID
	hasResolvedBy     bool
}

// IncidentID returns the durable incident window identity.
func (incident Incident) IncidentID() domain.IncidentID {
	return incident.incidentID
}

// OpenedAt returns the opening instant normalized to UTC.
func (incident Incident) OpenedAt() time.Time {
	return incident.openedAt
}

// ResolvedAt returns the resolving instant only for a closed window.
func (incident Incident) ResolvedAt() (time.Time, bool) {
	if !incident.hasResolvedAt {
		return time.Time{}, false
	}
	return incident.resolvedAt, true
}

// OpeningReason returns the closed vocabulary reason the window opened for.
func (incident Incident) OpeningReason() domain.IncidentReason {
	return incident.openingReason
}

// OpenedByCheckID returns the confirming CheckRun evidence when it is still retained.
func (incident Incident) OpenedByCheckID() (domain.CheckID, bool) {
	if !incident.hasOpenedBy {
		return domain.CheckID{}, false
	}
	return incident.openedByCheckID, true
}

// ResolvedByCheckID returns the recovering CheckRun evidence when it is still retained.
func (incident Incident) ResolvedByCheckID() (domain.CheckID, bool) {
	if !incident.hasResolvedBy {
		return domain.CheckID{}, false
	}
	return incident.resolvedByCheckID, true
}

// GetMonitorIncidents retrieves and validates a bounded durable incident-history window.
type GetMonitorIncidents struct {
	repository ports.MonitorIncidentRepository
}

// NewGetMonitorIncidents constructs the bounded incident read use case.
func NewGetMonitorIncidents(repository ports.MonitorIncidentRepository) GetMonitorIncidents {
	return GetMonitorIncidents{repository: repository}
}

// Execute validates input before storage and maps durable windows through the
// closed incident shape, sanitizing invalid storage rows.
func (useCase GetMonitorIncidents) Execute(
	ctx context.Context,
	monitorID domain.MonitorID,
	input IncidentInput,
) (IncidentCandidates, error) {
	if _, err := domain.NewMonitorID(monitorID.UUID()); err != nil {
		return IncidentCandidates{}, err
	}
	if input.Limit < 1 || input.Limit > 50 {
		return IncidentCandidates{}, ErrInvalidIncidentQuery
	}
	var anchor *ports.IncidentAnchor
	if input.Cursor != "" {
		decoded, err := DecodeIncidentCursor(input.Cursor)
		if err != nil {
			return IncidentCandidates{}, ErrInvalidIncidentQuery
		}
		anchor = &decoded
	}
	if err := ctx.Err(); err != nil {
		return IncidentCandidates{}, err
	}

	entries, err := useCase.repository.PageIncidents(ctx, monitorID, anchor, input.Limit+1)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return IncidentCandidates{}, context.Canceled
		case errors.Is(err, context.DeadlineExceeded):
			return IncidentCandidates{}, context.DeadlineExceeded
		case errors.Is(err, ports.ErrMonitorNotFound):
			return IncidentCandidates{}, ErrMonitorNotFound
		default:
			return IncidentCandidates{}, ErrPersistence
		}
	}

	items := make([]Incident, 0, len(entries))
	for _, entry := range entries {
		incident, err := incidentFromRecord(entry)
		if err != nil {
			return IncidentCandidates{}, ErrPersistence
		}
		items = append(items, incident)
	}
	return IncidentCandidates{Items: items, Limit: input.Limit}, nil
}

func incidentFromRecord(record ports.IncidentRecord) (Incident, error) {
	if _, err := domain.NewIncidentID(record.IncidentID.UUID()); err != nil {
		return Incident{}, err
	}
	if record.OpenedAt.IsZero() {
		return Incident{}, errInvalidIncidentRecord
	}
	switch record.OpeningReason {
	case domain.IncidentUnexpectedHTTPStatus, domain.IncidentProbeFailure:
	default:
		return Incident{}, errInvalidIncidentRecord
	}

	incident := Incident{
		incidentID:    record.IncidentID,
		openedAt:      record.OpenedAt.UTC(),
		openingReason: record.OpeningReason,
	}
	if record.ResolvedAt != nil {
		resolvedAt := record.ResolvedAt.UTC()
		if resolvedAt.IsZero() || resolvedAt.Before(incident.openedAt) {
			return Incident{}, errInvalidIncidentRecord
		}
		incident.resolvedAt = resolvedAt
		incident.hasResolvedAt = true
	}
	if record.OpenedByCheckID != nil {
		if _, err := domain.NewCheckID(record.OpenedByCheckID.UUID()); err != nil {
			return Incident{}, err
		}
		incident.openedByCheckID = *record.OpenedByCheckID
		incident.hasOpenedBy = true
	}
	if record.ResolvedByCheckID != nil {
		if _, err := domain.NewCheckID(record.ResolvedByCheckID.UUID()); err != nil {
			return Incident{}, err
		}
		incident.resolvedByCheckID = *record.ResolvedByCheckID
		incident.hasResolvedBy = true
	}
	return incident, nil
}
