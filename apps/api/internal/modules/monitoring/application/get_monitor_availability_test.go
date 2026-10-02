package application

import (
	"context"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"testing"
	"time"
)

func availabilityRecord(t *testing.T) ports.LatestCheckResultRecord {
	t.Helper()
	code, duration := 204, int64(10)
	return ports.LatestCheckResultRecord{CheckID: mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"), ResultKind: domain.CheckResultHTTPResponse, HTTPStatus: &code, DurationMS: &duration, CompletedAt: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
}

func TestGetMonitorAvailabilityOutcomes(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	for _, tc := range []struct {
		kind   domain.CheckResultKind
		code   int
		status domain.AvailabilityStatus
		reason domain.AvailabilityReason
	}{
		{domain.CheckResultHTTPResponse, 204, domain.AvailabilityAvailable, domain.AvailabilitySuccessfulResponse},
		{domain.CheckResultHTTPResponse, 500, domain.AvailabilityUnavailable, domain.AvailabilityUnexpectedHTTPStatus},
		{domain.CheckResultDNSFailure, 0, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{domain.CheckResultTimeout, 0, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{domain.CheckResultConnectError, 0, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{domain.CheckResultTLSError, 0, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{domain.CheckResultProtocolError, 0, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{domain.CheckResultPolicyRejected, 0, domain.AvailabilityUnknown, domain.AvailabilityPolicyRejected},
		{domain.CheckResultInternalError, 0, domain.AvailabilityUnknown, domain.AvailabilityExecutionFailure},
		{domain.CheckResultWorkerTimeout, 0, domain.AvailabilityUnknown, domain.AvailabilityExecutionFailure},
	} {
		t.Run(string(tc.kind)+string(tc.reason), func(t *testing.T) {
			record := availabilityRecord(t)
			record.ResultKind = tc.kind
			if tc.code == 0 {
				record.HTTPStatus = nil
			} else {
				record.HTTPStatus = &tc.code
			}
			if tc.kind == domain.CheckResultWorkerTimeout {
				record.DurationMS = nil
			}
			repo := &fakeLatestCheckResultRepository{record: record}
			now := record.CompletedAt.Add(time.Second)
			got, err := NewGetMonitorAvailability(repo, func() time.Time { return now }).Execute(context.Background(), id)
			evidence, ok := got.Evidence()
			if err != nil || got.Status() != tc.status || got.Reason() != tc.reason || !ok || evidence.CheckID != record.CheckID || !evidence.CompletedAt.Equal(record.CompletedAt) || !got.EvaluatedAt().Equal(now) || repo.calls != 1 {
				t.Fatalf("got %+v %v", got, err)
			}
			evidence.CompletedAt = time.Time{}
			again, _ := got.Evidence()
			if again.CompletedAt.IsZero() {
				t.Fatal("evidence is mutable")
			}
		})
	}
	t.Run("no result", func(t *testing.T) {
		repo := &fakeLatestCheckResultRepository{err: ports.ErrNoTerminalCheckResult}
		got, err := NewGetMonitorAvailability(repo, func() time.Time { return availabilityRecord(t).CompletedAt }).Execute(context.Background(), id)
		_, ok := got.Evidence()
		if err != nil || got.Status() != domain.AvailabilityUnknown || got.Reason() != domain.AvailabilityNoResult || ok {
			t.Fatalf("got %+v %v", got, err)
		}
	})
	for _, tc := range []struct {
		name               string
		mutate             func(*ports.LatestCheckResultRecord)
		repoErr, errorWant error
	}{
		{name: "missing", repoErr: ports.ErrMonitorNotFound, errorWant: ErrMonitorNotFound},
		{name: "canceled", repoErr: context.Canceled, errorWant: ErrPersistence},
		{name: "persistence", repoErr: errors.New("private database error"), errorWant: ErrPersistence},
		{name: "kind", mutate: func(r *ports.LatestCheckResultRecord) { r.ResultKind = "bad" }, errorWant: ErrPersistence},
		{name: "status", mutate: func(r *ports.LatestCheckResultRecord) { r.HTTPStatus = nil }, errorWant: ErrPersistence},
		{name: "duration", mutate: func(r *ports.LatestCheckResultRecord) { *r.DurationMS = -1 }, errorWant: ErrPersistence},
		{name: "ID", mutate: func(r *ports.LatestCheckResultRecord) { r.CheckID = domain.CheckID{} }, errorWant: ErrPersistence},
		{name: "time", mutate: func(r *ports.LatestCheckResultRecord) { r.CompletedAt = time.Time{} }, errorWant: ErrPersistence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := availabilityRecord(t)
			if tc.mutate != nil {
				tc.mutate(&record)
			}
			repo := &fakeLatestCheckResultRepository{record: record, err: tc.repoErr}
			calls := 0
			_, err := NewGetMonitorAvailability(repo, func() time.Time { calls++; return record.CompletedAt }).Execute(context.Background(), id)
			if !errors.Is(err, tc.errorWant) || calls != 0 || repo.calls != 1 {
				t.Fatalf("error %v clock %d reads %d", err, calls, repo.calls)
			}
		})
	}
	repo := &fakeLatestCheckResultRepository{}
	_, err := NewGetMonitorAvailability(repo, func() time.Time { t.Fatal("clock sampled"); return time.Time{} }).Execute(context.Background(), domain.MonitorID{})
	if !errors.Is(err, domain.ErrInvalidMonitorID) || repo.calls != 0 {
		t.Fatalf("invalid ID: %v", err)
	}
}

type orderedAvailabilityRepository struct {
	read func() (ports.LatestCheckResultRecord, error)
}

func (r orderedAvailabilityRepository) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	return r.read()
}

func TestGetMonitorAvailabilitySamplesClockAfterRead(t *testing.T) {
	for _, noResult := range []bool{false, true} {
		reads, clocks := 0, 0
		record := availabilityRecord(t)
		repo := orderedAvailabilityRepository{read: func() (ports.LatestCheckResultRecord, error) {
			reads++
			if noResult {
				return ports.LatestCheckResultRecord{}, ports.ErrNoTerminalCheckResult
			}
			return record, nil
		}}
		clock := func() time.Time {
			clocks++
			if reads != 1 {
				t.Fatal("sampled before read")
			}
			return record.CompletedAt.Add(time.Second)
		}
		_, err := NewGetMonitorAvailability(repo, clock).Execute(context.Background(), mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"))
		if err != nil || reads != 1 || clocks != 1 {
			t.Fatalf("reads=%d clocks=%d err=%v", reads, clocks, err)
		}
	}
}

func TestGetMonitorAvailabilityTimestampSafety(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	invalid := []time.Time{{}, time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(0, 1, 1, 0, 0, 0, 0, time.FixedZone("east", 3600)), time.Date(9999, 12, 31, 23, 59, 59, 0, time.FixedZone("west", -3600))}
	for _, value := range invalid {
		repo := &fakeLatestCheckResultRepository{record: availabilityRecord(t)}
		if _, err := NewGetMonitorAvailability(repo, func() time.Time { return value }).Execute(context.Background(), id); !errors.Is(err, ErrAvailabilityEvaluation) {
			t.Fatalf("clock %v: %v", value, err)
		}
		repo.record.CompletedAt = value
		calls := 0
		if _, err := NewGetMonitorAvailability(repo, func() time.Time { calls++; return availabilityRecord(t).CompletedAt }).Execute(context.Background(), id); !errors.Is(err, ErrPersistence) || calls != 0 {
			t.Fatalf("evidence %v: %v clock=%d", value, err, calls)
		}
	}
	record := availabilityRecord(t)
	repo := &fakeLatestCheckResultRepository{record: record}
	got, err := NewGetMonitorAvailability(repo, func() time.Time { return record.CompletedAt.Add(-time.Nanosecond) }).Execute(context.Background(), id)
	if err != nil || got.Reason() != domain.AvailabilityFutureResult {
		t.Fatalf("backward clock: %+v %v", got, err)
	}
	got, err = NewGetMonitorAvailability(repo, func() time.Time { return record.CompletedAt.In(time.FixedZone("east", 3600)) }).Execute(context.Background(), id)
	if err != nil || got.Status() != domain.AvailabilityAvailable || got.EvaluatedAt().Location() != time.UTC {
		t.Fatalf("UTC: %+v %v", got, err)
	}
}
