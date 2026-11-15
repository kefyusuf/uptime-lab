package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const availabilityID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
const availabilityCheckID = "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"

type availabilityHTTPRepository struct {
	recent []ports.LatestCheckResultRecord
	err    error
}

func (r availabilityHTTPRepository) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	if len(r.recent) > 0 {
		return r.recent[0], r.err
	}
	return ports.LatestCheckResultRecord{}, r.err
}

func (r availabilityHTTPRepository) RecentTerminalByMonitorID(context.Context, domain.MonitorID, int) ([]ports.LatestCheckResultRecord, error) {
	return r.recent, r.err
}

type availabilityHTTPFailure struct{ err error }

func (f availabilityHTTPFailure) Execute(context.Context, domain.MonitorID) (application.MonitorAvailability, error) {
	return application.MonitorAvailability{}, f.err
}

type availabilityHTTPSpec struct {
	kind domain.CheckResultKind
	code int
}

var (
	availSpecOK      = availabilityHTTPSpec{domain.CheckResultHTTPResponse, 204}
	availSpec500     = availabilityHTTPSpec{domain.CheckResultHTTPResponse, 500}
	availSpecDNS     = availabilityHTTPSpec{domain.CheckResultDNSFailure, 0}
	availSpecTimeout = availabilityHTTPSpec{domain.CheckResultTimeout, 0}
	availSpecPolicy  = availabilityHTTPSpec{domain.CheckResultPolicyRejected, 0}
	availSpecWorker  = availabilityHTTPSpec{domain.CheckResultWorkerTimeout, 0}
)

func availabilityHTTPRecords(checkID domain.CheckID, latest time.Time, specs ...availabilityHTTPSpec) []ports.LatestCheckResultRecord {
	records := make([]ports.LatestCheckResultRecord, len(specs))
	for index, spec := range specs {
		duration := int64(1)
		record := ports.LatestCheckResultRecord{
			CheckID:     checkID,
			ResultKind:  spec.kind,
			CompletedAt: latest.Add(-time.Duration(index) * time.Second),
		}
		switch spec.kind {
		case domain.CheckResultHTTPResponse:
			code := spec.code
			record.HTTPStatus = &code
			record.DurationMS = &duration
		case domain.CheckResultWorkerTimeout:
		default:
			record.DurationMS = &duration
		}
		records[index] = record
	}
	return records
}

