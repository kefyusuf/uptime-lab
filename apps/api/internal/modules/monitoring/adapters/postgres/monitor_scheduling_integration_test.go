//go:build integration

package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"testing"
	"time"
)

func TestSchedulingAtomicWritesAndRestartRead(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", time.Now().UTC())
	if got, err := repo.GetScheduling(ctx, id); err != nil || got != domain.SchedulingActive {
		t.Fatal(got, err)
	}
	for range 2 {
		if got, err := repo.SetScheduling(ctx, id, domain.SchedulingPaused); err != nil || got != domain.SchedulingPaused {
			t.Fatal(got, err)
		}
	}
	fresh, err := pgxpool.NewWithConfig(ctx, pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if got, err := NewRepository(fresh).GetScheduling(ctx, id); err != nil || got != domain.SchedulingPaused {
		t.Fatal(got, err)
	}
	missing := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd009")
	if _, err := repo.SetScheduling(ctx, missing, domain.SchedulingActive); !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.GetScheduling(ctx, missing); !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.SetScheduling(ctx, id, ""); !errors.Is(err, domain.ErrInvalidSchedulingState) {
		t.Fatal(err)
	}
	if got, err := repo.GetScheduling(ctx, id); err != nil || got != domain.SchedulingPaused {
		t.Fatal(got, err)
	}
}

func TestPausePreventsNewClaim(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", now)
	if _, err := repo.SetScheduling(ctx, id, domain.SchedulingPaused); err != nil {
		t.Fatal(err)
	}
	input := ports.ClaimDueCheckInput{CheckID: mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"), Now: now, DueBefore: now.Add(-time.Minute), Deadline: now.Add(20 * time.Second)}
	if _, err := repo.ClaimDueCheck(ctx, input); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("paused Monitor claimed: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitoring.check_runs`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestPauseLockAllowsAnotherMonitorClaim(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	first := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	second := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd002")
	insertExecutionMonitor(t, pool, first, "https://example.com/first", now.Add(-time.Hour))
	insertExecutionMonitor(t, pool, second, "https://example.com/second", now)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `UPDATE monitoring.monitors SET paused=true WHERE id=$1`, first.String()); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ClaimDueCheck(ctx, schedulingClaimInput(t, now, 101))
	if err != nil || got.MonitorID != second {
		t.Fatal(got, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimDueCheck(ctx, schedulingClaimInput(t, now, 102)); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatal(err)
	}
}

func TestResumeKeepsCadenceAndPendingGuard(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", now)
	if _, err := repo.SetScheduling(ctx, id, domain.SchedulingPaused); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetScheduling(ctx, id, domain.SchedulingActive); err != nil {
		t.Fatal(err)
	}
	input := schedulingClaimInput(t, now, 101)
	if _, err := repo.ClaimDueCheck(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetScheduling(ctx, id, domain.SchedulingPaused); err != nil {
		t.Fatal(err)
	}
	result, err := domain.NewHTTPResponseResult(5, 200)
	if err != nil {
		t.Fatal(err)
	}
	completion := ports.CompleteCheckInput{CheckID: input.CheckID, CompletedAt: now.Add(10 * time.Second), Result: result}
	for range 2 {
		if err = repo.CompleteCheck(ctx, completion); err != nil {
			t.Fatalf("paused completion/replay: %v", err)
		}
	}
	if _, err = repo.SetScheduling(ctx, id, domain.SchedulingActive); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimDueCheck(ctx, schedulingClaimInput(t, now.Add(70*time.Second-time.Microsecond), 102)); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatal(err)
	}
	if _, err = repo.ClaimDueCheck(ctx, schedulingClaimInput(t, now.Add(70*time.Second), 102)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimDueCheck(ctx, schedulingClaimInput(t, now.Add(71*time.Second), 103)); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatal(err)
	}
}

func TestPausedCompletionAndTimeoutReconciliation(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx := context.Background()
	repo := NewRepository(pool)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	first := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	second := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd002")
	insertExecutionMonitor(t, pool, first, "https://example.com/first", now)
	input := schedulingClaimInput(t, now, 101)
	if _, err := repo.ClaimDueCheck(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetScheduling(ctx, first, domain.SchedulingPaused); err != nil {
		t.Fatal(err)
	}
	insertExecutionMonitor(t, pool, second, "https://example.com/second", now.Add(time.Second))
	got, err := repo.ClaimDueCheck(ctx, schedulingClaimInput(t, input.Deadline, 102))
	if err != nil || got.MonitorID != second {
		t.Fatal(got, err)
	}
	latest, err := repo.LatestTerminalByMonitorID(ctx, first)
	if err != nil || latest.ResultKind != domain.CheckResultWorkerTimeout || !latest.CompletedAt.Equal(input.Deadline) {
		t.Fatal(latest, err)
	}
	result, err := domain.NewHTTPResponseResult(5, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteCheck(ctx, ports.CompleteCheckInput{CheckID: input.CheckID, CompletedAt: input.Deadline.Add(time.Second), Result: result}); !errors.Is(err, ports.ErrCheckRunConflict) {
		t.Fatal(err)
	}
}
