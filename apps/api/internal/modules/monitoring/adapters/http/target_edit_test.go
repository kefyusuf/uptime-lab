package monitoringhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

const targetEditTestPath = "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd006"

type editMonitorTargetStub struct {
	calls      int
	gotID      domain.MonitorID
	gotTarget  string
	editResult domain.Monitor
	err        error
}

func (stub *editMonitorTargetStub) Execute(_ context.Context, id domain.MonitorID, rawTarget string) (domain.Monitor, error) {
	stub.calls++
	stub.gotID = id
	stub.gotTarget = rawTarget
	return stub.editResult, stub.err
}

func targetEditTestHandler(stub *editMonitorTargetStub) *Handler {
	return NewHandlerWithTargetEdit(nil, nil, nil, nil, nil, nil, nil, &deleteMonitorStub{}, nil, stub)
}

func TestTargetEditValidationPrecedence(t *testing.T) {
	validBody := `{"targetUrl":"https://example.com/edited"}`
	for _, tc := range []struct {
		name, method, target, media, body string
		status                            int
	}{
		{name: "media before body", method: http.MethodPatch, target: targetEditTestPath, media: "text/plain", body: validBody, status: 415},
		{name: "method before id", method: http.MethodPut, target: "/monitors/invalid", media: "application/json", body: validBody, status: 405},
		{name: "id before body", method: http.MethodPatch, target: "/monitors/invalid", media: "application/json", body: validBody, status: 400},
		{name: "empty body", method: http.MethodPatch, target: targetEditTestPath, media: "application/json", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &editMonitorTargetStub{}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			if tc.media != "" {
				request.Header.Set("Content-Type", tc.media)
			}
			targetEditTestHandler(stub).ServeHTTP(recorder, request)
			if recorder.Code != tc.status || stub.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", recorder.Code, stub.calls, recorder.Header())
			}
		})
	}
}

