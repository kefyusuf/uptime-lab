package main

import (
	"testing"
	"time"
	"uuid"
)

func TestProductionMonitorIDGeneratorReturnsVersion7Identity(t *testing.T) {
	id, err := newMonitorID()
	if err != nil {
		t.Fatalf("newMonitorID() error = %v", err)
	}
	if id.UUID() == uuid.Nil() {
		t.Fatal("newMonitorID() returned nil UUID")
	}
	if got := id.UUID()[6] >> 4; got != 7 {
		t.Fatalf("newMonitorID() UUID version = %d, want 7", got)
	}
}

func TestProductionClockReturnsUTCMicrosecondPrecision(t *testing.T) {
	got := productionClock()
	if got.IsZero() {
		t.Fatal("productionClock() returned zero time")
	}
	if got.Location() != time.UTC {
		t.Fatalf("productionClock() location = %v, want UTC", got.Location())
	}
	if got.Nanosecond()%int(time.Microsecond) != 0 {
		t.Fatalf("productionClock() nanoseconds = %d, want microsecond precision", got.Nanosecond())
	}
}
