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

func TestRepositoryLatestTerminalByMonitorIDDistinguishesMissingMonitorAndNoResult(t *testing.T) {
	t.Run("missing Monitor", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)

		_, err := repository.LatestTerminalByMonitorID(
			context.Background(),
			mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe001"),
		)
		if !errors.Is(err, ports.ErrMonitorNotFound) {
			t.Fatalf("LatestTerminalByMonitorID() error = %v, want ErrMonitorNotFound", err)
		}
	})

	t.Run("Monitor without CheckRuns", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe002")
		insertExecutionMonitor(
			t,
			pool,
			monitorID,
			"https://example.com/no-result",
			time.Date(2026, time.September, 29, 10, 0, 0, 0, time.UTC),
		)

		_, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
		if !errors.Is(err, ports.ErrNoTerminalCheckResult) {
			t.Fatalf("LatestTerminalByMonitorID() error = %v, want ErrNoTerminalCheckResult", err)
		}
	})

	t.Run("pending-only Monitor", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe003")
		issuedAt := time.Date(2026, time.September, 29, 10, 0, 0, 0, time.UTC)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/pending-only", issuedAt.Add(-time.Hour))
		insertPendingRun(
			t,
			pool,
			mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe103"),
			monitorID,
			issuedAt,
			issuedAt.Add(20*time.Second),
		)

		_, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
		if !errors.Is(err, ports.ErrNoTerminalCheckResult) {
			t.Fatalf("LatestTerminalByMonitorID() error = %v, want ErrNoTerminalCheckResult", err)
		}
	})
}

func TestRepositoryLatestTerminalByMonitorIDReturnsTerminalShapes(t *testing.T) {
	tests := []struct {
		name       string
		checkID    string
		kind       domain.CheckResultKind
		httpStatus *int
		durationMS *int64
	}{
		{
			name:       "HTTP response",
			checkID:    "018f22d3-1d6a-7cc0-a37b-46fc3fafe201",
			kind:       domain.CheckResultHTTPResponse,
			httpStatus: latestIntPointer(204),
			durationMS: latestInt64Pointer(123),
		},
		{
			name:       "classified failure",
			checkID:    "018f22d3-1d6a-7cc0-a37b-46fc3fafe202",
			kind:       domain.CheckResultPolicyRejected,
			durationMS: latestInt64Pointer(0),
		},
		{
			name:    "worker timeout",
			checkID: "018f22d3-1d6a-7cc0-a37b-46fc3fafe203",
			kind:    domain.CheckResultWorkerTimeout,
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pool := openIntegrationPool(t)
			resetMonitors(t, pool)
			repository := NewRepository(pool)

			monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe010")
			issuedAt := time.Date(2026, time.September, 29, 11, index, 0, 0, time.UTC)
			completedAt := issuedAt.Add(5 * time.Second)
			checkID := mustExecutionCheckID(t, test.checkID)
			insertExecutionMonitor(t, pool, monitorID, "https://example.com/result-shape", issuedAt.Add(-time.Hour))
			insertLatestTerminalRun(
				t,
				pool,
				checkID,
				monitorID,
				issuedAt,
				completedAt,
				test.kind,
				test.httpStatus,
				test.durationMS,
			)

			got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
			if err != nil {
				t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
			}
			assertLatestResultRecord(
				t,
				got,
				checkID,
				test.kind,
				test.httpStatus,
				test.durationMS,
				completedAt,
			)
		})
	}
}

