//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func TestRepositoryClaimDueCheckImmediateMonitorWithoutHistory(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/immediate", now)

	checkID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd101")
	got, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   checkID,
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimDueCheck() error = %v", err)
	}
	if got.MonitorID != monitorID {
		t.Fatalf("MonitorID = %s, want %s", got.MonitorID, monitorID)
	}
	if got.TargetURL.String() != "https://example.com/immediate" {
		t.Fatalf("TargetURL = %q", got.TargetURL)
	}

	assertPendingCheckRun(t, pool, checkID, monitorID, now, now.Add(20*time.Second))
}

func TestRepositoryClaimDueCheckOrderingAndEligibility(t *testing.T) {
	t.Run("never checked orders by monitor created_at", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 12, 10, 0, 0, time.UTC)
		older := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd011")
		newer := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd012")
		insertExecutionMonitor(t, pool, newer, "https://example.com/newer", now)
		insertExecutionMonitor(t, pool, older, "https://example.com/older", now.Add(-time.Minute))

		got, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd111"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimDueCheck() error = %v", err)
		}
		if got.MonitorID != older {
			t.Fatalf("claimed MonitorID = %s, want older %s", got.MonitorID, older)
		}
	})

	t.Run("latest terminal completion determines ordering", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 12, 20, 0, 0, time.UTC)
		first := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd021")
		second := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd022")
		insertExecutionMonitor(t, pool, first, "https://example.com/first", now.Add(-time.Hour))
		insertExecutionMonitor(t, pool, second, "https://example.com/second", now.Add(-2*time.Hour))
		insertTerminalFailureRun(
			t,
			pool,
			mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd121"),
			first,
			now.Add(-3*time.Minute),
			now.Add(-2*time.Minute),
		)
		insertTerminalFailureRun(
			t,
			pool,
			mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd122"),
			second,
			now.Add(-2*time.Minute),
			now.Add(-90*time.Second),
		)

		got, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd123"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimDueCheck() error = %v", err)
		}
		if got.MonitorID != first {
			t.Fatalf("claimed MonitorID = %s, want oldest terminal %s", got.MonitorID, first)
		}
	})

	t.Run("monitor is not due before completed_at plus cadence", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 12, 30, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd031")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/not-due", now.Add(-time.Hour))
		insertTerminalFailureRun(
			t,
			pool,
			mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd131"),
			monitorID,
			now.Add(-40*time.Second),
			now.Add(-30*time.Second),
		)

		_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd132"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		if !errors.Is(err, ports.ErrNoDueCheck) {
			t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
		}
	})

	t.Run("pending monitor is excluded", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 12, 40, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd041")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/pending", now.Add(-time.Hour))
		insertPendingRun(
			t,
			pool,
			mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd141"),
			monitorID,
			now.Add(-5*time.Second),
			now.Add(15*time.Second),
		)

		_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd142"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		if !errors.Is(err, ports.ErrNoDueCheck) {
			t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
		}
	})

	t.Run("equal scheduling point uses MonitorID tie break", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 12, 50, 0, 0, time.UTC)
		createdAt := now.Add(-time.Hour)
		lower := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd051")
		higher := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd052")
		insertExecutionMonitor(t, pool, higher, "https://example.com/higher", createdAt)
		insertExecutionMonitor(t, pool, lower, "https://example.com/lower", createdAt)

		got, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd151"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimDueCheck() error = %v", err)
		}
		if got.MonitorID != lower {
			t.Fatalf("claimed MonitorID = %s, want tie-break %s", got.MonitorID, lower)
		}
	})
}

func TestRepositoryClaimDueCheckReconcilesExpiredPendingBeforeNewWork(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 13, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd061")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/recovery", now.Add(-time.Hour))

	expiredID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd161")
	expiredDeadline := now.Add(-10 * time.Second)
	insertPendingRun(t, pool, expiredID, monitorID, now.Add(-30*time.Second), expiredDeadline)

	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd162"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("first ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	assertWorkerTimeoutRun(t, pool, expiredID, expiredDeadline)

	later := expiredDeadline.Add(60 * time.Second)
	newID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd163")
	got, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   newID,
		Now:       later,
		DueBefore: later.Add(-60 * time.Second),
		Deadline:  later.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("second ClaimDueCheck() error = %v", err)
	}
	if got.MonitorID != monitorID {
		t.Fatalf("claimed MonitorID = %s, want %s", got.MonitorID, monitorID)
	}
	assertPendingCheckRun(t, pool, newID, monitorID, later, later.Add(20*time.Second))
}

