package domain

// SchedulingState controls eligibility for future claims, independently of availability.
type SchedulingState string

const (
	SchedulingActive SchedulingState = "active"
	SchedulingPaused SchedulingState = "paused"
)

func ParseSchedulingState(raw string) (SchedulingState, error) {
	state := SchedulingState(raw)
	if !state.Valid() {
		return "", ErrInvalidSchedulingState
	}
	return state, nil
}

func (state SchedulingState) Valid() bool {
	return state == SchedulingActive || state == SchedulingPaused
}
