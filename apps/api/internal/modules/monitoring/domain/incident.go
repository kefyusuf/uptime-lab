package domain

import (
	"fmt"
	"time"
	"uuid"
)

// IncidentID is the stable durable identity of one incident window.
type IncidentID struct {
	value uuid.UUID
}

// NewIncidentID validates and wraps a UUID as an IncidentID.
func NewIncidentID(value uuid.UUID) (IncidentID, error) {
	if value == uuid.Nil() {
		return IncidentID{}, ErrInvalidIncidentID
	}
	return IncidentID{value: value}, nil
}

// ParseIncidentID parses a textual UUID into an IncidentID.
func ParseIncidentID(raw string) (IncidentID, error) {
	value, err := uuid.Parse(raw)
	if err != nil {
		return IncidentID{}, fmt.Errorf("%w: %v", ErrInvalidIncidentID, err)
	}
	return NewIncidentID(value)
}

// UUID returns the underlying UUID value.
func (id IncidentID) UUID() uuid.UUID {
	return id.value
}

// String returns the RFC 9562 textual UUID representation.
func (id IncidentID) String() string {
	return id.value.String()
}

// IncidentReason is the closed vocabulary for why an incident opened, captured from
// the confirming failure and persisted for the life of the window.
type IncidentReason string

const (
	IncidentUnexpectedHTTPStatus IncidentReason = "unexpected_http_status"
	IncidentProbeFailure         IncidentReason = "probe_failure"
)

// IncidentState is the durable per-Monitor fold position: consecutive terminal
// failure/success streaks plus the currently open incident, if any. The zero value
// is the no-prior-outage start; state accrues lazily from the first completion.
type IncidentState struct {
	FailureStreak  int
	SuccessStreak  int
	OpenIncidentID *IncidentID
}

// IncidentActionKind discriminates the durable write the adapter must persist.
type IncidentActionKind int

const (
	IncidentActionNone IncidentActionKind = iota
	IncidentActionOpen
	IncidentActionResolve
)

// IncidentAction carries the payload of a fold transition so the adapter never
// re-derives identity, instant, reason or evidence. Open carries the new candidate
// window; Resolve carries the prior open incident being closed (the fold clears the
// pointer, so the resolved identity is only available here). None carries no payload.
type IncidentAction struct {
	Kind       IncidentActionKind
	IncidentID IncidentID
	At         time.Time
	Reason     IncidentReason
	CheckID    CheckID
}

// ApplyIncidentOutcome folds one classified terminal outcome into the durable incident
// state without I/O. N consecutive failures open the candidate incident; M consecutive
// successes resolve the open incident; an inconclusive result resets both streaks and
// leaves any open incident untouched. A first success with no open incident is none,
// and an already-open incident is never re-opened. The policy is total and deterministic.
func ApplyIncidentOutcome(
	state IncidentState,
	outcome TerminalOutcome,
	reason IncidentReason,
	at time.Time,
	checkID CheckID,
	candidateID IncidentID,
) (IncidentState, IncidentAction) {
	switch outcome {
	case TerminalInconclusive:
		state.FailureStreak = 0
		state.SuccessStreak = 0
		return state, IncidentAction{Kind: IncidentActionNone}
	case TerminalSuccess:
		state.FailureStreak = 0
		state.SuccessStreak++
		if state.OpenIncidentID != nil && state.SuccessStreak >= recoverySuccessThreshold {
			resolved := *state.OpenIncidentID
			state.OpenIncidentID = nil
			state.SuccessStreak = 0
			return state, IncidentAction{Kind: IncidentActionResolve, IncidentID: resolved, At: at, CheckID: checkID}
		}
		return state, IncidentAction{Kind: IncidentActionNone}
	case TerminalFailure:
		state.SuccessStreak = 0
		state.FailureStreak++
		if state.OpenIncidentID == nil && state.FailureStreak >= confirmationFailureThreshold {
			state.OpenIncidentID = &candidateID
			return state, IncidentAction{Kind: IncidentActionOpen, IncidentID: candidateID, At: at, Reason: reason, CheckID: checkID}
		}
		return state, IncidentAction{Kind: IncidentActionNone}
	default:
		return state, IncidentAction{Kind: IncidentActionNone}
	}
}