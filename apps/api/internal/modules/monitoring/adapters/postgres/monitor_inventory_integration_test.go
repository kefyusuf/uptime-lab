//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func TestInventoryEqualTimeUUIDOrder(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	at := time.Date(2026, 10, 5, 0, 0, 0, 123456000, time.UTC)
	ids := []string{"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"}
	for _, id := range ids {
		if err := repo.Create(ctx, mustMonitor(t, id, "http://web/", at)); err != nil {
			t.Fatal(err)
		}
	}
	newer := mustMonitor(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb1", "http://web/", at.Add(time.Microsecond))
	if err := repo.Create(ctx, newer); err != nil {
		t.Fatal(err)
	}
	first, err := repo.Candidates(ctx, nil, 2, 245760)
	if err != nil || len(first) != 2 || first[0].ID != newer.ID() || first[1].ID.String() != ids[1] {
		t.Fatal(first, err)
	}
	anchor := ports.InventoryAnchor{ID: first[1].ID, CreatedAt: first[1].CreatedAt}
	next, err := repo.Candidates(ctx, &anchor, 2, 245760)
	if err != nil || len(next) != 1 || next[0].ID.String() != ids[0] {
		t.Fatal(next, err)
	}
}

func TestInventoryPersistedMicroseconds(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	input := mustMonitor(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", "http://web/", time.Date(2026, 10, 5, 0, 0, 0, 123456789, time.UTC))
	if err := repo.Create(ctx, input); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.Candidates(ctx, nil, 2, 245760)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	stored, err := repo.ByID(ctx, input.ID())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CreatedAt.Nanosecond()%1000 != 0 || !rows[0].CreatedAt.Equal(stored.CreatedAt()) || rows[0].CreatedAt.Equal(input.CreatedAt()) {
		t.Fatal(rows[0], stored)
	}
	cursor, err := application.EncodeInventoryCursor(ports.InventoryAnchor{ID: rows[0].ID, CreatedAt: rows[0].CreatedAt})
	if err != nil {
		t.Fatal(err)
	}
	page, err := application.NewListMonitors(repo).Execute(ctx, application.InventoryInput{Limit: 20, Cursor: cursor})
	if err != nil || len(page.Items) != 0 {
		t.Fatal(page, err)
	}
}

func TestInventoryNonexistentAnchorAndConcurrentInsert(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	anchor := ports.InventoryAnchor{ID: mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"), CreatedAt: at}
	for i, delta := range []time.Duration{time.Microsecond, -time.Microsecond} {
		m := mustMonitor(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-%012x", 0x900+i), "http://web/", at.Add(delta))
		if err := repo.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.Candidates(ctx, &anchor, 21, 245760)
	if err != nil || len(rows) != 1 || !rows[0].CreatedAt.Before(at) {
		t.Fatal(rows, err)
	}
	anchor.CreatedAt = at.Add(-time.Hour)
	rows, err = repo.Candidates(ctx, &anchor, 21, 245760)
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}

func TestInventoryOversizedProjectionAndBounds(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	huge := mustMonitor(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", "http://web/"+strings.Repeat("x", 245760), at)
	if err := repo.Create(ctx, huge); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.Candidates(ctx, nil, 1, 245760)
	if err != nil || len(rows) != 1 || !rows[0].Oversized || rows[0].TargetURL != "" || rows[0].TargetBytes != int64(len(huge.TargetURL().String())) {
		t.Fatal(rows, err)
	}
	for _, bounds := range [][2]int{{0, 245760}, {52, 245760}, {1, 1}, {1, 245761}} {
		if _, err := repo.Candidates(ctx, nil, bounds[0], bounds[1]); err == nil {
			t.Fatal(bounds)
		}
	}
	for i := range 55 {
		m := mustMonitor(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-%012x", 0xa00+i), "http://web/", at.Add(time.Duration(i+1)*time.Microsecond))
		if err := repo.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = repo.Candidates(ctx, nil, 51, 245760)
	if err != nil || len(rows) != 51 {
		t.Fatal(len(rows), err)
	}
}

func TestInventoryCancellationDoesNotMutateRuns(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()
	at := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	m := mustMonitor(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", "http://web/", at)
	if err := repo.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO monitoring.check_runs(id,monitor_id,issued_at,deadline_at) VALUES('018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3',$1,$2,$3)`, m.ID().String(), at, at.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var raw string
		if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(c) ORDER BY id)::text FROM monitoring.check_runs c`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := snapshot()
	if _, err := repo.Candidates(ctx, nil, 21, 245760); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.Candidates(canceled, nil, 21, 245760); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if after := snapshot(); after != before {
		t.Fatal("inventory mutated CheckRuns")
	}
}