func TestRepositoryLatestTerminalByMonitorIDUsesDeterministicTerminalOrdering(t *testing.T) {
	t.Run("later completion wins", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe020")
		base := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/order-completion", base.Add(-time.Hour))

		older := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe210")
		newer := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe211")
		insertLatestTerminalRun(t, pool, older, monitorID, base, base.Add(5*time.Second), domain.CheckResultTimeout, nil, latestInt64Pointer(1))
		insertLatestTerminalRun(t, pool, newer, monitorID, base.Add(10*time.Second), base.Add(15*time.Second), domain.CheckResultTimeout, nil, latestInt64Pointer(2))

		got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
		if err != nil {
			t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
		}
		if got.CheckID != newer {
			t.Fatalf("CheckID = %s, want latest completion %s", got.CheckID, newer)
		}
	})

	t.Run("same completion uses later issued_at", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe021")
		base := time.Date(2026, time.September, 29, 12, 10, 0, 0, time.UTC)
		completedAt := base.Add(10 * time.Second)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/order-issued", base.Add(-time.Hour))

		earlierIssued := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe212")
		laterIssued := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe213")
		insertLatestTerminalRun(t, pool, earlierIssued, monitorID, base, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(1))
		insertLatestTerminalRun(t, pool, laterIssued, monitorID, base.Add(time.Second), completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(2))

		got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
		if err != nil {
			t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
		}
		if got.CheckID != laterIssued {
			t.Fatalf("CheckID = %s, want issued_at tie-break %s", got.CheckID, laterIssued)
		}
	})

	t.Run("same completion and issuance use UUID tie break", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		repository := NewRepository(pool)
		monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe022")
		issuedAt := time.Date(2026, time.September, 29, 12, 20, 0, 0, time.UTC)
		completedAt := issuedAt.Add(5 * time.Second)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/order-id", issuedAt.Add(-time.Hour))

		lower := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe214")
		higher := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe215")
		insertLatestTerminalRun(t, pool, lower, monitorID, issuedAt, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(1))
		insertLatestTerminalRun(t, pool, higher, monitorID, issuedAt, completedAt, domain.CheckResultTimeout, nil, latestInt64Pointer(2))

		got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
		if err != nil {
			t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
		}
		if got.CheckID != higher {
			t.Fatalf("CheckID = %s, want UUID tie-break %s", got.CheckID, higher)
		}
	})
}

func TestRepositoryLatestTerminalByMonitorIDIgnoresNewerPendingRun(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe030")
	base := time.Date(2026, time.September, 29, 13, 0, 0, 0, time.UTC)
	terminalID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe230")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/pending-invisible", base.Add(-time.Hour))
	insertLatestTerminalRun(
		t,
		pool,
		terminalID,
		monitorID,
		base,
		base.Add(5*time.Second),
		domain.CheckResultPolicyRejected,
		nil,
		latestInt64Pointer(0),
	)
	insertPendingRun(
		t,
		pool,
		mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe231"),
		monitorID,
		base.Add(10*time.Second),
		base.Add(30*time.Second),
	)

	got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
	if err != nil {
		t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
	}
	if got.CheckID != terminalID {
		t.Fatalf("CheckID = %s, want previous terminal %s", got.CheckID, terminalID)
	}
}

func TestRepositoryLatestTerminalByMonitorIDDoesNotReconcileExpiredPendingRun(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe040")
	base := time.Date(2026, time.September, 29, 13, 10, 0, 0, time.UTC)
	terminalID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe240")
	pendingID := mustExecutionCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe241")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/read-only", base.Add(-time.Hour))
	insertLatestTerminalRun(
		t,
		pool,
		terminalID,
		monitorID,
		base.Add(-time.Minute),
		base.Add(-55*time.Second),
		domain.CheckResultTimeout,
		nil,
		latestInt64Pointer(1),
	)
	insertPendingRun(t, pool, pendingID, monitorID, base.Add(-30*time.Second), base.Add(-10*time.Second))

	got, err := repository.LatestTerminalByMonitorID(context.Background(), monitorID)
	if err != nil {
		t.Fatalf("LatestTerminalByMonitorID() error = %v", err)
	}
	if got.CheckID != terminalID {
		t.Fatalf("CheckID = %s, want terminal %s", got.CheckID, terminalID)
	}

	var (
		completedAt *time.Time
		resultKind  *string
		httpStatus  *int
		durationMS  *int
	)
	if err := pool.QueryRow(
		context.Background(),
		`
			SELECT completed_at, result_kind, http_status, duration_ms
			FROM monitoring.check_runs
			WHERE id = $1::uuid
		`,
		pendingID.String(),
	).Scan(&completedAt, &resultKind, &httpStatus, &durationMS); err != nil {
		t.Fatalf("query expired pending row after read: %v", err)
	}
	if completedAt != nil || resultKind != nil || httpStatus != nil || durationMS != nil {
		t.Fatalf(
			"latest-result read mutated expired pending row: completed=%v kind=%v status=%v duration=%v",
			completedAt,
			resultKind,
			httpStatus,
			durationMS,
		)
	}
}

