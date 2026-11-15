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

type availabilityOutcome int

const (
	outcomeSuccess availabilityOutcome = iota
	outcomeFailure
	outcomeInconclusive
)

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
	if latestOutcome == outcomeInconclusive {
		return latest, nil
	}

	outcomes := make([]availabilityOutcome, 0, len(observations))
	for index := range observations {
		_, outcome, err := classifyAvailabilityObservation(observations[index])
		if err != nil {
			return Availability{}, err
		}
		if outcome == outcomeInconclusive {
			break
		}
		outcomes = append(outcomes, outcome)
	}

	leadingFailures := 0
	for leadingFailures < len(outcomes) && outcomes[leadingFailures] == outcomeFailure {
		leadingFailures++
	}
	if leadingFailures >= confirmationFailureThreshold {
		return latest, nil
	}
	if leadingFailures > 0 {
		return Availability{AvailabilityUnknown, AvailabilityAwaitingConfirmation}, nil
	}

	leadingSuccesses := 0
	for leadingSuccesses < len(outcomes) && outcomes[leadingSuccesses] == outcomeSuccess {
		leadingSuccesses++
	}
	priorFailures := 0
	for index := leadingSuccesses; index < len(outcomes) && outcomes[index] == outcomeFailure; index++ {
		priorFailures++
	}
	if priorFailures >= confirmationFailureThreshold && leadingSuccesses < recoverySuccessThreshold {
		return Availability{AvailabilityUnavailable, AvailabilityRecovering}, nil
	}
	return Availability{AvailabilityAvailable, AvailabilitySuccessfulResponse}, nil
}

func classifyAvailabilityObservation(observation AvailabilityObservation) (Availability, availabilityOutcome, error) {
	if !validAvailabilityTime(observation.CompletedAt) {
		return Availability{}, 0, ErrInvalidAvailability
	}
	var assessment Availability
	var outcome availabilityOutcome
	switch observation.Kind {
	case CheckResultHTTPResponse:
		if observation.HTTPStatus == nil || *observation.HTTPStatus < 100 || *observation.HTTPStatus > 599 {
			return Availability{}, 0, ErrInvalidAvailability
		}
		if *observation.HTTPStatus >= 200 && *observation.HTTPStatus <= 299 {
			assessment = Availability{AvailabilityAvailable, AvailabilitySuccessfulResponse}
			outcome = outcomeSuccess
		} else {
			assessment = Availability{AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus}
			outcome = outcomeFailure
		}
	case CheckResultDNSFailure, CheckResultTimeout, CheckResultConnectError, CheckResultTLSError, CheckResultProtocolError:
		assessment = Availability{AvailabilityUnavailable, AvailabilityProbeFailure}
		outcome = outcomeFailure
	case CheckResultPolicyRejected:
		assessment = Availability{AvailabilityUnknown, AvailabilityPolicyRejected}
		outcome = outcomeInconclusive
	case CheckResultInternalError, CheckResultWorkerTimeout:
		assessment = Availability{AvailabilityUnknown, AvailabilityExecutionFailure}
		outcome = outcomeInconclusive
	default:
		return Availability{}, 0, ErrInvalidAvailability
	}
	if observation.Kind != CheckResultHTTPResponse && observation.HTTPStatus != nil {
		return Availability{}, 0, ErrInvalidAvailability
	}
	return assessment, outcome, nil
}

func validAvailabilityTime(value time.Time) bool {
	return !value.IsZero() && value.UTC().Year() >= 0 && value.UTC().Year() <= 9999
}
