//go:build integration

package postgres

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// grantedRelationsForApplication reports the relation-level locks one named
// application currently holds, observed through pg_locks joined to pg_class.
func grantedRelationsForApplication(t *testing.T, pool *pgxpool.Pool, application string) []string {
	t.Helper()
	rows, err := pool.Query(
		context.Background(),
		`SELECT c.relname
		 FROM pg_locks l
		 JOIN pg_class c ON c.oid = l.relation
		 JOIN pg_stat_activity a ON a.pid = l.pid
		 WHERE a.application_name = $1 AND l.granted AND l.locktype = 'relation'`,
		application,
	)
	if err != nil {
		t.Fatalf("observe granted relation locks: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan granted relation: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate granted relations: %v", err)
	}
	return names
}

func containsRelation(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func TestOneOpenIncidentUnderConcurrentCompletion(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	ctx := context.Background()

	now := time.Date(2026, time.September, 26, 17, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd60e")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-concurrent", now.Add(-time.Hour))
	seedIncidentState(t, pool, monitorID, 2, 0)
	pending := incidentFixtureCheckID(t, 0)
	insertPendingRun(t, pool, pending, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

	result := incidentFailureResult(t)
	const attempts = 6
	start := make(chan struct{})
	results := make(chan error, attempts)
	var wait sync.WaitGroup
	for index := range attempts {
		wait.Add(1)
		go func(candidate domain.IncidentID) {
			defer wait.Done()
			<-start
			results <- repository.CompleteCheck(ctx, ports.CompleteCheckInput{
				CheckID:     pending,
				CompletedAt: now,
				Result:      result,
				IncidentID:  candidate,
			})
		}(incidentFixtureCandidateID(t, index))
	}
	close(start)
	wait.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Fatalf("concurrent CompleteCheck() error = %v, want nil (first opens, rest are exact duplicates)", err)
		}
	}
	if got := countOpenIncidents(t, pool, monitorID); got != 1 {
		t.Fatalf("open incidents after concurrent completion = %d, want 1", got)
	}
	var rawOpen string
	if err := pool.QueryRow(ctx,
		`SELECT open_incident_id::text FROM monitoring.incident_state WHERE monitor_id = $1::uuid`,
		monitorID.String()).Scan(&rawOpen); err != nil {
		t.Fatalf("read winning open incident: %v", err)
	}
	winner := mustIncidentID(t, rawOpen)
	assertIncidentState(t, pool, monitorID, 3, 0, &winner)
	assertIncidentWindow(t, pool, winner, monitorID, now, nil, "unexpected_http_status", &pending, nil)
}

