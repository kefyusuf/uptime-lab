package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
	goosedatabase "github.com/pressly/goose/v3/database"
)

var (
	// ErrSchemaIncompatible is the stable readiness result for migration-state mismatch or unreadable metadata.
	ErrSchemaIncompatible = errors.New("database schema is incompatible")

	// ErrInvalidMigrationSources means the embedded repository migration source set is not a valid compatibility target.
	ErrInvalidMigrationSources = errors.New("invalid embedded migration sources")
)

// CompatibilityChecker verifies that database migration metadata exactly matches repository-owned migrations.
// Check is read-only: it never creates or mutates migration metadata or application schema.
type CompatibilityChecker struct {
	db       *sql.DB
	store    goosedatabase.Store
	expected map[int64]struct{}
}

// NewCompatibilityChecker constructs a read-only migration-state checker without querying the database.
func NewCompatibilityChecker(db *sql.DB) (*CompatibilityChecker, error) {
	if db == nil {
		return nil, errors.New("compatibility database is required")
	}

	provider, err := NewProvider(db)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMigrationSources, err)
	}

	expected, err := expectedPositiveVersions(provider.ListSources())
	if err != nil {
		return nil, err
	}

	store, err := goosedatabase.NewStore(goosedatabase.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return nil, fmt.Errorf("construct migration metadata store: %w", err)
	}

	return &CompatibilityChecker{
		db:       db,
		store:    store,
		expected: expected,
	}, nil
}

// Check reports whether the existing database migration metadata exactly matches the embedded repository migrations.
func (checker *CompatibilityChecker) Check(ctx context.Context) error {
	rows, err := checker.store.ListMigrations(ctx, checker.db)
	if err != nil {
		return ErrSchemaIncompatible
	}

	return validateAppliedMigrations(checker.expected, rows)
}

func expectedPositiveVersions(sources []*goose.Source) (map[int64]struct{}, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("%w: no migrations", ErrInvalidMigrationSources)
	}

	expected := make(map[int64]struct{}, len(sources))
	for _, source := range sources {
		if source == nil || source.Version <= 0 {
			return nil, fmt.Errorf("%w: migration version must be positive", ErrInvalidMigrationSources)
		}
		if _, exists := expected[source.Version]; exists {
			return nil, fmt.Errorf("%w: duplicate migration version %d", ErrInvalidMigrationSources, source.Version)
		}
		expected[source.Version] = struct{}{}
	}

	return expected, nil
}

func validateAppliedMigrations(expected map[int64]struct{}, rows []*goosedatabase.ListMigrationsResult) error {
	seen := make(map[int64]struct{}, len(rows))
	zeroCount := 0

	for _, row := range rows {
		if row == nil || row.Version < 0 {
			return ErrSchemaIncompatible
		}
		if _, duplicate := seen[row.Version]; duplicate {
			return ErrSchemaIncompatible
		}
		seen[row.Version] = struct{}{}

		if !row.IsApplied {
			return ErrSchemaIncompatible
		}
		if row.Version == 0 {
			zeroCount++
			continue
		}
		if _, known := expected[row.Version]; !known {
			return ErrSchemaIncompatible
		}
	}

	if zeroCount != 1 {
		return ErrSchemaIncompatible
	}

	for version := range expected {
		if _, applied := seen[version]; !applied {
			return ErrSchemaIncompatible
		}
	}

	return nil
}
