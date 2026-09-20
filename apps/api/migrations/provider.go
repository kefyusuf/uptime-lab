package migrations

import (
	"database/sql"
	"errors"

	"github.com/pressly/goose/v3"
)

// NewProvider creates the PostgreSQL migration provider backed by embedded SQL migrations.
func NewProvider(db *sql.DB) (*goose.Provider, error) {
	if db == nil {
		return nil, errors.New("migration database is required")
	}

	return goose.NewProvider(
		goose.DialectPostgres,
		db,
		FS,
		goose.WithDisableGlobalRegistry(true),
	)
}
