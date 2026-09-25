package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

type registerMonitorStub struct {
	calls   int
	raw     string
	monitor domain.Monitor
	err     error
}

func (stub *registerMonitorStub) Execute(_ context.Context, rawTarget string) (domain.Monitor, error) {
	stub.calls++
	stub.raw = rawTarget
	return stub.monitor, stub.err
}

type getMonitorStub struct {
	calls   int
	id      domain.MonitorID
	monitor domain.Monitor
	err     error
}

func (stub *getMonitorStub) Execute(_ context.Context, id domain.MonitorID) (domain.Monitor, error) {
	stub.calls++
	stub.id = id
	return stub.monitor, stub.err
}

func TestHandlerPostMonitorSuccess(t *testing.T) {
	const rawTarget = "HTTP://Example.COM/health check?region=eu"
	createdAt := time.Date(2026, time.September, 23, 12, 34, 56, 123456000, time.UTC)
	monitor := mustMonitor(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", rawTarget, createdAt)
	register := &registerMonitorStub{monitor: monitor}
	get := &getMonitorStub{}
	handler := NewHandler(register, get)

	body, err := json.Marshal(map[string]string{"targetUrl": rawTarget})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	response := serve(handler, http.MethodPost, "/monitors", "application/json", string(body))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := response.Header().Get("Location"); got != "/monitors/"+monitor.ID().String() {
		t.Fatalf("Location = %q", got)
	}
	if register.calls != 1 || register.raw != rawTarget {
		t.Fatalf("RegisterMonitor calls/raw = %d/%q, want 1/%q", register.calls, register.raw, rawTarget)
	}
	if get.calls != 0 {
		t.Fatalf("GetMonitor calls = %d, want 0", get.calls)
	}

	got := decodeObject(t, response.Body.String())
	assertExactKeys(t, got, "id", "targetUrl", "createdAt")
	if got["id"] != monitor.ID().String() {
		t.Fatalf("id = %#v, want %q", got["id"], monitor.ID().String())
	}
	if got["targetUrl"] != rawTarget {
		t.Fatalf("targetUrl = %#v, want exact %q", got["targetUrl"], rawTarget)
	}
	if got["createdAt"] != "2026-09-23T12:34:56.123456Z" {
		t.Fatalf("createdAt = %#v, want canonical microsecond UTC instant", got["createdAt"])
	}
}

func TestHandlerPostAcceptsApplicationJSONParameters(t *testing.T) {
	monitor := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
		"https://example.com",
		time.Date(2026, time.September, 23, 12, 34, 56, 0, time.UTC),
	)
	register := &registerMonitorStub{monitor: monitor}
	handler := NewHandler(register, &getMonitorStub{})

	response := serve(
		handler,
		http.MethodPost,
		"/monitors",
		"application/json; charset=utf-8",
		`{"targetUrl":"https://example.com"}`,
	)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	if register.calls != 1 {
		t.Fatalf("RegisterMonitor calls = %d, want 1", register.calls)
	}
}

func TestHandlerPostTransportClassification(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "missing content type", body: `{"targetUrl":"https://example.com"}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unsupported content type", contentType: "text/plain", body: `{"targetUrl":"https://example.com"}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "malformed content type", contentType: "application/json; charset", body: `{"targetUrl":"https://example.com"}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "empty body", contentType: "application/json", body: "", wantStatus: http.StatusBadRequest},
		{name: "malformed json", contentType: "application/json", body: "{", wantStatus: http.StatusBadRequest},
		{name: "multiple json documents", contentType: "application/json", body: `{"targetUrl":"https://example.com"} {"targetUrl":"https://example.org"}`, wantStatus: http.StatusBadRequest},
		{name: "root scalar", contentType: "application/json", body: `"https://example.com"`, wantStatus: http.StatusUnprocessableEntity},
		{name: "root array", contentType: "application/json", body: `["https://example.com"]`, wantStatus: http.StatusUnprocessableEntity},
		{name: "root null", contentType: "application/json", body: "null", wantStatus: http.StatusUnprocessableEntity},
		{name: "missing target url", contentType: "application/json", body: "{}", wantStatus: http.StatusUnprocessableEntity},
		{name: "extra property", contentType: "application/json", body: `{"targetUrl":"https://example.com","extra":true}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "non string target url", contentType: "application/json", body: `{"targetUrl":42}`, wantStatus: http.StatusUnprocessableEntity},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			register := &registerMonitorStub{}
			handler := NewHandler(register, &getMonitorStub{})
			response := serve(handler, http.MethodPost, "/monitors", test.contentType, test.body)

			assertProblem(t, response, test.wantStatus)
			if register.calls != 0 {
				t.Fatalf("RegisterMonitor calls = %d, want 0", register.calls)
			}
		})
	}
}

