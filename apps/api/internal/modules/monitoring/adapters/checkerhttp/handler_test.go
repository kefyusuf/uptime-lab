package checkerhttp

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

type claimDueCheckStub struct {
	calls int
	work  application.CheckWork
	err   error
}

func (stub *claimDueCheckStub) Execute(_ context.Context) (application.CheckWork, error) {
	stub.calls++
	return stub.work, stub.err
}

type submitCheckResultStub struct {
	calls      int
	rawCheckID string
	input      application.SubmitCheckResultInput
	err        error
}

func (stub *submitCheckResultStub) Execute(
	_ context.Context,
	rawCheckID string,
	input application.SubmitCheckResultInput,
) error {
	stub.calls++
	stub.rawCheckID = rawCheckID
	stub.input = input
	return stub.err
}

func TestHandlerClaimSuccessReturnsExactWorkJSON(t *testing.T) {
	claim := &claimDueCheckStub{
		work: application.CheckWork{
			CheckID:      mustCheckID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"),
			MonitorID:    mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd001"),
			TargetURL:    mustTargetURL(t, "https://example.com/health"),
			Timeout:      10 * time.Second,
			MaxRedirects: 3,
		},
	}
	submit := &submitCheckResultStub{}
	handler := NewHandler(claim, submit)

	response := serve(handler, http.MethodPost, "/internal/checks/claim", "", "ignored body")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if claim.calls != 1 {
		t.Fatalf("ClaimDueCheck calls = %d, want 1", claim.calls)
	}
	if submit.calls != 0 {
		t.Fatalf("SubmitCheckResult calls = %d, want 0", submit.calls)
	}

	got := decodeObject(t, response.Body.String())
	assertExactKeys(t, got, "checkId", "monitorId", "targetUrl", "timeoutMs", "maxRedirects")
	if got["checkId"] != claim.work.CheckID.String() {
		t.Fatalf("checkId = %#v", got["checkId"])
	}
	if got["monitorId"] != claim.work.MonitorID.String() {
		t.Fatalf("monitorId = %#v", got["monitorId"])
	}
	if got["targetUrl"] != claim.work.TargetURL.String() {
		t.Fatalf("targetUrl = %#v", got["targetUrl"])
	}
	if got["timeoutMs"] != float64(10000) {
		t.Fatalf("timeoutMs = %#v, want 10000", got["timeoutMs"])
	}
	if got["maxRedirects"] != float64(3) {
		t.Fatalf("maxRedirects = %#v, want 3", got["maxRedirects"])
	}
}

func TestHandlerClaimNoWorkReturnsEmpty204(t *testing.T) {
	claim := &claimDueCheckStub{err: application.ErrNoDueCheck}
	handler := NewHandler(claim, &submitCheckResultStub{})

	response := serve(handler, http.MethodPost, "/internal/checks/claim", "", "")

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if response.Body.Len() != 0 {
		t.Fatalf("204 body = %q, want empty", response.Body.String())
	}
	if claim.calls != 1 {
		t.Fatalf("ClaimDueCheck calls = %d, want 1", claim.calls)
	}
}

func TestHandlerClaimErrorsAreSanitized(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "stable persistence", err: application.ErrPersistence},
		{name: "wrapped persistence", err: fmt.Errorf("wrapped: %w", application.ErrPersistence)},
		{name: "unexpected", err: errors.New("pgx password=secret table=monitoring.check_runs")},
	} {
		t.Run(test.name, func(t *testing.T) {
			claim := &claimDueCheckStub{err: test.err}
			handler := NewHandler(claim, &submitCheckResultStub{})

			response := serve(handler, http.MethodPost, "/internal/checks/claim", "", "")

			assertProblem(t, response, http.StatusInternalServerError)
			if claim.calls != 1 {
				t.Fatalf("ClaimDueCheck calls = %d, want 1", claim.calls)
			}
			assertNoSecrets(t, response.Body.String())
		})
	}
}

