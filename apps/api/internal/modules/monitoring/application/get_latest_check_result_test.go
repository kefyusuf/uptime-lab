package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type fakeLatestCheckResultRepository struct {
	calls  int
	gotID  domain.MonitorID
	record ports.LatestCheckResultRecord
	err    error
}

func (repository *fakeLatestCheckResultRepository) LatestTerminalByMonitorID(
	_ context.Context,
	monitorID domain.MonitorID,
) (ports.LatestCheckResultRecord, error) {
	repository.calls++
	repository.gotID = monitorID
	return repository.record, repository.err
}

func TestGetLatestCheckResultReturnsHTTPResponseFact(t *testing.T) {
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	checkID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
	status := 204
	duration := int64(123)
	location := time.FixedZone("UTC+3", 3*60*60)
	completedAt := time.Date(2026, time.September, 29, 15, 0, 0, 123000000, location)
	repository := &fakeLatestCheckResultRepository{
		record: ports.LatestCheckResultRecord{
			CheckID:     checkID,
			ResultKind:  domain.CheckResultHTTPResponse,
			HTTPStatus:  &status,
			DurationMS:  &duration,
			CompletedAt: completedAt,
		},
	}

	result, err := NewGetLatestCheckResult(repository).Execute(context.Background(), monitorID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.calls != 1 || repository.gotID != monitorID {
		t.Fatalf("repository calls=%d id=%v, want one call with %v", repository.calls, repository.gotID, monitorID)
	}
	if result.CheckID() != checkID || result.ResultKind() != domain.CheckResultHTTPResponse {
		t.Fatalf("result identity/kind = %s/%s", result.CheckID().String(), result.ResultKind())
	}
	if got, ok := result.HTTPStatus(); !ok || got != 204 {
		t.Fatalf("HTTPStatus() = %d/%v, want 204/true", got, ok)
	}
	if got, ok := result.DurationMS(); !ok || got != 123 {
		t.Fatalf("DurationMS() = %d/%v, want 123/true", got, ok)
	}
	if !result.CompletedAt().Equal(completedAt) || result.CompletedAt().Location() != time.UTC {
		t.Fatalf("CompletedAt() = %v (%v), want same instant in UTC", result.CompletedAt(), result.CompletedAt().Location())
	}
}

func TestGetLatestCheckResultReturnsEveryClassifiedFailureFact(t *testing.T) {
	kinds := []domain.CheckResultKind{
		domain.CheckResultDNSFailure,
		domain.CheckResultPolicyRejected,
		domain.CheckResultTimeout,
		domain.CheckResultConnectError,
		domain.CheckResultTLSError,
		domain.CheckResultProtocolError,
		domain.CheckResultInternalError,
	}

	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			duration := int64(20000)
			checkID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
			repository := &fakeLatestCheckResultRepository{
				record: ports.LatestCheckResultRecord{
					CheckID:     checkID,
					ResultKind:  kind,
					DurationMS:  &duration,
					CompletedAt: time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC),
				},
			}

			result, err := NewGetLatestCheckResult(repository).Execute(
				context.Background(),
				mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
			)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if result.ResultKind() != kind {
				t.Fatalf("ResultKind() = %q, want %q", result.ResultKind(), kind)
			}
			if _, ok := result.HTTPStatus(); ok {
				t.Fatal("classified failure unexpectedly exposes HTTP status")
			}
			if got, ok := result.DurationMS(); !ok || got != 20000 {
				t.Fatalf("DurationMS() = %d/%v, want 20000/true", got, ok)
			}
		})
	}
}

