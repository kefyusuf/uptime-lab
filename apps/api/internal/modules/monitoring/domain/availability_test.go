package domain

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var availabilityTestTime = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

func TestEvaluateAvailabilityHTTPStatusBoundaries(t *testing.T) {
	for _, code := range []int{100, 199, 200, 204, 299, 300, 399, 400, 499, 500, 599} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			wantStatus, wantReason := AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus
			if code >= 200 && code <= 299 {
				wantStatus, wantReason = AvailabilityAvailable, AvailabilitySuccessfulResponse
			}
			observation := AvailabilityObservation{Kind: CheckResultHTTPResponse, HTTPStatus: &code, CompletedAt: availabilityTestTime}
			before := observation
			got, err := EvaluateAvailability(&observation, availabilityTestTime)
			if err != nil || got.Status() != wantStatus || got.Reason() != wantReason {
				t.Fatalf("got %v, %v", got, err)
			}
			if !reflect.DeepEqual(observation, before) || *observation.HTTPStatus != code {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestEvaluateAvailabilityOutcomes(t *testing.T) {
	for _, tc := range []struct {
		kind   CheckResultKind
		status AvailabilityStatus
		reason AvailabilityReason
	}{
		{CheckResultDNSFailure, AvailabilityUnavailable, AvailabilityProbeFailure},
		{CheckResultTimeout, AvailabilityUnavailable, AvailabilityProbeFailure},
		{CheckResultConnectError, AvailabilityUnavailable, AvailabilityProbeFailure},
		{CheckResultTLSError, AvailabilityUnavailable, AvailabilityProbeFailure},
		{CheckResultProtocolError, AvailabilityUnavailable, AvailabilityProbeFailure},
		{CheckResultPolicyRejected, AvailabilityUnknown, AvailabilityPolicyRejected},
		{CheckResultInternalError, AvailabilityUnknown, AvailabilityExecutionFailure},
		{CheckResultWorkerTimeout, AvailabilityUnknown, AvailabilityExecutionFailure},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			got, err := EvaluateAvailability(&AvailabilityObservation{Kind: tc.kind, CompletedAt: availabilityTestTime}, availabilityTestTime)
			if err != nil || got.Status() != tc.status || got.Reason() != tc.reason {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
	got, err := EvaluateAvailability(nil, availabilityTestTime)
	if err != nil || got.Status() != AvailabilityUnknown || got.Reason() != AvailabilityNoResult {
		t.Fatalf("no result: %v %v", got, err)
	}
}

func TestEvaluateAvailabilityFreshnessBoundary(t *testing.T) {
	status200 := 200
	completedAt := availabilityTestTime
	observation := AvailabilityObservation{Kind: CheckResultHTTPResponse, HTTPStatus: &status200, CompletedAt: completedAt}
	for _, tc := range []struct {
		age    time.Duration
		reason AvailabilityReason
	}{
		{120 * time.Second, AvailabilitySuccessfulResponse}, {120*time.Second + time.Nanosecond, AvailabilityStaleResult},
	} {
		got, err := EvaluateAvailability(&observation, completedAt.Add(tc.age))
		if err != nil || got.Reason() != tc.reason {
			t.Fatalf("age %s: %v %v", tc.age, got, err)
		}
	}
}

func TestEvaluateAvailabilityFutureAndStalePrecedence(t *testing.T) {
	code := 200
	for _, kind := range []CheckResultKind{CheckResultHTTPResponse, CheckResultDNSFailure, CheckResultTimeout, CheckResultConnectError, CheckResultTLSError, CheckResultProtocolError, CheckResultPolicyRejected, CheckResultInternalError, CheckResultWorkerTimeout} {
		observation := AvailabilityObservation{Kind: kind, CompletedAt: availabilityTestTime}
		if kind == CheckResultHTTPResponse {
			observation.HTTPStatus = &code
		}
		for _, tc := range []struct {
			now    time.Time
			reason AvailabilityReason
		}{
			{availabilityTestTime.Add(-time.Nanosecond), AvailabilityFutureResult},
			{availabilityTestTime.Add(121 * time.Second), AvailabilityStaleResult},
		} {
			got, err := EvaluateAvailability(&observation, tc.now)
			if err != nil || got.Status() != AvailabilityUnknown || got.Reason() != tc.reason {
				t.Fatalf("%s: %v %v", kind, got, err)
			}
		}
	}
	observation := AvailabilityObservation{Kind: CheckResultHTTPResponse, HTTPStatus: &code, CompletedAt: availabilityTestTime.In(time.FixedZone("offset", 3*3600))}
	got, err := EvaluateAvailability(&observation, availabilityTestTime)
	if err != nil || got.Status() != AvailabilityAvailable {
		t.Fatalf("equivalent instant: %v %v", got, err)
	}
}

func TestEvaluateAvailabilityInvalidInput(t *testing.T) {
	invalid := 99
	valid := 200
	for _, o := range []*AvailabilityObservation{
		{Kind: "invalid", CompletedAt: availabilityTestTime},
		{Kind: CheckResultHTTPResponse, CompletedAt: availabilityTestTime},
		{Kind: CheckResultHTTPResponse, HTTPStatus: &invalid, CompletedAt: availabilityTestTime},
		{Kind: CheckResultDNSFailure, HTTPStatus: &valid, CompletedAt: availabilityTestTime},
		{Kind: CheckResultHTTPResponse, HTTPStatus: &valid},
		{Kind: CheckResultWorkerTimeout, CompletedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		if _, err := EvaluateAvailability(o, availabilityTestTime); !errors.Is(err, ErrInvalidAvailability) {
			t.Fatalf("accepted %+v: %v", o, err)
		}
	}
	for _, now := range []time.Time{{}, time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)} {
		if _, err := EvaluateAvailability(nil, now); !errors.Is(err, ErrInvalidAvailability) {
			t.Fatalf("invalid clock: %v", err)
		}
	}
}
