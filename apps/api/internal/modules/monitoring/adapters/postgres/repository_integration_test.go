//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func TestRepositoryCreateThenByIDRoundTrip(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	want := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
		"https://example.com/health?region=eu",
		time.Date(2026, time.September, 21, 1, 15, 0, 123456000, time.UTC),
	)

	if err := repository.Create(context.Background(), want); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repository.ByID(context.Background(), want.ID())
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}

	if got.ID() != want.ID() {
		t.Fatalf("ByID() ID = %s, want %s", got.ID(), want.ID())
	}
	if got.TargetURL() != want.TargetURL() {
		t.Fatalf("ByID() target = %q, want %q", got.TargetURL(), want.TargetURL())
	}
	if !got.CreatedAt().Equal(want.CreatedAt()) {
		t.Fatalf("ByID() created_at = %s, want %s", got.CreatedAt(), want.CreatedAt())
	}
}

func TestRepositoryAllowsDuplicateTargetURLs(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	target := "https://example.com/status"
	first := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2",
		target,
		time.Date(2026, time.September, 21, 1, 20, 0, 0, time.UTC),
	)
	second := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3",
		target,
		time.Date(2026, time.September, 21, 1, 21, 0, 0, time.UTC),
	)

	if err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if err := repository.Create(context.Background(), second); err != nil {
		t.Fatalf("second Create() error = %v", err)
	}

	var count int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.monitors WHERE target_url = $1",
		target,
	).Scan(&count); err != nil {
		t.Fatalf("count duplicate targets: %v", err)
	}
	if count != 2 {
		t.Fatalf("duplicate target row count = %d, want 2", count)
	}
}

func TestRepositoryByIDMapsMissingRowToPortSentinel(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb4")

	_, err := repository.ByID(context.Background(), id)
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("ByID() error = %v, want ErrMonitorNotFound", err)
	}
	if err.Error() != ports.ErrMonitorNotFound.Error() {
		t.Fatalf("ByID() error = %q, want stable %q", err, ports.ErrMonitorNotFound)
	}
}

func TestRepositoryDatabaseFailureMapsToStableApplicationPersistenceError(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, "ALTER TABLE monitoring.monitors RENAME TO monitors_unavailable_test"); err != nil {
		t.Fatalf("rename monitoring table: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(
			context.Background(),
			"ALTER TABLE monitoring.monitors_unavailable_test RENAME TO monitors",
		); err != nil {
			t.Errorf("restore monitoring table: %v", err)
		}
	})

	repository := NewRepository(pool)
	useCase := application.NewGetMonitor(repository)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb5")

	_, err := useCase.Execute(ctx, id)
	if !errors.Is(err, application.ErrPersistence) {
		t.Fatalf("GetMonitor error = %v, want ErrPersistence", err)
	}
	if err.Error() != application.ErrPersistence.Error() {
		t.Fatalf("GetMonitor error = %q, want stable %q", err, application.ErrPersistence)
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "pgx") ||
		strings.Contains(lower, "postgres") ||
		strings.Contains(lower, "monitoring.monitors") {
		t.Fatalf("infrastructure detail leaked into application error: %q", err)
	}
}

func TestRepositoryPreservesCreatedAtInstantWithUTCSemantics(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	location := time.FixedZone("UTC+3", 3*60*60)
	input := time.Date(2026, time.September, 21, 4, 30, 15, 987654000, location)
	want := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb6",
		"https://example.com/utc",
		input,
	)

	if err := repository.Create(context.Background(), want); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := repository.ByID(context.Background(), want.ID())
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}

	if !got.CreatedAt().Equal(input) {
		t.Fatalf("ByID() created_at = %s, want instant %s", got.CreatedAt(), input)
	}
	if got.CreatedAt().Location() != time.UTC {
		t.Fatalf("ByID() created_at location = %v, want UTC", got.CreatedAt().Location())
	}
}

func TestRepositoryUUIDRoundTripPreservesExactIdentity(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	want := mustMonitor(
		t,
		"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb7",
		"https://example.com/uuid",
		time.Date(2026, time.September, 21, 1, 35, 0, 0, time.UTC),
	)

	if err := repository.Create(context.Background(), want); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var storedID string
	if err := pool.QueryRow(
		context.Background(),
		"SELECT id::text FROM monitoring.monitors WHERE id = $1::uuid",
		want.ID().String(),
	).Scan(&storedID); err != nil {
		t.Fatalf("query stored UUID: %v", err)
	}
	if storedID != want.ID().String() {
		t.Fatalf("stored UUID = %q, want %q", storedID, want.ID())
	}

	got, err := repository.ByID(context.Background(), want.ID())
	if err != nil {
		t.Fatalf("ByID() error = %v", err)
	}
	if got.ID().String() != want.ID().String() {
		t.Fatalf("round-trip UUID = %q, want %q", got.ID(), want.ID())
	}
}

func TestRepositoryNeverGeneratesIdentity(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	id := "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb8"
	first := mustMonitor(
		t,
		id,
		"https://example.com/identity/one",
		time.Date(2026, time.September, 21, 1, 40, 0, 0, time.UTC),
	)
	second := mustMonitor(
		t,
		id,
		"https://example.com/identity/two",
		time.Date(2026, time.September, 21, 1, 41, 0, 0, time.UTC),
	)

	if err := repository.Create(context.Background(), first); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if err := repository.Create(context.Background(), second); err == nil {
		t.Fatal("second Create() error = nil, want duplicate primary-key failure")
	}

	var count int
	if err := pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM monitoring.monitors WHERE id = $1::uuid",
		id,
	).Scan(&count); err != nil {
		t.Fatalf("count provided identity: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for provided identity = %d, want 1", count)
	}
}

func TestRepositoryByIDRespectsCancelledContext(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)
	id := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb9")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repository.ByID(ctx, id)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ByID() error = %v, want context.Canceled", err)
	}
}

func openIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() error = %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() error = %v", err)
	}

	return pool
}

func resetMonitors(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := pool.Exec(
		ctx,
		"TRUNCATE TABLE monitoring.check_runs, monitoring.monitors",
	); err != nil {
		t.Fatalf("truncate monitoring execution state: %v", err)
	}
}

func mustMonitor(
	t *testing.T,
	rawID string,
	rawTarget string,
	createdAt time.Time,
) domain.Monitor {
	t.Helper()

	id := mustMonitorID(t, rawID)
	target, err := domain.NewTargetURL(rawTarget)
	if err != nil {
		t.Fatalf("NewTargetURL(%q) error = %v", rawTarget, err)
	}
	monitor, err := domain.NewMonitor(id, target, createdAt)
	if err != nil {
		t.Fatalf("NewMonitor() error = %v", err)
	}
	return monitor
}

func mustMonitorID(t *testing.T, raw string) domain.MonitorID {
	t.Helper()

	id, err := domain.ParseMonitorID(raw)
	if err != nil {
		t.Fatalf("ParseMonitorID(%q) error = %v", raw, err)
	}
	return id
}
