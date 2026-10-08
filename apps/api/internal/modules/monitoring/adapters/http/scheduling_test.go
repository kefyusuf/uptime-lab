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
)

const schedulingTestPath = "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd001/scheduling"

type schedulingStoreFake struct {
	state domain.SchedulingState
	calls int
	err   error
}

func (f *schedulingStoreFake) GetScheduling(context.Context, domain.MonitorID) (domain.SchedulingState, error) {
	f.calls++
	return f.state, f.err
}
func (f *schedulingStoreFake) SetScheduling(_ context.Context, _ domain.MonitorID, state domain.SchedulingState) (domain.SchedulingState, error) {
	f.calls++
	f.state = state
	return state, f.err
}
func schedulingTestHandler(f *schedulingStoreFake) *Handler {
	return NewHandlerWithScheduling(nil, nil, nil, nil, nil, application.NewGetMonitorScheduling(f), application.NewSetMonitorScheduling(f))
}

func TestSchedulingHTTPReadAndDesiredStateWrite(t *testing.T) {
	f := &schedulingStoreFake{state: domain.SchedulingActive}
	handler := schedulingTestHandler(f)
	for _, tc := range []struct{ method, body, state string }{{http.MethodGet, "", "active"}, {"PUT", `{"state":"paused"}`, "paused"}, {"PUT", `{"state":"paused"}`, "paused"}, {"PUT", `{"state":"active"}`, "active"}} {
		req := httptest.NewRequest(tc.method, schedulingTestPath, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		var got map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
			t.Fatal(err, recorder.Body.String())
		}
		if recorder.Code != 200 || len(got) != 1 || got["state"] != tc.state || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(recorder.Code, got, recorder.Header())
		}
	}
	if f.calls != 4 {
		t.Fatal(f.calls)
	}
}

