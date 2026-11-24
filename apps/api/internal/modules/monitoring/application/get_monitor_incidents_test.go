package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type fakeMonitorIncidentRepository struct {
	pageCalls  int
	gotCtx     context.Context
	gotMonitor domain.MonitorID
	gotAnchor  *ports.IncidentAnchor
	gotMaxRows int
	entries    []ports.IncidentRecord
	err        error
}

func (repository *fakeMonitorIncidentRepository) PageIncidents(ctx context.Context, monitorID domain.MonitorID, anchor *ports.IncidentAnchor, maxRows int) ([]ports.IncidentRecord, error) {
	repository.pageCalls++
	repository.gotCtx = ctx
	repository.gotMonitor = monitorID
	repository.gotAnchor = anchor
	repository.gotMaxRows = maxRows
	return repository.entries, repository.err
}

func TestGetMonitorIncidentsLimitValidation(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	validCursor, err := EncodeIncidentCursor(ports.IncidentAnchor{
		OpenedAt:   openedAt,
		IncidentID: mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
	})
	if err != nil {
		t.Fatalf("EncodeIncidentCursor() error = %v", err)
	}
	historyToken, err := EncodeHistoryCursor(ports.HistoryAnchor{
		CompletedAt: openedAt,
		CheckID:     mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"),
	})
	if err != nil {
		t.Fatalf("EncodeHistoryCursor() error = %v", err)
	}

	for _, tc := range []struct {
		name    string
		monitor domain.MonitorID
		limit   int
		cursor  string
	}{
		{name: "zero monitor", monitor: domain.MonitorID{}, limit: 20},
		{name: "zero limit", monitor: id, limit: 0},
		{name: "over limit", monitor: id, limit: 51},
		{name: "malformed cursor", monitor: id, limit: 20, cursor: "not-a-cursor"},
		{name: "inventory cursor", monitor: id, limit: 20, cursor: mustEncodeInventoryPosition(t, "2026-11-01T12:00:00.123456Z", "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")},
		{name: "history cursor", monitor: id, limit: 20, cursor: historyToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &fakeMonitorIncidentRepository{}
			useCase := NewGetMonitorIncidents(repository)

			_, err := useCase.Execute(context.Background(), tc.monitor, IncidentInput{Limit: tc.limit, Cursor: tc.cursor})
			if !errors.Is(err, ErrInvalidIncidentQuery) && !errors.Is(err, domain.ErrInvalidMonitorID) {
				t.Fatalf("Execute() error = %v, want an invalid-input rejection", err)
			}
			if repository.pageCalls != 0 {
				t.Fatalf("repository PageIncidents calls = %d, want 0", repository.pageCalls)
			}
		})
	}

	repository := &fakeMonitorIncidentRepository{}
	useCase := NewGetMonitorIncidents(repository)
	if _, err := useCase.Execute(context.Background(), id, IncidentInput{Limit: 20, Cursor: validCursor}); err != nil {
		t.Fatalf("valid Execute() error = %v", err)
	}
	if repository.gotMaxRows != 21 || repository.gotAnchor == nil || repository.gotAnchor.IncidentID.String() != "018f22d3-1d6a-7cc0-a37b-46fc3faff001" {
		t.Fatalf("valid Execute forwarding = maxRows %d anchor %+v, want limit+1 and the decoded anchor", repository.gotMaxRows, repository.gotAnchor)
	}
}

