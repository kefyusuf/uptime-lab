//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIncidentsMigrationUpDown(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	provider := applyMigrationsUp(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	assertIncidentTablesExist(t, ctx, db, true)

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("incident Down() error = %v", err)
	}
	assertIncidentTablesExist(t, ctx, db, false)
	assertPriorTablesSurviveIncidentDown(t, ctx, db)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("incident second Up() error = %v", err)
	}
	assertIncidentTablesExist(t, ctx, db, true)
}

func TestIncidentsSchemaAndInvariants(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	applyMigrationsUp(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	assertIncidentsColumns(t, ctx, db)
	assertIncidentStateColumns(t, ctx, db)
	assertIncidentsIndexes(t, ctx, db)
	assertIncidentStateIndexes(t, ctx, db)
	assertIncidentsForeignKeys(t, ctx, db)
	assertIncidentStateForeignKeys(t, ctx, db)

	t.Run("one open incident per monitor", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc01"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)

		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc11", monitorID,
			time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC), nil, "unexpected_http_status", nil, nil)
		insertIncidentExpectFailure(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc12", monitorID,
			time.Date(2026, time.September, 26, 9, 5, 0, 0, time.UTC), nil, "probe_failure", nil, nil)
		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc13", monitorID,
			time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC),
			timePtr(time.Date(2026, time.September, 26, 8, 30, 0, 0, time.UTC)), "probe_failure", nil, nil)

		if _, err := db.ExecContext(ctx,
			"UPDATE monitoring.incidents SET resolved_at = $2 WHERE id = $1",
			"018f22d3-1d6a-7cc0-a37b-46fc3fafdc11",
			time.Date(2026, time.September, 26, 9, 30, 0, 0, time.UTC)); err != nil {
			t.Fatalf("resolve open incident fixture: %v", err)
		}
		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc14", monitorID,
			time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC), nil, "unexpected_http_status", nil, nil)
	})

	t.Run("opening reason is closed", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc02"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)
		opened := time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC)
		resolved := timePtr(time.Date(2026, time.September, 26, 9, 30, 0, 0, time.UTC))

		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc21", monitorID, opened, resolved, "unexpected_http_status", nil, nil)
		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc22", monitorID, opened, resolved, "probe_failure", nil, nil)
		insertIncidentExpectFailure(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc23", monitorID, opened, resolved, "future_reason", nil, nil)
		insertIncidentExpectFailure(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc24", monitorID, opened, nil, "future_reason", nil, nil)
	})

	t.Run("resolved at cannot precede opened at", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc03"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)
		opened := time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC)

		insertIncidentExpectFailure(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc31", monitorID, opened,
			timePtr(time.Date(2026, time.September, 26, 8, 59, 59, 0, time.UTC)), "probe_failure", nil, nil)
		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc32", monitorID, opened,
			timePtr(opened), "probe_failure", nil, nil)
		insertIncidentExpectSuccess(t, ctx, db, "018f22d3-1d6a-7cc0-a37b-46fc3fafdc33", monitorID, opened,
			timePtr(time.Date(2026, time.September, 26, 9, 30, 0, 0, time.UTC)), "probe_failure", nil, nil)
	})

	t.Run("incident state streaks are non negative", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc04"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)

		insertIncidentStateExpectFailure(t, ctx, db, monitorID, -1, 0, nil)
		insertIncidentStateExpectFailure(t, ctx, db, monitorID, 0, -1, nil)
		insertIncidentStateExpectSuccess(t, ctx, db, monitorID, 0, 0, nil)
		insertIncidentStateExpectSuccess(t, ctx, db, monitorID, 3, 2, nil)
	})

	t.Run("incident state open pointer references incidents", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc05"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)
		incidentID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc51"
		insertIncidentExpectSuccess(t, ctx, db, incidentID, monitorID,
			time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC), nil, "unexpected_http_status", nil, nil)

		insertIncidentStateExpectFailure(t, ctx, db, monitorID, 3, 0, stringPtr("018f22d3-1d6a-7cc0-a37b-46fc3fafdc99"))
		insertIncidentStateExpectSuccess(t, ctx, db, monitorID, 3, 0, stringPtr(incidentID))
	})

	t.Run("evidence links set null when check run is retained away", func(t *testing.T) {
		monitorID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc06"
		insertIncidentFixtureMonitor(t, ctx, db, monitorID)
		openCheckID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc61"
		resolveCheckID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc62"
		insertTerminalCheckRunFixture(t, ctx, db, openCheckID, monitorID, time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC))
		insertTerminalCheckRunFixture(t, ctx, db, resolveCheckID, monitorID, time.Date(2026, time.September, 26, 9, 30, 0, 0, time.UTC))

		incidentID := "018f22d3-1d6a-7cc0-a37b-46fc3fafdc63"
		insertIncidentExpectSuccess(t, ctx, db, incidentID, monitorID,
			time.Date(2026, time.September, 26, 9, 0, 0, 0, time.UTC),
			timePtr(time.Date(2026, time.September, 26, 9, 30, 0, 0, time.UTC)),
			"unexpected_http_status", stringPtr(openCheckID), stringPtr(resolveCheckID))

		if _, err := db.ExecContext(ctx, "DELETE FROM monitoring.check_runs WHERE id = $1", openCheckID); err != nil {
			t.Fatalf("delete opening evidence check run: %v", err)
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM monitoring.check_runs WHERE id = $1", resolveCheckID); err != nil {
			t.Fatalf("delete resolving evidence check run: %v", err)
		}

		var openedBy, resolvedBy *string
		var exists bool
		if err := db.QueryRowContext(ctx,
			"SELECT opened_by_check_id, resolved_by_check_id, true FROM monitoring.incidents WHERE id = $1",
			incidentID).Scan(&openedBy, &resolvedBy, &exists); err != nil {
			t.Fatalf("incident must survive evidence deletion: %v", err)
		}
		if openedBy != nil || resolvedBy != nil {
			t.Fatalf("evidence links = (%v, %v), want both null after SET NULL", openedBy, resolvedBy)
		}
	})
}

