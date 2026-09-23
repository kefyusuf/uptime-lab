//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

type migrationMetadataRow struct {
	id        int64
	version   int64
	isApplied bool
	timestamp time.Time
}

func TestCompatibilityCheckerAgainstPostgreSQL(t *testing.T) {
	db := openIntegrationDB(t)

	t.Run("fresh database is incompatible without mutation", func(t *testing.T) {
		resetMigrationState(t, db)
		checker := mustCompatibilityChecker(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := checker.Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
			t.Fatalf("Check() error = %v, want ErrSchemaIncompatible", err)
		}

		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.goose_db_version') IS NOT NULL").Scan(&exists); err != nil {
			t.Fatalf("query goose metadata existence: %v", err)
		}
		if exists {
			t.Fatal("Check() created public.goose_db_version")
		}
	})

	t.Run("explicit migration makes schema compatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		checker := mustCompatibilityChecker(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := checker.Check(ctx); err != nil {
			t.Fatalf("Check() error = %v", err)
		}
	})

	t.Run("rollback makes schema incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		provider := applyMigrationsUp(t, db)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := provider.Down(ctx); err != nil {
			t.Fatalf("Down() error = %v", err)
		}

		checker := mustCompatibilityChecker(t, db)
		if err := checker.Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
			t.Fatalf("Check() error = %v, want ErrSchemaIncompatible", err)
		}
	})

	t.Run("unknown applied version is incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		execMetadata(t, db, "INSERT INTO public.goose_db_version (version_id, is_applied) VALUES (999, true)")
		assertIncompatible(t, db)
	})

	t.Run("duplicate metadata is incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		execMetadata(t, db, "INSERT INTO public.goose_db_version (version_id, is_applied) VALUES (1, true)")
		assertIncompatible(t, db)
	})

	t.Run("unapplied metadata is incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		execMetadata(t, db, "UPDATE public.goose_db_version SET is_applied = false WHERE version_id = 1")
		assertIncompatible(t, db)
	})

	t.Run("missing zero metadata is incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		execMetadata(t, db, "DELETE FROM public.goose_db_version WHERE version_id = 0")
		assertIncompatible(t, db)
	})

	t.Run("invalid zero metadata is incompatible", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		execMetadata(t, db, "UPDATE public.goose_db_version SET is_applied = false WHERE version_id = 0")
		assertIncompatible(t, db)
	})

	t.Run("check does not mutate migration metadata", func(t *testing.T) {
		resetMigrationState(t, db)
		applyMigrationsUp(t, db)
		before := readMigrationMetadata(t, db)

		checker := mustCompatibilityChecker(t, db)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := checker.Check(ctx); err != nil {
			t.Fatalf("Check() error = %v", err)
		}

		after := readMigrationMetadata(t, db)
		assertMigrationMetadataEqual(t, before, after)
	})
}

func mustCompatibilityChecker(t *testing.T, db *sql.DB) *CompatibilityChecker {
	t.Helper()
	checker, err := NewCompatibilityChecker(db)
	if err != nil {
		t.Fatalf("NewCompatibilityChecker() error = %v", err)
	}
	return checker
}

func applyMigrationsUp(t *testing.T, db *sql.DB) *goose.Provider {
	t.Helper()
	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	return provider
}

func assertIncompatible(t *testing.T, db *sql.DB) {
	t.Helper()
	checker := mustCompatibilityChecker(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := checker.Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("Check() error = %v, want ErrSchemaIncompatible", err)
	}
}

func execMetadata(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, statement); err != nil {
		t.Fatalf("mutate migration metadata fixture: %v", err)
	}
}

func readMigrationMetadata(t *testing.T, db *sql.DB) []migrationMetadataRow {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT id, version_id, is_applied, tstamp
		FROM public.goose_db_version
		ORDER BY id
	`)
	if err != nil {
		t.Fatalf("query migration metadata: %v", err)
	}
	defer rows.Close()

	var got []migrationMetadataRow
	for rows.Next() {
		var row migrationMetadataRow
		if err := rows.Scan(&row.id, &row.version, &row.isApplied, &row.timestamp); err != nil {
			t.Fatalf("scan migration metadata: %v", err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration metadata: %v", err)
	}
	return got
}

func assertMigrationMetadataEqual(t *testing.T, want, got []migrationMetadataRow) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("migration metadata row count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].id != want[i].id || got[i].version != want[i].version || got[i].isApplied != want[i].isApplied || !got[i].timestamp.Equal(want[i].timestamp) {
			t.Fatalf("migration metadata row[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