func TestNoMonitorLockDeadlock(t *testing.T) {
	t.Run("steady_state_fold_ignores_parked_monitor_lock", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		now := time.Date(2026, time.September, 26, 17, 10, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd60f")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-steady", now.Add(-time.Hour))
		seedIncidentState(t, pool, monitorID, 0, 0)
		pending := incidentFixtureCheckID(t, 0)
		insertPendingRun(t, pool, pending, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		holder, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Release()
		tx, err := holder.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(ctx, `SELECT id FROM monitoring.monitors WHERE id=$1 FOR UPDATE`, monitorID.String()); err != nil {
			t.Fatal(err)
		}

		trace := &claimQueryTrace{}
		cfg := pool.Config()
		cfg.ConnConfig.Tracer = trace
		cfg.ConnConfig.RuntimeParams["application_name"] = "incident-steady-fold"
		foldPool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer foldPool.Close()

		result := incidentFailureResult(t)
		candidate := incidentFixtureCandidateID(t, 0)
		done := make(chan error, 1)
		go func() {
			done <- NewRepository(foldPool).CompleteCheck(ctx, ports.CompleteCheckInput{
				CheckID:     pending,
				CompletedAt: now,
				Result:      result,
				IncidentID:  candidate,
			})
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("steady-state fold blocked by parked monitor lock: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("steady-state fold waited on a monitors lock; CompleteCheck must never lock monitors")
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		for _, sql := range trace.queries {
			if strings.Contains(sql, "monitoring.monitors") && strings.Contains(sql, "for update") {
				t.Fatalf("CompleteCheck issued a monitors lock: %s", sql)
			}
		}
		assertIncidentState(t, pool, monitorID, 1, 0, nil)
	})

	t.Run("completion_locks_check_run_before_fold_tables", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		now := time.Date(2026, time.September, 26, 17, 20, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd610")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-run-first", now.Add(-time.Hour))
		seedIncidentState(t, pool, monitorID, 0, 0)
		pending := incidentFixtureCheckID(t, 0)
		insertPendingRun(t, pool, pending, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		held, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = held.Rollback(context.Background()) }()
		if _, err := held.Exec(ctx, `SELECT id FROM monitoring.check_runs WHERE id=$1::uuid FOR UPDATE`, pending.String()); err != nil {
			t.Fatal(err)
		}

		cfg := pool.Config()
		cfg.ConnConfig.RuntimeParams["application_name"] = "incident-run-first"
		foldPool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer foldPool.Close()

		result := incidentFailureResult(t)
		candidate := incidentFixtureCandidateID(t, 0)
		done := make(chan error, 1)
		go func() {
			done <- NewRepository(foldPool).CompleteCheck(ctx, ports.CompleteCheckInput{
				CheckID:     pending,
				CompletedAt: now,
				Result:      result,
				IncidentID:  candidate,
			})
		}()
		waitForTransactionLockWait(t, pool)
		granted := grantedRelationsForApplication(t, pool, "incident-run-first")
		if !containsRelation(granted, "check_runs") {
			t.Fatalf("completion did not lock check_runs first: %v", granted)
		}
		for _, forbidden := range []string{"incident_state", "incidents", "monitors"} {
			if containsRelation(granted, forbidden) {
				t.Fatalf("completion locked %s before the CheckRun row lock: %v", forbidden, granted)
			}
		}
		if err := held.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("completion did not finish after the run-lock release")
		}
		assertIncidentState(t, pool, monitorID, 1, 0, nil)
	})

	t.Run("delete_and_open_fold_resolve_without_deadlock", func(t *testing.T) {
		tracer := newGatingTracer("INSERT INTO monitoring.incidents")
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		now := time.Date(2026, time.September, 26, 17, 30, 0, 0, time.UTC)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd611")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-delete-race", now.Add(-time.Hour))
		seedIncidentState(t, pool, monitorID, 2, 0)
		pending := incidentFixtureCheckID(t, 0)
		insertPendingRun(t, pool, pending, monitorID, now.Add(-time.Second), now.Add(19*time.Second))

		cfg, err := pgxpool.ParseConfig("")
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.Tracer = tracer
		foldPool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer foldPool.Close()
		if err := foldPool.Ping(ctx); err != nil {
			t.Fatal(err)
		}

		result := incidentFailureResult(t)
		candidate := incidentFixtureCandidateID(t, 0)
		completions := make(chan error, 1)
		go func() {
			completions <- NewRepository(foldPool).CompleteCheck(ctx, ports.CompleteCheckInput{
				CheckID:     pending,
				CompletedAt: now,
				Result:      result,
				IncidentID:  candidate,
			})
		}()
		select {
		case <-tracer.hit:
		case <-ctx.Done():
			t.Fatal("open fold never reached the incidents INSERT")
		}

		// The delete's first CheckRun sweep waits on the run-row lock the gated
		// completion holds, so deletion cannot reach its Monitor lock or incident
		// sweeps until the fold has committed and released the run.
		deletes := make(chan error, 1)
		go func() {
			deletes <- NewRepository(pool).Delete(ctx, monitorID)
		}()
		waitForTransactionLockWait(t, pool)
		close(tracer.release)

		select {
		case err := <-completions:
			if err != nil {
				t.Fatalf("open fold racing delete error = %v", err)
			}
		case <-ctx.Done():
			t.Fatal("open fold deadlocked with delete")
		}
		select {
		case err := <-deletes:
			if err != nil {
				t.Fatalf("Delete racing open fold error = %v", err)
			}
		case <-ctx.Done():
			t.Fatal("Delete deadlocked with the open fold")
		}

		assertMonitorRowCount(t, pool, monitorID, 0)
		assertMonitorCheckRunCount(t, pool, monitorID, 0)
		assertNoIncidentState(t, pool, monitorID)
		if got := countIncidents(t, pool, monitorID); got != 0 {
			t.Fatalf("incidents after racing delete = %d, want 0", got)
		}
	})
}
