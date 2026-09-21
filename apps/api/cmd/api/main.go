package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/config"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/database"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/observability"
)

func main() {
	os.Exit(run())
}

func run() int {
	bootstrapLogger := observability.NewLogger(os.Stderr, slog.LevelInfo, "runtime")

	runtimeConfig, err := config.Load()
	if err != nil {
		bootstrapLogger.Error(
			"invalid runtime configuration",
			"event", "startup_failed",
			"stage", "config",
		)
		return 1
	}

	logger := observability.NewLogger(os.Stderr, runtimeConfig.LogLevel, "runtime")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error(
			"database pool configuration failed",
			"event", "startup_failed",
			"stage", "database",
		)
		return 1
	}
	defer pool.Close()

	server := httpserver.New(runtimeConfig.HTTPAddr, pool)

	logger.Info(
		"operational HTTP server starting",
		"event", "startup",
		"http_addr", runtimeConfig.HTTPAddr,
	)

	if err := server.Run(ctx); err != nil {
		logger.Error(
			"operational HTTP server failed",
			"event", "server_failed",
		)
		return 1
	}

	logger.Info(
		"operational HTTP server stopped",
		"event", "shutdown_complete",
	)
	return 0
}
