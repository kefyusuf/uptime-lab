package database

import (
	"context"
	"testing"
	"time"
)

func TestNewPoolRejectsInvalidPostgresConfiguration(t *testing.T) {
	setPostgresEnv(t, "127.0.0.1", "not-a-port", "uptime_lab_test")

	if _, err := NewPool(context.Background()); err == nil {
		t.Fatal("NewPool() error = nil, want invalid PostgreSQL configuration error")
	}
}

func TestNewPoolDoesNotRequireDatabaseAvailabilityAtConstruction(t *testing.T) {
	setPostgresEnv(t, "127.0.0.1", "1", "uptime_lab_test")
	t.Setenv("PGCONNECT_TIMEOUT", "1")

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	if err := pool.Ping(ctx); err == nil {
		t.Fatal("pool.Ping() error = nil, want unavailable database error")
	}
}

func TestPoolCanBeClosedDuringShutdown(t *testing.T) {
	setPostgresEnv(t, "127.0.0.1", "1", "uptime_lab_test")

	pool, err := NewPool(context.Background())
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("Ping() after Close() error = nil, want closed-pool error")
	}
}

func setPostgresEnv(t *testing.T, host, port, database string) {
	t.Helper()

	t.Setenv("PGHOST", host)
	t.Setenv("PGPORT", port)
	t.Setenv("PGDATABASE", database)
	t.Setenv("PGUSER", "uptime_lab_test")
	t.Setenv("PGPASSWORD", "uptime_lab_test_password")
	t.Setenv("PGSSLMODE", "disable")
}
