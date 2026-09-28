package domain

// CheckResultKind is the normalized execution outcome vocabulary.
type CheckResultKind string

const (
	CheckResultHTTPResponse   CheckResultKind = "http_response"
	CheckResultDNSFailure     CheckResultKind = "dns_error"
	CheckResultPolicyRejected CheckResultKind = "policy_rejected"
	CheckResultTimeout        CheckResultKind = "timeout"
	CheckResultConnectError   CheckResultKind = "connect_error"
	CheckResultTLSError       CheckResultKind = "tls_error"
	CheckResultProtocolError  CheckResultKind = "protocol_error"
	CheckResultInternalError  CheckResultKind = "internal_error"
	CheckResultWorkerTimeout  CheckResultKind = "worker_timeout"

	maxCheckDurationMS int64 = 20000
)

// CheckResult is one validated normalized execution fact submitted by Rust.
type CheckResult struct {
	kind          CheckResultKind
	durationMS    int64
	httpStatus    int
	hasHTTPStatus bool
}

// NewHTTPResponseResult creates a normalized final HTTP response fact.
func NewHTTPResponseResult(durationMS int64, httpStatus int) (CheckResult, error) {
	if !validDuration(durationMS) || httpStatus < 100 || httpStatus > 599 {
		return CheckResult{}, ErrInvalidCheckResult
	}

	return CheckResult{
		kind:          CheckResultHTTPResponse,
		durationMS:    durationMS,
		httpStatus:    httpStatus,
		hasHTTPStatus: true,
	}, nil
}

// NewCheckFailureResult creates a normalized Rust-submittable failure fact.
func NewCheckFailureResult(kind CheckResultKind, durationMS int64) (CheckResult, error) {
	if !validDuration(durationMS) || !isRustFailureKind(kind) {
		return CheckResult{}, ErrInvalidCheckResult
	}

	return CheckResult{
		kind:       kind,
		durationMS: durationMS,
	}, nil
}

// Kind returns the normalized result kind.
func (result CheckResult) Kind() CheckResultKind {
	return result.kind
}

// DurationMS returns the monotonic probe duration in milliseconds.
func (result CheckResult) DurationMS() int64 {
	return result.durationMS
}

// HTTPStatus returns the final HTTP status only for http_response results.
func (result CheckResult) HTTPStatus() (int, bool) {
	if !result.hasHTTPStatus {
		return 0, false
	}
	return result.httpStatus, true
}

func validDuration(durationMS int64) bool {
	return durationMS >= 0 && durationMS <= maxCheckDurationMS
}

func isRustFailureKind(kind CheckResultKind) bool {
	switch kind {
	case CheckResultDNSFailure,
		CheckResultPolicyRejected,
		CheckResultTimeout,
		CheckResultConnectError,
		CheckResultTLSError,
		CheckResultProtocolError,
		CheckResultInternalError:
		return true
	default:
		return false
	}
}
