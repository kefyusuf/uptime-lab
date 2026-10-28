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
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
)

type transportDeletionUseCase struct{ calls atomic.Int32 }

func (useCase *transportDeletionUseCase) Execute(_ context.Context, _ domain.MonitorID) error {
	useCase.calls.Add(1)
	return nil
}

func startDeletionTransportServer(t *testing.T, deletion *transportDeletionUseCase) string {
	t.Helper()
	public := monitoringhttp.NewHandlerWithDeletion(nil, nil, nil, nil, nil, nil, nil, deletion)
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

func deleteThroughClient(t *testing.T, origin, rawPath string, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodDelete, origin+rawPath, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Timeout:       time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestProductionDeletionRejectsRawPathAliasesWithoutRedirect(t *testing.T) {
	deletion := &transportDeletionUseCase{}
	origin := startDeletionTransportServer(t, deletion)
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd002"
	for _, rawPath := range []string{
		"//monitors/" + id,
		"/x/../monitors/" + id,
		"/monitors/" + id + "/",
		"/%6donitors/" + id,
		"/monitors/" + id + "?",
		"/monitors/" + id + "?x=1",
	} {
		t.Run(rawPath, func(t *testing.T) {
			response := deleteThroughClient(t, origin, rawPath, "")
			payload, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusBadRequest ||
				response.Header.Get("Cache-Control") != "no-store" ||
				response.Header.Get("Location") != "" {
				t.Errorf("status/cache/location = %d/%q/%q, want 400/no-store/no redirect", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Location"))
			}
			if deletion.calls.Load() != 0 {
				t.Errorf("deletion calls = %d, want 0", deletion.calls.Load())
			}
			_ = payload
		})
	}
}

func TestProductionDeletionTransportSemantics(t *testing.T) {
	deletion := &transportDeletionUseCase{}
	origin := startDeletionTransportServer(t, deletion)
	const id = "018f22d3-1d6a-7cc0-a37b-46fc3fafd003"

	head, err := http.Head(origin + "/monitors/" + id)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, head.Body)
	_ = head.Body.Close()
	if head.StatusCode != http.StatusMethodNotAllowed ||
		head.Header.Get("Allow") != "GET, DELETE" ||
		head.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("HEAD status/allow/cache = %d/%q/%q", head.StatusCode, head.Header.Get("Allow"), head.Header.Get("Cache-Control"))
	}
	if deletion.calls.Load() != 0 {
		t.Fatalf("HEAD deletion calls = %d, want 0", deletion.calls.Load())
	}

	bodyResponse := deleteThroughClient(t, origin, "/monitors/"+id, "payload")
	if bodyResponse.StatusCode != http.StatusBadRequest || deletion.calls.Load() != 0 {
		t.Fatalf("body delete status = %d calls = %d, want 400/0", bodyResponse.StatusCode, deletion.calls.Load())
	}

	success := deleteThroughClient(t, origin, "/monitors/"+id, "")
	if success.StatusCode != http.StatusNoContent ||
		success.Header.Get("Cache-Control") != "no-store" ||
		success.Header.Get("Location") != "" {
		t.Fatalf("success status/cache/location = %d/%q/%q", success.StatusCode, success.Header.Get("Cache-Control"), success.Header.Get("Location"))
	}
	payload, _ := io.ReadAll(success.Body)
	if len(payload) != 0 {
		t.Fatalf("204 body = %q, want empty", payload)
	}
	if deletion.calls.Load() != 1 {
		t.Fatalf("deletion calls = %d, want 1", deletion.calls.Load())
	}
}
