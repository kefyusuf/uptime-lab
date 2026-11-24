//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
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

func mustIncidentID(t *testing.T, raw string) domain.IncidentID {
	t.Helper()
	id, err := domain.ParseIncidentID(raw)
	if err != nil {
		t.Fatalf("ParseIncidentID(%q) error = %v", raw, err)
	}
	return id
}

func incidentFixtureCheckID(t *testing.T, index int) domain.CheckID {
	t.Helper()
	return mustExecutionCheckID(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3fafe%03d", index))
}

func incidentFixtureCandidateID(t *testing.T, index int) domain.IncidentID {
	t.Helper()
	return mustIncidentID(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3faff%03d", index))
}

func incidentFailureResult(t *testing.T) domain.CheckResult {
	t.Helper()
	result, err := domain.NewHTTPResponseResult(10, 500)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult(500) error = %v", err)
	}
	return result
}

func incidentSuccessResult(t *testing.T) domain.CheckResult {
	t.Helper()
	result, err := domain.NewHTTPResponseResult(10, 200)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult(200) error = %v", err)
	}
	return result
}

func completePendingOutcome(
	t *testing.T,
	pool *pgxpool.Pool,
	repository *Repository,
	monitorID domain.MonitorID,
	checkID domain.CheckID,
	candidate domain.IncidentID,
	completedAt time.Time,
	result domain.CheckResult,
) {
	t.Helper()
	insertPendingRun(t, pool, checkID, monitorID, completedAt.Add(-time.Second), completedAt.Add(19*time.Second))
	if err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     checkID,
		CompletedAt: completedAt,
		Result:      result,
		IncidentID:  candidate,
	}); err != nil {
		t.Fatalf("CompleteCheck(%s) error = %v", checkID, err)
	}
}

func openIncidentViaFailures(
	t *testing.T,
	pool *pgxpool.Pool,
	repository *Repository,
	monitorID domain.MonitorID,
	base time.Time,
	startIndex int,
) (domain.IncidentID, domain.CheckID, time.Time) {
	t.Helper()
	var candidate domain.IncidentID
	var checkID domain.CheckID
	var openedAt time.Time
	for index := 0; index < 3; index++ {
		openedAt = base.Add(time.Duration(index) * time.Minute)
		checkID = incidentFixtureCheckID(t, startIndex+index)
		candidate = incidentFixtureCandidateID(t, startIndex+index)
		completePendingOutcome(t, pool, repository, monitorID, checkID, candidate, openedAt, incidentFailureResult(t))
	}
	return candidate, checkID, openedAt
}

func seedIncidentState(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID, failureStreak, successStreak int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO monitoring.incident_state (monitor_id, failure_streak, success_streak, open_incident_id)
		 VALUES ($1::uuid, $2, $3, NULL)`,
		monitorID.String(), failureStreak, successStreak); err != nil {
		t.Fatalf("seed incident state: %v", err)
	}
}

func assertIncidentState(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID, wantFailure, wantSuccess int, wantOpen *domain.IncidentID) {
	t.Helper()
	var failure, success int
	var rawOpen *string
	if err := pool.QueryRow(context.Background(),
		`SELECT failure_streak, success_streak, open_incident_id::text
		 FROM monitoring.incident_state WHERE monitor_id = $1::uuid`,
		monitorID.String()).Scan(&failure, &success, &rawOpen); err != nil {
		t.Fatalf("read incident state: %v", err)
	}
	if failure != wantFailure || success != wantSuccess {
		t.Fatalf("streaks = (failure %d, success %d), want (%d, %d)", failure, success, wantFailure, wantSuccess)
	}
	switch {
	case wantOpen == nil && rawOpen != nil:
		t.Fatalf("open incident = %s, want nil", *rawOpen)
	case wantOpen != nil && rawOpen == nil:
		t.Fatalf("open incident = nil, want %s", wantOpen)
	case wantOpen != nil && *rawOpen != wantOpen.String():
		t.Fatalf("open incident = %s, want %s", *rawOpen, wantOpen)
	}
}

func assertNoIncidentState(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM monitoring.incident_state WHERE monitor_id = $1::uuid)`,
		monitorID.String()).Scan(&exists); err != nil {
		t.Fatalf("probe incident state: %v", err)
	}
	if exists {
		t.Fatal("incident state row exists, want lazy absence")
	}
}

