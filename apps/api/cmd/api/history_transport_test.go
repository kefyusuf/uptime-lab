package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	monitoringhttp "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/http"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
)

type transportHistoryUseCase struct {
	entry     application.LatestCheckResult
	calls     atomic.Int32
	gotCursor string
	gotLimit  int
}

func (useCase *transportHistoryUseCase) Execute(_ context.Context, _ domain.MonitorID, input application.HistoryInput) (application.HistoryCandidates, error) {
	useCase.calls.Add(1)
	useCase.gotCursor = input.Cursor
	useCase.gotLimit = input.Limit
	return application.HistoryCandidates{Items: []application.LatestCheckResult{useCase.entry}, Limit: input.Limit}, nil
}

func buildHistoryEntry(checkID string, completedAt string) (application.LatestCheckResult, error) {
	parsed, err := domain.ParseCheckID(checkID)
	if err != nil {
		return application.LatestCheckResult{}, err
	}
	instant, err := time.Parse("2006-01-02T15:04:05.000000Z", completedAt)
	if err != nil {
		return application.LatestCheckResult{}, err
	}
	status := 204
	duration := int64(123)
	record := ports.LatestCheckResultRecord{
		CheckID:     parsed,
		ResultKind:  domain.CheckResultHTTPResponse,
		HTTPStatus:  &status,
		DurationMS:  &duration,
		CompletedAt: instant,
	}
	monitorID, err := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafd004")
	if err != nil {
		return application.LatestCheckResult{}, err
	}
	return application.NewGetLatestCheckResult(&latestRecordRepositoryStub{record: record}).Execute(
		context.Background(), monitorID,
	)
}

type latestRecordRepositoryStub struct{ record ports.LatestCheckResultRecord }

func (stub *latestRecordRepositoryStub) LatestTerminalByMonitorID(context.Context, domain.MonitorID) (ports.LatestCheckResultRecord, error) {
	return stub.record, nil
}

func startHistoryTransportServer(t *testing.T, history *transportHistoryUseCase) string {
	t.Helper()
	public := monitoringhttp.NewHandlerWithHistory(nil, nil, nil, nil, nil, nil, nil, nil, history)
	server := httpserver.New("", nil, newProductRouter(public, http.NotFoundHandler()))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("production server did not stop")
		}
	})
	return "http://" + listener.Addr().String()
}

func TestProductionHistoryRejectsRawPathAliasesWithoutRedirect(t *testing.T) {
	entry, err := buildHistoryEntry("7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675", "2026-11-01T12:00:00.123456Z")
	if err != nil {
		t.Fatal(err)
	}
	history := &transportHistoryUseCase{entry: entry}
	origin := startHistoryTransportServer(t, history)
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd004"
	for _, rawPath := range []string{
		"//monitors/" + id + "/history",
		"/x/../monitors/" + id + "/history",
		"/monitors/" + id + "/history/",
		"/%6donitors/" + id + "/history",
		"/monitors/" + id + "/history?",
		"/monitors/" + id + "/history?limit=abc",
	} {
		t.Run(rawPath, func(t *testing.T) {
			response, err := http.Get(origin + rawPath)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusBadRequest ||
				response.Header.Get("Cache-Control") != "no-store" ||
				response.Header.Get("Location") != "" {
				t.Errorf("status/cache/location = %d/%q/%q, want 400/no-store/no redirect", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Location"))
			}
			if history.calls.Load() != 0 {
				t.Errorf("history calls = %d, want 0", history.calls.Load())
			}
		})
	}
}

func TestProductionHistoryTransportSemantics(t *testing.T) {
	entry, err := buildHistoryEntry("7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675", "2026-11-01T12:00:00.123456Z")
	if err != nil {
		t.Fatal(err)
	}
	history := &transportHistoryUseCase{entry: entry}
	origin := startHistoryTransportServer(t, history)
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd005"

	head, err := http.Head(origin + "/monitors/" + id + "/history")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, head.Body)
	_ = head.Body.Close()
	if head.StatusCode != http.StatusMethodNotAllowed ||
		head.Header.Get("Allow") != "GET" ||
		head.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD status/allow/cache = %d/%q/%q", head.StatusCode, head.Header.Get("Allow"), head.Header.Get("Cache-Control"))
	}

	overGrammar, err := http.Get(origin + "/monitors/" + id + "/history?cursor=" + strings.Repeat("A", 128))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, overGrammar.Body)
	_ = overGrammar.Body.Close()
	if overGrammar.StatusCode != http.StatusBadRequest || history.calls.Load() != 0 {
		t.Fatalf("over-grammar query = %d calls %d, want 400/0", overGrammar.StatusCode, history.calls.Load())
	}

	response, err := http.Get(origin + "/monitors/" + id + "/history?limit=7")
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK ||
		response.Header.Get("Cache-Control") != "no-store" ||
		!strings.Contains(string(payload), `"checkId":"7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"`) ||
		!strings.Contains(string(payload), `"nextCursor":null`) {
		t.Fatalf("history response = %d %q", response.StatusCode, payload)
	}
	if history.calls.Load() != 1 || history.gotLimit != 7 || history.gotCursor != "" {
		t.Fatalf("history forwarding = calls %d limit %d cursor %q", history.calls.Load(), history.gotLimit, history.gotCursor)
	}
}
