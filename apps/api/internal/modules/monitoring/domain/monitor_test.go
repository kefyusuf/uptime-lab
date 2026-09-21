package domain

import (
	"errors"
	"testing"
	"time"
	"uuid"
)

func validTestMonitorParts(t *testing.T) (MonitorID, TargetURL) {
	t.Helper()

	id, err := NewMonitorID(uuid.NewV7())
	if err != nil {
		t.Fatalf("NewMonitorID() error = %v", err)
	}
	target, err := NewTargetURL("https://example.com/health")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	return id, target
}

func TestNewMonitorCreatesImmutableValueWithUTCInstant(t *testing.T) {
	id, target := validTestMonitorParts(t)
	location := time.FixedZone("UTC+3", 3*60*60)
	createdAt := time.Date(2026, time.September, 20, 22, 30, 0, 123456789, location)

	monitor, err := NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}
	if monitor.ID() != id {
		t.Fatalf("Monitor.ID() = %v, want %v", monitor.ID(), id)
	}
	if monitor.TargetURL() != target {
		t.Fatalf("Monitor.TargetURL() = %v, want %v", monitor.TargetURL(), target)
	}
	if !monitor.CreatedAt().Equal(createdAt) {
		t.Fatalf("Monitor.CreatedAt() = %v, want same instant as %v", monitor.CreatedAt(), createdAt)
	}
	if monitor.CreatedAt().Location() != time.UTC {
		t.Fatalf("Monitor.CreatedAt().Location() = %v, want UTC", monitor.CreatedAt().Location())
	}
}

func TestNewMonitorRejectsInvalidState(t *testing.T) {
	id, target := validTestMonitorParts(t)
	now := time.Now()

	tests := []struct {
		name   string
		id     MonitorID
		target TargetURL
		at     time.Time
		want   error
	}{
		{name: "zero id", target: target, at: now, want: ErrInvalidMonitorID},
		{name: "zero target", id: id, at: now, want: ErrInvalidTargetURL},
		{name: "zero created at", id: id, target: target, want: ErrInvalidCreatedAt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMonitor(tt.id, tt.target, tt.at)
			if !errors.Is(err, tt.want) {
				t.Fatalf("NewMonitor() error = %v, want %v", err, tt.want)
			}
		})
	}
}