func countIncidents(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM monitoring.incidents WHERE monitor_id = $1::uuid`,
		monitorID.String()).Scan(&count); err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	return count
}

func countOpenIncidents(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM monitoring.incidents WHERE monitor_id = $1::uuid AND resolved_at IS NULL`,
		monitorID.String()).Scan(&count); err != nil {
		t.Fatalf("count open incidents: %v", err)
	}
	return count
}

func equalOptionalTime(got, want *time.Time) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return got.Equal(*want)
}

func equalOptionalCheckID(got *string, want *domain.CheckID) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == want.String()
}

func assertIncidentWindow(
	t *testing.T,
	pool *pgxpool.Pool,
	incidentID domain.IncidentID,
	monitorID domain.MonitorID,
	wantOpenedAt time.Time,
	wantResolvedAt *time.Time,
	wantReason string,
	wantOpenedBy, wantResolvedBy *domain.CheckID,
) {
	t.Helper()
	var rawMonitor string
	var openedAt time.Time
	var resolvedAt *time.Time
	var reason string
	var openedBy, resolvedBy *string
	if err := pool.QueryRow(context.Background(),
		`SELECT monitor_id::text, opened_at, resolved_at, opening_reason, opened_by_check_id::text, resolved_by_check_id::text
		 FROM monitoring.incidents WHERE id = $1::uuid`,
		incidentID.String()).Scan(&rawMonitor, &openedAt, &resolvedAt, &reason, &openedBy, &resolvedBy); err != nil {
		t.Fatalf("read incident window %s: %v", incidentID, err)
	}
	if rawMonitor != monitorID.String() {
		t.Fatalf("incident monitor = %s, want %s", rawMonitor, monitorID)
	}
	if !openedAt.Equal(wantOpenedAt) {
		t.Fatalf("opened_at = %v, want %v", openedAt, wantOpenedAt)
	}
	if reason != wantReason {
		t.Fatalf("opening_reason = %q, want %q", reason, wantReason)
	}
	if !equalOptionalTime(resolvedAt, wantResolvedAt) {
		t.Fatalf("resolved_at = %v, want %v", resolvedAt, wantResolvedAt)
	}
	if !equalOptionalCheckID(openedBy, wantOpenedBy) {
		t.Fatalf("opened_by_check_id = %v, want %v", openedBy, wantOpenedBy)
	}
	if !equalOptionalCheckID(resolvedBy, wantResolvedBy) {
		t.Fatalf("resolved_by_check_id = %v, want %v", resolvedBy, wantResolvedBy)
	}
}

func TestCompleteCheckFoldsOpenOnThirdFailure(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd601")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-open", now.Add(-time.Hour))
	assertNoIncidentState(t, pool, monitorID)

	var openCandidate domain.IncidentID
	var openCheckID domain.CheckID
	var openAt time.Time
	for index := 0; index < 3; index++ {
		at := now.Add(time.Duration(index) * time.Minute)
		checkID := incidentFixtureCheckID(t, index)
		candidate := incidentFixtureCandidateID(t, index)
		completePendingOutcome(t, pool, repository, monitorID, checkID, candidate, at, incidentFailureResult(t))
		if index < 2 {
			assertIncidentState(t, pool, monitorID, index+1, 0, nil)
			if got := countIncidents(t, pool, monitorID); got != 0 {
				t.Fatalf("incidents after %d failures = %d, want 0", index+1, got)
			}
			continue
		}
		openCandidate, openCheckID, openAt = candidate, checkID, at
	}

	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)
	if got := countOpenIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("open incidents = %d, want 1", got)
	}
	assertIncidentWindow(t, pool, openCandidate, monitorID, openAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestCompleteCheckFoldsResolveOnSecondSuccess(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd602")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-resolve", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	firstSuccessAt := now.Add(3 * time.Minute)
	completePendingOutcome(t, pool, repository, monitorID, incidentFixtureCheckID(t, 3), incidentFixtureCandidateID(t, 3), firstSuccessAt, incidentSuccessResult(t))
	assertIncidentState(t, pool, monitorID, 0, 1, &openCandidate)
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, nil, "unexpected_http_status", &openCheckID, nil)

	secondSuccessAt := now.Add(4 * time.Minute)
	resolveCheckID := incidentFixtureCheckID(t, 4)
	completePendingOutcome(t, pool, repository, monitorID, resolveCheckID, incidentFixtureCandidateID(t, 4), secondSuccessAt, incidentSuccessResult(t))
	assertIncidentState(t, pool, monitorID, 0, 0, nil)
	if got := countOpenIncidents(t, pool, monitorID); got != 0 {
		t.Fatalf("open incidents after recovery = %d, want 0", got)
	}
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, &secondSuccessAt, "unexpected_http_status", &openCheckID, &resolveCheckID)
}

