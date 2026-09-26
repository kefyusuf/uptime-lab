package domain

import (
	"errors"
	"testing"
	"uuid"
)

func TestCheckIDAcceptsValidUUIDAndRoundTrips(t *testing.T) {
	raw := "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	id, err := ParseCheckID(raw)
	if err != nil {
		t.Fatalf("ParseCheckID() error = %v", err)
	}
	if id.String() != raw {
		t.Fatalf("CheckID.String() = %q, want %q", id.String(), raw)
	}
	if id.UUID().String() != raw {
		t.Fatalf("CheckID.UUID() = %q, want %q", id.UUID(), raw)
	}
}

func TestCheckIDRejectsInvalidAndEmptyUUID(t *testing.T) {
	for _, raw := range []string{"", "not-a-uuid"} {
		_, err := ParseCheckID(raw)
		if !errors.Is(err, ErrInvalidCheckID) {
			t.Fatalf("ParseCheckID(%q) error = %v, want ErrInvalidCheckID", raw, err)
		}
	}

	_, err := NewCheckID(uuid.Nil())
	if !errors.Is(err, ErrInvalidCheckID) {
		t.Fatalf("NewCheckID(uuid.Nil()) error = %v, want ErrInvalidCheckID", err)
	}
}
