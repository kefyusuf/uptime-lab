package domain

import (
	"fmt"
	"uuid"
)

// MonitorID is the stable product identity of a monitor.
type MonitorID struct {
	value uuid.UUID
}

// NewMonitorID validates and wraps a UUID as a MonitorID.
func NewMonitorID(value uuid.UUID) (MonitorID, error) {
	if value == uuid.Nil() {
		return MonitorID{}, ErrInvalidMonitorID
	}
	return MonitorID{value: value}, nil
}

// ParseMonitorID parses a textual UUID into a MonitorID.
func ParseMonitorID(raw string) (MonitorID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return MonitorID{}, fmt.Errorf("%w: %v", ErrInvalidMonitorID, err)
	}
	return NewMonitorID(value)
}

// UUID returns the underlying standard-library UUID value.
func (id MonitorID) UUID() uuid.UUID {
	return id.value
}

// String returns the RFC 9562 textual UUID representation.
func (id MonitorID) String() string {
	return id.value.String()
}

func (id MonitorID) isZero() bool {
	return id.value == uuid.Nil()
}