func TestGetMonitorIncidentsMapsNotFoundAndPersistence(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")

	repository := &fakeMonitorIncidentRepository{err: ports.ErrMonitorNotFound}
	useCase := NewGetMonitorIncidents(repository)
	_, err := useCase.Execute(context.Background(), id, IncidentInput{Limit: 20})
	if !errors.Is(err, ErrMonitorNotFound) {
		t.Fatalf("Execute() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ErrMonitorNotFound.Error() {
		t.Fatalf("not-found error = %q, want stable %q", err, ErrMonitorNotFound)
	}

	repository = &fakeMonitorIncidentRepository{err: errors.New("pgx connection password=secret")}
	_, err = NewGetMonitorIncidents(repository).Execute(context.Background(), id, IncidentInput{Limit: 20})
	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Execute() error = %v, want ErrPersistence", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("infrastructure detail leaked: %q", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository = &fakeMonitorIncidentRepository{}
	if _, err := NewGetMonitorIncidents(repository).Execute(ctx, id, IncidentInput{Limit: 20}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Execute() error = %v, want context.Canceled", err)
	}
	if repository.pageCalls != 0 {
		t.Fatalf("repository PageIncidents calls after cancellation = %d, want 0", repository.pageCalls)
	}
}

func TestGetMonitorIncidentsMapsOpenAndResolvedRecords(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	resolvedAt := parseIncidentInstant(t, "2026-11-01T12:05:00.000000Z")
	openedBy := mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675")
	resolvedBy := mustCheckID(t, "aa29443e-c597-4e7b-9202-a4762e0e04c0")
	entries := []ports.IncidentRecord{
		{
			IncidentID:    mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
			OpenedAt:      openedAt,
			OpeningReason: domain.IncidentUnexpectedHTTPStatus,
		},
		{
			IncidentID:        mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff002"),
			OpenedAt:          openedAt.Add(-time.Hour),
			ResolvedAt:        &resolvedAt,
			OpeningReason:     domain.IncidentProbeFailure,
			OpenedByCheckID:   &openedBy,
			ResolvedByCheckID: &resolvedBy,
		},
	}
	repository := &fakeMonitorIncidentRepository{entries: entries}
	useCase := NewGetMonitorIncidents(repository)

	page, err := useCase.Execute(context.Background(), id, IncidentInput{Limit: 20})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(page.Items) != 2 || page.Limit != 20 {
		t.Fatalf("page = %+v, want two items and limit 20", page)
	}
	open := page.Items[0]
	if open.IncidentID() != entries[0].IncidentID || open.OpeningReason() != domain.IncidentUnexpectedHTTPStatus || !open.OpenedAt().Equal(openedAt) {
		t.Fatalf("open item = %+v, want the first record", open)
	}
	if _, ok := open.ResolvedAt(); ok {
		t.Fatal("open item exposed a resolved instant")
	}
	if _, ok := open.OpenedByCheckID(); ok {
		t.Fatal("open item exposed a null opening evidence link")
	}
	resolved := page.Items[1]
	at, ok := resolved.ResolvedAt()
	if !ok || !at.Equal(resolvedAt) {
		t.Fatalf("resolved item instant = %v/%v, want %v/true", at, ok, resolvedAt)
	}
	if by, ok := resolved.OpenedByCheckID(); !ok || by != openedBy {
		t.Fatalf("resolved item opening evidence = %v/%v, want %v/true", by, ok, openedBy)
	}
	if by, ok := resolved.ResolvedByCheckID(); !ok || by != resolvedBy {
		t.Fatalf("resolved item resolving evidence = %v/%v, want %v/true", by, ok, resolvedBy)
	}
}

func TestGetMonitorIncidentsSanitizesInvalidRecords(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	resolvedAt := parseIncidentInstant(t, "2026-11-01T12:05:00.000000Z")
	early := openedAt.Add(-time.Minute)

	for _, tc := range []struct {
		name   string
		record ports.IncidentRecord
	}{
		{
			name: "unknown reason",
			record: ports.IncidentRecord{
				IncidentID:    mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
				OpenedAt:      openedAt,
				OpeningReason: domain.IncidentReason("maintenance"),
			},
		},
		{
			name: "zero opened instant",
			record: ports.IncidentRecord{
				IncidentID:    mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
				OpeningReason: domain.IncidentProbeFailure,
			},
		},
		{
			name: "resolved before opened",
			record: ports.IncidentRecord{
				IncidentID:    mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
				OpenedAt:      openedAt,
				ResolvedAt:    &early,
				OpeningReason: domain.IncidentProbeFailure,
			},
		},
		{
			name: "zero incident id",
			record: ports.IncidentRecord{
				OpenedAt:      openedAt,
				ResolvedAt:    &resolvedAt,
				OpeningReason: domain.IncidentProbeFailure,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &fakeMonitorIncidentRepository{entries: []ports.IncidentRecord{tc.record}}
			_, err := NewGetMonitorIncidents(repository).Execute(context.Background(), id, IncidentInput{Limit: 20})
			if !errors.Is(err, ErrPersistence) {
				t.Fatalf("Execute() error = %v, want ErrPersistence", err)
			}
		})
	}
}
