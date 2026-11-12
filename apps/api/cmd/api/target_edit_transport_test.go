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
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
)

type transportEditUseCase struct {
	monitor   domain.Monitor
	err       error
	calls     atomic.Int32
	gotID     domain.MonitorID
	gotTarget string
}

func (useCase *transportEditUseCase) Execute(_ context.Context, id domain.MonitorID, rawTarget string) (domain.Monitor, error) {
	useCase.calls.Add(1)
	useCase.gotID = id
	useCase.gotTarget = rawTarget
	return useCase.monitor, useCase.err
}

func buildEditedMonitor(t *testing.T, rawID, rawTarget string) domain.Monitor {
	t.Helper()
	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		t.Fatalf("ParseMonitorID() error = %v", err)
	}
	target, err := domain.NewTargetURL(rawTarget)
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	monitor, err := domain.NewMonitor(id, target, time.Date(2026, 11, 1, 12, 0, 0, 123456000, time.UTC))
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}
	return monitor
}

func startTargetEditTransportServer(t *testing.T, deletion *transportDeletionUseCase, edit *transportEditUseCase) string {
	t.Helper()
	public := monitoringhttp.NewHandlerWithTargetEdit(nil, nil, nil, nil, nil, nil, nil, deletion, nil, edit)
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
		case <-time.After(3 * time.Second):
			t.Error("production server did not stop")
		}
	})
	return "http://" + listener.Addr().String()
}

func patchTargetEdit(t *testing.T, origin, path, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPatch, origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	})
	return response
}

func TestProductionTargetEditTransportSemantics(t *testing.T) {
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd006"
	edit := &transportEditUseCase{monitor: buildEditedMonitor(t, id, "https://example.com/edited")}
	deletion := &transportDeletionUseCase{}
	origin := startTargetEditTransportServer(t, deletion, edit)

	head, err := http.Head(origin + "/monitors/" + id)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, head.Body)
	_ = head.Body.Close()
	if head.StatusCode != http.StatusMethodNotAllowed ||
		head.Header.Get("Allow") != "GET, PATCH, DELETE" ||
		head.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD status/allow/cache = %d/%q/%q", head.StatusCode, head.Header.Get("Allow"), head.Header.Get("Cache-Control"))
	}
	if edit.calls.Load() != 0 || deletion.calls.Load() != 0 {
		t.Fatalf("HEAD touched use cases: edit=%d delete=%d", edit.calls.Load(), deletion.calls.Load())
	}

	response := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"https://example.com/edited"}`)
	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK ||
		response.Header.Get("Cache-Control") != "no-store" ||
		!strings.Contains(string(payload), `"targetUrl":"https://example.com/edited"`) ||
		!strings.Contains(string(payload), `"createdAt":"2026-11-01T12:00:00.123456Z"`) {
		t.Fatalf("patch response = %d %v %q", response.StatusCode, response.Header, payload)
	}
	if edit.calls.Load() != 1 || edit.gotID.String() != id || edit.gotTarget != "https://example.com/edited" {
		t.Fatalf("edit forwarding = calls %d id %q target %q", edit.calls.Load(), edit.gotID.String(), edit.gotTarget)
	}

	edit.err = application.ErrMonitorNotFound
	notFound := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"https://example.com/edited"}`)
	notFoundBody, _ := io.ReadAll(notFound.Body)
	if notFound.StatusCode != http.StatusNotFound ||
		!strings.Contains(string(notFoundBody), "The monitor was not found.") {
		t.Fatalf("unknown monitor patch = %d %q", notFound.StatusCode, notFoundBody)
	}

	edit.err = domain.ErrInvalidTargetURL
	invalid := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"not-a-url"}`)
	invalidBody, _ := io.ReadAll(invalid.Body)
	if invalid.StatusCode != http.StatusUnprocessableEntity ||
		!strings.Contains(string(invalidBody), "violates the monitor contract") {
		t.Fatalf("invalid target patch = %d %q", invalid.StatusCode, invalidBody)
	}
	edit.err = nil

	overBudget := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"https://example.com/`+strings.Repeat("a", 16384)+`"}`)
	_, _ = io.Copy(io.Discard, overBudget.Body)
	_ = overBudget.Body.Close()
	if overBudget.StatusCode != http.StatusBadRequest {
		t.Fatalf("over-budget patch = %d, want 400", overBudget.StatusCode)
	}

	malformed := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"https://example.com/edited"`)
	malformedBody, _ := io.ReadAll(malformed.Body)
	if malformed.StatusCode != http.StatusBadRequest ||
		!strings.Contains(string(malformedBody), "malformed") {
		t.Fatalf("malformed patch = %d %q", malformed.StatusCode, malformedBody)
	}

	unknownKey := patchTargetEdit(t, origin, "/monitors/"+id, `{"targetUrl":"https://example.com/edited","extra":1}`)
	_, _ = io.Copy(io.Discard, unknownKey.Body)
	_ = unknownKey.Body.Close()
	if unknownKey.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unknown key patch = %d, want 422", unknownKey.StatusCode)
	}

	alias := patchTargetEdit(t, origin, "/x/../monitors/"+id, `{"targetUrl":"https://example.com/edited"}`)
	aliasBody, _ := io.ReadAll(alias.Body)
	if alias.StatusCode != http.StatusBadRequest ||
		alias.Header.Get("Location") != "" ||
		!strings.Contains(string(aliasBody), "Invalid monitor request.") {
		t.Fatalf("alias patch = %d %q %v", alias.StatusCode, aliasBody, alias.Header)
	}
	if edit.calls.Load() != 3 || deletion.calls.Load() != 0 {
		t.Fatalf("final use-case calls: edit=%d delete=%d", edit.calls.Load(), deletion.calls.Load())
	}
}

func TestProductionTargetEditRejectsRawPathAliasesWithoutRedirect(t *testing.T) {
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd007"
	edit := &transportEditUseCase{monitor: buildEditedMonitor(t, id, "https://example.com/edited")}
	deletion := &transportDeletionUseCase{}
	origin := startTargetEditTransportServer(t, deletion, edit)
	body := `{"targetUrl":"https://example.com/edited"}`
	for _, rawPath := range []string{
		"//monitors/" + id,
		"/x/../monitors/" + id,
		"/monitors/" + id + "/",
		"/%6donitors/" + id,
		"/monitors/" + id + "?edit=1",
	} {
		t.Run(rawPath, func(t *testing.T) {
			response := patchTargetEdit(t, origin, rawPath, body)
			if response.StatusCode != http.StatusBadRequest ||
				response.Header.Get("Cache-Control") != "no-store" ||
				response.Header.Get("Location") != "" {
				t.Errorf("status/cache/location = %d/%q/%q, want 400/no-store/no redirect", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Location"))
			}
			if edit.calls.Load() != 0 || deletion.calls.Load() != 0 {
				t.Errorf("use-case calls = edit %d delete %d, want 0", edit.calls.Load(), deletion.calls.Load())
			}
		})
	}
}