func TestIncidentsCompatibilityFiveMigrationSet(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	applyMigrationsUp(t, db)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	checker := mustCompatibilityChecker(t, db)
	if err := checker.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v, want nil for the five-migration set", err)
	}

	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	var versions []int64
	for _, source := range provider.ListSources() {
		versions = append(versions, source.Version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	want := []int64{1, 2, 3, 4, 5}
	if len(versions) != len(want) {
		t.Fatalf("migration versions = %v, want %v", versions, want)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Fatalf("migration version[%d] = %d, want %d", i, versions[i], want[i])
		}
	}
}

func assertIncidentTablesExist(t *testing.T, ctx context.Context, db *sql.DB, want bool) {
	t.Helper()
	for _, table := range []string{"monitoring.incidents", "monitoring.incident_state"} {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			t.Fatalf("query %s existence: %v", table, err)
		}
		if exists != want {
			t.Fatalf("%s exists = %v, want %v", table, exists, want)
		}
	}
}

func assertPriorTablesSurviveIncidentDown(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"monitoring.monitors", "monitoring.check_runs"} {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			t.Fatalf("query %s existence: %v", table, err)
		}
		if !exists {
			t.Fatalf("%s must survive incident rollback", table)
		}
	}
}

func assertIncidentsColumns(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringColumns(t, ctx, db, "incidents")
	want := []monitoringColumn{
		{name: "id", dataType: "uuid", isNullable: "NO"},
		{name: "monitor_id", dataType: "uuid", isNullable: "NO"},
		{name: "opened_at", dataType: "timestamp with time zone", isNullable: "NO"},
		{name: "resolved_at", dataType: "timestamp with time zone", isNullable: "YES"},
		{name: "opening_reason", dataType: "text", isNullable: "NO"},
		{name: "opened_by_check_id", dataType: "uuid", isNullable: "YES"},
		{name: "resolved_by_check_id", dataType: "uuid", isNullable: "YES"},
	}
	assertMonitoringColumns(t, "incidents", got, want)
}

func assertIncidentStateColumns(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringColumns(t, ctx, db, "incident_state")
	want := []monitoringColumn{
		{name: "monitor_id", dataType: "uuid", isNullable: "NO"},
		{name: "failure_streak", dataType: "integer", isNullable: "NO"},
		{name: "success_streak", dataType: "integer", isNullable: "NO"},
		{name: "open_incident_id", dataType: "uuid", isNullable: "YES"},
	}
	assertMonitoringColumns(t, "incident_state", got, want)
}

