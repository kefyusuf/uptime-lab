package domain

import (
	"errors"
	"testing"
)

func TestNewCheckResultAcceptsRustResultVocabulary(t *testing.T) {
	failures := []CheckResultKind{
		CheckResultDNSFailure,
		CheckResultPolicyRejected,
		CheckResultTimeout,
		CheckResultConnectError,
		CheckResultTLSError,
		CheckResultProtocolError,
		CheckResultInternalError,
	}
	for _, kind := range failures {
		result, err := NewCheckFailureResult(kind, 123)
		if err != nil {
			t.Fatalf("NewCheckFailureResult(%q) error = %v", kind, err)
		}
		if result.Kind() != kind || result.DurationMS() != 123 {
			t.Fatalf("result = (%q,%d), want (%q,123)", result.Kind(), result.DurationMS(), kind)
		}
		if _, ok := result.HTTPStatus(); ok {
			t.Fatalf("failure result %q unexpectedly has HTTP status", kind)
		}
	}

	response, err := NewHTTPResponseResult(20000, 599)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult() error = %v", err)
	}
	if response.Kind() != CheckResultHTTPResponse || response.DurationMS() != 20000 {
		t.Fatalf("HTTP result = (%q,%d)", response.Kind(), response.DurationMS())
	}
	if status, ok := response.HTTPStatus(); !ok || status != 599 {
		t.Fatalf("HTTPStatus() = (%d,%v), want (599,true)", status, ok)
	}
}

func TestCheckResultRejectsInvalidShapes(t *testing.T) {
	if _, err := NewCheckFailureResult(CheckResultWorkerTimeout, 10); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("worker_timeout error = %v, want ErrInvalidCheckResult", err)
	}
	if _, err := NewCheckFailureResult(CheckResultHTTPResponse, 10); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("http_response as failure error = %v, want ErrInvalidCheckResult", err)
	}
	if _, err := NewCheckFailureResult(CheckResultDNSFailure, -1); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("negative duration error = %v, want ErrInvalidCheckResult", err)
	}
	if _, err := NewCheckFailureResult(CheckResultDNSFailure, 20001); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("oversize duration error = %v, want ErrInvalidCheckResult", err)
	}
	if _, err := NewHTTPResponseResult(10, 99); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("status 99 error = %v, want ErrInvalidCheckResult", err)
	}
	if _, err := NewHTTPResponseResult(10, 600); !errors.Is(err, ErrInvalidCheckResult) {
		t.Fatalf("status 600 error = %v, want ErrInvalidCheckResult", err)
	}
}

func TestCheckResultDurationBoundary(t *testing.T) {
	for _, duration := range []int64{0, 20000} {
		if _, err := NewCheckFailureResult(CheckResultTimeout, duration); err != nil {
			t.Fatalf("duration %d rejected: %v", duration, err)
		}
		if _, err := NewHTTPResponseResult(duration, 200); err != nil {
			t.Fatalf("HTTP duration %d rejected: %v", duration, err)
		}
	}
}