func TestRepositoryClaimDueCheckConcurrentAttemptsCreateOnePendingRun(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 13, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd071")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/concurrent", now.Add(-time.Hour))

	inputIDs := []domain.CheckID{
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd171"),
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd172"),
	}

	start := make(chan struct{})
	results := make(chan error, len(inputIDs))
	var wait sync.WaitGroup
	for _, checkID := range inputIDs {
		wait.Add(1)
		go func(id domain.CheckID) {
			defer wait.Done()
			<-start
			_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
				CheckID:   id,
				Now:       now,
				DueBefore: now.Add(-60 * time.Second),
				Deadline:  now.Add(20 * time.Second),
			})
			results <- err
		}(checkID)
	}

	close(start)
	wait.Wait()
	close(results)

	successes := 0
	noWork := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ports.ErrNoDueCheck):
			noWork++
		default:
			t.Fatalf("concurrent ClaimDueCheck() unexpected error = %v", err)
		}
	}
	if successes != 1 || noWork != 1 {
		t.Fatalf("concurrent outcomes success=%d noWork=%d, want 1/1", successes, noWork)
	}

	var pendingCount int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.check_runs WHERE monitor_id = $1::uuid AND completed_at IS NULL",
		monitorID.String(),
	).Scan(&pendingCount); err != nil {
		t.Fatalf("count pending rows: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("pending rows = %d, want 1", pendingCount)
	}
}

func TestRepositoryClaimDueCheckIndependentMonitorsReceiveIndependentWork(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 13, 20, 0, 0, time.UTC)
	first := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd081")
	second := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd082")
	insertExecutionMonitor(t, pool, first, "https://example.com/one", now.Add(-2*time.Hour))
	insertExecutionMonitor(t, pool, second, "https://example.com/two", now.Add(-time.Hour))

	gotFirst, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd181"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("first ClaimDueCheck() error = %v", err)
	}
	gotSecond, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd182"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("second ClaimDueCheck() error = %v", err)
	}

	if gotFirst.MonitorID != first || gotSecond.MonitorID != second {
		t.Fatalf(
			"claimed monitors = (%s,%s), want (%s,%s)",
			gotFirst.MonitorID,
			gotSecond.MonitorID,
			first,
			second,
		)
	}
}

func TestRepositoryCompleteCheckResultSemantics(t *testing.T) {
	t.Run("first completion persists normalized result", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 13, 30, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd091")
		checkID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd191")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/result", now.Add(-time.Hour))
		insertPendingRun(t, pool, checkID, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		result, err := domain.NewHTTPResponseResult(125, 204)
		if err != nil {
			t.Fatalf("NewHTTPResponseResult() error = %v", err)
		}
		if err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     checkID,
			CompletedAt: now,
			Result:      result,
		}); err != nil {
			t.Fatalf("CompleteCheck() error = %v", err)
		}

		assertTerminalResult(t, pool, checkID, now, "http_response", executionIntPointer(204), executionIntPointer(125))
	})

	t.Run("exact duplicate is no-op and preserves completed_at", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 13, 40, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd092")
		checkID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd192")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/duplicate", now.Add(-time.Hour))
		insertPendingRun(t, pool, checkID, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		result, err := domain.NewCheckFailureResult(domain.CheckResultTimeout, 10000)
		if err != nil {
			t.Fatalf("NewCheckFailureResult() error = %v", err)
		}
		first := ports.CompleteCheckInput{CheckID: checkID, CompletedAt: now, Result: result}
		if err := repository.CompleteCheck(context.Background(), first); err != nil {
			t.Fatalf("first CompleteCheck() error = %v", err)
		}

		duplicate := first
		duplicate.CompletedAt = now.Add(5 * time.Second)
		if err := repository.CompleteCheck(context.Background(), duplicate); err != nil {
			t.Fatalf("duplicate CompleteCheck() error = %v", err)
		}

		assertTerminalResult(t, pool, checkID, now, "timeout", nil, executionIntPointer(10000))
	})

	t.Run("conflicting duplicate is rejected without mutation", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 13, 50, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd093")
		checkID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd193")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/conflict", now.Add(-time.Hour))
		insertPendingRun(t, pool, checkID, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		firstResult, _ := domain.NewCheckFailureResult(domain.CheckResultTimeout, 100)
		if err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     checkID,
			CompletedAt: now,
			Result:      firstResult,
		}); err != nil {
			t.Fatalf("first CompleteCheck() error = %v", err)
		}

		differentResult, _ := domain.NewCheckFailureResult(domain.CheckResultTimeout, 101)
		err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     checkID,
			CompletedAt: now.Add(time.Second),
			Result:      differentResult,
		})
		if !errors.Is(err, ports.ErrCheckRunConflict) {
			t.Fatalf("conflicting CompleteCheck() error = %v, want ErrCheckRunConflict", err)
		}

		assertTerminalResult(t, pool, checkID, now, "timeout", nil, executionIntPointer(100))
	})

	t.Run("late result terminalizes worker timeout at deadline and conflicts", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		now := time.Date(2026, time.September, 26, 14, 0, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd094")
		checkID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd194")
		deadline := now.Add(-time.Second)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/late", now.Add(-time.Hour))
		insertPendingRun(t, pool, checkID, monitorID, now.Add(-21*time.Second), deadline)

		result, _ := domain.NewCheckFailureResult(domain.CheckResultTimeout, 10000)
		err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     checkID,
			CompletedAt: now,
			Result:      result,
		})
		if !errors.Is(err, ports.ErrCheckRunConflict) {
			t.Fatalf("late CompleteCheck() error = %v, want ErrCheckRunConflict", err)
		}

		assertWorkerTimeoutRun(t, pool, checkID, deadline)
	})

	t.Run("unknown CheckID returns not found", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		result, _ := domain.NewCheckFailureResult(domain.CheckResultTimeout, 1)
		err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd195"),
			CompletedAt: time.Date(2026, time.September, 26, 14, 10, 0, 0, time.UTC),
			Result:      result,
		})
		if !errors.Is(err, ports.ErrCheckRunNotFound) {
			t.Fatalf("CompleteCheck() error = %v, want ErrCheckRunNotFound", err)
		}
	})
}

