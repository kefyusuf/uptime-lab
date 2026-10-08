package application

import (
	"context"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"testing"
)

func TestSchedulingUseCasesRejectInvalidInputBeforeRepository(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	f := &schedulingRepositoryFake{state: domain.SchedulingPaused}
	if _, err := NewGetMonitorScheduling(f).Execute(context.Background(), domain.MonitorID{}); !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatal(err)
	}
	if _, err := NewSetMonitorScheduling(f).Execute(context.Background(), domain.MonitorID{}, domain.SchedulingPaused); !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatal(err)
	}
	if _, err := NewSetMonitorScheduling(f).Execute(context.Background(), id, ""); !errors.Is(err, domain.ErrInvalidSchedulingState) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewGetMonitorScheduling(f).Execute(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := NewSetMonitorScheduling(f).Execute(ctx, id, domain.SchedulingPaused); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if f.calls != 0 {
		t.Fatalf("invalid requests reached storage %d times", f.calls)
	}
}

func TestSchedulingSetterRejectsMismatchedConfirmation(t *testing.T) {
	f := &schedulingRepositoryFake{state: domain.SchedulingActive}
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	if _, err := NewSetMonitorScheduling(f).Execute(context.Background(), id, domain.SchedulingPaused); err != ErrPersistence {
		t.Fatal(err)
	}
}
