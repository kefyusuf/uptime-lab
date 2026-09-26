package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type fakeCheckExecutionRepository struct {
	claimCalls    int
	claimInput    ports.ClaimDueCheckInput
	claimResult   ports.ClaimedCheck
	claimErr      error
	completeCalls int
	completeInput ports.CompleteCheckInput
	completeErr   error
}

func (repository *fakeCheckExecutionRepository) ClaimDueCheck(_ context.Context, input ports.ClaimDueCheckInput) (ports.ClaimedCheck, error) {
	repository.claimCalls++
	repository.claimInput = input
	return repository.claimResult, repository.claimErr
}

func (repository *fakeCheckExecutionRepository) CompleteCheck(_ context.Context, input ports.CompleteCheckInput) error {
	repository.completeCalls++
	repository.completeInput = input
	return repository.completeErr
}

func mustCheckID(t *testing.T, raw string) domain.CheckID {
	t.Helper()
	id, err := domain.ParseCheckID(raw)
	if err != nil {
		t.Fatalf("ParseCheckID(%q) error = %v", raw, err)
	}
	return id
}

func mustTargetURL(t *testing.T, raw string) domain.TargetURL {
	t.Helper()
	target, err := domain.NewTargetURL(raw)
	if err != nil {
		t.Fatalf("NewTargetURL(%q) error = %v", raw, err)
	}
	return target
}

func TestClaimDueCheckUsesServerPolicyAndReturnsWork(t *testing.T) {
	checkID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3")
	now := time.Date(2026, time.September, 26, 8, 0, 0, 123000000, time.UTC)
	repository := &fakeCheckExecutionRepository{
		claimResult: ports.ClaimedCheck{
			MonitorID: monitorID,
			TargetURL: mustTargetURL(t, "https://example.com/health"),
		},
	}
	idCalls := 0
	clockCalls := 0
	useCase := NewClaimDueCheck(
		repository,
		func() (domain.CheckID, error) {
			idCalls++
			return checkID, nil
		},
		func() time.Time {
			clockCalls++
			return now
		},
	)

	work, err := useCase.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.claimCalls != 1 || idCalls != 1 || clockCalls != 1 {
		t.Fatalf("calls claim=%d id=%d clock=%d, want 1 each", repository.claimCalls, idCalls, clockCalls)
	}
	if repository.claimInput.CheckID != checkID {
		t.Fatal("generated CheckID not forwarded")
	}
	if !repository.claimInput.Now.Equal(now) {
		t.Fatalf("Now = %v, want %v", repository.claimInput.Now, now)
	}
	if !repository.claimInput.DueBefore.Equal(now.Add(-60 * time.Second)) {
		t.Fatalf("DueBefore = %v", repository.claimInput.DueBefore)
	}
	if !repository.claimInput.Deadline.Equal(now.Add(20 * time.Second)) {
		t.Fatalf("Deadline = %v", repository.claimInput.Deadline)
	}
	if work.CheckID != checkID || work.MonitorID != monitorID {
		t.Fatal("work identities do not match claim")
	}
	if work.TargetURL.String() != "https://example.com/health" {
		t.Fatalf("TargetURL = %q", work.TargetURL.String())
	}
	if work.Timeout != 10*time.Second || work.MaxRedirects != 3 {
		t.Fatalf("policy = timeout %v redirects %d", work.Timeout, work.MaxRedirects)
	}
}

func TestClaimDueCheckMapsNoWorkAndFailures(t *testing.T) {
	checkID := mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	now := time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "no work", err: ports.ErrNoDueCheck, want: ErrNoDueCheck},
		{name: "persistence", err: errors.New("postgres secret"), want: ErrPersistence},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeCheckExecutionRepository{claimErr: test.err}
			useCase := NewClaimDueCheck(repository, func() (domain.CheckID, error) { return checkID, nil }, func() time.Time { return now })
			_, err := useCase.Execute(context.Background())
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if err.Error() != test.want.Error() {
				t.Fatalf("error = %q, want stable %q", err, test.want)
			}
		})
	}
}

func TestClaimDueCheckIDGenerationFailurePreventsPersistence(t *testing.T) {
	repository := &fakeCheckExecutionRepository{}
	generatorErr := errors.New("uuid generation failed")
	useCase := NewClaimDueCheck(
		repository,
		func() (domain.CheckID, error) { return domain.CheckID{}, generatorErr },
		func() time.Time { return time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC) },
	)

	_, err := useCase.Execute(context.Background())
	if !errors.Is(err, generatorErr) {
		t.Fatalf("Execute() error = %v, want generator error", err)
	}
	if repository.claimCalls != 0 {
		t.Fatalf("ClaimDueCheck calls = %d, want 0", repository.claimCalls)
	}
}