func TestHandlerResultAcceptsHTTPResponseAndExactDuplicate(t *testing.T) {
	const checkID = "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"
	for _, name := range []string{"first accepted result", "exact duplicate accepted"} {
		t.Run(name, func(t *testing.T) {
			submit := &submitCheckResultStub{}
			handler := NewHandler(&claimDueCheckStub{}, submit)

			response := serve(
				handler,
				http.MethodPut,
				"/internal/checks/"+checkID+"/result",
				"application/json; charset=utf-8",
				`{"kind":"http_response","durationMs":125,"httpStatus":204}`,
			)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
			}
			if response.Body.Len() != 0 {
				t.Fatalf("204 body = %q, want empty", response.Body.String())
			}
			if submit.calls != 1 || submit.rawCheckID != checkID {
				t.Fatalf("submit calls/id = %d/%q, want 1/%q", submit.calls, submit.rawCheckID, checkID)
			}
			if submit.input.Kind != domain.CheckResultHTTPResponse || submit.input.DurationMS != 125 {
				t.Fatalf("submit input = %#v", submit.input)
			}
			if submit.input.HTTPStatus == nil || *submit.input.HTTPStatus != 204 {
				t.Fatalf("HTTPStatus = %#v, want 204", submit.input.HTTPStatus)
			}
		})
	}
}

func TestHandlerResultAcceptsFailureResult(t *testing.T) {
	const checkID = "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"
	submit := &submitCheckResultStub{}
	handler := NewHandler(&claimDueCheckStub{}, submit)

	response := serve(
		handler,
		http.MethodPut,
		"/internal/checks/"+checkID+"/result",
		"application/json",
		`{"kind":"timeout","durationMs":10000}`,
	)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
	if submit.calls != 1 {
		t.Fatalf("SubmitCheckResult calls = %d, want 1", submit.calls)
	}
	if submit.input.Kind != domain.CheckResultTimeout || submit.input.DurationMS != 10000 || submit.input.HTTPStatus != nil {
		t.Fatalf("submit input = %#v", submit.input)
	}
}

func TestHandlerResultRejectsMalformedCheckIDBeforeUseCase(t *testing.T) {
	submit := &submitCheckResultStub{}
	handler := NewHandler(&claimDueCheckStub{}, submit)

	response := serve(
		handler,
		http.MethodPut,
		"/internal/checks/not-a-uuid/result",
		"application/json",
		`{"kind":"timeout","durationMs":1}`,
	)

	assertProblem(t, response, http.StatusBadRequest)
	if submit.calls != 0 {
		t.Fatalf("SubmitCheckResult calls = %d, want 0", submit.calls)
	}
}

