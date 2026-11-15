//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var limitRunIDs = []string{
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe340",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe341",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe342",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe343",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe344",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe345",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe346",
	"018f22d3-1d6a-7cc0-a37b-46fc3fafe347",
}

func TestRecentTerminalByMonitorIDUnknownMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	_, err := repository.RecentTerminalByMonitorID(
		context.Background(),
		mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe301"),
		5,
	)
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("RecentTerminalByMonitorID() error = %v, want ErrMonitorNotFound", err)
	}
}

func TestRecentTerminalByMonitorIDEmptyWhenNoTerminalRows(t *testing.T) {
	t.Run("no CheckRuns", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe302")
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/empty", time.Date(2026, time.September, 29, 10, 0, 0, 0, time.UTC))

		got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
		if err != nil {
			t.Fatalf("RecentTerminalByMonitorID() error = %v, want nil", err)
		}
		if len(got) != 0 {
			t.Fatalf("RecentTerminalByMonitorID() len = %d, want 0", len(got))
		}
	})

	t.Run("pending only", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe303")
		issuedAt := time.Date(2026, time.September, 29, 10, 0, 0, 0, time.UTC)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/pending", issuedAt.Add(-time.Hour))
		insertPendingRun(t, pool, mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe304"), monitorID, issuedAt, issuedAt.Add(20*time.Second))

		got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
		if err != nil {
			t.Fatalf("RecentTerminalByMonitorID() error = %v, want nil", err)
		}
		if len(got) != 0 {
			t.Fatalf("RecentTerminalByMonitorID() len = %d, want 0", len(got))
		}
	})
}

func TestRecentTerminalByMonitorIDOrdersNewestFirstAndMatchesLatest(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe310")
	base := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/order", base.Add(-time.Hour))

	oldest := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe311")
	middle := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe312")
	newest := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe313")
	insertLatestTerminalRun(t, pool, oldest, monitorID, base, base.Add(5*time.Second), domain.CheckResultTimeout, nil, latestInt64Pointer(1))
	insertLatestTerminalRun(t, pool, middle, monitorID, base.Add(10*time.Second), base.Add(15*time.Second), domain.CheckResultHTTPResponse, latestIntPointer(200), latestInt64Pointer(2))
	insertLatestTerminalRun(t, pool, newest, monitorID, base.Add(20*time.Second), base.Add(25*time.Second), domain.CheckResultHTTPResponse, latestIntPointer(503), latestInt64Pointer(3))

	got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
	if err != nil {
		t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("RecentTerminalByMonitorID() len = %d, want 3", len(got))
	}
	wantOrder := []domain.CheckID{newest, middle, oldest}
	for index, want := range wantOrder {
		if got[index].CheckID != want {
			t.Fatalf("RecentTerminalByMonitorID()[%d] = %s, want %s", index, got[index].CheckID, want)
		}
	}

	latest, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
	if err != nil {
		t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
	}
	if got[0].CheckID != latest.CheckID || got[0].ResultKind != latest.ResultKind || !got[0].CompletedAt.Equal(latest.CompletedAt) {
		t.Fatalf("RecentTerminalByMonitorID()[0] = %+v, want latest %+v", got[0], latest)
	}
}

func TestRecentTerminalByMonitorIDUsesThreeKeyTieBreak(t *testing.T) {
	t.Run("same completion uses later issued_at even with lower UUID", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe320")
		base := time.Date(2026, time.September, 29, 12, 10, 0, 0, time.UTC)
		completedAt := base.Add(10 * time.Second)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/issued-tie", base.Add(-time.Hour))

		earlierIssuedHigherID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe322")
		laterIssuedLowerID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe321")
		insertLatestTerminalRun(t, pool, earlierIssuedHigherID, monitorID, base, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(1))
		insertLatestTerminalRun(t, pool, laterIssuedLowerID, monitorID, base.Add(time.Second), completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(2))

		got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
		if err != nil {
			t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
		}
		if got[0].CheckID != laterIssuedLowerID {
			t.Fatalf("RecentTerminalByMonitorID()[0] = %s, want issued_at tie-break %s", got[0].CheckID, laterIssuedLowerID)
		}
	})

	t.Run("same completion and issuance use higher UUID", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe323")
		issuedAt := time.Date(2026, time.September, 29, 12, 20, 0, 0, time.UTC)
		completedAt := issuedAt.Add(5 * time.Second)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/id-tie", issuedAt.Add(-time.Hour))

		lower := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe324")
		higher := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe325")
		insertLatestTerminalRun(t, pool, lower, monitorID, issuedAt, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(1))
		insertLatestTerminalRun(t, pool, higher, monitorID, issuedAt, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(2))

		got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
		if err != nil {
			t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
		}
		if got[0].CheckID != higher {
			t.Fatalf("RecentTerminalByMonitorID()[0] = %s, want UUID tie-break %s", got[0].CheckID, higher)
		}
	})
}