func TestClaimDueCheckRejectsInvalidGeneratedIDBeforeClockOrPersistence(t *testing.T) {
	repository := &fakeCheckExecutionRepository{}
	clockCalls := 0
	useCase := NewClaimDueCheck(
		repository,
		func() (domain.CheckID, error) { return domain.CheckID{}, nil },
		func() time.Time {
			clockCalls++
			return time.Now()
		},
	)

	_, err := useCase.Execute(context.Background())
	if !errors.Is(err, domain.ErrInvalidCheckID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidCheckID", err)
	}
	if repository.claimCalls != 0 || clockCalls != 0 {
		t.Fatalf("invalid generated ID touched dependencies: claims=%d clocks=%d", repository.claimCalls, clockCalls)
	}
}

func TestSubmitCheckResultValidatesBeforePersistenceAndUsesServerClock(t *testing.T) {
	now := time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC)
	repository := &fakeCheckExecutionRepository{}
	clockCalls := 0
	useCase := NewSubmitCheckResult(repository, func() time.Time {
		clockCalls++
		return now
	})

	input := SubmitCheckResultInput{
		Kind:       domain.CheckResultHTTPResponse,
		DurationMS: 20000,
		HTTPStatus: intPointer(204),
	}
	if err := useCase.Execute(context.Background(), "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", input); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repository.completeCalls != 1 || clockCalls != 1 {
		t.Fatalf("calls complete=%d clock=%d, want 1 each", repository.completeCalls, clockCalls)
	}
	if !repository.completeInput.CompletedAt.Equal(now) {
		t.Fatalf("CompletedAt = %v, want %v", repository.completeInput.CompletedAt, now)
	}
	if repository.completeInput.Result.DurationMS() != 20000 {
		t.Fatalf("duration = %d, want 20000", repository.completeInput.Result.DurationMS())
	}
}

func TestSubmitCheckResultRejectsMalformedIDAndInvalidResultBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name  string
		id    string
		input SubmitCheckResultInput
		want  error
	}{
		{
			name:  "invalid id",
			id:    "not-a-uuid",
			input: SubmitCheckResultInput{Kind: domain.CheckResultTimeout, DurationMS: 1},
			want:  domain.ErrInvalidCheckID,
		},
		{
			name:  "duration too high",
			id:    "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
			input: SubmitCheckResultInput{Kind: domain.CheckResultTimeout, DurationMS: 20001},
			want:  domain.ErrInvalidCheckResult,
		},
		{
			name:  "failure with status",
			id:    "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
			input: SubmitCheckResultInput{Kind: domain.CheckResultTimeout, DurationMS: 1, HTTPStatus: intPointer(500)},
			want:  domain.ErrInvalidCheckResult,
		},
		{
			name:  "HTTP response missing status",
			id:    "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
			input: SubmitCheckResultInput{Kind: domain.CheckResultHTTPResponse, DurationMS: 1},
			want:  domain.ErrInvalidCheckResult,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeCheckExecutionRepository{}
			clockCalls := 0
			useCase := NewSubmitCheckResult(repository, func() time.Time {
				clockCalls++
				return time.Now()
			})
			err := useCase.Execute(context.Background(), test.id, test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if repository.completeCalls != 0 || clockCalls != 0 {
				t.Fatalf("invalid input touched dependencies: complete=%d clocks=%d", repository.completeCalls, clockCalls)
			}
		})
	}
}

func TestSubmitCheckResultMapsRepositoryOutcomes(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "unknown", err: ports.ErrCheckRunNotFound, want: ErrCheckRunNotFound},
		{name: "conflict", err: ports.ErrCheckRunConflict, want: ErrCheckRunConflict},
		{name: "persistence", err: errors.New("postgres secret"), want: ErrPersistence},
		{name: "exact duplicate success", err: nil, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeCheckExecutionRepository{completeErr: test.err}
			useCase := NewSubmitCheckResult(repository, func() time.Time {
				return time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC)
			})
			err := useCase.Execute(
				context.Background(),
				"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
				SubmitCheckResultInput{Kind: domain.CheckResultTimeout, DurationMS: 10},
			)
			if test.want == nil {
				if err != nil {
					t.Fatalf("Execute() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("Execute() error = %v, want %v", err, test.want)
			}
			if err.Error() != test.want.Error() {
				t.Fatalf("error = %q, want stable %q", err, test.want)
			}
		})
	}
}

func intPointer(value int) *int {
	return &value
}

var _ ports.CheckExecutionRepository = (*fakeCheckExecutionRepository)(nil)
