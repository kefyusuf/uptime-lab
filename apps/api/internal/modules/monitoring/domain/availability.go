package domain

import "time"

// AvailabilityStatus is the current product assessment vocabulary.
type AvailabilityStatus string

// AvailabilityReason explains an assessment without changing execution evidence.
type AvailabilityReason string

const (
	AvailabilityAvailable            AvailabilityStatus = "available"
	AvailabilityUnavailable          AvailabilityStatus = "unavailable"
	AvailabilityUnknown              AvailabilityStatus = "unknown"
	AvailabilitySuccessfulResponse   AvailabilityReason = "successful_response"
	AvailabilityUnexpectedHTTPStatus AvailabilityReason = "unexpected_http_status"
	AvailabilityProbeFailure         AvailabilityReason = "probe_failure"
	AvailabilityNoResult             AvailabilityReason = "no_result"
	AvailabilityFutureResult         AvailabilityReason = "future_result"
	AvailabilityStaleResult          AvailabilityReason = "stale_result"
	AvailabilityPolicyRejected       AvailabilityReason = "policy_rejected"
	AvailabilityExecutionFailure     AvailabilityReason = "execution_failure"
	AvailabilityAwaitingConfirmation AvailabilityReason = "awaiting_confirmation"
	AvailabilityRecovering           AvailabilityReason = "recovering"
)

const (
	confirmationFailureThreshold = 3
	recoverySuccessThreshold     = 2
)

// AvailabilitySequenceLimit bounds the newest-first terminal read the availability
// policy needs: enough rows to confirm an outage and gate a recovery.
const AvailabilitySequenceLimit = confirmationFailureThreshold + recoverySuccessThreshold

// Availability is an immutable assessment made at a caller-supplied instant.
type Availability struct {
	status AvailabilityStatus
	reason AvailabilityReason
}

func (a Availability) Status() AvailabilityStatus { return a.status }
func (a Availability) Reason() AvailabilityReason { return a.reason }

// AvailabilityObservation contains only the terminal facts needed by the policy.
type AvailabilityObservation struct {
	Kind        CheckResultKind
	HTTPStatus  *int
	CompletedAt time.Time
}

// EvaluateAvailability classifies a newest-first sequence of terminal observations
// without I/O. An empty sequence means no terminal result.
func EvaluateAvailability(observations []AvailabilityObservation, evaluatedAt time.Time) (Availability, error) {
	if !validAvailabilityTime(evaluatedAt) {
		return Availability{}, ErrInvalidAvailability
	}
	if len(observations) == 0 {
		return Availability{AvailabilityUnknown, AvailabilityNoResult}, nil
	}
	latest, latestOutcome, err := classifyAvailabilityObservation(observations[0])
	if err != nil {
		return Availability{}, err
	}
	if observations[0].CompletedAt.After(evaluatedAt) {
		return Availability{AvailabilityUnknown, AvailabilityFutureResult}, nil
	}
	if evaluatedAt.Sub(observations[0].CompletedAt) > 120*time.Second {
		return Availability{AvailabilityUnknown, AvailabilityStaleResult}, nil
	}
	if latestOutcome == TerminalInconclusive {
		return latest, nil
	}

	outcomes := make([]TerminalOutcome, 0, len(observations))
	for index := range observations {
		_, outcome, err := classifyAvailabilityObservation(observations[index])
		if err != nil {
			return Availability{}, err
		}
		if outcome == TerminalInconclusive {
			break
		}
		outcomes = append(outcomes, outcome)
	}

	leadingFailures := 0
	for leadingFailures < len(outcomes) && outcomes[leadingFailures] == TerminalFailure {
		leadingFailures++
	}
	if leadingFailures >= confirmationFailureThreshold {
		return latest, nil
	}
	if leadingFailures > 0 {
		return Availability{AvailabilityUnknown, AvailabilityAwaitingConfirmation}, nil
	}

	leadingSuccesses := 0
	for leadingSuccesses < len(outcomes) && outcomes[leadingSuccesses] == TerminalSuccess {
		leadingSuccesses++
	}
	priorFailures := 0
	for index := leadingSuccesses; index < len(outcomes) && outcomes[index] == TerminalFailure; index++ {
		priorFailures++
	}
	if priorFailures >= confirmationFailureThreshold && leadingSuccesses < recoverySuccessThreshold {
		return Availability{AvailabilityUnavailable, AvailabilityRecovering}, nil
	}
	return Availability{AvailabilityAvailable, AvailabilitySuccessfulResponse}, nil
}

func classifyAvailabilityObservation(observation AvailabilityObservation) (Availability, TerminalOutcome, error) {
	if !validAvailabilityTime(observation.CompletedAt) {
		return Availability{}, 0, ErrInvalidAvailability
	}
	outcome, reason, err := ClassifyTerminalOutcome(observation.Kind, observation.HTTPStatus)
	if err != nil {
		return Availability{}, 0, err
	}
	switch outcome {
	case TerminalSuccess:
		return Availability{AvailabilityAvailable, AvailabilitySuccessfulResponse}, outcome, nil
	case TerminalFailure:
		if reason == IncidentUnexpectedHTTPStatus {
			return Availability{AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus}, outcome, nil
		}
		return Availability{AvailabilityUnavailable, AvailabilityProbeFailure}, outcome, nil
	default:
		if observation.Kind == CheckResultPolicyRejected {
			return Availability{AvailabilityUnknown, AvailabilityPolicyRejected}, outcome, nil
		}
		return Availability{AvailabilityUnknown, AvailabilityExecutionFailure}, outcome, nil
	}
}

func validAvailabilityTime(value time.Time) bool {
	return !value.IsZero() && value.UTC().Year() >= 0 && value.UTC().Year() <= 9999
}