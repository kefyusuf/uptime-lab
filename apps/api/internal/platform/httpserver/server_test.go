package httpserver

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeReadinessChecker struct {
	calls atomic.Int32
	check func(context.Context) error
}

func (readiness *fakeReadinessChecker) Check(ctx context.Context) error {
	readiness.calls.Add(1)
	if readiness.check != nil {
		return readiness.check(ctx)
	}
	return nil
}

func TestLivezReturnsOKWithoutReadinessCheck(t *testing.T) {
	readiness := &fakeReadinessChecker{}
	server := New(":8080", readiness, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "ok\n" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "ok\n")
	}
	if got := readiness.calls.Load(); got != 0 {
		t.Fatalf("readiness check calls = %d, want 0", got)
	}
}

func TestReadyzReturnsOKWhenReadinessCheckSucceeds(t *testing.T) {
	readiness := &fakeReadinessChecker{}
	server := New(":8080", readiness, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "ok\n" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "ok\n")
	}
	if got := readiness.calls.Load(); got != 1 {
		t.Fatalf("readiness check calls = %d, want 1", got)
	}
}

func TestReadyzReturnsSanitizedServiceUnavailableOnReadinessFailure(t *testing.T) {
	const secret = "password=do-not-leak"
	readiness := &fakeReadinessChecker{
		check: func(context.Context) error {
			return errors.New("internal readiness failure " + secret)
		},
	}
	server := New(":8080", readiness, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if recorder.Body.String() != "unavailable\n" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "unavailable\n")
	}
	body := recorder.Body.String()
	if strings.Contains(body, secret) {
		t.Fatalf("readiness response leaked internal detail: %q", body)
	}
}

func TestReadyzBoundsReadinessCheckByTimeout(t *testing.T) {
	readiness := &fakeReadinessChecker{
		check: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	server := New(":8080", readiness, nil)
	server.readinessTimeout = 20 * time.Millisecond

	started := time.Now()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)
	elapsed := time.Since(started)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("readiness check elapsed = %s, want bounded timeout", elapsed)
	}
	if got := readiness.calls.Load(); got != 1 {
		t.Fatalf("readiness check calls = %d, want 1", got)
	}
}

func TestReadyzReturnsServiceUnavailableWithoutReadinessChecker(t *testing.T) {
	server := New(":8080", nil, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if recorder.Body.String() != "unavailable\n" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "unavailable\n")
	}
}

func TestProductHandlerReceivesNonOperationalPaths(t *testing.T) {
	var calls atomic.Int32
	product := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/products/example" {
			t.Fatalf("path = %q, want /products/example", request.URL.Path)
		}
		writer.WriteHeader(http.StatusAccepted)
	})
	server := New(":8080", &fakeReadinessChecker{}, product)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/products/example", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("product handler calls = %d, want 1", got)
	}
}

func TestOperationalRoutesTakePrecedenceOverProductHandler(t *testing.T) {
	var calls atomic.Int32
	product := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusTeapot)
	})
	server := New(":8080", &fakeReadinessChecker{}, product)

	for _, path := range []string{"/livez", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			server.httpServer.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
		})
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("product handler calls = %d, want 0 for operational routes", got)
	}
}

func TestNilProductHandlerReturnsNotFoundForNonOperationalPath(t *testing.T) {
	server := New(":8080", &fakeReadinessChecker{}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/monitors", nil)
	server.httpServer.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestOperationalEndpointsRejectNonGETMethods(t *testing.T) {
	server := New(":8080", &fakeReadinessChecker{}, nil)

	for _, path := range []string{"/livez", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, path, nil)
			server.httpServer.Handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
			}
			if got := recorder.Header().Get("Allow"); got != http.MethodGet {
				t.Fatalf("Allow = %q, want GET", got)
			}
		})
	}
}

func TestServerUsesExplicitHTTPHardeningTimeouts(t *testing.T) {
	server := New(":8080", &fakeReadinessChecker{}, nil)

	if server.httpServer.ReadHeaderTimeout <= 0 {
		t.Fatal("ReadHeaderTimeout must be explicit and positive")
	}
	if server.httpServer.ReadTimeout <= 0 {
		t.Fatal("ReadTimeout must be explicit and positive")
	}
	if server.httpServer.WriteTimeout <= 0 {
		t.Fatal("WriteTimeout must be explicit and positive")
	}
	if server.httpServer.IdleTimeout <= 0 {
		t.Fatal("IdleTimeout must be explicit and positive")
	}
	if server.httpServer.MaxHeaderBytes <= 0 {
		t.Fatal("MaxHeaderBytes must be explicit and positive")
	}
	if server.readinessTimeout <= 0 {
		t.Fatal("readinessTimeout must be explicit and positive")
	}
	if server.shutdownTimeout <= 0 {
		t.Fatal("shutdownTimeout must be explicit and positive")
	}
}

func TestServeShutsDownGracefullyWhenContextIsCancelled(t *testing.T) {
	server := New("127.0.0.1:0", &fakeReadinessChecker{}, nil)
	server.shutdownTimeout = time.Second

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(ctx, listener)
	}()

	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/livez")
	if err != nil {
		cancel()
		t.Fatalf("GET /livez error = %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Serve() error after cancellation = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not stop after context cancellation")
	}

	// Repeated and already-cancelled shutdown paths must remain safe.
	_ = server.Shutdown(context.Background())
	cancelled, cancelShutdown := context.WithCancel(context.Background())
	cancelShutdown()
	_ = server.Shutdown(cancelled)
}
