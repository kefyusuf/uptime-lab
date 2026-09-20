//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)


func TestProviderMigratesMonitoringSchemaUpDownUp(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)

	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	assertMonitoringSchemaUp(t, ctx, db)

	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	assertMonitoringSchemaDown(t, ctx, db)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	assertMonitoringSchemaUp(t, ctx, db)
}

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	config, err := pgx.ParseConfig("")
	if err != nil {
		t.Fatalf("pgx.ParseConfig() error = %v", err)
	}

	db := stdlib.OpenDB(*config)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}

	return db
}

func resetMigrationState(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS monitoring CASCADE"); err != nil {
		t.Fatalf("drop monitoring schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS public.goose_db_version"); err != nil {
		t.Fatalf("drop goose metadata table: %v", err)
	}
}

func assertMonitoringSchemaUp(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'monitoring'
		  AND table_name = 'monitors'
		ORDER BY ordinal_position
	`)
	if err != nil {
		t.Fatalf("query monitoring columns: %v", err)
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
			t.Fatalf("scan monitoring column: %v", err)
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate monitoring columns: %v", err)
	}

	want := []column{
		{name: "id", dataType: "uuid", isNullable: "NO"},
		{name: "target_url", dataType: "text", isNullable: "NO"},
		{name: "created_at", dataType: "timestamp with time zone", isNullable: "NO"},
	}
	if len(got) != len(want) {
		t.Fatalf("monitoring.monitors columns = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("monitoring.monitors column[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}

	var primaryKeyColumn string
	if err := db.QueryRowContext(ctx, `
		SELECT attribute.attname
		FROM pg_index index_def
		JOIN pg_attribute attribute
		  ON attribute.attrelid = index_def.indrelid
		 AND attribute.attnum = ANY(index_def.indkey)
		WHERE index_def.indrelid = 'monitoring.monitors'::regclass
		  AND index_def.indisprimary
	`).Scan(&primaryKeyColumn); err != nil {
		t.Fatalf("query primary key: %v", err)
	}
	if primaryKeyColumn != "id" {
		t.Fatalf("primary key column = %q, want id", primaryKeyColumn)
	}

	var publicMetadata bool
	var monitoringMetadata bool
	if err := db.QueryRowContext(ctx, `
		SELECT
			to_regclass('public.goose_db_version') IS NOT NULL,
			to_regclass('monitoring.goose_db_version') IS NOT NULL
	`).Scan(&publicMetadata, &monitoringMetadata); err != nil {
		t.Fatalf("query goose metadata location: %v", err)
	}
	if !publicMetadata {
		t.Fatal("public.goose_db_version is missing")
	}
	if monitoringMetadata {
		t.Fatal("goose metadata must remain outside monitoring schema")
	}
}

func assertMonitoringSchemaDown(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	var schemaExists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_namespace
			WHERE nspname = 'monitoring'
		)
	`).Scan(&schemaExists); err != nil {
		t.Fatalf("query monitoring schema: %v", err)
	}
	if schemaExists {
		t.Fatal("monitoring schema still exists after Down()")
	}

	var publicMetadata bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.goose_db_version') IS NOT NULL").Scan(&publicMetadata); err != nil {
		t.Fatalf("query goose metadata after Down(): %v", err)
	}
	if !publicMetadata {
		t.Fatal("goose metadata must survive application schema rollback")
	}
}
