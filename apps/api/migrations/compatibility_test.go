package migrations

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/pressly/goose/v3"
	goosedatabase "github.com/pressly/goose/v3/database"
)

func TestNewCompatibilityCheckerDoesNotRequireDatabaseConnectivity(t *testing.T) {
	checker, err := NewCompatibilityChecker(&sql.DB{})
	if err != nil {
		t.Fatalf("NewCompatibilityChecker() error = %v", err)
	}
	if checker == nil {
		t.Fatal("NewCompatibilityChecker() returned nil checker")
	}
}

func TestExpectedPositiveVersions(t *testing.T) {
	t.Run("accepts unique positive versions", func(t *testing.T) {
		got, err := expectedPositiveVersions([]*goose.Source{{Version: 1}, {Version: 3}})
		if err != nil {
			t.Fatalf("expectedPositiveVersions() error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected versions len = %d, want 2", len(got))
		}
		for _, version := range []int64{1, 3} {
			if _, ok := got[version]; !ok {
				t.Fatalf("expected version %d is missing", version)
			}
		}
	})

	tests := []struct {
		name    string
		sources []*goose.Source
	}{
		{name: "empty source set", sources: nil},
		{name: "nil source", sources: []*goose.Source{nil}},
		{name: "zero version", sources: []*goose.Source{{Version: 0}}},
		{name: "negative version", sources: []*goose.Source{{Version: -1}}},
		{name: "duplicate version", sources: []*goose.Source{{Version: 1}, {Version: 1}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := expectedPositiveVersions(test.sources)
			if !errors.Is(err, ErrInvalidMigrationSources) {
				t.Fatalf("expectedPositiveVersions() error = %v, want ErrInvalidMigrationSources", err)
			}
		})
	}
}

func TestValidateAppliedMigrations(t *testing.T) {
	expected := map[int64]struct{}{1: {}, 2: {}}
	row := func(version int64, applied bool) *goosedatabase.ListMigrationsResult {
		return &goosedatabase.ListMigrationsResult{Version: version, IsApplied: applied}
	}

	t.Run("exact set is compatible", func(t *testing.T) {
		err := validateAppliedMigrations(expected, []*goosedatabase.ListMigrationsResult{
			row(2, true),
			row(1, true),
			row(0, true),
		})
		if err != nil {
			t.Fatalf("validateAppliedMigrations() error = %v", err)
		}
	})

	tests := []struct {
		name string
		rows []*goosedatabase.ListMigrationsResult
	}{
		{name: "missing zero", rows: []*goosedatabase.ListMigrationsResult{row(2, true), row(1, true)}},
		{name: "duplicate zero", rows: []*goosedatabase.ListMigrationsResult{row(2, true), row(1, true), row(0, true), row(0, true)}},
		{name: "zero not applied", rows: []*goosedatabase.ListMigrationsResult{row(2, true), row(1, true), row(0, false)}},
		{name: "missing required version", rows: []*goosedatabase.ListMigrationsResult{row(1, true), row(0, true)}},
		{name: "unknown ahead version", rows: []*goosedatabase.ListMigrationsResult{row(3, true), row(2, true), row(1, true), row(0, true)}},
		{name: "duplicate positive version", rows: []*goosedatabase.ListMigrationsResult{row(2, true), row(1, true), row(1, true), row(0, true)}},
		{name: "positive version not applied", rows: []*goosedatabase.ListMigrationsResult{row(2, false), row(1, true), row(0, true)}},
		{name: "nil metadata row", rows: []*goosedatabase.ListMigrationsResult{row(2, true), nil, row(0, true)}},
		{name: "negative metadata version", rows: []*goosedatabase.ListMigrationsResult{row(2, true), row(1, true), row(-1, true), row(0, true)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAppliedMigrations(expected, test.rows)
			if !errors.Is(err, ErrSchemaIncompatible) {
				t.Fatalf("validateAppliedMigrations() error = %v, want ErrSchemaIncompatible", err)
			}
		})
	}
}
