package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	monitoringhttp "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/http"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
)

type transportInventoryRepository struct{ calls atomic.Int32 }

func (repository *transportInventoryRepository) Candidates(context.Context, *ports.InventoryAnchor, int, int) ([]ports.InventoryCandidate, error) {
	repository.calls.Add(1)
	return nil, nil
}

func TestProductionInventoryRejectsRawPathAliasesWithoutRedirect(t *testing.T) {
	repository := &transportInventoryRepository{}
	public := monitoringhttp.NewHandlerWithInventory(nil, nil, nil, nil, application.NewListMonitors(repository))
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
	client := &http.Client{
		Timeout:       time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	t.Cleanup(client.CloseIdleConnections)
	origin := "http://" + listener.Addr().String()
	for _, rawPath := range []string{
		"//monitors?limit=20",
		"/x/../monitors?limit=20",
		"/./monitors?limit=20",
		"/monitors/?limit=20",
		"/%6donitors?limit=20",
		"/monitors%2f?limit=20",
	} {
		t.Run(rawPath, func(t *testing.T) {
			response, err := client.Get(origin + rawPath)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusBadRequest || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Location") != "" {
				t.Errorf("status/cache/location = %d/%q/%q, want400/no-store/no redirect", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Location"))
			}
			if got := repository.calls.Load(); got != 0 {
				t.Errorf("repository calls = %d, want0", got)
			}
		})
	}
	response, err := client.Get(origin + "/monitors?limit=20")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != `{"items":[],"nextCursor":null}` || repository.calls.Load() != 1 {
		t.Fatalf("canonical inventory = %d/%s/%v, calls%d", response.StatusCode, body, err, repository.calls.Load())
	}
}
