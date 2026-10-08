package domain

import (
	"errors"
	"testing"
)

func TestSchedulingStateClosed(t *testing.T) {
	for _, raw := range []string{"active", "paused"} {
		got, err := ParseSchedulingState(raw)
		if err != nil || string(got) != raw || !got.Valid() {
			t.Fatalf("state %q: %q %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "disabled", "Active", "PAUSED", " active", "paused "} {
		if _, err := ParseSchedulingState(raw); !errors.Is(err, ErrInvalidSchedulingState) {
			t.Fatalf("accepted %q: %v", raw, err)
		}
		if SchedulingState(raw).Valid() {
			t.Fatalf("valid invalid value %q", raw)
		}
	}
}
