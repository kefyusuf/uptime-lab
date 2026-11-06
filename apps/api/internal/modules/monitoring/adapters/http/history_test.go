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
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const historyTestPath = "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd003/history"

type latestRecordStub struct{ record ports.LatestCheckResultRecord }

func (stub *latestRecordStub) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	return stub.record, nil
}

func historyTestInstant() time.Time {
	at, err := time.Parse("2006-01-02T15:04:05.000000Z", "2026-11-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}
	return at
}

type getMonitorHistoryStub struct {
	calls    int
	gotInput application.HistoryInput
	gotID    domain.MonitorID
	page     application.HistoryCandidates
	err      error
}

func (stub *getMonitorHistoryStub) Execute(_ context.Context, id domain.MonitorID, input application.HistoryInput) (application.HistoryCandidates, error) {
	stub.calls++
	stub.gotID = id
	stub.gotInput = input
	page := stub.page
	page.Limit = input.Limit
	return page, stub.err
}

func historyTestHandler(stub *getMonitorHistoryStub) *Handler {
	return NewHandlerWithHistory(nil, nil, nil, nil, nil, nil, nil, nil, stub)
}

func TestHistoryValidationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, method, target string
		status               int
	}{
		{name: "query before method", method: "HEAD", target: historyTestPath + "?", status: 400},
		{name: "method before id", method: "POST", target: "/monitors/invalid/history", status: 405},
		{name: "id before query", method: "GET", target: "/monitors/invalid/history?limit=abc", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &getMonitorHistoryStub{}
			recorder := httptest.NewRecorder()
			historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.target, nil))
			if recorder.Code != tc.status || stub.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", recorder.Code, stub.calls, recorder.Header())
			}
		})
	}
}

func TestHistoryQueryGrammar(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		limit       int
		cursor      string
	}{
		{name: "no query", query: "", limit: 20},
		{name: "limit only", query: "?limit=7", limit: 7},
		{name: "cursor only", query: "?cursor=" + historyTestCursor(t), limit: 20, cursor: historyTestCursor(t)},
		{name: "both in order", query: "?limit=7&cursor=" + historyTestCursor(t), limit: 7, cursor: historyTestCursor(t)},
		{name: "both reversed", query: "?cursor=" + historyTestCursor(t) + "&limit=7", limit: 7, cursor: historyTestCursor(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &getMonitorHistoryStub{}
			recorder := httptest.NewRecorder()
			historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, historyTestPath+tc.query, nil))
			if recorder.Code != 200 || stub.calls != 1 || stub.gotInput.Limit != tc.limit || stub.gotInput.Cursor != tc.cursor {
				t.Fatalf("status=%d calls=%d input=%+v", recorder.Code, stub.calls, stub.gotInput)
			}
		})
	}
	for _, rejected := range []string{
		"?limit=0", "?limit=51", "?limit=abc", "?limit=020", "?limit=+7", "?limit=7;8",
		"?limit=7&limit=7", "?cursor=" + historyTestCursor(t) + "&cursor=" + historyTestCursor(t),
		"?unknown=1", "?limit=", "?cursor", "?limit=%37", "?" + strings.Repeat("a", 129),
	} {
		t.Run(rejected, func(t *testing.T) {
			stub := &getMonitorHistoryStub{}
			recorder := httptest.NewRecorder()
			historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, historyTestPath+rejected, nil))
			if recorder.Code != 400 || stub.calls != 0 {
				t.Fatalf("status=%d calls=%d", recorder.Code, stub.calls)
			}
		})
	}
}

func historyTestCursor(t *testing.T) string {
	t.Helper()
	token, err := application.EncodeHistoryCursor(ports.HistoryAnchor{
		CompletedAt: historyTestInstant(),
		CheckID:     mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"),
	})
	if err != nil {
		t.Fatalf("EncodeHistoryCursor() error = %v", err)
	}
	return token
}

func TestHistoryMethodsAndNoStore(t *testing.T) {
	for _, method := range []string{"HEAD", "OPTIONS", "POST", "PUT", "DELETE", "PATCH"} {
		stub := &getMonitorHistoryStub{}
		recorder := httptest.NewRecorder()
		historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(method, historyTestPath, nil))
		if recorder.Code != http.StatusMethodNotAllowed ||
			recorder.Header().Get("Allow") != http.MethodGet ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			stub.calls != 0 {
			t.Fatal(method, recorder.Code, recorder.Header(), stub.calls)
		}
	}
}

func TestHistoryAliasesNeverRedirect(t *testing.T) {
	for _, target := range []string{
		"/x/.." + historyTestPath,
		"/" + historyTestPath,
		historyTestPath + "/",
		strings.Replace(historyTestPath, "monitors", "%6donitors", 1),
		strings.Replace(historyTestPath, "/history", "%2fhistory", 1),
	} {
		stub := &getMonitorHistoryStub{}
		recorder := httptest.NewRecorder()
		historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != 400 || recorder.Header().Get("Location") != "" || stub.calls != 0 {
			t.Fatal(target, recorder.Code, recorder.Header())
		}
	}
}

func TestHistoryErrorsAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{application.ErrMonitorNotFound, http.StatusNotFound},
		{errors.New("secret SQL host and target"), http.StatusInternalServerError},
		{context.Canceled, http.StatusInternalServerError},
	} {
		stub := &getMonitorHistoryStub{err: tc.err}
		recorder := httptest.NewRecorder()
		historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, historyTestPath, nil))
		if recorder.Code != tc.status ||
			stub.calls != 1 ||
			recorder.Header().Get("Content-Type") != "application/problem+json" ||
			recorder.Header().Get("Cache-Control") != "no-store" ||
			strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("status=%d body=%s calls=%d", recorder.Code, recorder.Body.String(), stub.calls)
		}
	}
}

func TestHistoryPageResponseShape(t *testing.T) {
	completedAt := historyTestInstant()
	status := 204
	duration := int64(123)
	recordRepository := &latestRecordStub{record: ports.LatestCheckResultRecord{
		CheckID:     mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"),
		ResultKind:  domain.CheckResultHTTPResponse,
		HTTPStatus:  &status,
		DurationMS:  &duration,
		CompletedAt: completedAt,
	}}
	monitorID, parseErr := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	if parseErr != nil {
		t.Fatalf("ParseMonitorID() error = %v", parseErr)
	}
	entry, entryErr := application.NewGetLatestCheckResult(recordRepository).Execute(context.Background(), monitorID)
	if entryErr != nil {
		t.Fatalf("latest-result mapping error = %v", entryErr)
	}
	stub := &getMonitorHistoryStub{page: application.HistoryCandidates{
		Items: []application.LatestCheckResult{entry},
		Limit: 20,
	}}
	recorder := httptest.NewRecorder()
	historyTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, historyTestPath, nil))
	if recorder.Code != 200 || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status=%d type=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := recorder.Body.String()
	for _, expected := range []string{`"checkId":"7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"`, `"resultKind":"http_response"`, `"httpStatus":204`, `"durationMs":123`, `"nextCursor":null`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body %q missing %q", body, expected)
		}
	}
}
