//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func TestDeleteRemovesMonitorAndAllCheckRuns(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd201")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/delete-all", now.Add(-time.Hour))
	insertPendingRun(
		t,
		pool,
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd301"),
		monitorID,
		now.Add(-30*time.Second),
		now.Add(-10*time.Second),
	)
	insertTerminalFailureRun(
		t,
		pool,
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd302"),
		monitorID,
		now.Add(-10*time.Minute),
		now.Add(-9*time.Minute),
	)

	if err := repository.Delete(context.Background(), monitorID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)
}

func TestCompletedRunRemovalKeepsTerminalEvidence(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd211")
	removed := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd212")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/kept", now.Add(-time.Hour))
	insertExecutionMonitor(t, pool, removed, "https://example.com/removed", now.Add(-2*time.Hour))
	keptRun := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd311")
	insertTerminalFailureRun(t, pool, keptRun, monitorID, now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	removedRun := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd312")
	insertTerminalFailureRun(t, pool, removedRun, removed, now.Add(-10*time.Minute), now.Add(-9*time.Minute))

	if err := repository.Delete(context.Background(), removed); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	assertMonitorCheckRunCount(t, pool, removed, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 1)
	var keptMonitor string
	if err := pool.QueryRow(
		context.Background(),
		"SELECT monitor_id::text FROM monitoring.check_runs WHERE id = $1::uuid",
		keptRun.String(),
	).Scan(&keptMonitor); err != nil {
		t.Fatalf("query kept terminal run: %v", err)
	}
	if keptMonitor != monitorID.String() {
		t.Fatalf("kept run monitor = %s, want %s", keptMonitor, monitorID)
	}
}

func TestDeleteRepeatedIsNotFound(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd221")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/repeated", now.Add(-time.Hour))

	if err := repository.Delete(context.Background(), monitorID); err != nil {
		t.Fatalf("first Delete() error = %v", err)
	}
	err := repository.Delete(context.Background(), monitorID)
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("second Delete() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ports.ErrMonitorNotFound.Error() {
		t.Fatalf("second Delete() error = %q, want stable %q", err, ports.ErrMonitorNotFound)
	}
	assertMonitorRowCount(t, pool, monitorID, 0)
}

func TestDeleteBeforeClaimPreventsClaim(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd231")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/gone", now.Add(-2*time.Hour))

	if err := repository.Delete(context.Background(), monitorID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd321"),
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if !errors.Is(err, ports.ErrNoDueCheck) {
		t.Fatalf("ClaimDueCheck() error = %v, want ErrNoDueCheck", err)
	}
}

func TestClaimBeforeDeleteRemovesPendingRun(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 40, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd241")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/inflight", now.Add(-2*time.Hour))
	claimedID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd331")

	claimed, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   claimedID,
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimDueCheck() error = %v", err)
	}
	if claimed.MonitorID != monitorID {
		t.Fatalf("claimed MonitorID = %s, want %s", claimed.MonitorID, monitorID)
	}

	if err := repository.Delete(context.Background(), monitorID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)

	result, err := domain.NewHTTPResponseResult(125, 204)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult() error = %v", err)
	}
	completeErr := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     claimedID,
		CompletedAt: now.Add(time.Second),
		Result:      result,
	})
	if !errors.Is(completeErr, ports.ErrCheckRunNotFound) {
		t.Fatalf("CompleteCheck() error = %v, want ErrCheckRunNotFound", completeErr)
	}
}

func TestDeleteSerializesWithPauseResume(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 9, 50, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd251")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/paused-delete", now.Add(-time.Hour))

	if _, err := repository.SetScheduling(context.Background(), monitorID, domain.SchedulingPaused); err != nil {
		t.Fatalf("SetScheduling(paused) error = %v", err)
	}
	if err := repository.Delete(context.Background(), monitorID); err != nil {
		t.Fatalf("Delete() after pause error = %v", err)
	}
	_, err := repository.SetScheduling(context.Background(), monitorID, domain.SchedulingActive)
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("SetScheduling() after delete error = %v, want ErrMonitorNotFound", err)
	}

	fresh := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd252")
	insertExecutionMonitor(t, pool, fresh, "https://example.com/delete-pause", now.Add(-time.Hour))
	if err := repository.Delete(context.Background(), fresh); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = repository.SetScheduling(context.Background(), fresh, domain.SchedulingPaused)
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("SetScheduling() after delete error = %v, want ErrMonitorNotFound", err)
	}
}

