//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCheckRunsMigrationSchemaAndInvariants(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	applyMigrationsUp(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	assertCheckRunsColumns(t, ctx, db)
	assertCheckRunsForeignKey(t, ctx, db)
	assertCheckRunsIndexes(t, ctx, db)

	monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	if _, err := db.ExecContext(
		ctx,
		"INSERT INTO monitoring.monitors (id, target_url, created_at) VALUES ($1, $2, $3)",
		monitorID,
		"https://example.com/health",
		time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("insert monitor fixture: %v", err)
	}

	t.Run("valid pending row succeeds", func(t *testing.T) {
		insertCheckRunExpectSuccess(
			t,
			ctx,
			db,
			"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3",
			monitorID,
			time.Date(2026, time.September, 26, 9, 1, 0, 0, time.UTC),
			time.Date(2026, time.September, 26, 9, 1, 20, 0, time.UTC),
			nil,
			nil,
			nil,
			nil,
		)
	})

	t.Run("second pending row for same monitor fails", func(t *testing.T) {
		insertCheckRunExpectFailure(
			t,
			ctx,
			db,
			"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb4",
			monitorID,
			time.Date(2026, time.September, 26, 9, 1, 1, 0, time.UTC),
			time.Date(2026, time.September, 26, 9, 1, 21, 0, time.UTC),
			nil,
			nil,
			nil,
			nil,
		)
	})

	if _, err := db.ExecContext(
		ctx,
		`UPDATE monitoring.check_runs
		 SET completed_at = $2,
		     result_kind = 'worker_timeout'
		 WHERE id = $1`,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3",
		time.Date(2026, time.September, 26, 9, 1, 20, 0, time.UTC),
	); err != nil {
		t.Fatalf("terminalize pending fixture: %v", err)
	}

	t.Run("multiple terminal historical rows are valid", func(t *testing.T) {
		insertCheckRunExpectSuccess(
			t,
			ctx,
			db,
			"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb5",
			monitorID,
			time.Date(2026, time.September, 26, 9, 3, 0, 0, time.UTC),
			time.Date(2026, time.September, 26, 9, 3, 20, 0, time.UTC),
			timePtr(time.Date(2026, time.September, 26, 9, 3, 1, 0, time.UTC)),
			stringPtr("http_response"),
			intPtr(204),
			intPtr(321),
		)
		insertCheckRunExpectSuccess(
			t,
			ctx,
			db,
			"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb6",
			monitorID,
			time.Date(2026, time.September, 26, 9, 4, 0, 0, time.UTC),
			time.Date(2026, time.September, 26, 9, 4, 20, 0, time.UTC),
			timePtr(time.Date(2026, time.September, 26, 9, 4, 2, 0, time.UTC)),
			stringPtr("timeout"),
			nil,
			intPtr(20000),
		)
	})

	tests := []struct {
		name        string
		completedAt *time.Time
		resultKind  *string
		httpStatus  *int
		durationMS  *int
		issuedAt    time.Time
		deadlineAt  time.Time
	}{
		{
			name:       "pending cannot carry result kind",
			resultKind: stringPtr("timeout"),
			issuedAt:   time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt: time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "terminal requires result kind",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "http response requires status",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("http_response"),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "http response status below range fails",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("http_response"),
			httpStatus:  intPtr(99),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "http response duration above range fails",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("http_response"),
			httpStatus:  intPtr(200),
			durationMS:  intPtr(20001),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "failure cannot carry HTTP status",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("dns_error"),
			httpStatus:  intPtr(500),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "failure requires duration",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("connect_error"),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "worker timeout cannot carry duration",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC)),
			resultKind:  stringPtr("worker_timeout"),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:        "unknown result kind fails",
			completedAt: timePtr(time.Date(2026, time.September, 26, 10, 0, 1, 0, time.UTC)),
			resultKind:  stringPtr("future_kind"),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
		{
			name:       "deadline must follow issued at",
			issuedAt:   time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt: time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
		},
		{
			name:        "completion cannot precede issued at",
			completedAt: timePtr(time.Date(2026, time.September, 26, 9, 59, 59, 0, time.UTC)),
			resultKind:  stringPtr("timeout"),
			durationMS:  intPtr(10),
			issuedAt:    time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC),
			deadlineAt:  time.Date(2026, time.September, 26, 10, 0, 20, 0, time.UTC),
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			insertCheckRunExpectFailure(
				t,
				ctx,
				db,
				fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-%012x", 0x500+index),
				monitorID,
				test.issuedAt,
				test.deadlineAt,
				test.completedAt,
				test.resultKind,
				test.httpStatus,
				test.durationMS,
			)
		})
	}
}

