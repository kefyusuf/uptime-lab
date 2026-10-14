//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"strings"
	"sync"
	"testing"
	"time"
)

// claimQueryTrace observes actual database statement boundaries, without a production hook.
type claimQueryTrace struct{ queries []string }

func (trace *claimQueryTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	trace.queries = append(trace.queries, strings.Join(strings.Fields(strings.ToLower(data.SQL)), " "))
	return ctx
}
func (*claimQueryTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestClaimRechecksLockedSchedulingState(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", now)
	trace := &claimQueryTrace{}
	config := pool.Config()
	config.ConnConfig.Tracer = trace
	config.ConnConfig.RuntimeParams["default_transaction_isolation"] = "repeatable read"
	claimPool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer claimPool.Close()
	if _, err = NewRepository(claimPool).ClaimDueCheck(ctx, schedulingClaimInput(t, now, 101)); err != nil {
		t.Fatal(err)
	}
	rechecked := false
	explicitIsolation := false
	for _, sql := range trace.queries {
		if sql == "begin isolation level read committed" {
			explicitIsolation = true
		}
		if strings.HasPrefix(sql, "select paused from monitoring.monitors") {
			rechecked = true
		}
		if strings.HasPrefix(sql, "insert into monitoring.check_runs") && !rechecked {
			t.Fatal("claim inserted without a separate locked scheduling read")
		}
	}
	if !rechecked || !explicitIsolation {
		t.Fatalf("locked recheck=%v explicit Read Committed=%v", rechecked, explicitIsolation)
	}
}

func schedulingClaimInput(t *testing.T, now time.Time, suffix int) ports.ClaimDueCheckInput {
	t.Helper()
	raw := fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3fafd%03d", suffix)
	return ports.ClaimDueCheckInput{CheckID: mustExecutionCheckID(t, raw), Now: now, DueBefore: now.Add(-time.Minute), Deadline: now.Add(20 * time.Second)}
}

func awaitSchedulingBlocker(t *testing.T, ctx context.Context, pool *pgxpool.Pool, application string, blocker uint32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND $2::int=ANY(pg_blocking_pids(pid)))`, application, int(blocker)).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("expected PostgreSQL blocker not observed", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestClaimSnapshotOverlapsCommittedPause(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", now)
	// A test-only updatable view gates the real candidate statement below LockRows.
	// The gate belongs solely to this test's database; product SQL remains unchanged.
	_, err := pool.Exec(ctx, `ALTER TABLE monitoring.monitors RENAME TO monitors_snapshot_base;
 CREATE FUNCTION monitoring.scheduling_snapshot_gate() RETURNS boolean LANGUAGE plpgsql VOLATILE AS $$
 BEGIN
 IF current_setting('application_name')='pause-snapshot-claim' THEN PERFORM pg_advisory_xact_lock(642080); END IF;
 RETURN true;
 END $$;
 CREATE VIEW monitoring.monitors AS SELECT * FROM monitoring.monitors_snapshot_base WHERE monitoring.scheduling_snapshot_gate();`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `DROP VIEW monitoring.monitors; DROP FUNCTION monitoring.scheduling_snapshot_gate(); ALTER TABLE monitoring.monitors_snapshot_base RENAME TO monitors;`); err != nil {
			t.Error(err)
		}
	})
	cfg := pool.Config()
	cfg.ConnConfig.RuntimeParams["application_name"] = "pause-snapshot-claim"
	claimPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer claimPool.Close()
	control, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer control.Release()
	if _, err = control.Exec(ctx, `SELECT pg_advisory_lock(642080)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = control.Exec(context.Background(), `SELECT pg_advisory_unlock(642080)`) }()
	done := make(chan error, 1)
	go func() {
		_, err := NewRepository(claimPool).ClaimDueCheck(ctx, schedulingClaimInput(t, now, 101))
		done <- err
	}()
	awaitSchedulingBlocker(t, ctx, pool, "pause-snapshot-claim", control.Conn().PgConn().PID())
	if _, err = NewRepository(pool).SetScheduling(ctx, id, domain.SchedulingPaused); err != nil {
		t.Fatal(err)
	}
	if _, err = control.Exec(ctx, `SELECT pg_advisory_unlock(642080)`); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if !errors.Is(err, ports.ErrNoDueCheck) {
			t.Fatalf("snapshot claim escaped committed pause: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM monitoring.check_runs`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestSchedulingSameStateSerializesOppositeWrite(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
	insertExecutionMonitor(t, pool, id, "https://example.com", time.Now().UTC())
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	tx, err := holder.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT id FROM monitoring.monitors WHERE id=$1 FOR UPDATE`, id.String()); err != nil {
		t.Fatal(err)
	}
	cfg := pool.Config()
	cfg.ConnConfig.RuntimeParams["application_name"] = "same-state-setter"
	setterPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer setterPool.Close()
	done := make(chan error, 1)
	go func() {
		_, err := NewRepository(setterPool).SetScheduling(ctx, id, domain.SchedulingActive)
		done <- err
	}()
	awaitSchedulingBlocker(t, ctx, pool, "same-state-setter", holder.Conn().PgConn().PID())
	if _, err = tx.Exec(ctx, `UPDATE monitoring.monitors SET paused=true WHERE id=$1`, id.String()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if state, err := NewRepository(pool).GetScheduling(ctx, id); err != nil || state != domain.SchedulingActive {
		t.Fatal(state, err)
	}
}

type claimInsertGate struct {
	entered chan uint32
	release chan struct{}
}

func (gate *claimInsertGate) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(strings.TrimSpace(data.SQL), "INSERT INTO monitoring.check_runs") {
		gate.entered <- conn.PgConn().PID()
		select {
		case <-gate.release:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (*claimInsertGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestClaimBeforePauseRetainsPendingRun(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback=%v", rollback), func(t *testing.T) {
			pool := openIntegrationPool(t)
			resetMonitors(t, pool)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
			id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
			insertExecutionMonitor(t, pool, id, "https://example.com", now)
			gate := &claimInsertGate{entered: make(chan uint32, 1), release: make(chan struct{})}
			var once sync.Once
			release := func() { once.Do(func() { close(gate.release) }) }
			cfg := pool.Config()
			cfg.ConnConfig.Tracer = gate
			claimPool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer claimPool.Close()
			defer release()
			cfg = pool.Config()
			cfg.ConnConfig.RuntimeParams["application_name"] = "pause-after-claim"
			setterPool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer setterPool.Close()
			claimCtx, cancelClaim := context.WithCancel(ctx)
			defer cancelClaim()
			claimDone := make(chan error, 1)
			pauseDone := make(chan error, 1)
			go func() {
				_, err := NewRepository(claimPool).ClaimDueCheck(claimCtx, schedulingClaimInput(t, now, 101))
				claimDone <- err
			}()
			var pid uint32
			select {
			case pid = <-gate.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			go func() {
				_, err := NewRepository(setterPool).SetScheduling(ctx, id, domain.SchedulingPaused)
				pauseDone <- err
			}()
			awaitSchedulingBlocker(t, ctx, pool, "pause-after-claim", pid)
			if rollback {
				cancelClaim()
			}
			release()
			select {
			case err = <-claimDone:
				if rollback {
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err = <-pauseDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var count int
			want := 1
			if rollback {
				want = 0
			}
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM monitoring.check_runs`).Scan(&count); err != nil || count != want {
				t.Fatal(count, err)
			}
			if state, err := NewRepository(pool).GetScheduling(ctx, id); err != nil || state != domain.SchedulingPaused {
				t.Fatal(state, err)
			}
		})
	}
}