type monitoringColumn struct {
	name       string
	dataType   string
	isNullable string
}

func monitoringColumns(t *testing.T, ctx context.Context, db *sql.DB, table string) []monitoringColumn {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'monitoring'
		  AND table_name = $1
		ORDER BY ordinal_position
	`, table)
	if err != nil {
		t.Fatalf("query %s columns: %v", table, err)
	}
	defer rows.Close()

	var got []monitoringColumn
	for rows.Next() {
		var value monitoringColumn
		if err := rows.Scan(&value.name, &value.dataType, &value.isNullable); err != nil {
			t.Fatalf("scan %s column: %v", table, err)
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", table, err)
	}
	return got
}

func assertMonitoringColumns(t *testing.T, table string, got, want []monitoringColumn) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("monitoring.%s columns = %#v, want %#v", table, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("monitoring.%s column[%d] = %#v, want %#v", table, i, got[i], want[i])
		}
	}
}

func assertIncidentsIndexes(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringIndexes(t, ctx, db, "incidents")
	for _, name := range []string{"incidents_pkey", "incidents_one_open_per_monitor_idx", "incidents_monitor_opened_idx"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("missing incidents index %q; got %#v", name, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("incidents index count = %d, want 3: %#v", len(got), got)
	}
	oneOpen := got["incidents_one_open_per_monitor_idx"]
	if !strings.Contains(oneOpen, "UNIQUE") || !strings.Contains(oneOpen, "(monitor_id)") || !strings.Contains(oneOpen, "(resolved_at IS NULL)") {
		t.Fatalf("one-open index definition = %q", oneOpen)
	}
	opened := got["incidents_monitor_opened_idx"]
	if !strings.Contains(opened, "(monitor_id, opened_at DESC, id DESC)") {
		t.Fatalf("opened keyset index definition = %q", opened)
	}
}

func assertIncidentStateIndexes(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringIndexes(t, ctx, db, "incident_state")
	if _, ok := got["incident_state_pkey"]; !ok {
		t.Fatalf("missing incident_state primary key; got %#v", got)
	}
	if len(got) != 1 {
		t.Fatalf("incident_state index count = %d, want 1: %#v", len(got), got)
	}
}

func monitoringIndexes(t *testing.T, ctx context.Context, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE schemaname = 'monitoring'
		  AND tablename = $1
		ORDER BY indexname
	`, table)
	if err != nil {
		t.Fatalf("query %s indexes: %v", table, err)
	}
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			t.Fatalf("scan %s index: %v", table, err)
		}
		got[name] = definition
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s indexes: %v", table, err)
	}
	return got
}

type monitoringForeignKey struct {
	referenced   string
	deleteAction string
}