func TestRecentTerminalByMonitorIDRespectsLimit(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe330")
	base := time.Date(2026, time.September, 29, 13, 0, 0, 0, time.UTC)
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/limit", base.Add(-time.Hour))

	for index := 0; index < 8; index++ {
		checkID := mustExecutionCheckID(t, limitRunIDs[index])
		issuedAt := base.Add(time.Duration(index) * time.Minute)
		insertLatestTerminalRun(t, pool, checkID, monitorID, issuedAt, issuedAt.Add(5*time.Second), domain.CheckResultTimeout, nil, latestInt64Pointer(int64(index)))
	}

	got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
	if err != nil {
		t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("RecentTerminalByMonitorID(limit=5) len = %d, want 5", len(got))
	}
	newest := mustExecutionCheckID(t, limitRunIDs[7])
	if got[0].CheckID != newest {
		t.Fatalf("RecentTerminalByMonitorID()[0] = %s, want newest %s", got[0].CheckID, newest)
	}

	two, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 2)
	if err != nil {
		t.Fatalf("RecentTerminalByMonitorID(limit=2) error = %v", err)
	}
	if len(two) != 2 {
		t.Fatalf("RecentTerminalByMonitorID(limit=2) len = %d, want 2", len(two))
	}
}

func TestRecentTerminalByMonitorIDIgnoresNewerPendingRun(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe350")
	base := time.Date(2026, time.September, 29, 13, 10, 0, 0, time.UTC)
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/pending-invisible", base.Add(-time.Hour))
	terminalID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe351")
	insertLatestTerminalRun(t, pool, terminalID, monitorID, base, base.Add(5*time.Second), domain.CheckResultPolicyRejected, nil, latestInt64Pointer(0))
	insertPendingRun(t, pool, mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe352"), monitorID, base.Add(10*time.Second), base.Add(30*time.Second))

	got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
	if err != nil {
		t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
	}
	if len(got) != 1 || got[0].CheckID != terminalID {
		t.Fatalf("RecentTerminalByMonitorID() = %+v, want only terminal %s", got, terminalID)
	}
}

func TestRecentTerminalByMonitorIDReturnsTerminalShapes(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe360")
	base := time.Date(2026, time.September, 29, 14, 0, 0, 0, time.UTC)
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/shapes", base.Add(-time.Hour))

	httpID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe361")
	workerID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe362")
	insertLatestTerminalRun(t, pool, workerID, monitorID, base, base.Add(5*time.Second), domain.CheckResultWorkerTimeout, nil, nil)
	insertLatestTerminalRun(t, pool, httpID, monitorID, base.Add(10*time.Second), base.Add(15*time.Second), domain.CheckResultHTTPResponse, latestIntPointer(204), latestInt64Pointer(123))

	got, err := repository.RecentTerminalByMonitorID(context.Background(), monitorID, 5)
	if err != nil {
		t.Fatalf("RecentTerminalByMonitorID() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("RecentTerminalByMonitorID() len = %d, want 2", len(got))
	}
	assertLatestResultRecord(t, got[0], httpID, domain.CheckResultHTTPResponse, latestIntPointer(204), latestInt64Pointer(123), base.Add(15*time.Second))
	assertLatestResultRecord(t, got[1], workerID, domain.CheckResultWorkerTimeout, nil, nil, base.Add(5*time.Second))
}

func TestRecentTerminalByMonitorIDRespectsCancelledContext(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repository.RecentTerminalByMonitorID(ctx, mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe370"), 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecentTerminalByMonitorID() error = %v, want context.Canceled", err)
	}
}

func TestRecentTerminalByMonitorIDSurfacesPersistenceFailure(t *testing.T) {
	pool := openIntegrationPool(t)
	repository := NewRepository(pool)
	pool.Close()

	_, err := repository.RecentTerminalByMonitorID(context.Background(), mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe380"), 5)
	if err == nil {
		t.Fatal("RecentTerminalByMonitorID() error = nil, want persistence failure")
	}
	if errors.Is(err, ports.ErrMonitorNotFound) || errors.Is(err, ports.ErrNoTerminalCheckResult) {
		t.Fatalf("infrastructure failure mapped to domain absence sentinel: %v", err)
	}
}
