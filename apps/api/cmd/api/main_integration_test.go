//go:build integration

package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/database"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
	"github.com/kefyusuf/uptime-lab/apps/api/migrations"
)

type monitorPayload struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl"`
	CreatedAt string `json:"createdAt"`
}

func TestProductionMonitoringCompositionAgainstPostgreSQL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Fatalf("database.NewPool() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() error = %v", err)
	}

	product, readiness, sqlDB, err := composeMonitoring(pool)
	if err != nil {
		t.Fatalf("composeMonitoring() error = %v", err)
	}
	sqlClosed := false
	defer func() {
		if !sqlClosed {
			_ = sqlDB.Close()
		}
	}()

	provider, err := migrations.NewProvider(sqlDB)
	if err != nil {
		t.Fatalf("migrations.NewProvider() error = %v", err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("provider.Down() error = %v", err)
	}

	server := httpserver.New("127.0.0.1:0", readiness, product)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	serveCtx, stopServer := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(serveCtx, listener)
	}()
	defer func() {
		stopServer()
		select {
		case err := <-serveResult:
			if err != nil {
				t.Errorf("server.Serve() shutdown error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server.Serve() did not stop")
		}
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := "http://" + listener.Addr().String()

	assertStatus(t, client, http.MethodGet, baseURL+"/livez", "", http.StatusOK)
	assertStatus(t, client, http.MethodGet, baseURL+"/readyz", "", http.StatusServiceUnavailable)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("provider.Up() error = %v", err)
	}
	assertStatus(t, client, http.MethodGet, baseURL+"/readyz", "", http.StatusOK)

	postBody := `{"targetUrl":"https://example.com/production-composition"}`
	postResponse, err := client.Post(baseURL+"/monitors", "application/json", strings.NewReader(postBody))
	if err != nil {
		t.Fatalf("POST /monitors error = %v", err)
	}
	postPayload := decodeMonitorPayload(t, postResponse)
	if postResponse.StatusCode != http.StatusCreated {
		t.Fatalf("POST /monitors status = %d, want %d", postResponse.StatusCode, http.StatusCreated)
	}
	if got := postResponse.Header.Get("Location"); got != "/monitors/"+postPayload.ID {
		t.Fatalf("POST Location = %q, want /monitors/%s", got, postPayload.ID)
	}
	if postPayload.TargetURL != "https://example.com/production-composition" {
		t.Fatalf("POST targetUrl = %q", postPayload.TargetURL)
	}

	createdAt, err := time.Parse(time.RFC3339Nano, postPayload.CreatedAt)
	if err != nil {
		t.Fatalf("parse POST createdAt %q: %v", postPayload.CreatedAt, err)
	}
	if createdAt.Location() != time.UTC {
		t.Fatalf("POST createdAt location = %v, want UTC", createdAt.Location())
	}
	if createdAt.Nanosecond()%int(time.Microsecond) != 0 {
		t.Fatalf("POST createdAt nanoseconds = %d, want microsecond precision", createdAt.Nanosecond())
	}

	getResponse, err := client.Get(baseURL + "/monitors/" + postPayload.ID)
	if err != nil {
		t.Fatalf("GET /monitors/{id} error = %v", err)
	}
	getPayload := decodeMonitorPayload(t, getResponse)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("GET /monitors/{id} status = %d, want %d", getResponse.StatusCode, http.StatusOK)
	}
	if getPayload.ID != postPayload.ID {
		t.Fatalf("GET id = %q, want %q", getPayload.ID, postPayload.ID)
	}
	if getPayload.TargetURL != postPayload.TargetURL {
		t.Fatalf("GET targetUrl = %q, want %q", getPayload.TargetURL, postPayload.TargetURL)
	}
	if getPayload.CreatedAt != postPayload.CreatedAt {
		t.Fatalf("GET createdAt = %q, want exact POST instant %q", getPayload.CreatedAt, postPayload.CreatedAt)
	}

	if err := sqlDB.Close(); err != nil {
		t.Fatalf("sqlDB.Close() error = %v", err)
	}
	sqlClosed = true
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() after sqlDB.Close() error = %v; shared pool must remain open", err)
	}
}

func assertStatus(t *testing.T, client *http.Client, method, url, body string, want int) {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, url, err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != want {
		t.Fatalf("%s %s status = %d, want %d", method, url, response.StatusCode, want)
	}
}

func decodeMonitorPayload(t *testing.T, response *http.Response) monitorPayload {
	t.Helper()
	defer response.Body.Close()

	var payload monitorPayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode monitor response: %v", err)
	}
	return payload
}
