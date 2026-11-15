package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func availabilityRecord(t *testing.T) ports.LatestCheckResultRecord {
	t.Helper()
	code, duration := 204, int64(10)
	return ports.LatestCheckResultRecord{CheckID: mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"), ResultKind: domain.CheckResultHTTPResponse, HTTPStatus: &code, DurationMS: &duration, CompletedAt: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
}

type availabilitySpec struct {
	kind domain.CheckResultKind
	code int
}

var (
	availOK       = availabilitySpec{domain.CheckResultHTTPResponse, 204}
	availHTTP500  = availabilitySpec{domain.CheckResultHTTPResponse, 500}
	availDNS      = availabilitySpec{domain.CheckResultDNSFailure, 0}
	availTimeout  = availabilitySpec{domain.CheckResultTimeout, 0}
	availConnect  = availabilitySpec{domain.CheckResultConnectError, 0}
	availTLS      = availabilitySpec{domain.CheckResultTLSError, 0}
	availProtocol = availabilitySpec{domain.CheckResultProtocolError, 0}
	availPolicy   = availabilitySpec{domain.CheckResultPolicyRejected, 0}
	availInternal = availabilitySpec{domain.CheckResultInternalError, 0}
	availWorker   = availabilitySpec{domain.CheckResultWorkerTimeout, 0}
)

func availabilitySequence(t *testing.T, specs ...availabilitySpec) []ports.LatestCheckResultRecord {
	t.Helper()
	base := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	records := make([]ports.LatestCheckResultRecord, len(specs))
	for index, spec := range specs {
		duration := int64(10)
		record := ports.LatestCheckResultRecord{
			CheckID:     mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"),
			ResultKind:  spec.kind,
			CompletedAt: base.Add(-time.Duration(index) * time.Second),
		}
		switch spec.kind {
		case domain.CheckResultHTTPResponse:
			code := spec.code
			record.HTTPStatus = &code
			record.DurationMS = &duration
		case domain.CheckResultWorkerTimeout:
		default:
			record.DurationMS = &duration
		}
		records[index] = record
	}
	return records
}

func TestGetMonitorAvailabilityOutcomes(t *testing.T) {
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	for _, tc := range []struct {
		name   string
		specs  []availabilitySpec
		status domain.AvailabilityStatus
		reason domain.AvailabilityReason
	}{
		{"single success", []availabilitySpec{availOK}, domain.AvailabilityAvailable, domain.AvailabilitySuccessfulResponse},
		{"single http failure awaits", []availabilitySpec{availHTTP500}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"single dns awaits", []availabilitySpec{availDNS}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"single timeout awaits", []availabilitySpec{availTimeout}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"single connect awaits", []availabilitySpec{availConnect}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"single tls awaits", []availabilitySpec{availTLS}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"single protocol awaits", []availabilitySpec{availProtocol}, domain.AvailabilityUnknown, domain.AvailabilityAwaitingConfirmation},
		{"confirmed probe failure", []availabilitySpec{availDNS, availDNS, availDNS}, domain.AvailabilityUnavailable, domain.AvailabilityProbeFailure},
		{"confirmed http failure", []availabilitySpec{availHTTP500, availHTTP500, availHTTP500}, domain.AvailabilityUnavailable, domain.AvailabilityUnexpectedHTTPStatus},
		{"policy rejected", []availabilitySpec{availPolicy}, domain.AvailabilityUnknown, domain.AvailabilityPolicyRejected},
		{"internal error", []availabilitySpec{availInternal}, domain.AvailabilityUnknown, domain.AvailabilityExecutionFailure},
		{"worker timeout", []availabilitySpec{availWorker}, domain.AvailabilityUnknown, domain.AvailabilityExecutionFailure},
		{"recovering after confirmed outage", []availabilitySpec{availOK, availDNS, availDNS, availDNS}, domain.AvailabilityUnavailable, domain.AvailabilityRecovering},
		{"recovered after confirmed outage", []availabilitySpec{availOK, availOK, availDNS, availDNS, availDNS}, domain.AvailabilityAvailable, domain.AvailabilitySuccessfulResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := availabilitySequence(t, tc.specs...)
			repo := &fakeLatestCheckResultRepository{recent: records}
			now := records[0].CompletedAt.Add(time.Second)
			got, err := NewGetMonitorAvailability(repo, func() time.Time { return now }).Execute(context.Background(), id)
			evidence, ok := got.Evidence()
			if err != nil || got.Status() != tc.status || got.Reason() != tc.reason || !ok || evidence.CheckID != records[0].CheckID || !evidence.CompletedAt.Equal(records[0].CompletedAt) || !got.EvaluatedAt().Equal(now) || repo.calls != 1 || repo.gotLimit != domain.AvailabilitySequenceLimit {
				t.Fatalf("got %+v %v (calls=%d limit=%d)", got, err, repo.calls, repo.gotLimit)
			}
			evidence.CompletedAt = time.Time{}
			again, _ := got.Evidence()
			if again.CompletedAt.IsZero() {
				t.Fatal("evidence is mutable")
			}
		})
	}
	t.Run("no result", func(t *testing.T) {
		repo := &fakeLatestCheckResultRepository{}
		got, err := NewGetMonitorAvailability(repo, func() time.Time { return availabilityRecord(t).CompletedAt }).Execute(context.Background(), id)
		_, ok := got.Evidence()
		if err != nil || got.Status() != domain.AvailabilityUnknown || got.Reason() != domain.AvailabilityNoResult || ok || repo.gotLimit != domain.AvailabilitySequenceLimit {
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
			repo := &fakeLatestCheckResultRepository{recent: []ports.LatestCheckResultRecord{record}, recentErr: tc.repoErr}
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
	read func() ([]ports.LatestCheckResultRecord, error)
}

func (r orderedAvailabilityRepository) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	return ports.LatestCheckResultRecord{}, nil
}

func (r orderedAvailabilityRepository) RecentTerminalByMonitorID(context.Context, domain.MonitorID, int) ([]ports.LatestCheckResultRecord, error) {
	return r.read()
}

func TestGetMonitorAvailabilitySamplesClockAfterRead(t *testing.T) {
	for _, noResult := range []bool{false, true} {
		reads, clocks := 0, 0
		record := availabilityRecord(t)
		repo := orderedAvailabilityRepository{read: func() ([]ports.LatestCheckResultRecord, error) {
			reads++
			if noResult {
				return nil, nil
			}
			return []ports.LatestCheckResultRecord{record}, nil
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
		repo := &fakeLatestCheckResultRepository{recent: []ports.LatestCheckResultRecord{availabilityRecord(t)}}
		if _, err := NewGetMonitorAvailability(repo, func() time.Time { return value }).Execute(context.Background(), id); !errors.Is(err, ErrAvailabilityEvaluation) {
			t.Fatalf("clock %v: %v", value, err)
		}
		record := availabilityRecord(t)
		record.CompletedAt = value
		repo = &fakeLatestCheckResultRepository{recent: []ports.LatestCheckResultRecord{record}}
		calls := 0
		if _, err := NewGetMonitorAvailability(repo, func() time.Time { calls++; return availabilityRecord(t).CompletedAt }).Execute(context.Background(), id); !errors.Is(err, ErrPersistence) || calls != 0 {
			t.Fatalf("evidence %v: %v clock=%d", value, err, calls)
		}
	}
	record := availabilityRecord(t)
	repo := &fakeLatestCheckResultRepository{recent: []ports.LatestCheckResultRecord{record}}
	got, err := NewGetMonitorAvailability(repo, func() time.Time { return record.CompletedAt.Add(-time.Nanosecond) }).Execute(context.Background(), id)
	if err != nil || got.Reason() != domain.AvailabilityFutureResult {
		t.Fatalf("backward clock: %+v %v", got, err)
	}
	got, err = NewGetMonitorAvailability(repo, func() time.Time { return record.CompletedAt.In(time.FixedZone("east", 3600)) }).Execute(context.Background(), id)
	if err != nil || got.Status() != domain.AvailabilityAvailable || got.EvaluatedAt().Location() != time.UTC {
		t.Fatalf("UTC: %+v %v", got, err)
	}
}
