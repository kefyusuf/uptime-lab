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

type fakeMonitorRepository struct {
	createCalls int
	created     []domain.Monitor
	createErr   error
	byIDCalls   int
	byIDMonitor domain.Monitor
	byIDErr     error
}

func (repository *fakeMonitorRepository) Create(_ context.Context, monitor domain.Monitor) error {
	repository.createCalls++
	repository.created = append(repository.created, monitor)
	return repository.createErr
}

func (repository *fakeMonitorRepository) ByID(_ context.Context, _ domain.MonitorID) (domain.Monitor, error) {
	repository.byIDCalls++
	return repository.byIDMonitor, repository.byIDErr
}

func mustMonitorID(t *testing.T, raw string) domain.MonitorID {
	t.Helper()
	id, err := domain.ParseMonitorID(raw)
	if err != nil {
		t.Fatalf("ParseMonitorID(%q) error = %v", raw, err)
	}
	return id
}

func TestRegisterMonitorCreatesAndPersistsMonitor(t *testing.T) {
	repository := &fakeMonitorRepository{}
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	location := time.FixedZone("UTC+3", 3*60*60)
	now := time.Date(2026, time.September, 20, 22, 45, 0, 123, location)
	idCalls := 0
	clockCalls := 0

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) {
			idCalls++
			return id, nil
		},
		func() time.Time {
			clockCalls++
			return now
		},
	)

	monitor, err := useCase.Execute(context.Background(), "https://example.com/health?x=1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.createCalls != 1 {
		t.Fatalf("repository Create calls = %d, want 1", repository.createCalls)
	}
	if len(repository.created) != 1 {
		t.Fatalf("created monitors = %d, want 1", len(repository.created))
	}
	if monitor.ID() != id || repository.created[0].ID() != id {
		t.Fatal("generated MonitorID was not persisted and returned")
	}
	if got := monitor.TargetURL().String(); got != "https://example.com/health?x=1" {
		t.Fatalf("TargetURL = %q", got)
	}
	if !monitor.CreatedAt().Equal(now) || monitor.CreatedAt().Location() != time.UTC {
		t.Fatalf("CreatedAt = %v (%v), want same instant normalized to UTC", monitor.CreatedAt(), monitor.CreatedAt().Location())
	}
	if idCalls != 1 {
		t.Fatalf("ID generator calls = %d, want 1", idCalls)
	}
	if clockCalls != 1 {
		t.Fatalf("clock calls = %d, want 1", clockCalls)
	}
}

func TestRegisterMonitorRejectsInvalidTargetBeforeDependenciesOrPersistence(t *testing.T) {
	repository := &fakeMonitorRepository{}
	idCalls := 0
	clockCalls := 0

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) {
			idCalls++
			return domain.MonitorID{}, nil
		},
		func() time.Time {
			clockCalls++
			return time.Now()
		},
	)

	_, err := useCase.Execute(context.Background(), "example.com")
	if !errors.Is(err, domain.ErrInvalidTargetURL) {
		t.Fatalf("Execute() error = %v, want ErrInvalidTargetURL", err)
	}
	if repository.createCalls != 0 || idCalls != 0 || clockCalls != 0 {
		t.Fatalf("invalid target touched dependencies: creates=%d ids=%d clocks=%d", repository.createCalls, idCalls, clockCalls)
	}
}

func TestRegisterMonitorPropagatesIDGeneratorFailureWithoutClockOrPersistence(t *testing.T) {
	repository := &fakeMonitorRepository{}
	generatorErr := errors.New("secure identity generation failed")
	clockCalls := 0

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) {
			return domain.MonitorID{}, generatorErr
		},
		func() time.Time {
			clockCalls++
			return time.Now()
		},
	)

	_, err := useCase.Execute(context.Background(), "https://example.com")
	if !errors.Is(err, generatorErr) {
		t.Fatalf("Execute() error = %v, want generator error", err)
	}
	if errors.Is(err, domain.ErrInvalidTargetURL) {
		t.Fatalf("generator error = %v, must not be classified as invalid target", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository Create calls = %d, want 0", repository.createCalls)
	}
	if clockCalls != 0 {
		t.Fatalf("clock calls = %d, want 0 after generator failure", clockCalls)
	}
}

func TestRegisterMonitorMapsRepositoryFailureToStableApplicationError(t *testing.T) {
	repository := &fakeMonitorRepository{createErr: errors.New("postgres password=secret")}
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) { return id, nil },
		func() time.Time { return time.Date(2026, time.September, 20, 20, 0, 0, 0, time.UTC) },
	)

	_, err := useCase.Execute(context.Background(), "https://example.com")
	if !errors.Is(err, ErrPersistence) {
		t.Fatalf("Execute() error = %v, want ErrPersistence", err)
	}
	if err.Error() != ErrPersistence.Error() {
		t.Fatalf("application persistence error = %q, want stable %q", err, ErrPersistence)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres") {
		t.Fatalf("infrastructure details leaked in application error: %q", err)
	}
}

func TestRegisterMonitorRejectsInvalidGeneratedIDBeforePersistence(t *testing.T) {
	repository := &fakeMonitorRepository{}
	clockCalls := 0
	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) { return domain.MonitorID{}, nil },
		func() time.Time {
			clockCalls++
			return time.Date(2026, time.September, 20, 20, 0, 0, 0, time.UTC)
		},
	)

	_, err := useCase.Execute(context.Background(), "https://example.com")
	if !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidMonitorID", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository Create calls = %d, want 0", repository.createCalls)
	}
	if clockCalls != 1 {
		t.Fatalf("clock calls = %d, want 1", clockCalls)
	}
}

func TestRegisterMonitorUsesDeterministicClockOnce(t *testing.T) {
	repository := &fakeMonitorRepository{}
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	fixed := time.Date(2026, time.September, 20, 19, 30, 0, 999, time.UTC)
	calls := 0

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) { return id, nil },
		func() time.Time {
			calls++
			return fixed
		},
	)

	monitor, err := useCase.Execute(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("clock calls = %d, want 1", calls)
	}
	if monitor.CreatedAt() != fixed {
		t.Fatalf("CreatedAt = %v, want %v", monitor.CreatedAt(), fixed)
	}
}

func TestRegisterMonitorDoesNotPreRejectDuplicateTargetText(t *testing.T) {
	repository := &fakeMonitorRepository{}
	ids := []domain.MonitorID{
		mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
		mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"),
	}
	index := 0

	useCase := NewRegisterMonitor(
		repository,
		func() (domain.MonitorID, error) {
			id := ids[index]
			index++
			return id, nil
		},
		func() time.Time { return time.Date(2026, time.September, 20, 20, 0, 0, 0, time.UTC) },
	)

	for range 2 {
		if _, err := useCase.Execute(context.Background(), "https://example.com/same"); err != nil {
			t.Fatalf("Execute() duplicate target error = %v", err)
		}
	}

	if repository.createCalls != 2 {
		t.Fatalf("repository Create calls = %d, want 2", repository.createCalls)
	}
	if repository.created[0].TargetURL().String() != repository.created[1].TargetURL().String() {
		t.Fatal("duplicate target text was altered or rejected")
	}
}

var _ ports.MonitorRepository = (*fakeMonitorRepository)(nil)
