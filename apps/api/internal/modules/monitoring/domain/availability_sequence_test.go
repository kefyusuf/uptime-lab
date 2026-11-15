package domain

import (
	"errors"
	"testing"
	"time"
)

type obsSpec struct {
	kind CheckResultKind
	code int
}

var (
	ok200    = obsSpec{CheckResultHTTPResponse, 200}
	http500  = obsSpec{CheckResultHTTPResponse, 500}
	dns      = obsSpec{CheckResultDNSFailure, 0}
	timeout  = obsSpec{CheckResultTimeout, 0}
	policy   = obsSpec{CheckResultPolicyRejected, 0}
	worker   = obsSpec{CheckResultWorkerTimeout, 0}
	internal = obsSpec{CheckResultInternalError, 0}
)

func sequence(specs ...obsSpec) []AvailabilityObservation {
	observations := make([]AvailabilityObservation, len(specs))
	for index, spec := range specs {
		observation := AvailabilityObservation{
			Kind:        spec.kind,
			CompletedAt: availabilityTestTime.Add(-time.Duration(index) * time.Second),
		}
		if spec.kind == CheckResultHTTPResponse {
			code := spec.code
			observation.HTTPStatus = &code
		}
		observations[index] = observation
	}
	return observations
}

func TestEvaluateAvailabilitySequencePolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sequence   []AvailabilityObservation
		wantStatus AvailabilityStatus
		wantReason AvailabilityReason
	}{
		{"single failure awaits confirmation", sequence(dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"two failures await confirmation", sequence(dns, dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"three failures confirm unavailable", sequence(dns, dns, dns), AvailabilityUnavailable, AvailabilityProbeFailure},
		{"four failures stay unavailable", sequence(dns, dns, dns, dns), AvailabilityUnavailable, AvailabilityProbeFailure},
		{"http failure reason surfaces", sequence(http500, http500, http500), AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus},
		{"latest failure reason wins mixed", sequence(http500, timeout, dns), AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus},
		{"probe latest reason wins mixed", sequence(timeout, http500, http500), AvailabilityUnavailable, AvailabilityProbeFailure},
		{"first success available immediately", sequence(ok200), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"success after one failure available", sequence(ok200, dns), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"success after two failures available", sequence(ok200, dns, dns), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"one success after confirmed outage recovering", sequence(ok200, dns, dns, dns), AvailabilityUnavailable, AvailabilityRecovering},
		{"two successes after confirmed outage available", sequence(ok200, ok200, dns, dns, dns), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"three successes after confirmed outage available", sequence(ok200, ok200, ok200, dns, dns, dns), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"leading failures stop at success", sequence(dns, ok200, dns, dns, dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"two leading failures stop at success", sequence(dns, dns, ok200, dns, dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"inconclusive latest precedence over failures", sequence(policy, dns, dns, dns), AvailabilityUnknown, AvailabilityPolicyRejected},
		{"worker latest precedence over failures", sequence(worker, dns, dns, dns), AvailabilityUnknown, AvailabilityExecutionFailure},
		{"internal latest precedence over failures", sequence(internal, dns, dns, dns), AvailabilityUnknown, AvailabilityExecutionFailure},
		{"inconclusive reset truncates failure streak", sequence(dns, dns, policy, dns, dns, dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"single failure then reset awaits", sequence(dns, policy, dns, dns, dns), AvailabilityUnknown, AvailabilityAwaitingConfirmation},
		{"reset blocks recovery confirmation", sequence(ok200, policy, dns, dns, dns), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"success run then confirmed outage then success", sequence(ok200, ok200, dns, dns, dns, ok200), AvailabilityAvailable, AvailabilitySuccessfulResponse},
		{"one success then confirmed outage then success", sequence(ok200, dns, dns, dns, ok200), AvailabilityUnavailable, AvailabilityRecovering},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateAvailability(tc.sequence, availabilityTestTime)
			if err != nil || got.Status() != tc.wantStatus || got.Reason() != tc.wantReason {
				t.Fatalf("got %v/%v err=%v, want %v/%v", got.Status(), got.Reason(), err, tc.wantStatus, tc.wantReason)
			}
		})
	}
}

func TestEvaluateAvailabilityValidatesWalkedPrefixOnly(t *testing.T) {
	invalidRow := AvailabilityObservation{Kind: "invalid", CompletedAt: availabilityTestTime.Add(-time.Second)}

	t.Run("invalid row inside walked prefix errors", func(t *testing.T) {
		observations := sequence(dns)
		observations = append(observations, invalidRow)
		if _, err := EvaluateAvailability(observations, availabilityTestTime); !errors.Is(err, ErrInvalidAvailability) {
			t.Fatalf("err = %v, want ErrInvalidAvailability", err)
		}
	})

	t.Run("invalid row beyond inconclusive reset is ignored", func(t *testing.T) {
		observations := sequence(dns, dns, policy)
		observations = append(observations, invalidRow)
		got, err := EvaluateAvailability(observations, availabilityTestTime)
		if err != nil || got.Status() != AvailabilityUnknown || got.Reason() != AvailabilityAwaitingConfirmation {
			t.Fatalf("got %v/%v err=%v, want unknown/awaiting_confirmation", got.Status(), got.Reason(), err)
		}
	})
}

func TestEvaluateAvailabilityFreshnessAppliesOnlyToLatest(t *testing.T) {
	older := AvailabilityObservation{Kind: CheckResultDNSFailure, CompletedAt: availabilityTestTime.Add(-10 * time.Minute)}
	observations := []AvailabilityObservation{httpObservation(200, availabilityTestTime), older}
	got, err := EvaluateAvailability(observations, availabilityTestTime)
	if err != nil || got.Status() != AvailabilityAvailable || got.Reason() != AvailabilitySuccessfulResponse {
		t.Fatalf("got %v/%v err=%v, want available despite stale older row", got.Status(), got.Reason(), err)
	}
}

func TestEvaluateAvailabilityStaleAndFutureLatestPrecedenceOverSequence(t *testing.T) {
	confirmed := sequence(dns, dns, dns)
	for _, tc := range []struct {
		name       string
		now        time.Time
		wantReason AvailabilityReason
	}{
		{"stale latest", availabilityTestTime.Add(121 * time.Second), AvailabilityStaleResult},
		{"future latest", availabilityTestTime.Add(-time.Nanosecond), AvailabilityFutureResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateAvailability(confirmed, tc.now)
			if err != nil || got.Status() != AvailabilityUnknown || got.Reason() != tc.wantReason {
				t.Fatalf("got %v/%v err=%v, want unknown/%v", got.Status(), got.Reason(), err, tc.wantReason)
			}
		})
	}
}
