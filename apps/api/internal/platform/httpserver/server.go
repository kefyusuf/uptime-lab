package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	defaultReadinessTimeout = 2 * time.Second
	defaultShutdownTimeout  = 10 * time.Second
	readHeaderTimeout       = 5 * time.Second
	readTimeout             = 10 * time.Second
	writeTimeout            = 10 * time.Second
	idleTimeout             = 60 * time.Second
	maxHeaderBytes          = 1 << 20
)

// ReadinessChecker is the generic capability required by /readyz.
type ReadinessChecker interface {
	Check(context.Context) error
}

// Server exposes platform-owned operational health and delegates product traffic.
type Server struct {
	httpServer       *http.Server
	readiness        ReadinessChecker
	readinessTimeout time.Duration
	shutdownTimeout  time.Duration
}

// New constructs the HTTP server with platform operational routes taking precedence over product traffic.
func New(addr string, readiness ReadinessChecker, product http.Handler) *Server {
	server := &Server{
		readiness:        readiness,
		readinessTimeout: defaultReadinessTimeout,
		shutdownTimeout:  defaultShutdownTimeout,
	}

	if product == nil {
		product = http.NotFoundHandler()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", server.handleLivez)
	mux.HandleFunc("/readyz", server.handleReadyz)
	mux.Handle("/", product)

	server.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	return server
}

// Run listens on the configured address and serves until the context is cancelled.
func (server *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen for operational HTTP: %w", err)
	}
	return server.Serve(ctx, listener)
}

// Serve runs on an existing listener and performs bounded graceful shutdown.
func (server *Server) Serve(ctx context.Context, listener net.Listener) error {
	result := make(chan error, 1)
	go func() {
		result <- server.httpServer.Serve(listener)
	}()

	select {
	case err := <-result:
		return normalizeServeError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), server.shutdownTimeout)
		shutdownErr := server.httpServer.Shutdown(shutdownCtx)
		cancel()

		if shutdownErr != nil {
			_ = server.httpServer.Close()
		}

		serveErr := <-result
		if normalized := normalizeServeError(serveErr); normalized != nil {
			return normalized
		}
		if shutdownErr != nil {
			return fmt.Errorf("shutdown operational HTTP: %w", shutdownErr)
		}
		return nil
	}
}

// Shutdown permits explicit, repeatable server shutdown by callers and tests.
func (server *Server) Shutdown(ctx context.Context) error {
	return server.httpServer.Shutdown(ctx)
}

func (server *Server) handleLivez(writer http.ResponseWriter, request *http.Request) {
	if !allowGET(writer, request) {
		return
	}
	writePlain(writer, http.StatusOK, "ok\n")
}

func (server *Server) handleReadyz(writer http.ResponseWriter, request *http.Request) {
	if !allowGET(writer, request) {
		return
	}

	if server.readiness == nil {
		writePlain(writer, http.StatusServiceUnavailable, "unavailable\n")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), server.readinessTimeout)
	defer cancel()

	if err := server.readiness.Check(ctx); err != nil {
		writePlain(writer, http.StatusServiceUnavailable, "unavailable\n")
		return
	}

	writePlain(writer, http.StatusOK, "ok\n")
}

func allowGET(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet {
		return true
	}

	writer.Header().Set("Allow", http.MethodGet)
	writePlain(writer, http.StatusMethodNotAllowed, "method not allowed\n")
	return false
}

func writePlain(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = io.WriteString(writer, body)
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve operational HTTP: %w", err)
}
