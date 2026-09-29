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

	"uuid"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/database"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
	"github.com/kefyusuf/uptime-lab/apps/api/migrations"
)

type monitorPayload struct {
	ID        string `json:"id"`
	TargetURL string `json:"targetUrl"`
	CreatedAt string `json:"createdAt"`
}

type checkWorkPayload struct {
	CheckID      string `json:"checkId"`
	MonitorID    string `json:"monitorId"`
	TargetURL    string `json:"targetUrl"`
	TimeoutMS    int64  `json:"timeoutMs"`
	MaxRedirects int    `json:"maxRedirects"`
}

type latestCheckResultPayload struct {
	CheckID     string `json:"checkId"`
	ResultKind  string `json:"resultKind"`
	HTTPStatus  *int   `json:"httpStatus,omitempty"`
	DurationMS  *int64 `json:"durationMs,omitempty"`
	CompletedAt string `json:"completedAt"`
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

	var checkRunsExists bool
	if err := pool.QueryRow(
		ctx,
		"SELECT to_regclass('monitoring.check_runs') IS NOT NULL",
	).Scan(&checkRunsExists); err != nil {
		t.Fatalf("query check_runs existence before server start: %v", err)
	}
	if checkRunsExists {
		t.Fatal("monitoring.check_runs exists before explicit migration")
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

	if err := pool.QueryRow(
		ctx,
		"SELECT to_regclass('monitoring.check_runs') IS NOT NULL",
	).Scan(&checkRunsExists); err != nil {
		t.Fatalf("query check_runs existence after readiness check: %v", err)
	}
	if checkRunsExists {
		t.Fatal("/readyz applied migration unexpectedly")
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("provider.Up() error = %v", err)
	}
	assertStatus(t, client, http.MethodGet, baseURL+"/readyz", "", http.StatusOK)

	missingMonitorID := uuid.NewV7().String()
	assertStatus(
		t,
		client,
		http.MethodGet,
		baseURL+"/monitors/"+missingMonitorID+"/latest-result",
		"",
		http.StatusNotFound,
	)

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

	assertStatus(
		t,
		client,
		http.MethodGet,
		baseURL+"/monitors/"+postPayload.ID+"/latest-result",
		"",
		http.StatusNoContent,
	)

	claimResponse := doRequest(t, client, http.MethodPost, baseURL+"/internal/checks/claim", "", "")
	if claimResponse.StatusCode != http.StatusOK {
		defer claimResponse.Body.Close()
		body, _ := io.ReadAll(claimResponse.Body)
		t.Fatalf("POST /internal/checks/claim status = %d, want %d; body=%s", claimResponse.StatusCode, http.StatusOK, body)
	}
	claimPayload := decodeCheckWorkPayload(t, claimResponse)
	if _, err := uuid.Parse(claimPayload.CheckID); err != nil {
		t.Fatalf("claim checkId %q is not UUID: %v", claimPayload.CheckID, err)
	}
	if claimPayload.MonitorID != postPayload.ID {
		t.Fatalf("claim monitorId = %q, want %q", claimPayload.MonitorID, postPayload.ID)
	}
	if claimPayload.TargetURL != postPayload.TargetURL {
		t.Fatalf("claim targetUrl = %q, want %q", claimPayload.TargetURL, postPayload.TargetURL)
	}
	if claimPayload.TimeoutMS != 10000 || claimPayload.MaxRedirects != 3 {
		t.Fatalf("claim policy = timeoutMs %d / maxRedirects %d, want 10000 / 3", claimPayload.TimeoutMS, claimPayload.MaxRedirects)
	}

	assertStatus(t, client, http.MethodPost, baseURL+"/internal/checks/claim", "", http.StatusNoContent)

	resultBody := `{"kind":"http_response","durationMs":123,"httpStatus":204}`
	assertStatusWithContentType(
		t,
		client,
		http.MethodPut,
		baseURL+"/internal/checks/"+claimPayload.CheckID+"/result",
		"application/json",
		resultBody,
		http.StatusNoContent,
	)

	var (
		persistedCompletedAt time.Time
		persistedKind        string
		persistedStatus      int
		persistedDurationMS  int64
	)
	if err := pool.QueryRow(
		ctx,
		`
			SELECT completed_at, result_kind, http_status, duration_ms
			FROM monitoring.check_runs
			WHERE id = $1::uuid
		`,
		claimPayload.CheckID,
	).Scan(
		&persistedCompletedAt,
		&persistedKind,
		&persistedStatus,
		&persistedDurationMS,
	); err != nil {
		t.Fatalf("query persisted CheckRun result: %v", err)
	}
	if persistedKind != "http_response" || persistedStatus != 204 || persistedDurationMS != 123 {
		t.Fatalf(
			"persisted CheckRun result = kind=%q status=%d duration=%d, want http_response/204/123",
			persistedKind,
			persistedStatus,
			persistedDurationMS,
		)
	}

	latestHTTPResponse := doRequest(
		t,
		client,
		http.MethodGet,
		baseURL+"/monitors/"+postPayload.ID+"/latest-result",
		"",
		"",
	)
	if latestHTTPResponse.StatusCode != http.StatusOK {
		defer latestHTTPResponse.Body.Close()
		body, _ := io.ReadAll(latestHTTPResponse.Body)
		t.Fatalf(
			"GET latest HTTP result status = %d, want %d; body=%s",
			latestHTTPResponse.StatusCode,
			http.StatusOK,
			body,
		)
	}
	latestHTTP := decodeLatestCheckResultPayload(
		t,
		latestHTTPResponse,
		"checkId",
		"resultKind",
		"httpStatus",
		"durationMs",
		"completedAt",
	)
	if latestHTTP.CheckID != claimPayload.CheckID {
		t.Fatalf("latest HTTP checkId = %q, want %q", latestHTTP.CheckID, claimPayload.CheckID)
	}
	if latestHTTP.ResultKind != "http_response" {
		t.Fatalf("latest HTTP resultKind = %q, want http_response", latestHTTP.ResultKind)
	}
	if latestHTTP.HTTPStatus == nil || *latestHTTP.HTTPStatus != persistedStatus {
		t.Fatalf("latest HTTP httpStatus = %v, want %d", latestHTTP.HTTPStatus, persistedStatus)
	}
	if latestHTTP.DurationMS == nil || *latestHTTP.DurationMS != persistedDurationMS {
		t.Fatalf("latest HTTP durationMs = %v, want %d", latestHTTP.DurationMS, persistedDurationMS)
	}
	if latestHTTP.CompletedAt != persistedCompletedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf(
			"latest HTTP completedAt = %q, want %q",
			latestHTTP.CompletedAt,
			persistedCompletedAt.UTC().Format(time.RFC3339Nano),
		)
	}

	assertStatusWithContentType(
		t,
		client,
		http.MethodPut,
		baseURL+"/internal/checks/"+claimPayload.CheckID+"/result",
		"application/json",
		resultBody,
		http.StatusNoContent,
	)

	var duplicateCompletedAt time.Time
	if err := pool.QueryRow(
		ctx,
		"SELECT completed_at FROM monitoring.check_runs WHERE id = $1::uuid",
		claimPayload.CheckID,
	).Scan(&duplicateCompletedAt); err != nil {
		t.Fatalf("query duplicate CheckRun completed_at: %v", err)
	}
	if !duplicateCompletedAt.Equal(persistedCompletedAt) {
		t.Fatalf(
			"duplicate result changed completed_at from %v to %v",
			persistedCompletedAt,
			duplicateCompletedAt,
		)
	}
	assertStatusWithContentType(
		t,
		client,
		http.MethodPut,
		baseURL+"/internal/checks/"+claimPayload.CheckID+"/result",
		"application/json",
		`{"kind":"http_response","durationMs":124,"httpStatus":204}`,
		http.StatusConflict,
	)
	assertStatusWithContentType(
		t,
		client,
		http.MethodPut,
		baseURL+"/internal/checks/"+uuid.NewV7().String()+"/result",
		"application/json",
		`{"kind":"timeout","durationMs":10}`,
		http.StatusNotFound,
	)

	latePostResponse, err := client.Post(
		baseURL+"/monitors",
		"application/json",
		strings.NewReader(`{"targetUrl":"https://example.com/late-result"}`),
	)
	if err != nil {
		t.Fatalf("POST late Monitor error = %v", err)
	}
	lateMonitor := decodeMonitorPayload(t, latePostResponse)
	if latePostResponse.StatusCode != http.StatusCreated {
		t.Fatalf("POST late Monitor status = %d, want %d", latePostResponse.StatusCode, http.StatusCreated)
	}

	lateClaimResponse := doRequest(t, client, http.MethodPost, baseURL+"/internal/checks/claim", "", "")
	if lateClaimResponse.StatusCode != http.StatusOK {
		defer lateClaimResponse.Body.Close()
		body, _ := io.ReadAll(lateClaimResponse.Body)
		t.Fatalf("late claim status = %d, want %d; body=%s", lateClaimResponse.StatusCode, http.StatusOK, body)
	}
	lateClaim := decodeCheckWorkPayload(t, lateClaimResponse)
	if lateClaim.MonitorID != lateMonitor.ID {
		t.Fatalf("late claim monitorId = %q, want %q", lateClaim.MonitorID, lateMonitor.ID)
	}

	if _, err := pool.Exec(
		ctx,
		`
			UPDATE monitoring.check_runs
			SET deadline_at = issued_at + interval '1 microsecond'
			WHERE id = $1::uuid
		`,
		lateClaim.CheckID,
	); err != nil {
		t.Fatalf("force late CheckRun deadline: %v", err)
	}
	time.Sleep(2 * time.Millisecond)

	assertStatusWithContentType(
		t,
		client,
		http.MethodPut,
		baseURL+"/internal/checks/"+lateClaim.CheckID+"/result",
		"application/json",
		`{"kind":"timeout","durationMs":1}`,
		http.StatusConflict,
	)

	var (
		lateKind       string
		lateCompleted  time.Time
		lateDeadline   time.Time
		lateDurationMS *int64
	)
	if err := pool.QueryRow(
		ctx,
		`
			SELECT result_kind, completed_at, deadline_at, duration_ms
			FROM monitoring.check_runs
			WHERE id = $1::uuid
		`,
		lateClaim.CheckID,
	).Scan(&lateKind, &lateCompleted, &lateDeadline, &lateDurationMS); err != nil {
		t.Fatalf("query late CheckRun: %v", err)
	}
	if lateKind != "worker_timeout" {
		t.Fatalf("late result_kind = %q, want worker_timeout", lateKind)
	}
	if !lateCompleted.Equal(lateDeadline) {
		t.Fatalf("late completed_at = %v, want deadline_at %v", lateCompleted, lateDeadline)
	}
	if lateDurationMS != nil {
		t.Fatalf("late duration_ms = %v, want NULL", *lateDurationMS)
	}

	latestWorkerTimeoutResponse := doRequest(
		t,
		client,
		http.MethodGet,
		baseURL+"/monitors/"+lateMonitor.ID+"/latest-result",
		"",
		"",
	)
	if latestWorkerTimeoutResponse.StatusCode != http.StatusOK {
		defer latestWorkerTimeoutResponse.Body.Close()
		body, _ := io.ReadAll(latestWorkerTimeoutResponse.Body)
		t.Fatalf(
			"GET latest worker_timeout status = %d, want %d; body=%s",
			latestWorkerTimeoutResponse.StatusCode,
			http.StatusOK,
			body,
		)
	}
	latestWorkerTimeout := decodeLatestCheckResultPayload(
		t,
		latestWorkerTimeoutResponse,
		"checkId",
		"resultKind",
		"completedAt",
	)
	if latestWorkerTimeout.CheckID != lateClaim.CheckID {
		t.Fatalf(
			"latest worker_timeout checkId = %q, want %q",
			latestWorkerTimeout.CheckID,
			lateClaim.CheckID,
		)
	}
	if latestWorkerTimeout.ResultKind != "worker_timeout" {
		t.Fatalf(
			"latest worker_timeout resultKind = %q, want worker_timeout",
			latestWorkerTimeout.ResultKind,
		)
	}
	if latestWorkerTimeout.HTTPStatus != nil || latestWorkerTimeout.DurationMS != nil {
		t.Fatalf(
			"latest worker_timeout optional fields = status=%v duration=%v, want absent",
			latestWorkerTimeout.HTTPStatus,
			latestWorkerTimeout.DurationMS,
		)
	}
	if latestWorkerTimeout.CompletedAt != lateCompleted.UTC().Format(time.RFC3339Nano) {
		t.Fatalf(
			"latest worker_timeout completedAt = %q, want %q",
			latestWorkerTimeout.CompletedAt,
			lateCompleted.UTC().Format(time.RFC3339Nano),
		)
	}

	if _, err := pool.Exec(
		ctx,
		"INSERT INTO public.goose_db_version (version_id, is_applied) VALUES (999, true)",
	); err != nil {
		t.Fatalf("insert incompatible migration metadata: %v", err)
	}
	assertStatus(t, client, http.MethodGet, baseURL+"/livez", "", http.StatusOK)
	assertStatus(t, client, http.MethodGet, baseURL+"/readyz", "", http.StatusServiceUnavailable)
	if _, err := pool.Exec(
		ctx,
		"DELETE FROM public.goose_db_version WHERE version_id = 999",
	); err != nil {
		t.Fatalf("remove incompatible migration metadata: %v", err)
	}
	assertStatus(t, client, http.MethodGet, baseURL+"/readyz", "", http.StatusOK)

	if err := sqlDB.Close(); err != nil {
		t.Fatalf("sqlDB.Close() error = %v", err)
	}
	sqlClosed = true
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() after sqlDB.Close() error = %v; shared pool must remain open", err)
	}
}

