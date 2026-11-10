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

func TestMonitorWithTargetURLPreservesIdentityAndCreationTime(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	target, err := domain.NewTargetURL("https://example.com/old")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	createdAt := time.Date(2026, 11, 1, 12, 0, 0, 123456000, time.UTC)
	monitor, err := domain.NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}

	newTarget, err := domain.NewTargetURL("https://example.com/new-target?region=eu")
	if err != nil {
		t.Fatalf("NewTargetURL(new) error = %v", err)
	}

	edited, err := monitor.WithTargetURL(newTarget)
	if err != nil {
		t.Fatalf("WithTargetURL() error = %v", err)
	}
	if edited.ID() != monitor.ID() {
		t.Fatalf("edited ID = %s, want %s", edited.ID(), monitor.ID())
	}
	if !edited.CreatedAt().Equal(monitor.CreatedAt()) {
		t.Fatalf("edited createdAt = %s, want unchanged %s", edited.CreatedAt(), monitor.CreatedAt())
	}
	if edited.TargetURL().String() != "https://example.com/new-target?region=eu" {
		t.Fatalf("edited target = %q", edited.TargetURL().String())
	}
	// The original value stays immutable.
	if monitor.TargetURL().String() != "https://example.com/old" {
		t.Fatalf("original monitor mutated: %q", monitor.TargetURL().String())
	}
}

func TestMonitorWithTargetURLRejectsInvalidTarget(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	target, err := domain.NewTargetURL("https://example.com/old")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	createdAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	monitor, err := domain.NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}

	invalid, invalidErr := domain.NewTargetURL("not-a-url")
	if invalidErr == nil {
		t.Fatalf("NewTargetURL(not-a-url) error = nil, want rejection")
	}
	if _, err := monitor.WithTargetURL(invalid); !errors.Is(err, domain.ErrInvalidTargetURL) {
		t.Fatalf("WithTargetURL(invalid) error = %v, want ErrInvalidTargetURL", err)
	}
}

func TestEditMonitorTargetRejectsInvalidInputBeforeRepository(t *testing.T) {
	repository := &fakeMonitorTargetEditRepository{}

	useCase := NewEditMonitorTarget(repository)
	for _, tc := range []struct {
		name      string
		monitorID domain.MonitorID
		rawTarget string
	}{
		{name: "zero monitor", monitorID: domain.MonitorID{}},
		{name: "empty target"},
		{name: "invalid target", rawTarget: "not-a-url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := useCase.Execute(context.Background(), tc.monitorID, tc.rawTarget)
			if err == nil {
				t.Fatalf("Execute() error = nil, want rejection")
			}
			if repository.editCalls != 0 {
				t.Fatalf("repository Edit calls = %d, want 0", repository.editCalls)
			}
		})
	}
}

func TestEditMonitorTargetMapsNotFoundAndPersistence(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")

	repository := &fakeMonitorTargetEditRepository{editErr: ports.ErrMonitorNotFound}
	useCase := NewEditMonitorTarget(repository)
	_, err := useCase.Execute(context.Background(), id, "https://example.com/new")
	if !errors.Is(err, ErrMonitorNotFound) {
		t.Fatalf("Execute() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ErrMonitorNotFound.Error() {
		t.Fatalf("not-found error = %q, want stable %q", err, ErrMonitorNotFound)
	}

	repository = &fakeMonitorTargetEditRepository{editErr: errors.New("pgx connection password=secret")}
	_, err = NewEditMonitorTarget(repository).Execute(context.Background(), id, "https://example.com/new")
	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Execute() error = %v, want ErrPersistence", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("infrastructure detail leaked: %q", err)
	}
}

func TestEditMonitorTargetReturnsEditedMonitor(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	createdAt := time.Date(2026, 11, 1, 12, 0, 0, 123456000, time.UTC)
	edited := domain.Monitor{}
	target, err := domain.NewTargetURL("https://example.com/edited")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	edited, err = domain.NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}

	repository := &fakeMonitorTargetEditRepository{editResult: edited}
	useCase := NewEditMonitorTarget(repository)

	got, err := useCase.Execute(context.Background(), id, "https://example.com/edited")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID() != edited.ID() || got.TargetURL().String() != "https://example.com/edited" || !got.CreatedAt().Equal(createdAt) {
		t.Fatalf("edited monitor = %+v", got)
	}
	if repository.editCalls != 1 {
		t.Fatalf("repository Edit calls = %d, want 1", repository.editCalls)
	}
}

func TestEditMonitorTargetPreservesCanceledContext(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorTargetEditRepository{}
	useCase := NewEditMonitorTarget(repository)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := useCase.Execute(ctx, id, "https://example.com/new"); err != nil {
		t.Fatalf("canceled Execute() error = %v", err)
	}
	if !errors.Is(repository.gotCtx.Err(), context.Canceled) {
		t.Fatalf("repository received context error = %v, want context.Canceled", repository.gotCtx.Err())
	}
}
