package monitoringhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

const deletionTestPath = "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd002"

type deleteMonitorStub struct {
	calls int
	ids   []domain.MonitorID
	err   error
}

func (stub *deleteMonitorStub) Execute(_ context.Context, id domain.MonitorID) error {
	stub.calls++
	stub.ids = append(stub.ids, id)
	return stub.err
}

func deletionTestHandler(stub *deleteMonitorStub) *Handler {
	return NewHandlerWithDeletion(nil, nil, nil, nil, nil, nil, nil, stub)
}

func TestDeleteRemovesMonitorWithEmpty204(t *testing.T) {
	stub := &deleteMonitorStub{}
	handler := deletionTestHandler(stub)
	response := serve(handler, http.MethodDelete, deletionTestPath, "", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", response.Code, response.Body.String())
	}
	if response.Body.Len() != 0 {
		t.Fatalf("204 body = %q, want empty", response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	if response.Header().Get("Location") != "" {
		t.Fatalf("Location = %q, want absent", response.Header().Get("Location"))
	}
	if stub.calls != 1 || stub.ids[0].String() != "018f22d3-1d6a-7cc0-a37b-46fc3fafd002" {
		t.Fatalf("use case calls/ids = %d/%v", stub.calls, stub.ids)
	}
}

func TestDeleteValidationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, method, target string
		length               int64
		chunked              bool
		status               int
	}{
		{name: "query before method", method: "HEAD", target: deletionTestPath + "?", status: 400},
		{name: "method before id", method: "POST", target: "/monitors/invalid", status: 405},
		{name: "id before body", method: http.MethodDelete, target: "/monitors/invalid", length: 1, status: 400},
		{name: "declared body", method: http.MethodDelete, target: deletionTestPath, length: 1, status: 400},
		{name: "chunked body", method: http.MethodDelete, target: deletionTestPath, chunked: true, status: 400},
		{name: "get query", method: http.MethodGet, target: deletionTestPath + "?x=1", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &deleteMonitorStub{}
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader("x"))
			if tc.length != 0 {
				req.ContentLength = tc.length
			}
			if tc.chunked {
				req.TransferEncoding = []string{"chunked"}
			}
			recorder := httptest.NewRecorder()
			deletionTestHandler(stub).ServeHTTP(recorder, req)
			if recorder.Code != tc.status || stub.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", recorder.Code, stub.calls, recorder.Header())
			}
		})
	}
}

func TestDeleteMethodsAndNoStore(t *testing.T) {
	for _, method := range []string{"HEAD", "OPTIONS", "POST", "PUT", "PATCH"} {
		stub := &deleteMonitorStub{}
		recorder := httptest.NewRecorder()
		deletionTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(method, deletionTestPath, nil))
		if recorder.Code != http.StatusMethodNotAllowed ||
			recorder.Header().Get("Allow") != "GET, DELETE" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			stub.calls != 0 {
			t.Fatal(method, recorder.Code, recorder.Header(), stub.calls)
		}
	}
}

func TestDeleteErrorsAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{application.ErrMonitorNotFound, http.StatusNotFound},
		{errors.New("secret SQL host and target"), http.StatusInternalServerError},
		{context.Canceled, http.StatusInternalServerError},
	} {
		stub := &deleteMonitorStub{err: tc.err}
		recorder := httptest.NewRecorder()
		deletionTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, deletionTestPath, nil))
		if recorder.Code != tc.status ||
			stub.calls != 1 ||
			recorder.Header().Get("Content-Type") != "application/problem+json" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("status=%d body=%s calls=%d", recorder.Code, recorder.Body.String(), stub.calls)
		}
	}
}

func TestDeleteAliasesNeverRedirect(t *testing.T) {
	for _, target := range []string{
		"/x/.." + deletionTestPath,
		"/" + deletionTestPath,
		deletionTestPath + "/",
		strings.Replace(deletionTestPath, "monitors", "%6donitors", 1),
		strings.Replace(deletionTestPath, "/018f22d3", "%2f018f22d3", 1),
	} {
		stub := &deleteMonitorStub{}
		recorder := httptest.NewRecorder()
		deletionTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, target, nil))
		if recorder.Code != 400 ||
			recorder.Header().Get("Location") != "" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			stub.calls != 0 {
			t.Fatal(target, recorder.Code, recorder.Header(), stub.calls)
		}
	}
}

func TestLegacyResourceKeepsGetOnlyAllow(t *testing.T) {
	stub := &deleteMonitorStub{}
	handler := NewHandlerWithLatestResult(&registerMonitorStub{}, &getMonitorStub{}, &getLatestCheckResultStub{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, deletionTestPath, nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodGet {
		t.Fatal(recorder.Code, recorder.Header())
	}
	if stub.calls != 0 {
		t.Fatal(stub.calls)
	}
}
