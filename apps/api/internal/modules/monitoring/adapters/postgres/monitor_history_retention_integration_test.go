//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func insertRetentionRows(
	t *testing.T,
	pool *pgxpool.Pool,
	monitorID domain.MonitorID,
	count int,
	base time.Time,
) []domain.CheckID {
	t.Helper()
	ids := make([]domain.CheckID, 0, count)
	for index := 0; index < count; index++ {
		checkID := historyCheckID(t, index)
		ids = append(ids, checkID)
		completedAt := base.Add(time.Duration(index) * time.Second)
		insertTerminalRun(t, pool, checkID, monitorID, completedAt.Add(-20*time.Second), completedAt, "timeout", nil, int64Ptr(1))
	}
	return ids
}

func countTerminalRows(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.check_runs WHERE monitor_id = $1::uuid AND completed_at IS NOT NULL",
		monitorID.String(),
	).Scan(&count); err != nil {
		t.Fatalf("count terminal rows: %v", err)
	}
	return count
}

func claimForRetention(t *testing.T, repository *Repository, now time.Time, base time.Time) error {
	t.Helper()
	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c801"),
		Now:       now,
		DueBefore: base,
		Deadline:  now.Add(20 * time.Second),
	})
	return err
}

func TestRetentionConvergesToFiftyNewest(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd501")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	ids := insertRetentionRows(t, pool, monitorID, 55, base)

	if err := claimForRetention(t, repository, now, base); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows after retention = %d, want 50", got)
	}
	for _, removed := range ids[:5] {
		var exists bool
		if err := pool.QueryRow(
			context.Background(),
			"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
			removed.String(),
		).Scan(&exists); err != nil {
			t.Fatalf("query removed row: %v", err)
		}
		if exists {
			t.Fatalf("oldest row %s survived retention", removed)
		}
	}
	for _, kept := range ids[5:] {
		var exists bool
		if err := pool.QueryRow(
			context.Background(),
			"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
			kept.String(),
		).Scan(&exists); err != nil {
			t.Fatalf("query kept row: %v", err)
		}
		if !exists {
			t.Fatalf("kept row %s was removed", kept)
		}
	}
}

func TestRetentionPreservesPendingRows(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd502")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention-pending", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	insertRetentionRows(t, pool, monitorID, 55, base)
	pending := historyCheckID(t, 90)
	insertPendingRun(t, pool, pending, monitorID, now.Add(-5*time.Second), now.Add(15*time.Second))

	if err := claimForRetention(t, repository, now, base); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows after retention = %d, want 50", got)
	}
	var pendingExists bool
	if err := pool.QueryRow(
		context.Background(),
		"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid AND completed_at IS NULL)",
		pending.String(),
	).Scan(&pendingExists); err != nil {
		t.Fatalf("query pending row: %v", err)
	}
	if !pendingExists {
		t.Fatal("pending row did not survive retention")
	}
}

func TestRetentionTerminalizesExpiredPendingThenRetains(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd503")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention-expired", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	insertRetentionRows(t, pool, monitorID, 54, base)
	expired := historyCheckID(t, 91)
	insertPendingRun(t, pool, expired, monitorID, now.Add(-40*time.Second), now.Add(-10*time.Second))

	if err := claimForRetention(t, repository, now, base); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows after reconciliation and retention = %d, want 50", got)
	}
	var expiredExists bool
	if err := pool.QueryRow(
		context.Background(),
		"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
		expired.String(),
	).Scan(&expiredExists); err != nil {
		t.Fatalf("query expired row: %v", err)
	}
	if !expiredExists {
		t.Fatal("terminalized expired row should survive as the newest terminal row (rank 1)")
	}
	var oldestBaseGone bool
	if err := pool.QueryRow(
		context.Background(),
		"SELECT NOT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
		mustExecutionCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c000").String(),
	).Scan(&oldestBaseGone); err != nil {
		t.Fatalf("query oldest base row: %v", err)
	}
	if !oldestBaseGone {
		t.Fatal("oldest base row should have been removed by retention")
	}
}

func TestRetentionPartitionedByMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 30, 0, 0, time.UTC)
	overCap := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd504")
	underCap := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd505")
	insertExecutionMonitor(t, pool, overCap, "https://example.com/retention-over", now.Add(-2*time.Hour))
	insertExecutionMonitor(t, pool, underCap, "https://example.com/retention-under", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	insertRetentionRows(t, pool, overCap, 55, base)
	underCapIDs := make([]domain.CheckID, 0, 2)
	for index := 0; index < 2; index++ {
		checkID := historyCheckID(t, 200+index)
		underCapIDs = append(underCapIDs, checkID)
		insertTerminalRun(t, pool, checkID, underCap, base.Add(-20*time.Second), base.Add(time.Duration(index)*time.Second), "timeout", nil, int64Ptr(1))
	}

	if err := claimForRetention(t, repository, now, base); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, overCap); got != 50 {
		t.Fatalf("over-cap Monitor rows = %d, want 50", got)
	}
	entries, err := repository.Page(context.Background(), underCap, nil, 50)
	if err != nil {
		t.Fatalf("under-cap Page() error = %v", err)
	}
	assertHistoryOrder(t, entries, []domain.CheckID{underCapIDs[1], underCapIDs[0]})
}