func TestHandlerResultTransportClassification(t *testing.T) {
	const path = "/internal/checks/018f22d3-1d6a-7cc0-a37b-46fc3fafd101/result"

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "missing content type", body: `{"kind":"timeout","durationMs":1}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unsupported content type", contentType: "text/plain", body: `{"kind":"timeout","durationMs":1}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "malformed content type", contentType: "application/json; charset", body: `{"kind":"timeout","durationMs":1}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "empty body", contentType: "application/json", body: "", wantStatus: http.StatusBadRequest},
		{name: "malformed json", contentType: "application/json", body: "{", wantStatus: http.StatusBadRequest},
		{name: "multiple json documents", contentType: "application/json", body: `{"kind":"timeout","durationMs":1} {"kind":"timeout","durationMs":1}`, wantStatus: http.StatusBadRequest},
		{name: "root scalar", contentType: "application/json", body: `"timeout"`, wantStatus: http.StatusUnprocessableEntity},
		{name: "root array", contentType: "application/json", body: `["timeout"]`, wantStatus: http.StatusUnprocessableEntity},
		{name: "root null", contentType: "application/json", body: "null", wantStatus: http.StatusUnprocessableEntity},
		{name: "missing kind", contentType: "application/json", body: `{"durationMs":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "missing duration", contentType: "application/json", body: `{"kind":"timeout"}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "extra property", contentType: "application/json", body: `{"kind":"timeout","durationMs":1,"message":"secret"}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "kind not string", contentType: "application/json", body: `{"kind":1,"durationMs":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "duration not integer", contentType: "application/json", body: `{"kind":"timeout","durationMs":1.5}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "duration negative", contentType: "application/json", body: `{"kind":"timeout","durationMs":-1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "duration above maximum", contentType: "application/json", body: `{"kind":"timeout","durationMs":20001}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "unknown kind", contentType: "application/json", body: `{"kind":"future_kind","durationMs":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "worker timeout forbidden", contentType: "application/json", body: `{"kind":"worker_timeout","durationMs":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "HTTP response missing status", contentType: "application/json", body: `{"kind":"http_response","durationMs":1}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "HTTP response status not integer", contentType: "application/json", body: `{"kind":"http_response","durationMs":1,"httpStatus":200.5}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "HTTP response status below range", contentType: "application/json", body: `{"kind":"http_response","durationMs":1,"httpStatus":99}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "HTTP response status above range", contentType: "application/json", body: `{"kind":"http_response","durationMs":1,"httpStatus":600}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "failure cannot include status", contentType: "application/json", body: `{"kind":"timeout","durationMs":1,"httpStatus":500}`, wantStatus: http.StatusUnprocessableEntity},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			submit := &submitCheckResultStub{}
			handler := NewHandler(&claimDueCheckStub{}, submit)

			response := serve(handler, http.MethodPut, path, test.contentType, test.body)

			assertProblem(t, response, test.wantStatus)
			if submit.calls != 0 {
				t.Fatalf("SubmitCheckResult calls = %d, want 0", submit.calls)
			}
		})
	}
}

func TestHandlerResultApplicationErrorMappingIsSanitized(t *testing.T) {
	const path = "/internal/checks/018f22d3-1d6a-7cc0-a37b-46fc3fafd101/result"
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unknown", err: application.ErrCheckRunNotFound, wantStatus: http.StatusNotFound},
		{name: "wrapped unknown", err: fmt.Errorf("wrapped: %w", application.ErrCheckRunNotFound), wantStatus: http.StatusNotFound},
		{name: "conflict", err: application.ErrCheckRunConflict, wantStatus: http.StatusConflict},
		{name: "wrapped conflict", err: fmt.Errorf("wrapped: %w", application.ErrCheckRunConflict), wantStatus: http.StatusConflict},
		{name: "invalid result", err: domain.ErrInvalidCheckResult, wantStatus: http.StatusUnprocessableEntity},
		{name: "persistence", err: application.ErrPersistence, wantStatus: http.StatusInternalServerError},
		{name: "unexpected", err: errors.New("dns tls pgx password=secret filesystem=/tmp/key"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			submit := &submitCheckResultStub{err: test.err}
			handler := NewHandler(&claimDueCheckStub{}, submit)
			response := serve(
				handler,
				http.MethodPut,
				path,
				"application/json",
				`{"kind":"timeout","durationMs":1}`,
			)

			assertProblem(t, response, test.wantStatus)
			if submit.calls != 1 {
				t.Fatalf("SubmitCheckResult calls = %d, want 1", submit.calls)
			}
			assertNoSecrets(t, response.Body.String())
		})
	}
}