func TestCompleteCheckLateTimeoutInconclusiveReset(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd603")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-late", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	lateCheckID := incidentFixtureCheckID(t, 10)
	deadline := now.Add(3*time.Minute - time.Second)
	insertPendingRun(t, pool, lateCheckID, monitorID, now.Add(2*time.Minute), deadline)
	err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     lateCheckID,
		CompletedAt: now.Add(3 * time.Minute),
		Result:      incidentFailureResult(t),
		IncidentID:  incidentFixtureCandidateID(t, 10),
	})
	if !errors.Is(err, ports.ErrCheckRunConflict) {
		t.Fatalf("late CompleteCheck() error = %v, want ErrCheckRunConflict", err)
	}

	assertWorkerTimeoutRun(t, pool, lateCheckID, deadline)
	assertIncidentState(t, pool, monitorID, 0, 0, &openCandidate)
	if got := countOpenIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("inconclusive reset resolved the open incident: open=%d, want 1", got)
	}
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestClaimReconciliationResetsFold(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd604")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-reconcile", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	claimNow := now.Add(3 * time.Minute)
	expiredCheckID := incidentFixtureCheckID(t, 20)
	expiredDeadline := claimNow.Add(-10 * time.Second)
	insertPendingRun(t, pool, expiredCheckID, monitorID, expiredDeadline.Add(-20*time.Second), expiredDeadline)

	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   incidentFixtureCheckID(t, 21),
		Now:       claimNow,
		DueBefore: claimNow.Add(-60 * time.Second),
		Deadline:  claimNow.Add(20 * time.Second),
	})
	if err != nil && !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v", err)
	}

	assertWorkerTimeoutRun(t, pool, expiredCheckID, expiredDeadline)
	assertIncidentState(t, pool, monitorID, 0, 0, &openCandidate)
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestCompleteCheckDuplicateDoesNotRefold(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 40, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd605")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-duplicate", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     openCheckID,
		CompletedAt: openedAt.Add(5 * time.Second),
		Result:      incidentFailureResult(t),
		IncidentID:  incidentFixtureCandidateID(t, 30),
	})
	if err != nil {
		t.Fatalf("duplicate CompleteCheck() error = %v", err)
	}

	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)
	if got := countIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("duplicate re-folded incidents = %d, want 1", got)
	}
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestCompleteCheckConflictDoesNotFold(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 15, 50, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd606")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-conflict", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	conflicting, err := domain.NewHTTPResponseResult(11, 500)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult() error = %v", err)
	}
	err = repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     openCheckID,
		CompletedAt: openedAt.Add(time.Second),
		Result:      conflicting,
		IncidentID:  incidentFixtureCandidateID(t, 31),
	})
	if !errors.Is(err, ports.ErrCheckRunConflict) {
		t.Fatalf("conflicting CompleteCheck() error = %v, want ErrCheckRunConflict", err)
	}
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)
	if got := countIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("conflict folded incidents = %d, want 1", got)
	}

	streakMonitor := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd607")
	insertExecutionMonitor(t, pool, streakMonitor, "https://example.com/incident-streak-conflict", now.Add(-time.Hour))
	streakCheck := incidentFixtureCheckID(t, 40)
	completePendingOutcome(t, pool, repository, streakMonitor, streakCheck, incidentFixtureCandidateID(t, 40), now, incidentFailureResult(t))
	assertIncidentState(t, pool, streakMonitor, 1, 0, nil)

	streakConflict, _ := domain.NewHTTPResponseResult(12, 500)
	err = repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     streakCheck,
		CompletedAt: now.Add(time.Second),
		Result:      streakConflict,
		IncidentID:  incidentFixtureCandidateID(t, 41),
	})
	if !errors.Is(err, ports.ErrCheckRunConflict) {
		t.Fatalf("streak conflicting CompleteCheck() error = %v, want ErrCheckRunConflict", err)
	}
	assertIncidentState(t, pool, streakMonitor, 1, 0, nil)
	if got := countIncidents(t, pool, streakMonitor); got != 0 {
		t.Fatalf("streak conflict folded incidents = %d, want 0", got)
	}
}

