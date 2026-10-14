package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring"
	checkerhttp "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/checkerhttp"
	monitoringhttp "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/http"
	monitoringpostgres "github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters/postgres"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/config"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/database"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/httpserver"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/platform/observability"
	"github.com/kefyusuf/uptime-lab/apps/api/migrations"
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

	product, readiness, migrationDB, err := composeMonitoring(pool)
	if err != nil {
		logger.Error(
			"monitoring runtime composition failed",
			"event", "startup_failed",
			"stage", "monitoring",
		)
		return 1
	}
	defer migrationDB.Close()

	server := httpserver.New(runtimeConfig.HTTPAddr, readiness, product)

	logger.Info(
		"HTTP server starting",
		"event", "startup",
		"http_addr", runtimeConfig.HTTPAddr,
	)

	if err := server.Run(ctx); err != nil {
		logger.Error(
			"HTTP server failed",
			"event", "server_failed",
		)
		return 1
	}

	logger.Info(
		"HTTP server stopped",
		"event", "shutdown_complete",
	)
	return 0
}

func composeMonitoring(pool *pgxpool.Pool) (http.Handler, *migrations.CompatibilityChecker, *sql.DB, error) {
	migrationDB := stdlib.OpenDBFromPool(pool)
	readiness, err := migrations.NewCompatibilityChecker(migrationDB)
	if err != nil {
		_ = migrationDB.Close()
		return nil, nil, nil, fmt.Errorf("construct schema compatibility checker: %w", err)
	}

	repository := monitoringpostgres.NewRepository(pool)
	publicModule := monitoring.NewModuleWithScheduling(repository, repository, repository, newMonitorID, productionClock, repository)
	executionModule := monitoring.NewExecutionModule(repository, newCheckID, productionClock)

	publicHandler := monitoringhttp.NewHandlerWithScheduling(
		publicModule.RegisterMonitor,
		publicModule.GetMonitor,
		publicModule.GetLatestCheckResult,
		publicModule.GetMonitorAvailability,
		publicModule.ListMonitors,
		publicModule.GetMonitorScheduling,
		publicModule.SetMonitorScheduling,
	)
	internalHandler := checkerhttp.NewHandler(
		executionModule.ClaimDueCheck,
		executionModule.SubmitCheckResult,
	)
	product := newProductRouter(publicHandler, internalHandler)

	return product, readiness, migrationDB, nil
}

func newMonitorID() (domain.MonitorID, error) {
	return domain.NewMonitorID(uuid.NewV7())
}

func newCheckID() (domain.CheckID, error) {
	return domain.NewCheckID(uuid.NewV7())
}

func newProductRouter(publicHandler, internalHandler http.Handler) http.Handler {
	if publicHandler == nil {
		publicHandler = http.NotFoundHandler()
	}
	if internalHandler == nil {
		internalHandler = http.NotFoundHandler()
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/internal/checks/") {
			internalHandler.ServeHTTP(writer, request)
			return
		}
		publicHandler.ServeHTTP(writer, request)
	})
}

func productionClock() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