func doRequest(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	contentType string,
	body string,
) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, url, err)
	}
	return response
}

func assertStatusWithContentType(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	contentType string,
	body string,
	want int,
) {
	t.Helper()
	response := doRequest(t, client, method, url, contentType, body)
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != want {
		t.Fatalf("%s %s status = %d, want %d", method, url, response.StatusCode, want)
	}
}

func decodeCheckWorkPayload(t *testing.T, response *http.Response) checkWorkPayload {
	t.Helper()
	defer response.Body.Close()

	var payload checkWorkPayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode check work response: %v", err)
	}
	return payload
}

func decodeLatestCheckResultPayload(
	t *testing.T,
	response *http.Response,
	wantKeys ...string,
) latestCheckResultPayload {
	t.Helper()
	defer response.Body.Close()

	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("latest-result Content-Type = %q, want application/json", got)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read latest-result response: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode latest-result JSON object: %v; body=%q", err, body)
	}
	if len(raw) != len(wantKeys) {
		t.Fatalf("latest-result keys = %v, want exactly %v", raw, wantKeys)
	}
	for _, key := range wantKeys {
		if _, ok := raw[key]; !ok {
			t.Fatalf("latest-result missing key %q: %v", key, raw)
		}
	}

	var payload latestCheckResultPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode latest-result payload: %v", err)
	}
	return payload
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
