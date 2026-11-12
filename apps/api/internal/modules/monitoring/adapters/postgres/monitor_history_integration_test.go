//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func insertTerminalRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	monitorID domain.MonitorID,
	issuedAt time.Time,
	completedAt time.Time,
	kind string,
	httpStatus *int,
	durationMS *int64,
) {
	t.Helper()
	if _, err := pool.Exec(
		context.Background(),
		`INSERT INTO monitoring.check_runs (
			id, monitor_id, issued_at, deadline_at, completed_at, result_kind, http_status, duration_ms
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`,
		checkID.String(),
		monitorID.String(),
		issuedAt,
		issuedAt.Add(20*time.Second),
		completedAt,
		kind,
		httpStatus,
		durationMS,
	); err != nil {
		t.Fatalf("insert terminal run: %v", err)
	}
}

func historyCheckID(t *testing.T, index int) domain.CheckID {
	t.Helper()
	return mustExecutionCheckID(t, fmt.Sprintf("7d9b2eb0-52bb-4dd7-8934-c5ce30d5c%03x", index))
}

func assertHistoryOrder(t *testing.T, entries []ports.LatestCheckResultRecord, want []domain.CheckID) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("history rows = %d, want %d", len(entries), len(want))
	}
	for index, entry := range entries {
		if entry.CheckID != want[index] {
			t.Fatalf("history row %d = %s, want %s", index, entry.CheckID, want[index])
		}
	}
}

func TestHistoryFirstPageOrdersTerminalRows(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd401")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history", now.Add(-time.Hour))
	newest := historyCheckID(t, 1)
	middle := historyCheckID(t, 2)
	oldest := historyCheckID(t, 3)
	insertTerminalFailureRun(t, pool, newest, monitorID, now.Add(-3*time.Minute), now.Add(-time.Minute))
	insertTerminalFailureRun(t, pool, middle, monitorID, now.Add(-4*time.Minute), now.Add(-2*time.Minute))
	insertTerminalFailureRun(t, pool, oldest, monitorID, now.Add(-5*time.Minute), now.Add(-3*time.Minute))

	entries, err := repository.Page(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	assertHistoryOrder(t, entries, []domain.CheckID{newest, middle, oldest})
	if entries[0].ResultKind != domain.CheckResultTimeout || entries[0].DurationMS == nil || *entries[0].DurationMS != 1 {
		t.Fatalf("mapped row = kind %q duration %v", entries[0].ResultKind, entries[0].DurationMS)
	}
}

func TestHistoryPagingBoundariesTwentyAndFifty(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd402")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-bounds", now.Add(-time.Hour))
	ids := make([]domain.CheckID, 0, 22)
	for index := 0; index < 22; index++ {
		checkID := historyCheckID(t, index)
		ids = append(ids, checkID)
		insertTerminalRun(t, pool, checkID, monitorID, now.Add(-time.Duration(index+1)*time.Minute), now.Add(-time.Duration(index+30)*time.Second), "timeout", nil, int64Ptr(1))
	}

	page, err := repository.Page(context.Background(), monitorID, nil, 21)
	if err != nil {
		t.Fatalf("Page(21) error = %v", err)
	}
	if len(page) != 21 {
		t.Fatalf("Page(21) rows = %d, want exactly 21", len(page))
	}
	assertHistoryOrder(t, page, ids[:21])

	full, err := repository.Page(context.Background(), monitorID, nil, 51)
	if err != nil {
		t.Fatalf("Page(51) error = %v", err)
	}
	assertHistoryOrder(t, full, ids)
}

func int64Ptr(value int64) *int64 { return &value }

func TestHistoryContinuationNamesLastIncludedRow(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd403")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-continuation", now.Add(-time.Hour))
	newest := historyCheckID(t, 11)
	middle := historyCheckID(t, 12)
	oldest := historyCheckID(t, 13)
	insertTerminalFailureRun(t, pool, newest, monitorID, now.Add(-3*time.Minute), now.Add(-time.Minute))
	insertTerminalFailureRun(t, pool, middle, monitorID, now.Add(-4*time.Minute), now.Add(-2*time.Minute))
	insertTerminalFailureRun(t, pool, oldest, monitorID, now.Add(-5*time.Minute), now.Add(-3*time.Minute))

	anchor := ports.HistoryAnchor{CompletedAt: now.Add(-time.Minute), CheckID: newest}
	older, err := repository.Page(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("anchored Page() error = %v", err)
	}
	assertHistoryOrder(t, older, []domain.CheckID{middle, oldest})

	oldestAnchor := ports.HistoryAnchor{CompletedAt: now.Add(-3 * time.Minute), CheckID: oldest}
	none, err := repository.Page(context.Background(), monitorID, &oldestAnchor, 50)
	if err != nil {
		t.Fatalf("anchored Page() at oldest error = %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("anchored Page() at oldest = %d rows, want 0", len(none))
	}
}

func TestHistoryTieBreaksEqualCompletionInstants(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd404")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-ties", now.Add(-time.Hour))
	instant := now.Add(-time.Minute)
	for index := 0; index < 3; index++ {
		insertTerminalFailureRun(t, pool, historyCheckID(t, 21+index), monitorID, instant.Add(-time.Duration(index+1)*time.Minute), instant)
	}

	entries, err := repository.Page(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("history rows = %d, want 3", len(entries))
	}
	ordered := make([]string, 0, 3)
	for _, entry := range entries {
		ordered = append(ordered, entry.CheckID.String())
	}
	if !sort.StringsAreSorted(reverse(ordered)) {
		t.Fatalf("tie-break order = %v, want check id descending", ordered)
	}
}