func TestHandlerPostApplicationErrorMappingIsSanitized(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid target", err: domain.ErrInvalidTargetURL, wantStatus: http.StatusUnprocessableEntity},
		{name: "wrapped invalid target", err: fmt.Errorf("wrapped: %w", domain.ErrInvalidTargetURL), wantStatus: http.StatusUnprocessableEntity},
		{name: "persistence", err: application.ErrPersistence, wantStatus: http.StatusInternalServerError},
		{name: "wrapped persistence", err: fmt.Errorf("wrapped: %w", application.ErrPersistence), wantStatus: http.StatusInternalServerError},
		{name: "unexpected internal error", err: errors.New("pgx password=secret host=db"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			register := &registerMonitorStub{err: test.err}
			handler := NewHandler(register, &getMonitorStub{})
			response := serve(
				handler,
				http.MethodPost,
				"/monitors",
				"application/json",
				`{"targetUrl":"https://example.com"}`,
			)

			assertProblem(t, response, test.wantStatus)
			if register.calls != 1 {
				t.Fatalf("RegisterMonitor calls = %d, want 1", register.calls)
			}
			body := response.Body.String()
			for _, secret := range []string{"password=secret", "host=db", "pgx"} {
				if strings.Contains(body, secret) {
					t.Fatalf("problem body leaked %q: %s", secret, body)
				}
			}
		})
	}
}

func TestHandlerGetMonitorSuccess(t *testing.T) {
	createdAt := time.Date(2026, time.September, 23, 9, 8, 7, 654321000, time.UTC)
	monitor := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
		"https://example.com/health?x=1",
		createdAt,
	)
	get := &getMonitorStub{monitor: monitor}
	register := &registerMonitorStub{}
	handler := NewHandler(register, get)

	response := serve(handler, http.MethodGet, "/monitors/"+monitor.ID().String(), "", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if get.calls != 1 || get.id != monitor.ID() {
		t.Fatalf("GetMonitor calls/id = %d/%v, want 1/%v", get.calls, get.id, monitor.ID())
	}
	if register.calls != 0 {
		t.Fatalf("RegisterMonitor calls = %d, want 0", register.calls)
	}

	got := decodeObject(t, response.Body.String())
	assertExactKeys(t, got, "id", "targetUrl", "createdAt")
	if got["id"] != monitor.ID().String() {
		t.Fatalf("id = %#v, want %q", got["id"], monitor.ID().String())
	}
	if got["targetUrl"] != monitor.TargetURL().String() {
		t.Fatalf("targetUrl = %#v, want %q", got["targetUrl"], monitor.TargetURL().String())
	}
	if got["createdAt"] != "2026-09-23T09:08:07.654321Z" {
		t.Fatalf("createdAt = %#v, want canonical microsecond UTC instant", got["createdAt"])
	}
}

func TestHandlerGetRejectsInvalidMonitorIDBeforeUseCase(t *testing.T) {
	get := &getMonitorStub{}
	handler := NewHandler(&registerMonitorStub{}, get)

	response := serve(handler, http.MethodGet, "/monitors/not-a-uuid", "", "")

	assertProblem(t, response, http.StatusBadRequest)
	if get.calls != 0 {
		t.Fatalf("GetMonitor calls = %d, want 0", get.calls)
	}
}

func TestHandlerGetApplicationErrorMappingIsSanitized(t *testing.T) {
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not found", err: application.ErrMonitorNotFound, wantStatus: http.StatusNotFound},
		{name: "wrapped not found", err: fmt.Errorf("wrapped: %w", application.ErrMonitorNotFound), wantStatus: http.StatusNotFound},
		{name: "persistence", err: application.ErrPersistence, wantStatus: http.StatusInternalServerError},
		{name: "wrapped persistence", err: fmt.Errorf("wrapped: %w", application.ErrPersistence), wantStatus: http.StatusInternalServerError},
		{name: "unexpected", err: errors.New("sql credential=secret table=monitoring.monitors"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			get := &getMonitorStub{err: test.err}
			handler := NewHandler(&registerMonitorStub{}, get)
			response := serve(handler, http.MethodGet, "/monitors/"+id, "", "")

			assertProblem(t, response, test.wantStatus)
			if get.calls != 1 {
				t.Fatalf("GetMonitor calls = %d, want 1", get.calls)
			}
			body := response.Body.String()
			for _, secret := range []string{"credential=secret", "monitoring.monitors", "sql"} {
				if strings.Contains(body, secret) {
					t.Fatalf("problem body leaked %q: %s", secret, body)
				}
			}
		})
	}
}

