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
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const incidentsTestPath = "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd003/incidents"

type getMonitorIncidentsStub struct {
	calls    int
	gotInput application.IncidentInput
	gotID    domain.MonitorID
	page     application.IncidentCandidates
	err      error
}

func (stub *getMonitorIncidentsStub) Execute(_ context.Context, id domain.MonitorID, input application.IncidentInput) (application.IncidentCandidates, error) {
	stub.calls++
	stub.gotID = id
	stub.gotInput = input
	page := stub.page
	page.Limit = input.Limit
	return page, stub.err
}

type incidentRecordStub struct {
	records    []ports.IncidentRecord
	err        error
	calls      int
	gotMaxRows int
	gotAnchor  *ports.IncidentAnchor
}

func (stub *incidentRecordStub) PageIncidents(_ context.Context, _ domain.MonitorID, anchor *ports.IncidentAnchor, maxRows int) ([]ports.IncidentRecord, error) {
	stub.calls++
	stub.gotAnchor = anchor
	stub.gotMaxRows = maxRows
	return stub.records, stub.err
}

func incidentsTestHandler(stub *getMonitorIncidentsStub) *Handler {
	return NewHandlerWithIncidents(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, stub)
}

func incidentsTestMonitorID(t *testing.T) domain.MonitorID {
	t.Helper()
	id, err := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafd003")
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	return id
}

func mustIncidentTestID(t *testing.T, raw string) domain.IncidentID {
	t.Helper()
	id, err := domain.ParseIncidentID(raw)
	if err != nil {
		t.Fatalf("ParseIncidentID() error = %v", err)
	}
	return id
}

