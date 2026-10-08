//go:build integration

package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/database"
	"github.com/kefyusuf/uptime-lab/apps/api/migrations"
)

func TestSchedulingCompositionPersistsState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	product, _, sqlDB, err := composeMonitoring(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	provider, err := migrations.NewProvider(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd007"
	if _, err := pool.Exec(ctx, `INSERT INTO monitoring.monitors (id,target_url,created_at) VALUES ($1,'https://example.com',now())`, id); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM monitoring.monitors WHERE id=$1`, id) }()
	path := "/monitors/" + id + "/scheduling"
	for _, state := range []string{"paused", "paused", "active"} {
		req := httptest.NewRequest("PUT", path, strings.NewReader(`{"state":"`+state+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		product.ServeHTTP(w, req)
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"state":"`+state+`"}` {
			t.Fatal(w.Code, w.Body.String())
		}
		var paused bool
		if err := pool.QueryRow(ctx, `SELECT paused FROM monitoring.monitors WHERE id=$1`, id).Scan(&paused); err != nil {
			t.Fatal(err)
		}
		if paused != (state == "paused") {
			t.Fatal("write not persisted", state, paused)
		}
		fresh, _, freshDB, err := composeMonitoring(pool)
		if err != nil {
			t.Fatal(err)
		}
		read := httptest.NewRecorder()
		fresh.ServeHTTP(read, httptest.NewRequest("GET", path, nil))
		freshDB.Close()
		if read.Code != 200 || read.Body.String() != w.Body.String() {
			t.Fatal("fresh composition disagrees", read.Code, read.Body.String())
		}
	}
}