func TestRacingDeletesOneCommitsOtherNotFound(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 10, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd261")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/racing", now.Add(-time.Hour))
	insertPendingRun(
		t,
		pool,
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd341"),
		monitorID,
		now.Add(-30*time.Second),
		now.Add(-10*time.Second),
	)

	const attempts = 4
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wait sync.WaitGroup
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results <- repository.Delete(context.Background(), monitorID)
		}()
	}

	close(start)
	wait.Wait()
	close(results)

	successes := 0
	notFound := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ports.ErrMonitorNotFound):
			notFound++
		default:
			t.Fatalf("racing Delete() unexpected error = %v", err)
		}
	}
	if successes != 1 || notFound != attempts-1 {
		t.Fatalf("racing outcomes success=%d notFound=%d, want 1/%d", successes, notFound, attempts-1)
	}
	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)
}

func TestReSweepRemovesClaimCommittedBetweenSweepAndLock(t *testing.T) {
	tracer := newGatingTracer("UPDATE monitoring.check_runs")
	pool := openGatedIntegrationPool(t, tracer)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 10, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd271")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/resweep", now.Add(-2*time.Hour))
	// The expired run reconciles to completed_at = deadline_at = now-70s, which
	// is still older than DueBefore = now-60s, so the gated claim may claim this
	// Monitor after reconciling it.
	expiredID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd351")
	insertPendingRun(
		t,
		pool,
		expiredID,
		monitorID,
		now.Add(-80*time.Second),
		now.Add(-70*time.Second),
	)

	type claimOutcome struct {
		claimed ports.ClaimedCheck
		err     error
	}
	claims := make(chan claimOutcome, 1)
	go func() {
		claimed, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd352"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		claims <- claimOutcome{claimed: claimed, err: err}
	}()

	select {
	case <-tracer.hit:
	case <-time.After(10 * time.Second):
		t.Fatal("ClaimDueCheck() never reached the gated reconciliation update")
	}

	// The delete's batched sweep waits on the run-row lock the gated claim
	// transaction holds, so the delete cannot reach its lock statement until
	// this claim has committed its freshly inserted pending run.
	type deleteOutcome struct {
		err error
	}
	deletes := make(chan deleteOutcome, 1)
	go func() {
		deletes <- deleteOutcome{err: repository.Delete(context.Background(), monitorID)}
	}()
	waitForTransactionLockWait(t, pool)

	close(tracer.release)

	claimedID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd352")
	select {
	case got := <-claims:
		if got.err != nil {
			t.Fatalf("interposed ClaimDueCheck() error = %v", got.err)
		}
		if got.claimed.MonitorID != monitorID {
			t.Fatalf("interposed claim MonitorID = %s, want %s", got.claimed.MonitorID, monitorID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("interposed ClaimDueCheck() did not finish after release")
	}

	select {
	case got := <-deletes:
		if got.err != nil {
			t.Fatalf("Delete() after interposed claim error = %v", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Delete() did not finish after the interposed claim")
	}

	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)

	result, err := domain.NewHTTPResponseResult(125, 204)
	if err != nil {
		t.Fatalf("NewHTTPResponseResult() error = %v", err)
	}
	completeErr := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     claimedID,
		CompletedAt: now.Add(time.Second),
		Result:      result,
	})
	if !errors.Is(completeErr, ports.ErrCheckRunNotFound) {
		t.Fatalf("CompleteCheck() for removed claim error = %v, want ErrCheckRunNotFound", completeErr)
	}
}

