package domain

// TerminalOutcome is the shared success/failure/inconclusive classification of one
// terminal check result. The availability policy and the durable incident fold both
// consume it so the two projections agree on every transition.
type TerminalOutcome int

const (
	TerminalSuccess TerminalOutcome = iota
	TerminalFailure
	TerminalInconclusive
)

// ClassifyTerminalOutcome maps terminal execution facts to the shared outcome and,
// for a failure, the incident opening reason. It enforces the same result-shape
// invariants the availability policy always enforced: an http_response carries a
// status in [100,599], no other kind carries a status, and unknown kinds are invalid.
func ClassifyTerminalOutcome(kind CheckResultKind, httpStatus *int) (TerminalOutcome, IncidentReason, error) {
	switch kind {
	case CheckResultHTTPResponse:
		if httpStatus == nil || *httpStatus < 100 || *httpStatus > 599 {
			return 0, "", ErrInvalidAvailability
		}
		if *httpStatus >= 200 && *httpStatus <= 299 {
			return TerminalSuccess, "", nil
		}
		return TerminalFailure, IncidentUnexpectedHTTPStatus, nil
	case CheckResultDNSFailure, CheckResultTimeout, CheckResultConnectError, CheckResultTLSError, CheckResultProtocolError:
		if httpStatus != nil {
			return 0, "", ErrInvalidAvailability
		}
		return TerminalFailure, IncidentProbeFailure, nil
	case CheckResultPolicyRejected, CheckResultInternalError, CheckResultWorkerTimeout:
		if httpStatus != nil {
			return 0, "", ErrInvalidAvailability
		}
		return TerminalInconclusive, "", nil
	default:
		return 0, "", ErrInvalidAvailability
	}
}