func TestGetLatestCheckResultReturnsWorkerTimeoutWithoutProbeFields(t *testing.T) {
	checkID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
	repository := &fakeLatestCheckResultRepository{
		record: ports.LatestCheckResultRecord{
			CheckID:     checkID,
			ResultKind:  domain.CheckResultWorkerTimeout,
			CompletedAt: time.Date(2026, time.September, 29, 12, 0, 20, 0, time.UTC),
		},
	}

	result, err := NewGetLatestCheckResult(repository).Execute(
		context.Background(),
		mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ResultKind() != domain.CheckResultWorkerTimeout {
		t.Fatalf("ResultKind() = %q, want worker_timeout", result.ResultKind())
	}
	if _, ok := result.HTTPStatus(); ok {
		t.Fatal("worker_timeout unexpectedly exposes HTTP status")
	}
	if _, ok := result.DurationMS(); ok {
		t.Fatal("worker_timeout unexpectedly exposes duration")
	}
}

func TestGetLatestCheckResultMapsRepositoryOutcomes(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "monitor missing", err: ports.ErrMonitorNotFound, want: ErrMonitorNotFound},
		{name: "no terminal result", err: ports.ErrNoTerminalCheckResult, want: ErrNoTerminalCheckResult},
		{name: "persistence", err: errors.New("pgx password=secret"), want: ErrPersistence},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeLatestCheckResultRepository{err: test.err}
			_, err := NewGetLatestCheckResult(repository).Execute(
				context.Background(),
				mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if err.Error() != test.want.Error() {
				t.Fatalf("error = %q, want stable %q", err, test.want)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "pgx") {
				t.Fatalf("infrastructure details leaked in application error: %q", err)
			}
		})
	}
}

func TestGetLatestCheckResultRejectsZeroMonitorIDBeforeRepositoryAccess(t *testing.T) {
	repository := &fakeLatestCheckResultRepository{}

	_, err := NewGetLatestCheckResult(repository).Execute(context.Background(), domain.MonitorID{})
	if !errors.Is(err, domain.ErrInvalidMonitorID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidMonitorID", err)
	}
	if repository.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.calls)
	}
}

func TestGetLatestCheckResultRejectsImpossiblePersistedShapes(t *testing.T) {
	validCheckID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
	completedAt := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	status := 200
	duration := int64(1)
	tooLong := int64(20001)

	tests := []struct {
		name   string
		record ports.LatestCheckResultRecord
	}{
		{
			name: "zero check id",
			record: ports.LatestCheckResultRecord{
				ResultKind:  domain.CheckResultWorkerTimeout,
				CompletedAt: completedAt,
			},
		},
		{
			name: "zero completion",
			record: ports.LatestCheckResultRecord{
				CheckID:    validCheckID,
				ResultKind: domain.CheckResultWorkerTimeout,
			},
		},
		{
			name: "HTTP response missing status",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultHTTPResponse,
				DurationMS:  &duration,
				CompletedAt: completedAt,
			},
		},
		{
			name: "HTTP response missing duration",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultHTTPResponse,
				HTTPStatus:  &status,
				CompletedAt: completedAt,
			},
		},
		{
			name: "HTTP response invalid status",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultHTTPResponse,
				HTTPStatus:  intPointer(99),
				DurationMS:  &duration,
				CompletedAt: completedAt,
			},
		},
		{
			name: "failure with status",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultTimeout,
				HTTPStatus:  &status,
				DurationMS:  &duration,
				CompletedAt: completedAt,
			},
		},
		{
			name: "failure missing duration",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultTimeout,
				CompletedAt: completedAt,
			},
		},
		{
			name: "failure invalid duration",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultTimeout,
				DurationMS:  &tooLong,
				CompletedAt: completedAt,
			},
		},
		{
			name: "worker timeout with status",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultWorkerTimeout,
				HTTPStatus:  &status,
				CompletedAt: completedAt,
			},
		},
		{
			name: "worker timeout with duration",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultWorkerTimeout,
				DurationMS:  &duration,
				CompletedAt: completedAt,
			},
		},
		{
			name: "unknown result kind",
			record: ports.LatestCheckResultRecord{
				CheckID:     validCheckID,
				ResultKind:  domain.CheckResultKind("future_kind"),
				DurationMS:  &duration,
				CompletedAt: completedAt,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeLatestCheckResultRepository{record: test.record}
			_, err := NewGetLatestCheckResult(repository).Execute(
				context.Background(),
				mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
			)
			if !errors.Is(err, ErrPersistence) {
				t.Fatalf("Execute() error = %v, want ErrPersistence", err)
			}
			if err.Error() != ErrPersistence.Error() {
				t.Fatalf("error = %q, want stable %q", err, ErrPersistence)
			}
		})
	}
}

var _ ports.LatestCheckResultRepository = (*fakeLatestCheckResultRepository)(nil)