func TestHandlerKnownResourcesRejectUnsupportedMethodsWithoutUseCaseExecution(t *testing.T) {
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	tests := []struct {
		name      string
		method    string
		path      string
		wantAllow string
	}{
		{name: "collection get", method: http.MethodGet, path: "/monitors", wantAllow: http.MethodPost},
		{name: "collection head", method: http.MethodHead, path: "/monitors", wantAllow: http.MethodPost},
		{name: "collection put", method: http.MethodPut, path: "/monitors", wantAllow: http.MethodPost},
		{name: "collection patch", method: http.MethodPatch, path: "/monitors", wantAllow: http.MethodPost},
		{name: "collection delete", method: http.MethodDelete, path: "/monitors", wantAllow: http.MethodPost},
		{name: "collection options", method: http.MethodOptions, path: "/monitors", wantAllow: http.MethodPost},
		{name: "resource post", method: http.MethodPost, path: "/monitors/" + id, wantAllow: http.MethodGet},
		{name: "resource head", method: http.MethodHead, path: "/monitors/" + id, wantAllow: http.MethodGet},
		{name: "resource put", method: http.MethodPut, path: "/monitors/" + id, wantAllow: http.MethodGet},
		{name: "resource patch", method: http.MethodPatch, path: "/monitors/" + id, wantAllow: http.MethodGet},
		{name: "resource delete", method: http.MethodDelete, path: "/monitors/" + id, wantAllow: http.MethodGet},
		{name: "resource options", method: http.MethodOptions, path: "/monitors/" + id, wantAllow: http.MethodGet},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			register := &registerMonitorStub{}
			get := &getMonitorStub{}
			handler := NewHandler(register, get)
			response := serve(handler, test.method, test.path, "", "")

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusMethodNotAllowed, response.Body.String())
			}
			if got := response.Header().Get("Allow"); got != test.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, test.wantAllow)
			}
			if register.calls != 0 || get.calls != 0 {
				t.Fatalf("use-case calls register/get = %d/%d, want 0/0", register.calls, get.calls)
			}
		})
	}
}

func TestHandlerUnknownAndNonExactPathsRemainNotFound(t *testing.T) {
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	paths := []string{
		"/",
		"/unknown",
		"/Monitors",
		"/monitors/",
		"/monitors//" + id,
		"/monitors/" + id + "/extra",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			register := &registerMonitorStub{}
			get := &getMonitorStub{}
			handler := NewHandler(register, get)
			response := serve(handler, http.MethodGet, path, "", "")

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
			}
			if register.calls != 0 || get.calls != 0 {
				t.Fatalf("use-case calls register/get = %d/%d, want 0/0", register.calls, get.calls)
			}
		})
	}
}

func serve(handler http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func mustMonitor(t *testing.T, rawID, rawTarget string, createdAt time.Time) domain.Monitor {
	t.Helper()

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	target, err := domain.NewTargetURL(rawTarget)
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	monitor, err := domain.NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}
	return monitor
}

func decodeObject(t *testing.T, body string) map[string]any {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal([]byte(body), &object); err != nil {
		t.Fatalf("decode JSON object: %v; body=%q", err, body)
	}
	if object == nil {
		t.Fatalf("decoded object is nil; body=%q", body)
	}
	return object
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	problem := decodeObject(t, response.Body.String())
	assertExactKeys(t, problem, "type", "title", "status", "detail")
	if problem["type"] != "about:blank" {
		t.Fatalf("problem.type = %#v, want about:blank", problem["type"])
	}
	if problem["status"] != float64(wantStatus) {
		t.Fatalf("problem.status = %#v, want %d", problem["status"], wantStatus)
	}
	if title, ok := problem["title"].(string); !ok || title == "" {
		t.Fatalf("problem.title = %#v, want non-empty string", problem["title"])
	}
	if detail, ok := problem["detail"].(string); !ok || detail == "" {
		t.Fatalf("problem.detail = %#v, want non-empty string", problem["detail"])
	}
	if _, exists := problem["instance"]; exists {
		t.Fatal("problem.instance must be omitted")
	}
}

func assertExactKeys(t *testing.T, object map[string]any, keys ...string) {
	t.Helper()

	if len(object) != len(keys) {
		t.Fatalf("object keys = %v, want exactly %v", object, keys)
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			t.Fatalf("object missing key %q: %v", key, object)
		}
	}
}
