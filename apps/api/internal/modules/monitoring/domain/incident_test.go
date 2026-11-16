package domain

import (
	"errors"
	"testing"
	"time"
	"uuid"
)

var (
	incidentCandidateID    = mustParseIncidentID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcc0")
	incidentExistingOpenID = mustParseIncidentID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcc1")
	incidentEvidenceCheck  = mustParseIncidentCheckID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	incidentResolveCheck   = mustParseIncidentCheckID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
)

func mustParseIncidentID(raw string) IncidentID {
	id, err := ParseIncidentID(raw)
	if err != nil {
		panic(err)
	}
	return id
}

func mustParseIncidentCheckID(raw string) CheckID {
	id, err := ParseCheckID(raw)
	if err != nil {
		panic(err)
	}
	return id
}

func TestApplyIncidentOutcomeOpenBoundary(t *testing.T) {
	state := IncidentState{}
	for index := 0; index < confirmationFailureThreshold-1; index++ {
		var action IncidentAction
		state, action = ApplyIncidentOutcome(state, TerminalFailure, IncidentProbeFailure, availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
		if action.Kind != IncidentActionNone {
			t.Fatalf("failure %d below N opened: %+v", index+1, action)
		}
		if state.OpenIncidentID != nil {
			t.Fatalf("failure %d below N set open pointer", index+1)
		}
	}
	if state.FailureStreak != confirmationFailureThreshold-1 || state.SuccessStreak != 0 {
		t.Fatalf("streaks before N = %d/%d", state.FailureStreak, state.SuccessStreak)
	}

	openedAt := availabilityTestTime.Add(time.Second)
	state, action := ApplyIncidentOutcome(state, TerminalFailure, IncidentUnexpectedHTTPStatus, openedAt, incidentEvidenceCheck, incidentCandidateID)
	if action.Kind != IncidentActionOpen {
		t.Fatalf("Nth failure action = %+v, want open", action)
	}
	if action.IncidentID != incidentCandidateID || action.At != openedAt || action.Reason != IncidentUnexpectedHTTPStatus || action.CheckID != incidentEvidenceCheck {
		t.Fatalf("open payload = %+v", action)
	}
	if state.OpenIncidentID == nil || *state.OpenIncidentID != incidentCandidateID {
		t.Fatalf("open pointer = %v, want candidate", state.OpenIncidentID)
	}
	if state.FailureStreak != confirmationFailureThreshold || state.SuccessStreak != 0 {
		t.Fatalf("streaks at open = %d/%d", state.FailureStreak, state.SuccessStreak)
	}
}

func TestApplyIncidentOutcomeResolveBoundary(t *testing.T) {
	state := IncidentState{FailureStreak: confirmationFailureThreshold, OpenIncidentID: &incidentExistingOpenID}

	state, action := ApplyIncidentOutcome(state, TerminalSuccess, "", availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if action.Kind != IncidentActionNone {
		t.Fatalf("M-1 success resolved early: %+v", action)
	}
	if state.OpenIncidentID == nil || *state.OpenIncidentID != incidentExistingOpenID {
		t.Fatal("M-1 success cleared the open pointer")
	}
	if state.SuccessStreak != recoverySuccessThreshold-1 || state.FailureStreak != 0 {
		t.Fatalf("streaks at M-1 = %d/%d", state.FailureStreak, state.SuccessStreak)
	}

	resolvedAt := availabilityTestTime.Add(time.Second)
	state, action = ApplyIncidentOutcome(state, TerminalSuccess, "", resolvedAt, incidentResolveCheck, incidentCandidateID)
	if action.Kind != IncidentActionResolve {
		t.Fatalf("Mth success action = %+v, want resolve", action)
	}
	if action.IncidentID != incidentExistingOpenID {
		t.Fatalf("resolve payload id = %v, want the prior open incident", action.IncidentID)
	}
	if action.At != resolvedAt || action.CheckID != incidentResolveCheck {
		t.Fatalf("resolve payload = %+v", action)
	}
	if state.OpenIncidentID != nil {
		t.Fatal("resolve left the open pointer set")
	}
	if state.SuccessStreak != 0 || state.FailureStreak != 0 {
		t.Fatalf("streaks after resolve = %d/%d", state.FailureStreak, state.SuccessStreak)
	}
}

func TestApplyIncidentOutcomeInconclusiveReset(t *testing.T) {
	state := IncidentState{FailureStreak: 0, SuccessStreak: recoverySuccessThreshold - 1, OpenIncidentID: &incidentExistingOpenID}
	state, action := ApplyIncidentOutcome(state, TerminalInconclusive, "", availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if action.Kind != IncidentActionNone {
		t.Fatalf("inconclusive action = %+v, want none", action)
	}
	if state.FailureStreak != 0 || state.SuccessStreak != 0 {
		t.Fatalf("inconclusive did not zero streaks: %d/%d", state.FailureStreak, state.SuccessStreak)
	}
	if state.OpenIncidentID == nil || *state.OpenIncidentID != incidentExistingOpenID {
		t.Fatal("inconclusive resolved or changed the open incident")
	}

	partial := IncidentState{FailureStreak: confirmationFailureThreshold - 1}
	partial, action = ApplyIncidentOutcome(partial, TerminalInconclusive, "", availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if action.Kind != IncidentActionNone || partial.FailureStreak != 0 || partial.SuccessStreak != 0 || partial.OpenIncidentID != nil {
		t.Fatalf("inconclusive from partial failure streak = %+v %+v", partial, action)
	}
}

func TestApplyIncidentOutcomeImmediateFirstSuccess(t *testing.T) {
	state, action := ApplyIncidentOutcome(IncidentState{}, TerminalSuccess, "", availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if action.Kind != IncidentActionNone {
		t.Fatalf("first success with no open incident = %+v, want none", action)
	}
	if state.SuccessStreak != 1 || state.FailureStreak != 0 || state.OpenIncidentID != nil {
		t.Fatalf("first success state = %+v", state)
	}
}

func TestApplyIncidentOutcomeNoReopenWhileOpen(t *testing.T) {
	state := IncidentState{FailureStreak: confirmationFailureThreshold, OpenIncidentID: &incidentExistingOpenID}
	for index := 0; index < confirmationFailureThreshold+2; index++ {
		var action IncidentAction
		state, action = ApplyIncidentOutcome(state, TerminalFailure, IncidentProbeFailure, availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
		if action.Kind != IncidentActionNone {
			t.Fatalf("failure %d while open re-opened: %+v", index+1, action)
		}
		if state.OpenIncidentID == nil || *state.OpenIncidentID != incidentExistingOpenID {
			t.Fatalf("failure %d while open changed the pointer to %v", index+1, state.OpenIncidentID)
		}
	}
	if state.FailureStreak != confirmationFailureThreshold*2+2 {
		t.Fatalf("failure streak while open = %d, want it to keep counting", state.FailureStreak)
	}
}

func TestApplyIncidentOutcomeCounterResets(t *testing.T) {
	state, _ := ApplyIncidentOutcome(IncidentState{SuccessStreak: 1}, TerminalFailure, IncidentProbeFailure, availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if state.SuccessStreak != 0 || state.FailureStreak != 1 {
		t.Fatalf("failure did not reset success streak: %+v", state)
	}
	state, _ = ApplyIncidentOutcome(IncidentState{FailureStreak: confirmationFailureThreshold - 1}, TerminalSuccess, "", availabilityTestTime, incidentEvidenceCheck, incidentCandidateID)
	if state.FailureStreak != 0 || state.SuccessStreak != 1 {
		t.Fatalf("success did not reset failure streak: %+v", state)
	}
}

func TestSharedClassifierParityWithAvailability(t *testing.T) {
	for _, code := range []int{100, 199, 200, 204, 299, 300, 399, 400, 499, 500, 599} {
		status := code
		outcome, reason, err := ClassifyTerminalOutcome(CheckResultHTTPResponse, &status)
		wantOutcome, wantReason := TerminalFailure, IncidentUnexpectedHTTPStatus
		wantStatus, wantAvailReason := AvailabilityUnavailable, AvailabilityUnexpectedHTTPStatus
		if code >= 200 && code <= 299 {
			wantOutcome, wantReason = TerminalSuccess, ""
			wantStatus, wantAvailReason = AvailabilityAvailable, AvailabilitySuccessfulResponse
		}
		if err != nil || outcome != wantOutcome || reason != wantReason {
			t.Fatalf("http %d classifier = %v/%v err=%v, want %v/%v", code, outcome, reason, err, wantOutcome, wantReason)
		}
		got, err := EvaluateAvailability(repeatObservation(httpObservation(code, availabilityTestTime), confirmationFailureThreshold), availabilityTestTime)
		if err != nil || got.Status() != wantStatus || got.Reason() != wantAvailReason {
			t.Fatalf("http %d availability = %v/%v err=%v, want %v/%v", code, got.Status(), got.Reason(), err, wantStatus, wantAvailReason)
		}
	}

	for _, kind := range []CheckResultKind{CheckResultDNSFailure, CheckResultTimeout, CheckResultConnectError, CheckResultTLSError, CheckResultProtocolError} {
		outcome, reason, err := ClassifyTerminalOutcome(kind, nil)
		if err != nil || outcome != TerminalFailure || reason != IncidentProbeFailure {
			t.Fatalf("%s classifier = %v/%v err=%v, want failure/probe_failure", kind, outcome, reason, err)
		}
		got, err := EvaluateAvailability(repeatObservation(AvailabilityObservation{Kind: kind, CompletedAt: availabilityTestTime}, confirmationFailureThreshold), availabilityTestTime)
		if err != nil || got.Status() != AvailabilityUnavailable || got.Reason() != AvailabilityProbeFailure {
			t.Fatalf("%s availability = %v/%v err=%v", kind, got.Status(), got.Reason(), err)
		}
	}

	for _, tc := range []struct {
		kind        CheckResultKind
		availReason AvailabilityReason
	}{
		{CheckResultPolicyRejected, AvailabilityPolicyRejected},
		{CheckResultInternalError, AvailabilityExecutionFailure},
		{CheckResultWorkerTimeout, AvailabilityExecutionFailure},
	} {
		outcome, reason, err := ClassifyTerminalOutcome(tc.kind, nil)
		if err != nil || outcome != TerminalInconclusive || reason != "" {
			t.Fatalf("%s classifier = %v/%v err=%v, want inconclusive", tc.kind, outcome, reason, err)
		}
		got, err := EvaluateAvailability([]AvailabilityObservation{{Kind: tc.kind, CompletedAt: availabilityTestTime}}, availabilityTestTime)
		if err != nil || got.Status() != AvailabilityUnknown || got.Reason() != tc.availReason {
			t.Fatalf("%s availability = %v/%v err=%v", tc.kind, got.Status(), got.Reason(), err)
		}
	}

	valid := 200
	for _, tc := range []struct {
		kind   CheckResultKind
		status *int
	}{
		{CheckResultHTTPResponse, nil},
		{CheckResultHTTPResponse, intPtr(99)},
		{CheckResultHTTPResponse, intPtr(600)},
		{CheckResultDNSFailure, &valid},
		{CheckResultWorkerTimeout, &valid},
		{CheckResultKind("invalid"), nil},
	} {
		if _, _, err := ClassifyTerminalOutcome(tc.kind, tc.status); !errors.Is(err, ErrInvalidAvailability) {
			t.Fatalf("classifier accepted %s/%v: %v", tc.kind, tc.status, err)
		}
	}
}

func TestIncidentIDValueObject(t *testing.T) {
	raw := "018f22d3-1d6a-7cc0-a37b-46fc3fafdcc0"
	id, err := ParseIncidentID(raw)
	if err != nil {
		t.Fatalf("ParseIncidentID() error = %v", err)
	}
	if id.String() != raw || id.UUID().String() != raw {
		t.Fatalf("IncidentID round trip = %q / %q, want %q", id.String(), id.UUID().String(), raw)
	}
	for _, bad := range []string{"", "not-a-uuid"} {
		if _, err := ParseIncidentID(bad); !errors.Is(err, ErrInvalidIncidentID) {
			t.Fatalf("ParseIncidentID(%q) error = %v, want ErrInvalidIncidentID", bad, err)
		}
	}
	if _, err := NewIncidentID(uuid.Nil()); !errors.Is(err, ErrInvalidIncidentID) {
		t.Fatalf("NewIncidentID(nil) error = %v, want ErrInvalidIncidentID", err)
	}
}

func intPtr(value int) *int {
	return &value
}