func TestReconciliationOverlapRemainsAtomic(t *testing.T) {
	tracer := newGatingTracer("UPDATE monitoring.check_runs")
	pool := openGatedIntegrationPool(t, tracer)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, time.September, 28, 10, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd281")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/overlap", now.Add(-2*time.Hour))
	// After reconciliation this Monitor stays eligible: the terminal completion
	// lands at now-70s, older than DueBefore = now-60s.
	expiredID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd361")
	insertPendingRun(
		t,
		pool,
		expiredID,
		monitorID,
		now.Add(-80*time.Second),
		now.Add(-70*time.Second),
	)

	type claimOutcome struct {
		claimed ports.ClaimedCheck
		err     error
	}
	claims := make(chan claimOutcome, 1)
	go func() {
		claimed, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
			CheckID:   mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd362"),
			Now:       now,
			DueBefore: now.Add(-60 * time.Second),
			Deadline:  now.Add(20 * time.Second),
		})
		claims <- claimOutcome{claimed: claimed, err: err}
	}()

	select {
	case <-tracer.hit:
	case <-time.After(10 * time.Second):
		t.Fatal("ClaimDueCheck() never reached the gated reconciliation update")
	}

	// The delete's batched sweep waits on the run-row lock held by the claim
	// transaction, so the Monitor lock is never taken while another
	// transaction holds a CheckRun lock and waits for this Monitor.
	type deleteOutcome struct {
		err error
	}
	deletes := make(chan deleteOutcome, 1)
	go func() {
		deletes <- deleteOutcome{err: repository.Delete(context.Background(), monitorID)}
	}()
	waitForTransactionLockWait(t, pool)

	close(tracer.release)

	var claimedMonitor domain.MonitorID
	select {
	case got := <-claims:
		if got.err != nil {
			t.Fatalf("overlapped ClaimDueCheck() error = %v", got.err)
		}
		claimedMonitor = got.claimed.MonitorID
	case <-time.After(10 * time.Second):
		t.Fatal("overlapped ClaimDueCheck() did not finish after release")
	}
	if claimedMonitor != monitorID {
		t.Fatalf("overlapped claim MonitorID = %s, want %s", claimedMonitor, monitorID)
	}

	select {
	case got := <-deletes:
		if got.err != nil {
			t.Fatalf("overlapped Delete() error = %v", got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("overlapped Delete() did not finish")
	}

	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)
}

func assertMonitorRowCount(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.monitors WHERE id = $1::uuid",
		monitorID.String(),
	).Scan(&count); err != nil {
		t.Fatalf("count monitor rows: %v", err)
	}
	if count != want {
		t.Fatalf("monitor rows = %d, want %d", count, want)
	}
}

func assertMonitorCheckRunCount(t *testing.T, pool *pgxpool.Pool, monitorID domain.MonitorID, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.check_runs WHERE monitor_id = $1::uuid",
		monitorID.String(),
	).Scan(&count); err != nil {
		t.Fatalf("count check runs: %v", err)
	}
	if count != want {
		t.Fatalf("check run rows = %d, want %d", count, want)
	}
}

// waitForTransactionLockWait returns once some transaction is waiting for a
// row lock held by another open transaction, observed through pg_locks. This
// proves the delete reached its blocked sweep before the gated claim resumes.
func waitForTransactionLockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(
			context.Background(),
			"SELECT count(*) FROM pg_locks WHERE NOT granted AND locktype = 'transactionid'",
		).Scan(&waiting); err != nil {
			t.Fatalf("observe pg_locks: %v", err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("delete never blocked on the claim transaction's run lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// gatingTracer is a test-only pgx tracer that signals once the first matching
// query has executed and holds its transaction open until the test releases
// it, creating a deterministic post-execution, pre-commit statement boundary.
type gatingTracer struct {
	match     string
	hit       chan struct{}
	release   chan struct{}
	mu        sync.Mutex
	triggered bool
}

type gatedQueryContextKey struct{}

func newGatingTracer(match string) *gatingTracer {
	return &gatingTracer{match: match, hit: make(chan struct{}, 1), release: make(chan struct{})}
}

func (tracer *gatingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tracer.mu.Lock()
	shouldGate := !tracer.triggered && strings.Contains(data.SQL, tracer.match)
	if shouldGate {
		tracer.triggered = true
	}
	tracer.mu.Unlock()
	if !shouldGate {
		return ctx
	}
	select {
	case tracer.hit <- struct{}{}:
	case <-ctx.Done():
		return ctx
	}
	return context.WithValue(ctx, gatedQueryContextKey{}, true)
}

func (tracer *gatingTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(gatedQueryContextKey{}) == nil {
		return
	}
	select {
	case <-tracer.release:
	case <-ctx.Done():
	}
}

func openGatedIntegrationPool(t *testing.T, tracer *gatingTracer) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig() error = %v", err)
	}
	config.ConnConfig.Tracer = tracer

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() error = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() error = %v", err)
	}

	return pool
}