func incidentsTestInstant(offsetMinutes int) time.Time {
	at, err := time.Parse("2006-01-02T15:04:05.000000Z", "2026-11-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}
	return at.Add(time.Duration(offsetMinutes) * time.Minute)
}

func incidentsTestCursor(t *testing.T) string {
	t.Helper()
	token, err := application.EncodeIncidentCursor(ports.IncidentAnchor{
		OpenedAt:   incidentsTestInstant(0),
		IncidentID: mustIncidentTestID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
	})
	if err != nil {
		t.Fatalf("EncodeIncidentCursor() error = %v", err)
	}
	return token
}

func incidentTestViews(t *testing.T, records ...ports.IncidentRecord) []application.Incident {
	t.Helper()
	page, err := application.NewGetMonitorIncidents(&incidentRecordStub{records: records}).Execute(
		context.Background(), incidentsTestMonitorID(t), application.IncidentInput{Limit: 50})
	if err != nil {
		t.Fatalf("build incident views: %v", err)
	}
	return page.Items
}

func incidentTestRecords(t *testing.T) []ports.IncidentRecord {
	t.Helper()
	openedAt := incidentsTestInstant(0)
	resolvedOpenedAt := incidentsTestInstant(-10)
	resolvedAt := incidentsTestInstant(-5)
	nullEvidenceOpenedAt := incidentsTestInstant(-20)
	openedBy := mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675")
	resolvedOpenedBy := mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c676")
	resolvedBy := mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c677")
	return []ports.IncidentRecord{
		{
			IncidentID:      mustIncidentTestID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001"),
			OpenedAt:        openedAt,
			OpeningReason:   domain.IncidentUnexpectedHTTPStatus,
			OpenedByCheckID: &openedBy,
		},
		{
			IncidentID:        mustIncidentTestID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff002"),
			OpenedAt:          resolvedOpenedAt,
			ResolvedAt:        &resolvedAt,
			OpeningReason:     domain.IncidentProbeFailure,
			OpenedByCheckID:   &resolvedOpenedBy,
			ResolvedByCheckID: &resolvedBy,
		},
		{
			IncidentID:    mustIncidentTestID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff004"),
			OpenedAt:      nullEvidenceOpenedAt,
			OpeningReason: domain.IncidentProbeFailure,
		},
	}
}

func TestIncidentsSerializesOpenAndResolved(t *testing.T) {
	records := incidentTestRecords(t)
	handler := NewHandlerWithIncidents(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		application.NewGetMonitorIncidents(&incidentRecordStub{records: records}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath, nil))
	if recorder.Code != 200 || recorder.Header().Get("Content-Type") != "application/json" || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d headers=%v", recorder.Code, recorder.Header())
	}
	var page struct {
		Items      []map[string]json.RawMessage `json:"items"`
		NextCursor *string                      `json:"nextCursor"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page: %v; body=%s", err, recorder.Body.String())
	}
	if len(page.Items) != 3 || page.NextCursor != nil {
		t.Fatalf("items=%d nextCursor=%v", len(page.Items), page.NextCursor)
	}
	byID := map[string]map[string]json.RawMessage{}
	for _, item := range page.Items {
		var id string
		if err := json.Unmarshal(item["incidentId"], &id); err != nil {
			t.Fatalf("incidentId: %v", err)
		}
		byID[id] = item
	}
	open := byID["018f22d3-1d6a-7cc0-a37b-46fc3faff001"]
	if open == nil {
		t.Fatal("missing open incident")
	}
	if _, ok := open["resolvedAt"]; ok {
		t.Fatalf("open incident must omit resolvedAt: %v", open)
	}
	if _, ok := open["resolvedByCheckId"]; ok {
		t.Fatalf("open incident must omit resolvedByCheckId: %v", open)
	}
	if string(open["openingReason"]) != `"unexpected_http_status"` || string(open["openedByCheckId"]) != `"7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"` {
		t.Fatalf("open incident shape: %v", open)
	}
	resolved := byID["018f22d3-1d6a-7cc0-a37b-46fc3faff002"]
	if resolved == nil {
		t.Fatal("missing resolved incident")
	}
	if _, ok := resolved["resolvedAt"]; !ok {
		t.Fatalf("resolved incident must include resolvedAt: %v", resolved)
	}
	if string(resolved["resolvedByCheckId"]) != `"7d9b2eb0-52bb-4dd7-8934-c5ce30d5c677"` || string(resolved["openingReason"]) != `"probe_failure"` {
		t.Fatalf("resolved incident shape: %v", resolved)
	}
	nullEvidence := byID["018f22d3-1d6a-7cc0-a37b-46fc3faff004"]
	if nullEvidence == nil {
		t.Fatal("missing null-evidence incident")
	}
	if _, ok := nullEvidence["openedByCheckId"]; ok {
		t.Fatalf("null evidence must omit openedByCheckId: %v", nullEvidence)
	}
	if _, ok := nullEvidence["resolvedByCheckId"]; ok {
		t.Fatalf("null evidence must omit resolvedByCheckId: %v", nullEvidence)
	}
	if _, ok := nullEvidence["resolvedAt"]; ok {
		t.Fatalf("null evidence open incident must omit resolvedAt: %v", nullEvidence)
	}
}

func TestIncidentsTransportGuardsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name, method, target string
		status               int
	}{
		{name: "query before method", method: "HEAD", target: incidentsTestPath + "?", status: 400},
		{name: "method before id", method: "POST", target: "/monitors/invalid/incidents", status: 405},
		{name: "id before query", method: "GET", target: "/monitors/invalid/incidents?limit=abc", status: 400},
		{name: "empty query", method: "GET", target: incidentsTestPath + "?", status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.target, nil))
			if recorder.Code != tc.status || stub.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", recorder.Code, stub.calls, recorder.Header())
			}
		})
	}
	t.Run("request body rejected", func(t *testing.T) {
		stub := &getMonitorIncidentsStub{}
		recorder := httptest.NewRecorder()
		incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath, strings.NewReader("x")))
		if recorder.Code != 400 || stub.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("status=%d calls=%d", recorder.Code, stub.calls)
		}
	})
	t.Run("transfer encoding rejected", func(t *testing.T) {
		stub := &getMonitorIncidentsStub{}
		request := httptest.NewRequest(http.MethodGet, incidentsTestPath, nil)
		request.TransferEncoding = []string{"chunked"}
		recorder := httptest.NewRecorder()
		incidentsTestHandler(stub).ServeHTTP(recorder, request)
		if recorder.Code != 400 || stub.calls != 0 {
			t.Fatalf("status=%d calls=%d", recorder.Code, stub.calls)
		}
	})
	for _, method := range []string{"HEAD", "OPTIONS", "POST", "PUT", "DELETE", "PATCH"} {
		t.Run("method "+method, func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(method, incidentsTestPath, nil))
			if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodGet || recorder.Header().Get("Cache-Control") != "no-store" || stub.calls != 0 {
				t.Fatalf("%s status=%d headers=%v calls=%d", method, recorder.Code, recorder.Header(), stub.calls)
			}
		})
	}
	for _, target := range []string{
		"/x/.." + incidentsTestPath,
		"/" + incidentsTestPath,
		incidentsTestPath + "/",
		strings.Replace(incidentsTestPath, "monitors", "%6donitors", 1),
		strings.Replace(incidentsTestPath, "/incidents", "%2fincidents", 1),
	} {
		t.Run("alias "+target, func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != 400 || recorder.Header().Get("Location") != "" || stub.calls != 0 {
				t.Fatalf("%s status=%d headers=%v", target, recorder.Code, recorder.Header())
			}
		})
	}
	for _, tc := range []struct {
		err    error
		status int
	}{
		{application.ErrMonitorNotFound, http.StatusNotFound},
		{errors.New("secret SQL host and target"), http.StatusInternalServerError},
		{context.Canceled, http.StatusInternalServerError},
	} {
		t.Run(fmt.Sprintf("error %d", tc.status), func(t *testing.T) {
			stub := &getMonitorIncidentsStub{err: tc.err}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath, nil))
			if recorder.Code != tc.status || stub.calls != 1 || recorder.Header().Get("Content-Type") != "application/problem+json" || recorder.Header().Get("Cache-Control") != "no-store" || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("status=%d body=%s calls=%d", recorder.Code, recorder.Body.String(), stub.calls)
			}
		})
	}
}

func TestIncidentsQueryGrammarAndBudget(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		limit       int
		cursor      string
	}{
		{name: "no query", query: "", limit: 20},
		{name: "limit only", query: "?limit=7", limit: 7},
		{name: "cursor only", query: "?cursor=" + incidentsTestCursor(t), limit: 20, cursor: incidentsTestCursor(t)},
		{name: "both in order", query: "?limit=7&cursor=" + incidentsTestCursor(t), limit: 7, cursor: incidentsTestCursor(t)},
		{name: "both reversed", query: "?cursor=" + incidentsTestCursor(t) + "&limit=7", limit: 7, cursor: incidentsTestCursor(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath+tc.query, nil))
			if recorder.Code != 200 || stub.calls != 1 || stub.gotInput.Limit != tc.limit || stub.gotInput.Cursor != tc.cursor {
				t.Fatalf("status=%d calls=%d input=%+v", recorder.Code, stub.calls, stub.gotInput)
			}
		})
	}
	for _, rejected := range []string{
		"?limit=0", "?limit=51", "?limit=abc", "?limit=020", "?limit=+7", "?limit=7;8",
		"?limit=7&limit=7", "?cursor=" + incidentsTestCursor(t) + "&cursor=" + incidentsTestCursor(t),
		"?unknown=1", "?limit=", "?cursor", "?limit=%37", "?" + strings.Repeat("a", 129),
	} {
		t.Run(rejected, func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath+rejected, nil))
			if recorder.Code != 400 || stub.calls != 0 {
				t.Fatalf("status=%d calls=%d", recorder.Code, stub.calls)
			}
		})
	}
	t.Run("empty page", func(t *testing.T) {
		body, err := encodeIncidentPage(nil, 20)
		if err != nil || string(body) != `{"items":[],"nextCursor":null}` {
			t.Fatalf("body=%s err=%v", body, err)
		}
	})
	t.Run("full page within budget", func(t *testing.T) {
		records := make([]ports.IncidentRecord, 50)
		for index := range records {
			records[index] = ports.IncidentRecord{
				IncidentID:    mustIncidentTestID(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3faff%03d", index)),
				OpenedAt:      incidentsTestInstant(-index),
				OpeningReason: domain.IncidentProbeFailure,
			}
		}
		views := incidentTestViews(t, records...)
		body, err := encodeIncidentPage(views, 50)
		if err != nil || len(body) > 245760 {
			t.Fatalf("len=%d err=%v", len(body), err)
		}
		var page struct {
			Items      []map[string]json.RawMessage `json:"items"`
			NextCursor *string                      `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &page); err != nil || len(page.Items) != 50 || page.NextCursor != nil {
			t.Fatalf("items=%d nextCursor=%v err=%v", len(page.Items), page.NextCursor, err)
		}
	})
	t.Run("lookahead drives version3 cursor", func(t *testing.T) {
		records := incidentTestRecords(t)
		views := incidentTestViews(t, records...)
		body, err := encodeIncidentPage(views, 2)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		var page struct {
			Items      []map[string]json.RawMessage `json:"items"`
			NextCursor *string                      `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &page); err != nil || len(page.Items) != 2 || page.NextCursor == nil {
			t.Fatalf("items=%d nextCursor=%v err=%v", len(page.Items), page.NextCursor, err)
		}
		anchor, err := application.DecodeIncidentCursor(*page.NextCursor)
		if err != nil || anchor.IncidentID != views[1].IncidentID() || !anchor.OpenedAt.Equal(views[1].OpenedAt()) {
			t.Fatalf("anchor=%+v err=%v", anchor, err)
		}
	})
}

func TestIncidentsCursorVersion3Rejection(t *testing.T) {
	historyToken, err := application.EncodeHistoryCursor(ports.HistoryAnchor{
		CompletedAt: incidentsTestInstant(0),
		CheckID:     mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"),
	})
	if err != nil {
		t.Fatalf("EncodeHistoryCursor() error = %v", err)
	}
	inventoryToken, err := application.EncodeInventoryCursor(ports.InventoryAnchor{
		ID:        incidentsTestMonitorID(t),
		CreatedAt: incidentsTestInstant(0),
	})
	if err != nil {
		t.Fatalf("EncodeInventoryCursor() error = %v", err)
	}
	for _, token := range []string{historyToken, inventoryToken, incidentsTestCursor(t) + "=", strings.Repeat("A", 88)} {
		t.Run("rejects "+token[:8], func(t *testing.T) {
			stub := &getMonitorIncidentsStub{}
			recorder := httptest.NewRecorder()
			incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath+"?cursor="+token, nil))
			if recorder.Code != 400 || stub.calls != 0 {
				t.Fatalf("status=%d calls=%d", recorder.Code, stub.calls)
			}
		})
	}
	t.Run("accepts version3", func(t *testing.T) {
		stub := &getMonitorIncidentsStub{}
		recorder := httptest.NewRecorder()
		incidentsTestHandler(stub).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, incidentsTestPath+"?cursor="+incidentsTestCursor(t), nil))
		if recorder.Code != 200 || stub.calls != 1 || stub.gotInput.Cursor != incidentsTestCursor(t) {
			t.Fatalf("status=%d calls=%d input=%+v", recorder.Code, stub.calls, stub.gotInput)
		}
	})
}