func TestSchedulingStrictJSON(t *testing.T) {
	for _, body := range []string{`{"state":"paused","state":"active"}`, `{"state":"paused","st\u0061te":"paused"}`, `{"State":"paused"}`, `{"state":"paused","other":1}`, `{"state":"paused"} {}`, `{"state":"paused"} true`, `null`, `[]`, `{}`, `{"state":null}`, `{"state":true}`, `{"state":"disabled"}`, `{"state":"Paused"}`, ``, `{"state":"paused",}`} {
		t.Run(body, func(t *testing.T) {
			f := &schedulingStoreFake{state: domain.SchedulingActive}
			req := httptest.NewRequest("PUT", schedulingTestPath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			schedulingTestHandler(f).ServeHTTP(recorder, req)
			if recorder.Code != 400 || f.calls != 0 || recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("invalid JSON reached storage: status=%d calls=%d", recorder.Code, f.calls)
			}
		})
	}
	f := &schedulingStoreFake{state: domain.SchedulingActive}
	req := httptest.NewRequest("PUT", schedulingTestPath, strings.NewReader(`{"st\u0061te":"paused"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	schedulingTestHandler(f).ServeHTTP(recorder, req)
	if recorder.Code != 200 || f.calls != 1 {
		t.Fatal(recorder.Code, f.calls)
	}
}

func TestSchedulingByteLimit(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, size := range []int{1024, 1025} {
			body := `{"state":"paused"}`
			body += strings.Repeat(" ", size-len(body))
			f := &schedulingStoreFake{state: domain.SchedulingActive}
			req := httptest.NewRequest("PUT", schedulingTestPath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if streamed {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			recorder := httptest.NewRecorder()
			schedulingTestHandler(f).ServeHTTP(recorder, req)
			wantStatus, wantCalls := 200, 1
			if size == 1025 {
				wantStatus, wantCalls = 413, 0
			}
			if recorder.Code != wantStatus || f.calls != wantCalls {
				t.Fatalf("size=%d streamed=%v status=%d calls=%d", size, streamed, recorder.Code, f.calls)
			}
		}
	}
}

func TestSchedulingHTTPValidationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, method, target, media, encoding, body string
		length                                      int64
		chunked                                     bool
		status                                      int
	}{
		{name: "query before method", method: "HEAD", target: schedulingTestPath + "?", status: 400},
		{name: "method before id", method: "POST", target: "/monitors/invalid/scheduling", status: 405},
		{name: "id before length", method: "PUT", target: "/monitors/invalid/scheduling", length: 1025, status: 400},
		{name: "length before media", method: "PUT", target: schedulingTestPath, length: 1025, status: 413},
		{name: "missing media", method: "PUT", target: schedulingTestPath, body: `{"state":"paused"}`, status: 415},
		{name: "foreign charset", method: "PUT", target: schedulingTestPath, media: "application/json; charset=iso-8859-1", status: 415},
		{name: "extra parameter", method: "PUT", target: schedulingTestPath, media: "application/json; other=1", status: 415},
		{name: "encoding", method: "PUT", target: schedulingTestPath, media: "application/json", encoding: "gzip", status: 415},
		{name: "get length", method: "GET", target: schedulingTestPath, length: 1, body: "x", status: 400},
		{name: "get chunked", method: "GET", target: schedulingTestPath, chunked: true, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &schedulingStoreFake{state: domain.SchedulingActive}
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.media)
			req.Header.Set("Content-Encoding", tc.encoding)
			if tc.length != 0 {
				req.ContentLength = tc.length
			}
			if tc.chunked {
				req.TransferEncoding = []string{"chunked"}
			}
			w := httptest.NewRecorder()
			schedulingTestHandler(f).ServeHTTP(w, req)
			if w.Code != tc.status || f.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d calls=%d headers=%v", w.Code, f.calls, w.Header())
			}
		})
	}
	for _, media := range []string{"application/json", "application/json; charset=UTF-8"} {
		f := &schedulingStoreFake{state: domain.SchedulingActive}
		req := httptest.NewRequest("PUT", schedulingTestPath, strings.NewReader(`{"state":"paused"}`))
		req.Header.Set("Content-Type", media)
		w := httptest.NewRecorder()
		schedulingTestHandler(f).ServeHTTP(w, req)
		if w.Code != 200 || f.calls != 1 {
			t.Fatal(w.Code, f.calls)
		}
	}
}

func TestSchedulingMethodsAndNoStore(t *testing.T) {
	for _, method := range []string{"HEAD", "OPTIONS", "POST", "DELETE", "PATCH"} {
		f := &schedulingStoreFake{state: domain.SchedulingActive}
		w := httptest.NewRecorder()
		schedulingTestHandler(f).ServeHTTP(w, httptest.NewRequest(method, schedulingTestPath, nil))
		if w.Code != 405 || w.Header().Get("Allow") != "GET, PUT" || w.Header().Get("Cache-Control") != "no-store" || f.calls != 0 {
			t.Fatal(method, w.Code, w.Header(), f.calls)
		}
	}
}

func TestSchedulingAliasesNeverRedirect(t *testing.T) {
	for _, target := range []string{"/x/.." + schedulingTestPath, "/" + schedulingTestPath, schedulingTestPath + "/", strings.Replace(schedulingTestPath, "monitors", "%6donitors", 1), schedulingTestPath + "?a=1", schedulingTestPath + "?", strings.Replace(schedulingTestPath, "/scheduling", "%2fscheduling", 1), strings.Replace(schedulingTestPath, "/scheduling", `\scheduling`, 1)} {
		f := &schedulingStoreFake{state: domain.SchedulingActive}
		w := httptest.NewRecorder()
		schedulingTestHandler(f).ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		if w.Code != 400 || w.Header().Get("Location") != "" || w.Header().Get("Cache-Control") != "no-store" || f.calls != 0 {
			t.Fatal(target, w.Code, w.Header(), f.calls)
		}
	}
}

func TestSchedulingErrorsAreSanitized(t *testing.T) {
	for _, method := range []string{"GET", "PUT"} {
		for _, tc := range []struct {
			err    error
			state  domain.SchedulingState
			status int
		}{
			{ports.ErrMonitorNotFound, domain.SchedulingActive, 404},
			{errors.New("secret SQL host and target"), domain.SchedulingActive, 500},
			{context.Canceled, domain.SchedulingActive, 500},
			{nil, domain.SchedulingState("invalid"), 500},
		} {
			if method == "PUT" && tc.err == nil {
				continue
			} // The fake returns the desired state for writes.
			f := &schedulingStoreFake{state: tc.state, err: tc.err}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, schedulingTestPath, strings.NewReader(`{"state":"paused"}`))
			req.Header.Set("Content-Type", "application/json")
			if method == "GET" {
				req = httptest.NewRequest(method, schedulingTestPath, nil)
			}
			schedulingTestHandler(f).ServeHTTP(w, req)
			if w.Code != tc.status || f.calls != 1 || w.Header().Get("Content-Type") != "application/problem+json" || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "secret") {
				t.Fatal(method, w.Code, w.Body.String(), f.calls)
			}
		}
	}
}

type brokenSchedulingBody struct{}

func (brokenSchedulingBody) Read([]byte) (int, error) {
	return 0, errors.New("secret body read failure")
}
func (brokenSchedulingBody) Close() error { return nil }
func TestSchedulingBodyReadFailure(t *testing.T) {
	f := &schedulingStoreFake{state: domain.SchedulingActive}
	req := httptest.NewRequest("PUT", schedulingTestPath, nil)
	req.Body = brokenSchedulingBody{}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	schedulingTestHandler(f).ServeHTTP(w, req)
	if w.Code != 400 || f.calls != 0 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Code, w.Body.String(), f.calls)
	}
}