func monitoringForeignKeys(t *testing.T, ctx context.Context, db *sql.DB, table string) map[string]monitoringForeignKey {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
		SELECT attribute.attname, referenced.relname, constraint_def.confdeltype
		FROM pg_constraint constraint_def
		JOIN pg_class source
		  ON source.oid = constraint_def.conrelid
		JOIN pg_namespace namespace
		  ON namespace.oid = source.relnamespace
		JOIN pg_class referenced
		  ON referenced.oid = constraint_def.confrelid
		JOIN pg_attribute attribute
		  ON attribute.attrelid = constraint_def.conrelid
		 AND attribute.attnum = constraint_def.conkey[1]
		WHERE namespace.nspname = 'monitoring'
		  AND source.relname = $1
		  AND constraint_def.contype = 'f'
	`, table)
	if err != nil {
		t.Fatalf("query %s foreign keys: %v", table, err)
	}
	defer rows.Close()

	got := map[string]monitoringForeignKey{}
	for rows.Next() {
		var column, referenced, deleteAction string
		if err := rows.Scan(&column, &referenced, &deleteAction); err != nil {
			t.Fatalf("scan %s foreign key: %v", table, err)
		}
		got[column] = monitoringForeignKey{referenced: referenced, deleteAction: deleteAction}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s foreign keys: %v", table, err)
	}
	return got
}

func assertIncidentsForeignKeys(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringForeignKeys(t, ctx, db, "incidents")
	want := map[string]monitoringForeignKey{
		"monitor_id":           {referenced: "monitors", deleteAction: "a"},
		"opened_by_check_id":   {referenced: "check_runs", deleteAction: "n"},
		"resolved_by_check_id": {referenced: "check_runs", deleteAction: "n"},
	}
	if len(got) != len(want) {
		t.Fatalf("incidents foreign keys = %#v, want %#v", got, want)
	}
	for column, expected := range want {
		if got[column] != expected {
			t.Fatalf("incidents foreign key %q = %#v, want %#v", column, got[column], expected)
		}
	}
	for column, key := range got {
		if key.referenced == "incident_state" {
			t.Fatalf("incidents.%s references incident_state, creating a foreign key cycle", column)
		}
	}
}

func assertIncidentStateForeignKeys(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	got := monitoringForeignKeys(t, ctx, db, "incident_state")
	want := map[string]monitoringForeignKey{
		"monitor_id":       {referenced: "monitors", deleteAction: "a"},
		"open_incident_id": {referenced: "incidents", deleteAction: "a"},
	}
	if len(got) != len(want) {
		t.Fatalf("incident_state foreign keys = %#v, want %#v", got, want)
	}
	for column, expected := range want {
		if got[column] != expected {
			t.Fatalf("incident_state foreign key %q = %#v, want %#v", column, got[column], expected)
		}
	}
}

func insertIncidentFixtureMonitor(t *testing.T, ctx context.Context, db *sql.DB, monitorID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO monitoring.monitors (id, target_url, created_at) VALUES ($1, $2, $3)",
		monitorID, "https://example.com/health",
		time.Date(2026, time.September, 26, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("insert incident monitor fixture: %v", err)
	}
}

func insertTerminalCheckRunFixture(t *testing.T, ctx context.Context, db *sql.DB, checkID, monitorID string, completedAt time.Time) {
	t.Helper()
	insertCheckRunExpectSuccess(t, ctx, db, checkID, monitorID,
		completedAt.Add(-time.Second), completedAt.Add(20*time.Second),
		timePtr(completedAt), stringPtr("http_response"), intPtr(200), intPtr(100))
}

func insertIncidentExpectSuccess(t *testing.T, ctx context.Context, db *sql.DB, id, monitorID string, openedAt time.Time, resolvedAt *time.Time, openingReason string, openedByCheckID, resolvedByCheckID *string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO monitoring.incidents (
			id, monitor_id, opened_at, resolved_at, opening_reason, opened_by_check_id, resolved_by_check_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, monitorID, openedAt, resolvedAt, openingReason, openedByCheckID, resolvedByCheckID); err != nil {
		t.Fatalf("insert valid incident: %v", err)
	}
}

func insertIncidentExpectFailure(t *testing.T, ctx context.Context, db *sql.DB, id, monitorID string, openedAt time.Time, resolvedAt *time.Time, openingReason string, openedByCheckID, resolvedByCheckID *string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO monitoring.incidents (
			id, monitor_id, opened_at, resolved_at, opening_reason, opened_by_check_id, resolved_by_check_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, monitorID, openedAt, resolvedAt, openingReason, openedByCheckID, resolvedByCheckID); err == nil {
		t.Fatal("invalid incident insert unexpectedly succeeded")
	}
}

func insertIncidentStateExpectSuccess(t *testing.T, ctx context.Context, db *sql.DB, monitorID string, failureStreak, successStreak int, openIncidentID *string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO monitoring.incident_state (monitor_id, failure_streak, success_streak, open_incident_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (monitor_id) DO UPDATE
		SET failure_streak = EXCLUDED.failure_streak,
		    success_streak = EXCLUDED.success_streak,
		    open_incident_id = EXCLUDED.open_incident_id`,
		monitorID, failureStreak, successStreak, openIncidentID); err != nil {
		t.Fatalf("insert valid incident state: %v", err)
	}
}

func insertIncidentStateExpectFailure(t *testing.T, ctx context.Context, db *sql.DB, monitorID string, failureStreak, successStreak int, openIncidentID *string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO monitoring.incident_state (monitor_id, failure_streak, success_streak, open_incident_id)
		VALUES ($1, $2, $3, $4)`,
		monitorID, failureStreak, successStreak, openIncidentID); err == nil {
		t.Fatal("invalid incident state insert unexpectedly succeeded")
	}
}
