package domain

import (
	"errors"
	"testing"
	"uuid"
)

func TestNewMonitorIDAcceptsVersion7UUID(t *testing.T) {
	raw := uuid.NewV7()
	if got := raw[6] >> 4; got != 7 {
		t.Fatalf("uuid.NewV7() version nibble = %d, want 7", got)
	}

	id, err := NewMonitorID(raw)
	if err != nil {
		t.Fatalf("NewMonitorID() error = %v", err)
	}
	if id.UUID() != raw {
		t.Fatalf("NewMonitorID() UUID = %s, want %s", id.UUID(), raw)
	}

	same, err := NewMonitorID(raw)
	if err != nil {
		t.Fatalf("second NewMonitorID() error = %v", err)
	}
	if id != same {
		t.Fatal("MonitorID values wrapping the same UUID must compare equal")
	}
}

func TestNewMonitorIDRejectsNilUUID(t *testing.T) {
	_, err := NewMonitorID(uuid.Nil())
	if !errors.Is(err, ErrInvalidMonitorID) {
		t.Fatalf("NewMonitorID(uuid.Nil()) error = %v, want ErrInvalidMonitorID", err)
	}
}

func TestParseMonitorIDRoundTrips(t *testing.T) {
	raw := uuid.NewV7()
	id, err := ParseMonitorID(raw.String())
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	if id.UUID() != raw {
		t.Fatalf("ParseMonitorID() UUID = %s, want %s", id.UUID(), raw)
	}
}