func assertCheckRunsColumns(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'monitoring'
		  AND table_name = 'check_runs'
		ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("query check_runs columns: %v", err)
	}
	defer rows.Close()

	type column struct {
		name       string
		dataType   string
		isNullable string
	}
	var got []column
	for rows.Next() {
		var value column
		if err := rows.Scan(&value.name, &value.dataType, &value.isNullable); err != nil {
			t.Fatalf("scan check_runs column: %v", err)
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate check_runs columns: %v", err)
	}

	want := []column{
		{name: "id", dataType: "uuid", isNullable: "NO"},
		{name: "monitor_id", dataType: "uuid", isNullable: "NO"},
		{name: "issued_at", dataType: "timestamp with time zone", isNullable: "NO"},
		{name: "deadline_at", dataType: "timestamp with time zone", isNullable: "NO"},
		{name: "completed_at", dataType: "timestamp with time zone", isNullable: "YES"},
		{name: "result_kind", dataType: "text", isNullable: "YES"},
		{name: "http_status", dataType: "integer", isNullable: "YES"},
		{name: "duration_ms", dataType: "integer", isNullable: "YES"},
	}
	if len(got) != len(want) {
		t.Fatalf("monitoring.check_runs columns = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("monitoring.check_runs column[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func assertCheckRunsTableDown(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var exists bool
	if err := db.QueryRowContext(
		ctx,
		"SELECT to_regclass('monitoring.check_runs') IS NOT NULL",
	).Scan(&exists); err != nil {
		t.Fatalf("query check_runs existence: %v", err)
	}
	if exists {
		t.Fatal("monitoring.check_runs still exists after rolling back migration 00002")
	}
}

func assertCheckRunsForeignKey(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var referencedTable string
	if err := db.QueryRowContext(ctx, `
		SELECT referenced.relname
		FROM pg_constraint constraint_def
		JOIN pg_class source
		  ON source.oid = constraint_def.conrelid
		JOIN pg_class referenced
		  ON referenced.oid = constraint_def.confrelid
		JOIN pg_namespace namespace
		  ON namespace.oid = source.relnamespace
		WHERE namespace.nspname = 'monitoring'
		  AND source.relname = 'check_runs'
		  AND constraint_def.contype = 'f'
	`).Scan(&referencedTable); err != nil {
		t.Fatalf("query check_runs foreign key: %v", err)
	}
	if referencedTable != "monitors" {
		t.Fatalf("check_runs foreign key references %q, want monitors", referencedTable)
	}
}

func assertCheckRunsIndexes(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'monitoring'
		  AND tablename = 'check_runs'
		ORDER BY indexname
	`)
	if err != nil {
		t.Fatalf("query check_runs indexes: %v", err)
	}
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatalf("scan check_runs index: %v", err)
		}
		got[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate check_runs indexes: %v", err)
	}

	for _, name := range []string{
		"check_runs_pkey",
		"check_runs_one_pending_per_monitor_idx",
		"check_runs_latest_terminal_idx",
	} {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing check_runs index %q; got %#v", name, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("check_runs index count = %d, want 3: %#v", len(got), got)
	}

	pending := got["check_runs_one_pending_per_monitor_idx"]
	if !strings.Contains(pending, "UNIQUE") ||
		!strings.Contains(pending, "(monitor_id)") ||
		!strings.Contains(pending, "(completed_at IS NULL)") {
		t.Fatalf("pending index definition = %q", pending)
	}

	latest := got["check_runs_latest_terminal_idx"]
	if !strings.Contains(latest, "(monitor_id, completed_at DESC)") ||
		!strings.Contains(latest, "(completed_at IS NOT NULL)") {
		t.Fatalf("latest terminal index definition = %q", latest)
	}
}

func insertCheckRunExpectSuccess(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	id string,
	monitorID string,
	issuedAt time.Time,
	deadlineAt time.Time,
	completedAt *time.Time,
	resultKind *string,
	httpStatus *int,
	durationMS *int,
) {
	t.Helper()
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO monitoring.check_runs (
			id, monitor_id, issued_at, deadline_at, completed_at, result_kind, http_status, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id,
		monitorID,
		issuedAt,
		deadlineAt,
		completedAt,
		resultKind,
		httpStatus,
		durationMS,
	); err != nil {
		t.Fatalf("insert valid check run: %v", err)
	}
}

func insertCheckRunExpectFailure(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	id string,
	monitorID string,
	issuedAt time.Time,
	deadlineAt time.Time,
	completedAt *time.Time,
	resultKind *string,
	httpStatus *int,
	durationMS *int,
) {
	t.Helper()
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO monitoring.check_runs (
			id, monitor_id, issued_at, deadline_at, completed_at, result_kind, http_status, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id,
		monitorID,
		issuedAt,
		deadlineAt,
		completedAt,
		resultKind,
		httpStatus,
		durationMS,
	); err == nil {
		t.Fatal("invalid check run insert unexpectedly succeeded")
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
