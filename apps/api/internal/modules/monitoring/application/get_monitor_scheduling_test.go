package application

import (
	"context"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"testing"
)

type schedulingRepositoryFake struct {
	state domain.SchedulingState
	err   error
	calls int
}

func (f *schedulingRepositoryFake) GetScheduling(context.Context, domain.MonitorID) (domain.SchedulingState, error) {
	f.calls++
	return f.state, f.err
}
func (f *schedulingRepositoryFake) SetScheduling(context.Context, domain.MonitorID, domain.SchedulingState) (domain.SchedulingState, error) {
	f.calls++
	return f.state, f.err
}

func TestSchedulingUseCasesMapNotFoundAndPersistence(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	for _, tc := range []struct{ input, want error }{{ports.ErrMonitorNotFound, ErrMonitorNotFound}, {errors.New("private database failure"), ErrPersistence}, {context.Canceled, context.Canceled}, {context.DeadlineExceeded, context.DeadlineExceeded}} {
		f := &schedulingRepositoryFake{err: tc.input}
		for _, read := range []bool{true, false} {
			var err error
			if read {
				_, err = NewGetMonitorScheduling(f).Execute(context.Background(), id)
			} else {
				_, err = NewSetMonitorScheduling(f).Execute(context.Background(), id, domain.SchedulingPaused)
			}
			if !errors.Is(err, tc.want) || err.Error() != tc.want.Error() {
				t.Fatalf("error %v want %v", err, tc.want)
			}
		}
	}
}

func TestSchedulingUseCasesRejectInvalidRepositoryState(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	for _, state := range []domain.SchedulingState{"", "disabled"} {
		f := &schedulingRepositoryFake{state: state}
		if _, err := NewGetMonitorScheduling(f).Execute(context.Background(), id); err != ErrPersistence {
			t.Fatal(err)
		}
		if _, err := NewSetMonitorScheduling(f).Execute(context.Background(), id, domain.SchedulingPaused); err != ErrPersistence {
			t.Fatal(err)
		}
	}
}

func TestSchedulingUseCasesReturnConfirmedState(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	for _, state := range []domain.SchedulingState{domain.SchedulingActive, domain.SchedulingPaused} {
		f := &schedulingRepositoryFake{state: state}
		if got, err := NewGetMonitorScheduling(f).Execute(context.Background(), id); err != nil || got != state {
			t.Fatal(got, err)
		}
		if got, err := NewSetMonitorScheduling(f).Execute(context.Background(), id, state); err != nil || got != state {
			t.Fatal(got, err)
		}
	}
}