func TestRepositoryCheckExecutionSurfacesPersistenceFailures(t *testing.T) {
	pool := openIntegrationPool(t)
	repository := NewRepository(pool)
	pool.Close()

	now := time.Date(2026, time.September, 26, 14, 15, 0, 0, time.UTC)
	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd1a0"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err == nil {
		t.Fatal("ClaimDueCheck() error = nil, want persistence failure")
	}
	if errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, must not map infrastructure failure to ErrNoDueCheck", err)
	}

	result, resultErr := domain.NewCheckFailureResult(domain.CheckResultTimeout, 1)
	if resultErr != nil {
		t.Fatalf("NewCheckFailureResult() error = %v", resultErr)
	}
	err = repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd1a3"),
		CompletedAt: now,
		Result:      result,
	})
	if err == nil {
		t.Fatal("CompleteCheck() error = nil, want persistence failure")
	}
	if errors.Is(err, ports.ErrCheckRunNotFound) || errors.Is(err, ports.ErrCheckRunConflict) {
		t.Fatalf("CompleteCheck() error = %v, must not map infrastructure failure to semantic result", err)
	}
}

func TestRepositoryCheckExecutionRespectsCancelledContext(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	now := time.Date(2026, time.September, 26, 14, 20, 0, 0, time.UTC)
	_, err := repository.ClaimDueCheck(ctx, ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd1a1"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ClaimDueCheck() error = %v, want context.Canceled", err)
	}

	result, _ := domain.NewCheckFailureResult(domain.CheckResultTimeout, 1)
	err = repository.CompleteCheck(ctx, ports.CompleteCheckInput{
		CheckID:     mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd1a2"),
		CompletedAt: now,
		Result:      result,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompleteCheck() error = %v, want context.Canceled", err)
	}
}

func insertExecutionMonitor(
	t *testing.T,
	pool *pgxpool.Pool,
	monitorID domain.MonitorID,
	targetURL string,
	createdAt time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(
		context.Background(),
		"INSERT INTO monitoring.monitors (id, target_url, created_at) VALUES ($1::uuid, $2, $3)",
		monitorID.String(),
		targetURL,
		createdAt,
	); err != nil {
		t.Fatalf("insert execution monitor: %v", err)
	}
}

func insertPendingRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	monitorID domain.MonitorID,
	issuedAt time.Time,
	deadlineAt time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(
		context.Background(),
		`INSERT INTO monitoring.check_runs (
			id, monitor_id, issued_at, deadline_at
		) VALUES ($1::uuid, $2::uuid, $3, $4)`,
		checkID.String(),
		monitorID.String(),
		issuedAt,
		deadlineAt,
	); err != nil {
		t.Fatalf("insert pending run: %v", err)
	}
}

func insertTerminalFailureRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	monitorID domain.MonitorID,
	issuedAt time.Time,
	completedAt time.Time,
) {
	t.Helper()
	if _, err := pool.Exec(
		context.Background(),
		`INSERT INTO monitoring.check_runs (
			id, monitor_id, issued_at, deadline_at, completed_at, result_kind, duration_ms
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, 'timeout', 1)`,
		checkID.String(),
		monitorID.String(),
		issuedAt,
		issuedAt.Add(20*time.Second),
		completedAt,
	); err != nil {
		t.Fatalf("insert terminal run: %v", err)
	}
}

func assertPendingCheckRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	monitorID domain.MonitorID,
	issuedAt time.Time,
	deadlineAt time.Time,
) {
	t.Helper()

	var (
		gotMonitorID string
		gotIssuedAt  time.Time
		gotDeadline  time.Time
		completedAt  *time.Time
		resultKind   *string
		httpStatus   *int
		durationMS   *int
	)
	if err := pool.QueryRow(
		context.Background(),
		`SELECT monitor_id::text, issued_at, deadline_at, completed_at, result_kind, http_status, duration_ms
		 FROM monitoring.check_runs
		 WHERE id = $1::uuid`,
		checkID.String(),
	).Scan(
		&gotMonitorID,
		&gotIssuedAt,
		&gotDeadline,
		&completedAt,
		&resultKind,
		&httpStatus,
		&durationMS,
	); err != nil {
		t.Fatalf("query pending check run: %v", err)
	}
	if gotMonitorID != monitorID.String() ||
		!gotIssuedAt.Equal(issuedAt) ||
		!gotDeadline.Equal(deadlineAt) ||
		completedAt != nil ||
		resultKind != nil ||
		httpStatus != nil ||
		durationMS != nil {
		t.Fatalf(
			"pending row = monitor=%s issued=%v deadline=%v completed=%v kind=%v status=%v duration=%v",
			gotMonitorID,
			gotIssuedAt,
			gotDeadline,
			completedAt,
			resultKind,
			httpStatus,
			durationMS,
		)
	}
}

func assertWorkerTimeoutRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	wantCompletedAt time.Time,
) {
	t.Helper()

	var (
		completedAt time.Time
		resultKind  string
		httpStatus  *int
		durationMS  *int
	)
	if err := pool.QueryRow(
		context.Background(),
		`SELECT completed_at, result_kind, http_status, duration_ms
		 FROM monitoring.check_runs
		 WHERE id = $1::uuid`,
		checkID.String(),
	).Scan(&completedAt, &resultKind, &httpStatus, &durationMS); err != nil {
		t.Fatalf("query worker timeout run: %v", err)
	}
	if !completedAt.Equal(wantCompletedAt) ||
		resultKind != "worker_timeout" ||
		httpStatus != nil ||
		durationMS != nil {
		t.Fatalf(
			"worker timeout = completed=%v kind=%q status=%v duration=%v",
			completedAt,
			resultKind,
			httpStatus,
			durationMS,
		)
	}
}

func assertTerminalResult(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	wantCompletedAt time.Time,
	wantKind string,
	wantStatus *int,
	wantDuration *int,
) {
	t.Helper()

	var (
		completedAt time.Time
		resultKind  string
		httpStatus  *int
		durationMS  *int
	)
	if err := pool.QueryRow(
		context.Background(),
		`SELECT completed_at, result_kind, http_status, duration_ms
		 FROM monitoring.check_runs
		 WHERE id = $1::uuid`,
		checkID.String(),
	).Scan(&completedAt, &resultKind, &httpStatus, &durationMS); err != nil {
		t.Fatalf("query terminal result: %v", err)
	}
	if !completedAt.Equal(wantCompletedAt) ||
		resultKind != wantKind ||
		!equalOptionalInt(httpStatus, wantStatus) ||
		!equalOptionalInt(durationMS, wantDuration) {
		t.Fatalf(
			"terminal result = completed=%v kind=%q status=%v duration=%v",
			completedAt,
			resultKind,
			httpStatus,
			durationMS,
		)
	}
}

func mustExecutionCheckID(t *testing.T, raw string) domain.CheckID {
	t.Helper()
	id, err := domain.ParseCheckID(raw)
	if err != nil {
		t.Fatalf("ParseCheckID(%q) error = %v", raw, err)
	}
	return id
}

func executionIntPointer(value int) *int {
	return &value
}

func equalOptionalInt(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
