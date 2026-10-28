package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type fakeMonitorDeletionRepository struct {
	deleteCalls int
	gotCtx      context.Context
	deletedIDs  []domain.MonitorID
	deleteErr   error
}

func (repository *fakeMonitorDeletionRepository) Delete(ctx context.Context, id domain.MonitorID) error {
	repository.deleteCalls++
	repository.gotCtx = ctx
	repository.deletedIDs = append(repository.deletedIDs, id)
	return repository.deleteErr
}

func TestDeleteMonitorRejectsInvalidIDBeforeRepositoryAccess(t *testing.T) {
	repository := &fakeMonitorDeletionRepository{}
	useCase := NewDeleteMonitor(repository)

	err := useCase.Execute(context.Background(), domain.MonitorID{})
	if !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidMonitorID", err)
	}
	if repository.deleteCalls != 0 {
		t.Fatalf("repository Delete calls = %d, want 0", repository.deleteCalls)
	}
}

func TestDeleteMonitorCallsRepositoryOnceWithValidID(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorDeletionRepository{}
	useCase := NewDeleteMonitor(repository)

	if err := useCase.Execute(context.Background(), id); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.deleteCalls != 1 {
		t.Fatalf("repository Delete calls = %d, want 1", repository.deleteCalls)
	}
	if repository.deletedIDs[0] != id {
		t.Fatalf("deleted ID = %v, want %v", repository.deletedIDs[0], id)
	}
}

func TestDeleteMonitorForwardsCanceledContext(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorDeletionRepository{}
	useCase := NewDeleteMonitor(repository)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := useCase.Execute(ctx, id); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !errors.Is(repository.gotCtx.Err(), context.Canceled) {
		t.Fatalf("repository received context error = %v, want context.Canceled", repository.gotCtx.Err())
	}
}

func TestDeleteMonitorMapsPortNotFound(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorDeletionRepository{deleteErr: ports.ErrMonitorNotFound}
	useCase := NewDeleteMonitor(repository)

	err := useCase.Execute(context.Background(), id)
	if !errors.Is(err, ErrMonitorNotFound) {
		t.Fatalf("Execute() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ErrMonitorNotFound.Error() {
		t.Fatalf("not-found error = %q, want stable %q", err, ErrMonitorNotFound)
	}
}

func TestDeleteMonitorMapsOtherFailureToStablePersistenceError(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	repository := &fakeMonitorDeletionRepository{deleteErr: errors.New("pgx connection password=secret")}
	useCase := NewDeleteMonitor(repository)

	err := useCase.Execute(context.Background(), id)
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
