package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const availabilityID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
const availabilityCheckID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"

type availabilityHTTPRepository struct {
	record ports.LatestCheckResultRecord
	err    error
}

func (r availabilityHTTPRepository) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	return r.record, r.err
}

type availabilityHTTPFailure struct{ err error }

func (f availabilityHTTPFailure) Execute(context.Context, domain.MonitorID) (application.MonitorAvailability, error) {
	return application.MonitorAvailability{}, f.err
}

func TestAvailabilityHTTPResponses(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123000000, time.UTC)
	checkID, err := domain.ParseCheckID(availabilityCheckID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		kind           domain.CheckResultKind
		code           int
		age            time.Duration
		repoErr        error
		status, reason string
	}{
		{"success", domain.CheckResultHTTPResponse, 204, time.Second, nil, "available", "successful_response"},
		{"http failure", domain.CheckResultHTTPResponse, 500, time.Second, nil, "unavailable", "unexpected_http_status"},
		{"probe failure", domain.CheckResultTimeout, 0, time.Second, nil, "unavailable", "probe_failure"},
		{"policy", domain.CheckResultPolicyRejected, 0, time.Second, nil, "unknown", "policy_rejected"},
		{"execution", domain.CheckResultWorkerTimeout, 0, time.Second, nil, "unknown", "execution_failure"},
		{"stale", domain.CheckResultHTTPResponse, 200, 121 * time.Second, nil, "unknown", "stale_result"},
		{"future", domain.CheckResultHTTPResponse, 200, -time.Second, nil, "unknown", "future_result"},
		{"no result", "", 0, 0, ports.ErrNoTerminalCheckResult, "unknown", "no_result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			duration := int64(1)
			record := ports.LatestCheckResultRecord{CheckID: checkID, ResultKind: tc.kind, DurationMS: &duration, CompletedAt: now.Add(-tc.age)}
			if tc.code != 0 {
				record.HTTPStatus = &tc.code
			}
			if tc.kind == domain.CheckResultWorkerTimeout {
				record.DurationMS = nil
			}
			useCase := application.NewGetMonitorAvailability(availabilityHTTPRepository{record, tc.repoErr}, func() time.Time { return now })
			handler := NewHandlerWithAvailability(nil, nil, nil, useCase)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/monitors/"+availabilityID+"/availability", nil))
			if recorder.Code != 200 || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("%d %v", recorder.Code, recorder.Header())
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			var status, reason, evaluated string
			json.Unmarshal(payload["status"], &status)
			json.Unmarshal(payload["reason"], &reason)
			json.Unmarshal(payload["evaluatedAt"], &evaluated)
			if status != tc.status || reason != tc.reason || evaluated != now.Format(time.RFC3339Nano) {
				t.Fatalf("%s", recorder.Body.String())
			}
			if tc.reason == "no_result" {
				if len(payload) != 3 || payload["evidence"] != nil {
					t.Fatal("no-result evidence present")
				}
				return
			}
			var evidence map[string]string
			if err := json.Unmarshal(payload["evidence"], &evidence); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 4 || len(evidence) != 2 || evidence["checkId"] != availabilityCheckID || evidence["completedAt"] != record.CompletedAt.Format(time.RFC3339Nano) {
				t.Fatalf("%s", recorder.Body.String())
			}
		})
	}
	for _, tc := range []struct {
		name, id string
		err      error
		code     int
	}{
		{"UUID", "bad", nil, 400}, {"missing", availabilityID, application.ErrMonitorNotFound, 404},
		{"persistence", availabilityID, application.ErrPersistence, 500}, {"clock", availabilityID, application.ErrAvailabilityEvaluation, 500},
		{"private", availabilityID, errors.New("private credential error"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandlerWithAvailability(nil, nil, nil, availabilityHTTPFailure{tc.err})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/monitors/"+tc.id+"/availability", nil))
			if recorder.Code != tc.code || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Content-Type") != "application/problem+json" || strings.Contains(recorder.Body.String(), "private") {
				t.Fatalf("%d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAvailabilityHTTPMethodsAndPaths(t *testing.T) {
	handler := NewHandlerWithAvailability(nil, nil, nil, availabilityHTTPFailure{application.ErrMonitorNotFound})
	for _, method := range []string{"POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/monitors/"+availabilityID+"/availability", nil))
		if recorder.Code != 405 || recorder.Header().Get("Allow") != "GET" || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %v", method, recorder.Code, recorder.Header())
		}
	}
	for _, path := range []string{"/monitors/" + availabilityID + "/availability/", "/monitors/" + availabilityID + "/availability/extra", "/monitors//availability", "/monitors/" + availabilityID + "/unknown"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
		if recorder.Code != 404 {
			t.Fatalf("%s: %d", path, recorder.Code)
		}
	}
	for _, legacy := range []*Handler{NewHandler(nil, nil), NewHandlerWithLatestResult(nil, nil, nil), NewHandlerWithAvailability(nil, nil, nil, nil)} {
		recorder := httptest.NewRecorder()
		legacy.ServeHTTP(recorder, httptest.NewRequest("GET", "/monitors/"+availabilityID+"/availability", nil))
		if recorder.Code != 404 {
			t.Fatalf("legacy: %d", recorder.Code)
		}
	}
}
