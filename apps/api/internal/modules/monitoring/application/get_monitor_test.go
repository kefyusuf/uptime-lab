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

func TestGetMonitorReturnsExistingMonitor(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	target, err := domain.NewTargetURL("https://example.com")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	want, err := domain.NewMonitor(id, target, time.Date(2026, time.September, 20, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}

	repository := &fakeMonitorRepository{byIDMonitor: want}
	useCase := NewGetMonitor(repository)

	got, err := useCase.Execute(context.Background(), id)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID() != want.ID() || got.TargetURL() != want.TargetURL() || got.CreatedAt() != want.CreatedAt() {
		t.Fatalf("Execute() monitor = %#v, want %#v", got, want)
	}
	if repository.byIDCalls != 1 {
		t.Fatalf("repository ByID calls = %d, want 1", repository.byIDCalls)
	}
}

func TestGetMonitorMapsPortNotFound(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorRepository{byIDErr: ports.ErrMonitorNotFound}
	useCase := NewGetMonitor(repository)

	_, err := useCase.Execute(context.Background(), id)
	if !errors.Is(err, ErrMonitorNotFound) {
		t.Fatalf("Execute() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ErrMonitorNotFound.Error() {
		t.Fatalf("not-found error = %q, want stable %q", err, ErrMonitorNotFound)
	}
}

func TestGetMonitorMapsOtherRepositoryFailureToStablePersistenceError(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorRepository{byIDErr: errors.New("pgx connection password=secret")}
	useCase := NewGetMonitor(repository)

	_, err := useCase.Execute(context.Background(), id)
	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Execute() error = %v, want ErrPersistence", err)
	}
	if err.Error() != ErrPersistence.Error() {
		t.Fatalf("application persistence error = %q, want stable %q", err, ErrPersistence)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pgx") {
		t.Fatalf("infrastructure details leaked in application error: %q", err)
	}
}

func TestGetMonitorRejectsZeroIDBeforeRepositoryAccess(t *testing.T) {
	repository := &fakeMonitorRepository{}
	useCase := NewGetMonitor(repository)

	_, err := useCase.Execute(context.Background(), domain.MonitorID{})
	if !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidMonitorID", err)
	}
	if repository.byIDCalls != 0 {
		t.Fatalf("repository ByID calls = %d, want 0", repository.byIDCalls)
	}
}