func TestHandlerKnownResourcesRejectUnsupportedMethods(t *testing.T) {
	const checkID = "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"
	tests := []struct {
		name      string
		method    string
		path      string
		wantAllow string
	}{
		{name: "claim get", method: http.MethodGet, path: "/internal/checks/claim", wantAllow: http.MethodPost},
		{name: "claim head", method: http.MethodHead, path: "/internal/checks/claim", wantAllow: http.MethodPost},
		{name: "claim put", method: http.MethodPut, path: "/internal/checks/claim", wantAllow: http.MethodPost},
		{name: "claim delete", method: http.MethodDelete, path: "/internal/checks/claim", wantAllow: http.MethodPost},
		{name: "result get", method: http.MethodGet, path: "/internal/checks/" + checkID + "/result", wantAllow: http.MethodPut},
		{name: "result post", method: http.MethodPost, path: "/internal/checks/" + checkID + "/result", wantAllow: http.MethodPut},
		{name: "result head", method: http.MethodHead, path: "/internal/checks/" + checkID + "/result", wantAllow: http.MethodPut},
		{name: "result patch", method: http.MethodPatch, path: "/internal/checks/" + checkID + "/result", wantAllow: http.MethodPut},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claim := &claimDueCheckStub{}
			submit := &submitCheckResultStub{}
			handler := NewHandler(claim, submit)
			response := serve(handler, test.method, test.path, "", "")

			assertProblem(t, response, http.StatusMethodNotAllowed)
			if got := response.Header().Get("Allow"); got != test.wantAllow {
				t.Fatalf("Allow = %q, want %q", got, test.wantAllow)
			}
			if claim.calls != 0 || submit.calls != 0 {
				t.Fatalf("use-case calls claim/submit = %d/%d, want 0/0", claim.calls, submit.calls)
			}
		})
	}
}

func TestHandlerUnknownAndNonExactPathsRemainNotFound(t *testing.T) {
	const checkID = "018f22d3-1d6a-7cc0-a37b-46fc3fafd101"
	paths := []string{
		"/",
		"/internal",
		"/internal/checks",
		"/internal/checks/",
		"/internal/checks/claim/",
		"/internal/checks//result",
		"/internal/checks/" + checkID,
		"/internal/checks/" + checkID + "/result/",
		"/internal/checks/" + checkID + "//result",
		"/internal/checks/" + checkID + "/result/extra",
		"/Internal/checks/claim",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			claim := &claimDueCheckStub{}
			submit := &submitCheckResultStub{}
			handler := NewHandler(claim, submit)
			response := serve(handler, http.MethodGet, path, "", "")

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
			}
			if claim.calls != 0 || submit.calls != 0 {
				t.Fatalf("use-case calls claim/submit = %d/%d, want 0/0", claim.calls, submit.calls)
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

func decodeObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode JSON object: %v; body=%s", err, raw)
	}
	return value
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()

	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	problem := decodeObject(t, response.Body.String())
	assertExactKeys(t, problem, "type", "title", "status", "detail")
	if problem["type"] != "about:blank" {
		t.Fatalf("problem.type = %#v, want about:blank", problem["type"])
	}
	if problem["status"] != float64(status) {
		t.Fatalf("problem.status = %#v, want %d", problem["status"], status)
	}
	if title, ok := problem["title"].(string); !ok || title == "" {
		t.Fatalf("problem.title = %#v, want non-empty string", problem["title"])
	}
	if detail, ok := problem["detail"].(string); !ok || detail == "" {
		t.Fatalf("problem.detail = %#v, want non-empty string", problem["detail"])
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

func assertNoSecrets(t *testing.T, body string) {
	t.Helper()
	for _, secret := range []string{
		"password=secret",
		"monitoring.check_runs",
		"pgx",
		"dns",
		"tls",
		"filesystem",
		"/tmp/key",
	} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(secret)) {
			t.Fatalf("problem body leaked %q: %s", secret, body)
		}
	}
}

func mustCheckID(t *testing.T, raw string) domain.CheckID {
	t.Helper()
	id, err := domain.ParseCheckID(raw)
	if err != nil {
		t.Fatalf("ParseCheckID(%q) error = %v", raw, err)
	}
	return id
}

func mustMonitorID(t *testing.T, raw string) domain.MonitorID {
	t.Helper()
	id, err := domain.ParseMonitorID(raw)
	if err != nil {
		t.Fatalf("ParseMonitorID(%q) error = %v", raw, err)
	}
	return id
}

func mustTargetURL(t *testing.T, raw string) domain.TargetURL {
	t.Helper()
	target, err := domain.NewTargetURL(raw)
	if err != nil {
		t.Fatalf("NewTargetURL(%q) error = %v", raw, err)
	}
	return target
}
