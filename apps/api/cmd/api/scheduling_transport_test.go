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

type transportSchedulingRepository struct{ calls atomic.Int32 }

func (r *transportSchedulingRepository) GetScheduling(context.Context, domain.MonitorID) (domain.SchedulingState, error) {
	r.calls.Add(1)
	return domain.SchedulingActive, nil
}
func (r *transportSchedulingRepository) SetScheduling(_ context.Context, _ domain.MonitorID, state domain.SchedulingState) (domain.SchedulingState, error) {
	r.calls.Add(1)
	return state, nil
}

func TestSchedulingProductionTransport(t *testing.T) {
	r := &transportSchedulingRepository{}
	public := monitoringhttp.NewHandlerWithScheduling(nil, nil, nil, nil, nil, application.NewGetMonitorScheduling(r), application.NewSetMonitorScheduling(r))
	server := httpserver.New("", nil, newProductRouter(public, http.NotFoundHandler()))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	origin := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t.Cleanup(client.CloseIdleConnections)
	canonical := "/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafd001/scheduling"
	for _, target := range []string{"/" + canonical, "/x/.." + canonical, canonical + "/", strings.Replace(canonical, "monitors", "%6donitors", 1), canonical + "?", strings.Replace(canonical, "/scheduling", "%2fscheduling", 1)} {
		response, err := client.Get(origin + target)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Location") != "" || r.calls.Load() != 0 {
			t.Fatal(target, response.StatusCode, response.Header, r.calls.Load())
		}
	}
	for _, method := range []string{"HEAD", "OPTIONS"} {
		req, _ := http.NewRequest(method, origin+canonical, nil)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 405 || response.Header.Get("Allow") != "GET, PUT" || response.Header.Get("Cache-Control") != "no-store" || r.calls.Load() != 0 || (method == "HEAD" && len(body) != 0) {
			t.Fatal(method, response.StatusCode, string(body), err, response.Header)
		}
	}
}
