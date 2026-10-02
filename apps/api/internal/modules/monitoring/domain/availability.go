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
)

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

// EvaluateAvailability applies freshness before outcome classification without I/O.
func EvaluateAvailability(observation *AvailabilityObservation, evaluatedAt time.Time) (Availability, error) {
	if !validAvailabilityTime(evaluatedAt) {
		return Availability{}, ErrInvalidAvailability
	}
	if observation == nil {
		return Availability{AvailabilityUnknown, AvailabilityNoResult}, nil
	}
	if !validAvailabilityTime(observation.CompletedAt) {
		return Availability{}, ErrInvalidAvailability
	}
	var assessment Availability
	switch observation.Kind {
	case CheckResultHTTPResponse:
		if observation.HTTPStatus == nil || *observation.HTTPStatus < 100 || *observation.HTTPStatus > 599 {
			return Availability{}, ErrInvalidAvailability
		}
		assessment = Availability{AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus}
		if *observation.HTTPStatus >= 200 && *observation.HTTPStatus <= 299 {
			assessment = Availability{AvailabilityAvailable, AvailabilitySuccessfulResponse}
		}
	case CheckResultDNSFailure, CheckResultTimeout, CheckResultConnectError, CheckResultTLSError, CheckResultProtocolError:
		assessment = Availability{AvailabilityUnavailable, AvailabilityProbeFailure}
	case CheckResultPolicyRejected:
		assessment = Availability{AvailabilityUnknown, AvailabilityPolicyRejected}
	case CheckResultInternalError, CheckResultWorkerTimeout:
		assessment = Availability{AvailabilityUnknown, AvailabilityExecutionFailure}
	default:
		return Availability{}, ErrInvalidAvailability
	}
	if observation.Kind != CheckResultHTTPResponse && observation.HTTPStatus != nil {
		return Availability{}, ErrInvalidAvailability
	}
	if observation.CompletedAt.After(evaluatedAt) {
		return Availability{AvailabilityUnknown, AvailabilityFutureResult}, nil
	}
	if evaluatedAt.Sub(observation.CompletedAt) > 120*time.Second {
		return Availability{AvailabilityUnknown, AvailabilityStaleResult}, nil
	}
	return assessment, nil
}

func validAvailabilityTime(value time.Time) bool {
	return !value.IsZero() && value.UTC().Year() >= 0 && value.UTC().Year() <= 9999
}
