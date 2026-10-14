//go:build integration

package migrations

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInventoryIndexMigration(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	provider, err := NewProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.UpTo(ctx, 2); err != nil {
		t.Fatal(err)
	}
	checker := mustCompatibilityChecker(t, db)
	if err := checker.Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatalf("missing inventory migration must be incompatible: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := checker.Check(ctx); err != nil {
		t.Fatal(err)
	}
	var definition string
	var unique bool
	if err := db.QueryRowContext(ctx, `SELECT pg_get_indexdef(indexrelid), indisunique FROM pg_index WHERE indexrelid=to_regclass('monitoring.monitors_inventory_order_idx')`).Scan(&definition, &unique); err != nil {
		t.Fatal(err)
	}
	if unique || !strings.Contains(definition, "USING btree (created_at DESC, id DESC)") {
		t.Fatal(definition, unique)
	}
	var plan string
	if err := db.QueryRowContext(ctx, `EXPLAIN SELECT id FROM monitoring.monitors WHERE (created_at,id)<('2026-10-05T00:00:00Z'::timestamptz,'018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2'::uuid) ORDER BY created_at DESC,id DESC LIMIT 21`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan == "" {
		t.Fatal("missing query plan")
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('monitoring.monitors_inventory_order_idx') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Fatal(exists, err)
	}
	if err := checker.Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := checker.Check(ctx); err != nil {
		t.Fatal(err)
	}
}