func TestRepositoryLatestTerminalByMonitorIDRespectsCancelledContext(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe050")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repository.LatestTerminalByMonitorID(ctx, monitorID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LatestTerminalByMonitorID() error = %v, want context.Canceled", err)
	}
}

func TestRepositoryLatestTerminalByMonitorIDSurfacesPersistenceFailure(t *testing.T) {
	pool := openIntegrationPool(t)
	repository := NewRepository(pool)
	pool.Close()

	_, err := repository.LatestTerminalByMonitorID(
		context.Background(),
		mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafe060"),
	)
	if err == nil {
		t.Fatal("LatestTerminalByMonitorID() error = nil, want persistence failure")
	}
	if errors.Is(err, ports.ErrMonitorNotFound) || errors.Is(err, ports.ErrNoTerminalCheckResult) {
		t.Fatalf("infrastructure failure mapped to domain absence sentinel: %v", err)
	}
}

func insertLatestTerminalRun(
	t *testing.T,
	pool *pgxpool.Pool,
	checkID domain.CheckID,
	monitorID domain.MonitorID,
	issuedAt time.Time,
	completedAt time.Time,
	kind domain.CheckResultKind,
	httpStatus *int,
	durationMS *int64,
) {
	t.Helper()

	var statusArg any
	if httpStatus != nil {
		statusArg = *httpStatus
	}
	var durationArg any
	if durationMS != nil {
		durationArg = *durationMS
	}

	if _, err := pool.Exec(
		context.Background(),
		`
			INSERT INTO monitoring.check_runs (
				id,
				monitor_id,
				issued_at,
				deadline_at,
				completed_at,
				result_kind,
				http_status,
				duration_ms
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
		`,
		checkID.String(),
		monitorID.String(),
		issuedAt,
		issuedAt.Add(20*time.Second),
		completedAt,
		string(kind),
		statusArg,
		durationArg,
	); err != nil {
		t.Fatalf("insert latest terminal run: %v", err)
	}
}

func assertLatestResultRecord(
	t *testing.T,
	got ports.LatestCheckResultRecord,
	wantCheckID domain.CheckID,
	wantKind domain.CheckResultKind,
	wantHTTPStatus *int,
	wantDurationMS *int64,
	wantCompletedAt time.Time,
) {
	t.Helper()

	if got.CheckID != wantCheckID {
		t.Fatalf("CheckID = %s, want %s", got.CheckID, wantCheckID)
	}
	if got.ResultKind != wantKind {
		t.Fatalf("ResultKind = %q, want %q", got.ResultKind, wantKind)
	}
	if !equalLatestOptionalInt(got.HTTPStatus, wantHTTPStatus) {
		t.Fatalf("HTTPStatus = %v, want %v", got.HTTPStatus, wantHTTPStatus)
	}
	if !equalLatestOptionalInt64(got.DurationMS, wantDurationMS) {
		t.Fatalf("DurationMS = %v, want %v", got.DurationMS, wantDurationMS)
	}
	if !got.CompletedAt.Equal(wantCompletedAt) {
		t.Fatalf("CompletedAt = %v, want %v", got.CompletedAt, wantCompletedAt)
	}
}

func latestIntPointer(value int) *int {
	return &value
}

func latestInt64Pointer(value int64) *int64 {
	return &value
}

func equalLatestOptionalInt(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}

func equalLatestOptionalInt64(got, want *int64) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