func TestAvailabilityHTTPResponses(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123000000, time.UTC)
	checkID, err := domain.ParseCheckID(availabilityCheckID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		specs          []availabilityHTTPSpec
		age            time.Duration
		status, reason string
	}{
		{"success", []availabilityHTTPSpec{availSpecOK}, time.Second, "available", "successful_response"},
		{"http failure awaits", []availabilityHTTPSpec{availSpec500}, time.Second, "unknown", "awaiting_confirmation"},
		{"probe failure awaits", []availabilityHTTPSpec{availSpecTimeout}, time.Second, "unknown", "awaiting_confirmation"},
		{"confirmed http failure", []availabilityHTTPSpec{availSpec500, availSpec500, availSpec500}, time.Second, "unavailable", "unexpected_http_status"},
		{"confirmed probe failure", []availabilityHTTPSpec{availSpecDNS, availSpecDNS, availSpecDNS}, time.Second, "unavailable", "probe_failure"},
		{"recovering", []availabilityHTTPSpec{availSpecOK, availSpecDNS, availSpecDNS, availSpecDNS}, time.Second, "unavailable", "recovering"},
		{"recovered", []availabilityHTTPSpec{availSpecOK, availSpecOK, availSpecDNS, availSpecDNS, availSpecDNS}, time.Second, "available", "successful_response"},
		{"policy", []availabilityHTTPSpec{availSpecPolicy}, time.Second, "unknown", "policy_rejected"},
		{"execution", []availabilityHTTPSpec{availSpecWorker}, time.Second, "unknown", "execution_failure"},
		{"stale", []availabilityHTTPSpec{availSpecOK}, 121 * time.Second, "unknown", "stale_result"},
		{"future", []availabilityHTTPSpec{availSpecOK}, -time.Second, "unknown", "future_result"},
		{"no result", nil, 0, "unknown", "no_result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			records := availabilityHTTPRecords(checkID, now.Add(-tc.age), tc.specs...)
			useCase := application.NewGetMonitorAvailability(availabilityHTTPRepository{recent: records}, func() time.Time { return now })
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
			if len(payload) != 4 || len(evidence) != 2 || evidence["checkId"] != availabilityCheckID || evidence["completedAt"] != records[0].CompletedAt.Format(time.RFC3339Nano) {
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

func availabilityTCP(t *testing.T, handler http.Handler) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func availabilityUseCase(specs []availabilityHTTPSpec, now time.Time) application.GetMonitorAvailability {
	checkID, err := domain.ParseCheckID(availabilityCheckID)
	if err != nil {
		panic(err)
	}
	records := availabilityHTTPRecords(checkID, now.Add(-time.Second), specs...)
	return application.NewGetMonitorAvailability(availabilityHTTPRepository{recent: records}, func() time.Time { return now })
}

func TestAvailabilitySerializesRecovering(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123000000, time.UTC)
	useCase := availabilityUseCase([]availabilityHTTPSpec{availSpecOK, availSpecDNS, availSpecDNS, availSpecDNS}, now)
	baseURL := availabilityTCP(t, NewHandlerWithAvailability(nil, nil, nil, useCase))
	response, err := http.Get(baseURL + "/monitors/" + availabilityID + "/availability")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%d %v %s", response.StatusCode, response.Header, body)
	}
	var got struct {
		Status      string `json:"status"`
		Reason      string `json:"reason"`
		EvaluatedAt string `json:"evaluatedAt"`
		Evidence    *struct {
			CheckID     string `json:"checkId"`
			CompletedAt string `json:"completedAt"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "unavailable" || got.Reason != "recovering" || got.EvaluatedAt != now.Format(time.RFC3339Nano) || got.Evidence == nil || got.Evidence.CheckID != availabilityCheckID || got.Evidence.CompletedAt != now.Add(-time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("%s", body)
	}
}

func TestAvailabilitySerializesAwaitingConfirmation(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123000000, time.UTC)
	useCase := availabilityUseCase([]availabilityHTTPSpec{availSpecDNS}, now)
	baseURL := availabilityTCP(t, NewHandlerWithAvailability(nil, nil, nil, useCase))
	response, err := http.Get(baseURL + "/monitors/" + availabilityID + "/availability")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%d %v %s", response.StatusCode, response.Header, body)
	}
	var got struct {
		Status   string `json:"status"`
		Reason   string `json:"reason"`
		Evidence *struct {
			CheckID string `json:"checkId"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "unknown" || got.Reason != "awaiting_confirmation" || got.Evidence == nil || got.Evidence.CheckID != availabilityCheckID {
		t.Fatalf("%s", body)
	}
}

func TestAvailabilityTransportGuardsUnchanged(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 123000000, time.UTC)
	recovering := NewHandlerWithAvailability(nil, nil, nil, availabilityUseCase([]availabilityHTTPSpec{availSpecOK, availSpecDNS, availSpecDNS, availSpecDNS}, now))
	baseURL := availabilityTCP(t, recovering)
	path := "/monitors/" + availabilityID + "/availability"

	head, err := http.Head(baseURL + path)
	if err != nil {
		t.Fatal(err)
	}
	head.Body.Close()
	if head.StatusCode != 405 || head.Header.Get("Allow") != "GET" || head.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD: %d %v", head.StatusCode, head.Header)
	}

	malformed, err := http.Get(baseURL + "/monitors/bad/availability")
	if err != nil {
		t.Fatal(err)
	}
	malformedBody, _ := io.ReadAll(malformed.Body)
	malformed.Body.Close()
	if malformed.StatusCode != 400 || malformed.Header.Get("Cache-Control") != "no-store" || malformed.Header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("malformed: %d %v %s", malformed.StatusCode, malformed.Header, malformedBody)
	}

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"not found", ports.ErrMonitorNotFound},
		{"private", errors.New("private credential error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandlerWithAvailability(nil, nil, nil, application.NewGetMonitorAvailability(availabilityHTTPRepository{err: tc.err}, func() time.Time { return now }))
			url := availabilityTCP(t, handler)
			response, err := http.Get(url + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			want := 500
			if errors.Is(tc.err, ports.ErrMonitorNotFound) {
				want = 404
			}
			if response.StatusCode != want || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Type") != "application/problem+json" || strings.Contains(string(body), "private") {
				t.Fatalf("%d %v %s", response.StatusCode, response.Header, body)
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