func TestRetentionNeverRemovesLatestTerminalRow(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 40, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd506")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention-latest", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	insertRetentionRows(t, pool, monitorID, 50, base)

	if err := claimForRetention(t, repository, now, base); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows = %d, want 50", got)
	}
	latest := historyCheckID(t, 49)
	var exists bool
	if err := pool.QueryRow(
		context.Background(),
		"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
		latest.String(),
	).Scan(&exists); err != nil {
		t.Fatalf("query latest row: %v", err)
	}
	if !exists {
		t.Fatal("latest terminal row was removed")
	}
}

func TestRetentionTieBreaksEqualCompletionInstants(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 12, 50, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd507")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention-ties", now.Add(-2*time.Hour))
	instant := now.Add(-30 * time.Minute)
	ids := make([]domain.CheckID, 0, 55)
	for index := 0; index < 55; index++ {
		checkID := historyCheckID(t, index)
		ids = append(ids, checkID)
		insertTerminalRun(t, pool, checkID, monitorID, instant.Add(-time.Duration(index+1)*time.Minute), instant, "timeout", nil, int64Ptr(1))
	}

	if err := claimForRetention(t, repository, now, instant.Add(-time.Second)); !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows after retention = %d, want 50", got)
	}
	for _, removedID := range ids[:5] {
		var exists bool
		if err := pool.QueryRow(
			context.Background(),
			"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
			removedID.String(),
		).Scan(&exists); err != nil {
			t.Fatalf("query removed row: %v", err)
		}
		if exists {
			t.Fatalf("lexically smallest tied row %s survived", removedID)
		}
	}
	for _, keptID := range ids[5:] {
		var exists bool
		if err := pool.QueryRow(
			context.Background(),
			"SELECT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
			keptID.String(),
		).Scan(&exists); err != nil {
			t.Fatalf("query kept row: %v", err)
		}
		if !exists {
			t.Fatalf("tied kept row %s was removed", keptID)
		}
	}
}

func TestRetentionRacingCompletionResolvesWithoutDeadlock(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 7, 13, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd508")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/retention-race", now.Add(-2*time.Hour))
	base := now.Add(-time.Hour)
	insertRetentionRows(t, pool, monitorID, 55, base)
	parked := historyCheckID(t, 0)

	held, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin parking transaction: %v", err)
	}
	if _, err := held.Exec(
		context.Background(),
		"SELECT id FROM monitoring.check_runs WHERE id = $1::uuid FOR UPDATE",
		parked.String(),
	); err != nil {
		t.Fatalf("park over-cap row lock: %v", err)
	}

	claims := make(chan error, 1)
	go func() {
		claims <- claimForRetention(t, repository, now, base)
	}()
	waitForTransactionLockWait(t, pool)

	result, buildErr := domain.NewCheckFailureResult(domain.CheckResultTimeout, 1)
	if buildErr != nil {
		t.Fatalf("build result: %v", buildErr)
	}
	completions := make(chan error, 1)
	go func() {
		completions <- repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
			CheckID:     parked,
			CompletedAt: time.Now(),
			Result:      result,
		})
	}()
	waitForTransactionLockWait(t, pool)

	if err := held.Commit(context.Background()); err != nil {
		t.Fatalf("release parked lock: %v", err)
	}

	select {
	case err := <-claims:
		if !errors.Is(err, ports.ErrNoDueCheck) {
			t.Fatalf("racing ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("racing ClaimDueCheck() did not finish after release")
	}

	select {
	case err := <-completions:
		if err != nil && !errors.Is(err, ports.ErrCheckRunNotFound) && !errors.Is(err, ports.ErrCheckRunConflict) {
			t.Fatalf("racing CompleteCheck() error = %v, want nil, ErrCheckRunNotFound or ErrCheckRunConflict", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("racing CompleteCheck() did not finish after release")
	}

	if got := countTerminalRows(t, pool, monitorID); got != 50 {
		t.Fatalf("terminal rows after retention = %d, want 50", got)
	}
	var parkedGone bool
	if err := pool.QueryRow(
		context.Background(),
		"SELECT NOT EXISTS (SELECT 1 FROM monitoring.check_runs WHERE id = $1::uuid)",
		parked.String(),
	).Scan(&parkedGone); err != nil {
		t.Fatalf("query parked row: %v", err)
	}
	if !parkedGone {
		t.Fatal("parked over-cap row survived retention")
	}
}