func reverse(values []string) []string {
	reversed := make([]string, len(values))
	for index, value := range values {
		reversed[len(values)-1-index] = value
	}
	return reversed
}

func TestHistoryExcludesPendingRows(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 40, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd405")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-pending", now.Add(-time.Hour))
	terminal := historyCheckID(t, 31)
	pending := historyCheckID(t, 32)
	insertTerminalFailureRun(t, pool, terminal, monitorID, now.Add(-3*time.Minute), now.Add(-2*time.Minute))
	insertPendingRun(t, pool, pending, monitorID, now.Add(-10*time.Second), now.Add(10*time.Second))

	entries, err := repository.Page(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	assertHistoryOrder(t, entries, []domain.CheckID{terminal})
}

func TestHistoryEmptyAndUnknownMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 50, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd406")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-empty", now.Add(-time.Hour))

	entries, err := repository.Page(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("empty Page() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty Page() rows = %d, want 0", len(entries))
	}

	unknown := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd407")
	if _, err := repository.Page(context.Background(), unknown, nil, 50); !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("unknown Page() error = %v, want ErrMonitorNotFound", err)
	}
}

func TestHistoryAnchoredPositionWithoutMatchingRow(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 13, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd408")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-position", now.Add(-time.Hour))
	newer := historyCheckID(t, 41)
	older := historyCheckID(t, 42)
	insertTerminalFailureRun(t, pool, newer, monitorID, now.Add(-3*time.Minute), now.Add(-30*time.Second))
	insertTerminalFailureRun(t, pool, older, monitorID, now.Add(-4*time.Minute), now.Add(-90*time.Second))

	// A position strictly between the two rows without a matching CheckRun.
	anchor := ports.HistoryAnchor{CompletedAt: now.Add(-60 * time.Second), CheckID: mustExecutionCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c6ff")}
	entries, err := repository.Page(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("anchored Page() error = %v", err)
	}
	assertHistoryOrder(t, entries, []domain.CheckID{older})
}

func TestHistoryPositionStableUnderNewerCompletions(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 13, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd409")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-stable", now.Add(-time.Hour))
	newest := historyCheckID(t, 51)
	older := historyCheckID(t, 52)
	insertTerminalFailureRun(t, pool, newest, monitorID, now.Add(-3*time.Minute), now.Add(-time.Minute))
	insertTerminalFailureRun(t, pool, older, monitorID, now.Add(-4*time.Minute), now.Add(-2*time.Minute))

	anchor := ports.HistoryAnchor{CompletedAt: now.Add(-time.Minute), CheckID: newest}
	first, err := repository.Page(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("first anchored Page() error = %v", err)
	}

	fresher := historyCheckID(t, 53)
	insertTerminalFailureRun(t, pool, fresher, monitorID, now.Add(-2*time.Minute), now.Add(-5*time.Second))

	second, err := repository.Page(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("second anchored Page() error = %v", err)
	}
	assertHistoryOrder(t, first, []domain.CheckID{older})
	assertHistoryOrder(t, second, []domain.CheckID{older})
}

func TestDeletionRacingHistoryResolvesPageOr404(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 13, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd410")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/history-race", now.Add(-time.Hour))
	terminal := historyCheckID(t, 61)
	insertTerminalFailureRun(t, pool, terminal, monitorID, now.Add(-3*time.Minute), now.Add(-2*time.Minute))

	// Park the delete's sweep on a run-row lock held by an open test
	// transaction, then run the history read to completion against the
	// pre-delete snapshot before letting the delete commit.
	held, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin parking transaction: %v", err)
	}
	if _, err := held.Exec(
		context.Background(),
		"SELECT id FROM monitoring.check_runs WHERE monitor_id = $1::uuid FOR UPDATE",
		monitorID.String(),
	); err != nil {
		t.Fatalf("park run-row lock: %v", err)
	}

	type historyOutcome struct {
		entries []ports.LatestCheckResultRecord
		err     error
	}
	reads := make(chan historyOutcome, 1)
	go func() {
		entries, err := repository.Page(context.Background(), monitorID, nil, 50)
		reads <- historyOutcome{entries: entries, err: err}
	}()
	readsDone := make(chan struct{})
	go func() {
		outcome := <-reads
		if outcome.err != nil {
			t.Errorf("racing Page() error = %v", outcome.err)
		} else if len(outcome.entries) != 1 || outcome.entries[0].CheckID != terminal {
			t.Errorf("pre-delete history rows = %+v, want the parked snapshot row", outcome.entries)
		}
		close(readsDone)
	}()

	deletes := make(chan error, 1)
	go func() {
		deletes <- repository.Delete(context.Background(), monitorID)
	}()
	waitForTransactionLockWait(t, pool)

	// The pinned property is that the history read completes against the
	// pre-delete snapshot while the delete is parked on the run-row lock, so
	// require the read's outcome before releasing the lock; otherwise runner
	// scheduling could commit the delete before the read takes its snapshot.
	select {
	case <-readsDone:
	case <-time.After(10 * time.Second):
		t.Fatal("racing Page() did not finish while the delete was parked")
	}

	if err := held.Commit(context.Background()); err != nil {
		t.Fatalf("release parked lock: %v", err)
	}

	select {
	case err := <-deletes:
		if err != nil {
			t.Fatalf("racing Delete() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("racing Delete() did not finish after release")
	}

	assertMonitorRowCount(t, pool, monitorID, 0)
	assertMonitorCheckRunCount(t, pool, monitorID, 0)
}
