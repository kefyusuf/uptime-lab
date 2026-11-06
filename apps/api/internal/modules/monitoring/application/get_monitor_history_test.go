package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type fakeMonitorHistoryRepository struct {
	pageCalls  int
	gotCtx     context.Context
	gotMonitor domain.MonitorID
	gotAnchor  *ports.HistoryAnchor
	gotMaxRows int
	entries    []ports.LatestCheckResultRecord
	err        error
}

func (repository *fakeMonitorHistoryRepository) Page(ctx context.Context, monitorID domain.MonitorID, anchor *ports.HistoryAnchor, maxRows int) ([]ports.LatestCheckResultRecord, error) {
	repository.pageCalls++
	repository.gotCtx = ctx
	repository.gotMonitor = monitorID
	repository.gotAnchor = anchor
	repository.gotMaxRows = maxRows
	return repository.entries, repository.err
}

func TestGetMonitorHistoryRejectsInvalidInputBeforeRepository(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	validCursor, err := EncodeHistoryCursor(ports.HistoryAnchor{
		CompletedAt: parseHistoryInstant(t, "2026-11-01T12:00:00.123456Z"),
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &fakeMonitorHistoryRepository{}
			useCase := NewGetMonitorHistory(repository)

			_, err := useCase.Execute(context.Background(), tc.monitor, HistoryInput{Limit: tc.limit, Cursor: tc.cursor})
			if !errors.Is(err, ErrInvalidHistoryQuery) && !errors.Is(err, domain.ErrInvalidMonitorID) {
				t.Fatalf("Execute() error = %v, want an invalid-input rejection", err)
			}
			if repository.pageCalls != 0 {
				t.Fatalf("repository Page calls = %d, want 0", repository.pageCalls)
			}
		})
	}

	repository := &fakeMonitorHistoryRepository{}
	useCase := NewGetMonitorHistory(repository)
	if _, err := useCase.Execute(context.Background(), id, HistoryInput{Limit: 20, Cursor: validCursor}); err != nil {
		t.Fatalf("valid Execute() error = %v", err)
	}
	if repository.gotMaxRows != 21 || repository.gotAnchor == nil || repository.gotAnchor.CheckID.String() != "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675" {
		t.Fatalf("valid Execute forwarding = maxRows %d anchor %+v, want limit+1 and the decoded anchor", repository.gotMaxRows, repository.gotAnchor)
	}
}

func TestGetMonitorHistoryMapsNotFoundAndPersistence(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")

	repository := &fakeMonitorHistoryRepository{err: ports.ErrMonitorNotFound}
	useCase := NewGetMonitorHistory(repository)
	_, err := useCase.Execute(context.Background(), id, HistoryInput{Limit: 20})
	if !errors.Is(err, ErrMonitorNotFound) {
		t.Fatalf("Execute() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ErrMonitorNotFound.Error() {
		t.Fatalf("not-found error = %q, want stable %q", err, ErrMonitorNotFound)
	}

	repository = &fakeMonitorHistoryRepository{err: errors.New("pgx connection password=secret")}
	_, err = NewGetMonitorHistory(repository).Execute(context.Background(), id, HistoryInput{Limit: 20})
	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Execute() error = %v, want ErrPersistence", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("infrastructure detail leaked: %q", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository = &fakeMonitorHistoryRepository{}
	if _, err := NewGetMonitorHistory(repository).Execute(ctx, id, HistoryInput{Limit: 20}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Execute() error = %v, want context.Canceled", err)
	}
	if repository.pageCalls != 0 {
		t.Fatalf("repository Page calls after cancellation = %d, want 0", repository.pageCalls)
	}
}

func TestGetMonitorHistoryMapsEntriesThroughLatestResultShape(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	first := parseHistoryInstant(t, "2026-11-01T12:00:00.123456Z")
	second := parseHistoryInstant(t, "2026-11-01T11:59:00.000000Z")
	status := 204
	duration := int64(123)
	entries := []ports.LatestCheckResultRecord{
		{CheckID: mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"), ResultKind: domain.CheckResultHTTPResponse, HTTPStatus: &status, DurationMS: &duration, CompletedAt: first},
		{CheckID: mustCheckID(t, "aa29443e-c597-4e7b-9202-a4762e0e04c0"), ResultKind: domain.CheckResultPolicyRejected, DurationMS: &duration, CompletedAt: second},
	}
	repository := &fakeMonitorHistoryRepository{entries: entries}
	useCase := NewGetMonitorHistory(repository)

	page, err := useCase.Execute(context.Background(), id, HistoryInput{Limit: 20})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].CheckID() != entries[0].CheckID || page.Items[1].ResultKind() != domain.CheckResultPolicyRejected {
		t.Fatalf("mapped items = %+v, want the ordered records", page.Items)
	}
	if status, ok := page.Items[0].HTTPStatus(); !ok || status != 204 {
		t.Fatalf("first item http status = %d/%v, want 204/true", status, ok)
	}
	if _, ok := page.Items[1].HTTPStatus(); ok {
		t.Fatal("second item exposed an HTTP status for a classified failure")
	}
}
