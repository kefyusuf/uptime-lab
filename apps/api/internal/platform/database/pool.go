package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds a PostgreSQL pool from libpq-compatible PG environment settings.
//
// Pool construction is intentionally lazy with respect to connectivity. Operational
// readiness owns live database availability checks.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL configuration: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	return pool, nil
}