func TestTargetEditBodyGrammar(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{name: "duplicate keys", body: `{"targetUrl":"https://a.test/","targetUrl":"https://b.test/"}`},
		{name: "unknown keys", body: `{"targetUrl":"https://a.test/","extra":1}`},
		{name: "missing targetUrl", body: `{"other":"https://a.test/"}`},
		{name: "non string target", body: `{"targetUrl":42}`},
		{name: "empty object", body: `{}`},
		{name: "array", body: `[]`},
		{name: "null", body: `null`},
		{name: "trailing json", body: `{"targetUrl":"https://a.test/"} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &editMonitorTargetStub{}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPatch, targetEditTestPath, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			targetEditTestHandler(stub).ServeHTTP(recorder, request)
			if recorder.Code != 422 && recorder.Code != 400 {
				t.Fatalf("status=%d", recorder.Code)
			}
			if stub.calls != 0 {
				t.Fatalf("use case calls = %d, want 0", stub.calls)
			}
		})
	}
	stub := &editMonitorTargetStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, targetEditTestPath, strings.NewReader(`{"targetUrl":"https://example.com/edited","extra":1}`))
	request.Header.Set("Content-Type", "application/json")
	targetEditTestHandler(stub).ServeHTTP(recorder, request)
	if recorder.Code != 422 || stub.calls != 0 {
		t.Fatalf("unknown key = %d calls %d", recorder.Code, stub.calls)
	}
}

func TestTargetEditSuccess(t *testing.T) {
	stub := &editMonitorTargetStub{editResult: mustEditedMonitor(t)}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, targetEditTestPath, strings.NewReader(`{"targetUrl":"https://example.com/edited"}`))
	request.Header.Set("Content-Type", "application/json")
	targetEditTestHandler(stub).ServeHTTP(recorder, request)
	if recorder.Code != 200 || stub.calls != 1 || stub.gotID != targetEditTestID(t) || stub.gotTarget != "https://example.com/edited" {
		t.Fatalf("status=%d calls=%d target=%q", recorder.Code, stub.calls, stub.gotTarget)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("headers=%v", recorder.Header())
	}
	if !strings.Contains(recorder.Body.String(), `"targetUrl":"https://example.com/edited"`) {
		t.Fatalf("body=%q", recorder.Body.String())
	}
}

func mustEditedMonitor(t *testing.T) domain.Monitor {
	t.Helper()
	id, err := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafd006")
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	target, err := domain.NewTargetURL("https://example.com/edited")
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	monitor, err := domain.NewMonitor(id, target, time.Date(2026, 11, 1, 12, 0, 0, 123456000, time.UTC))
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}
	return monitor
}

func targetEditTestID(t *testing.T) domain.MonitorID {
	t.Helper()
	id, err := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafd006")
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	return id
}

func TestTargetEditErrorsAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{application.ErrMonitorNotFound, http.StatusNotFound},
		{domain.ErrInvalidTargetURL, http.StatusUnprocessableEntity},
		{errors.New("secret SQL host and target"), http.StatusInternalServerError},
		{context.Canceled, http.StatusInternalServerError},
	} {
		stub := &editMonitorTargetStub{err: tc.err}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, targetEditTestPath, strings.NewReader(`{"targetUrl":"https://example.com/edited"}`))
		request.Header.Set("Content-Type", "application/json")
		targetEditTestHandler(stub).ServeHTTP(recorder, request)
		if recorder.Code != tc.status ||
			stub.calls != 1 ||
			recorder.Header().Get("Content-Type") != "application/problem+json" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("status=%d body=%s calls=%d", recorder.Code, recorder.Body.String(), stub.calls)
		}
	}
}

func TestTargetEditMethodsAndNoStore(t *testing.T) {
	for _, method := range []string{"HEAD", "OPTIONS", "POST", "PUT", "TRACE"} {
		stub := &editMonitorTargetStub{}
		recorder := httptest.NewRecorder()
		targetEditTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(method, targetEditTestPath, nil))
		if recorder.Code != http.StatusMethodNotAllowed ||
			recorder.Header().Get("Allow") != "GET, PATCH, DELETE" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			stub.calls != 0 {
			t.Fatal(method, recorder.Code, recorder.Header(), stub.calls)
		}
	}
}

func TestTargetEditAliasesNeverRedirect(t *testing.T) {
	for _, target := range []string{
		"/x/.." + targetEditTestPath,
		"/" + targetEditTestPath,
		targetEditTestPath + "/",
		strings.Replace(targetEditTestPath, "monitors", "%6donitors", 1),
		targetEditTestPath + "?edit=1",
	} {
		stub := &editMonitorTargetStub{}
		recorder := httptest.NewRecorder()
		targetEditTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, target, strings.NewReader(`{"targetUrl":"https://example.com/x"}`)))
		if recorder.Code != 400 && recorder.Code != 405 {
			t.Fatalf("target=%s status=%d", target, recorder.Code)
		}
		if recorder.Header().Get("Location") != "" || stub.calls != 0 {
			t.Fatalf("target=%s location=%q calls=%d", target, recorder.Header().Get("Location"), stub.calls)
		}
	}
}

func TestTargetEditBudget(t *testing.T) {
	stub := &editMonitorTargetStub{}
	budget := 16384
	body := `{"targetUrl":"` + strings.Repeat("a", budget) + `"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, targetEditTestPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	targetEditTestHandler(stub).ServeHTTP(recorder, request)
	if recorder.Code != 400 && recorder.Code != 413 {
		t.Fatalf("over-budget status=%d", recorder.Code)
	}
	if stub.calls != 0 {
		t.Fatalf("use case calls = %d, want 0", stub.calls)
	}
}

func TestLegacyResourceAllowIncludesPatch(t *testing.T) {
	stub := &editMonitorTargetStub{}
	handler := targetEditTestHandler(stub)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, targetEditTestPath, nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, PATCH, DELETE" {
		t.Fatal(recorder.Code, recorder.Header())
	}
	if stub.calls != 0 {
		t.Fatal(stub.calls)
	}
}