func TestEditLeavesIncidentStateUntouched(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 16, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd608")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-edit", now.Add(-time.Hour))

	completePendingOutcome(t, pool, repository, monitorID, incidentFixtureCheckID(t, 0), incidentFixtureCandidateID(t, 0), now, incidentFailureResult(t))
	completePendingOutcome(t, pool, repository, monitorID, incidentFixtureCheckID(t, 1), incidentFixtureCandidateID(t, 1), now.Add(time.Minute), incidentFailureResult(t))
	assertIncidentState(t, pool, monitorID, 2, 0, nil)

	editedTarget, err := domain.NewTargetURL("https://example.com/incident-edited")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	if _, err := repository.EditTarget(context.Background(), monitorID, editedTarget); err != nil {
		t.Fatalf("EditTarget() error = %v", err)
	}
	assertIncidentState(t, pool, monitorID, 2, 0, nil)
	if got := countIncidents(t, pool, monitorID); got != 0 {
		t.Fatalf("edit fabricated incidents = %d, want 0", got)
	}

	openCandidate := incidentFixtureCandidateID(t, 2)
	openCheckID := incidentFixtureCheckID(t, 2)
	openAt := now.Add(2 * time.Minute)
	completePendingOutcome(t, pool, repository, monitorID, openCheckID, openCandidate, openAt, incidentFailureResult(t))
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	secondTarget, _ := domain.NewTargetURL("https://example.com/incident-edited-again")
	if _, err := repository.EditTarget(context.Background(), monitorID, secondTarget); err != nil {
		t.Fatalf("second EditTarget() error = %v", err)
	}
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)
	assertIncidentWindow(t, pool, openCandidate, monitorID, openAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestPauseFreezesFold(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 16, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd609")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-pause", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	if _, err := repository.SetScheduling(context.Background(), monitorID, domain.SchedulingPaused); err != nil {
		t.Fatalf("SetScheduling(paused) error = %v", err)
	}
	claimNow := now.Add(10 * time.Minute)
	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   incidentFixtureCheckID(t, 20),
		Now:       claimNow,
		DueBefore: claimNow.Add(-60 * time.Second),
		Deadline:  claimNow.Add(20 * time.Second),
	})
	if !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("paused ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)
	if got := countOpenIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("pause changed open incidents = %d, want 1", got)
	}
	assertIncidentWindow(t, pool, openCandidate, monitorID, openedAt, nil, "unexpected_http_status", &openCheckID, nil)
}

func TestIncidentDurabilityAcrossRestart(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 16, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd60a")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-restart", now.Add(-time.Hour))

	openCandidate, openCheckID, openedAt := openIncidentViaFailures(t, pool, repository, monitorID, now, 0)
	assertIncidentState(t, pool, monitorID, 3, 0, &openCandidate)

	restartPool := openIntegrationPool(t)
	restartRepository := NewRepository(restartPool)
	assertIncidentState(t, restartPool, monitorID, 3, 0, &openCandidate)

	completePendingOutcome(t, restartPool, restartRepository, monitorID, incidentFixtureCheckID(t, 3), incidentFixtureCandidateID(t, 3), now.Add(3*time.Minute), incidentSuccessResult(t))
	assertIncidentState(t, restartPool, monitorID, 0, 1, &openCandidate)

	resolveCheckID := incidentFixtureCheckID(t, 4)
	resolveAt := now.Add(4 * time.Minute)
	completePendingOutcome(t, restartPool, restartRepository, monitorID, resolveCheckID, incidentFixtureCandidateID(t, 4), resolveAt, incidentSuccessResult(t))
	assertIncidentState(t, restartPool, monitorID, 0, 0, nil)
	assertIncidentWindow(t, restartPool, openCandidate, monitorID, openedAt, &resolveAt, "unexpected_http_status", &openCheckID, &resolveCheckID)
}

func TestLazyZeroedInitNoBackfill(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 26, 16, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd60b")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-lazy", now.Add(-2*time.Hour))

	for index := 0; index < 5; index++ {
		insertTerminalFailureRun(
			t,
			pool,
			incidentFixtureCheckID(t, 40+index),
			monitorID,
			now.Add(-time.Hour),
			now.Add(-time.Hour+time.Duration(index)*time.Second),
		)
	}
	assertNoIncidentState(t, pool, monitorID)
	if got := countIncidents(t, pool, monitorID); got != 0 {
		t.Fatalf("migration backfilled incidents = %d, want 0", got)
	}

	completePendingOutcome(t, pool, repository, monitorID, incidentFixtureCheckID(t, 50), incidentFixtureCandidateID(t, 50), now, incidentFailureResult(t))
	assertIncidentState(t, pool, monitorID, 1, 0, nil)
	if got := countIncidents(t, pool, monitorID); got != 0 {
		t.Fatalf("lazy init replayed retained evidence: incidents = %d, want 0", got)
	}
}
