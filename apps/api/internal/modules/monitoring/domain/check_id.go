package domain

import (
	"fmt"
	"uuid"
)

// CheckID is the stable durable identity of one check execution.
type CheckID struct {
	value uuid.UUID
}

// NewCheckID validates and wraps a UUID as a CheckID.
func NewCheckID(value uuid.UUID) (CheckID, error) {
	if value == uuid.Nil() {
		return CheckID{}, ErrInvalidCheckID
	}
	return CheckID{value: value}, nil
}

// ParseCheckID parses a textual UUID into a CheckID.
func ParseCheckID(raw string) (CheckID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return CheckID{}, fmt.Errorf("%w: %v", ErrInvalidCheckID, err)
	}
	return NewCheckID(value)
}

// UUID returns the underlying UUID value.
func (id CheckID) UUID() uuid.UUID {
	return id.value
}

// String returns the RFC 9562 textual UUID representation.
func (id CheckID) String() string {
	return id.value.String()
}
