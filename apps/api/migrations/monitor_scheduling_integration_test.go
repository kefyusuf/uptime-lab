//go:build integration

package migrations

import (
	"context"
	"errors"
	"testing"
)

func TestSchedulingMigrationDefaultsExistingAndNewMonitors(t *testing.T) {
	db := openIntegrationDB(t)
	resetMigrationState(t, db)
	provider, err := NewProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = provider.UpTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	const oldID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	const newID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"
	if _, err = db.ExecContext(ctx, `INSERT INTO monitoring.monitors(id,target_url,created_at) VALUES($1,'https://example.com',now())`, oldID); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO monitoring.monitors(id,target_url,created_at) VALUES($1,'https://example.com',now())`, newID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{oldID, newID} {
		var paused bool
		if err = db.QueryRowContext(ctx, `SELECT paused FROM monitoring.monitors WHERE id=$1`, id).Scan(&paused); err != nil || paused {
			t.Fatalf("default state for %s: %v %v", id, paused, err)
		}
	}
	if err = mustCompatibilityChecker(t, db).Check(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if err = mustCompatibilityChecker(t, db).Check(ctx); !errors.Is(err, ErrSchemaIncompatible) {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM monitoring.monitors`).Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err = mustCompatibilityChecker(t, db).Check(ctx); err != nil {
		t.Fatal(err)
	}
}
