package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorRepository = (*Repository)(nil)

// Repository persists Monitoring aggregates in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs the PostgreSQL Monitoring repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create persists one immutable monitor using its application-assigned identity.
func (repository *Repository) Create(ctx context.Context, monitor domain.Monitor) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	_, err := repository.pool.Exec(
		ctx,
		`
			INSERT INTO monitoring.monitors (id, target_url, created_at)
			VALUES ($1::uuid, $2, $3)
		`,
		monitor.ID().String(),
		monitor.TargetURL().String(),
		monitor.CreatedAt(),
	)
	if err != nil {
		return fmt.Errorf("create monitor: %w", err)
	}

	return nil
}

// ByID retrieves one monitor and maps persistence representation back to domain values.
func (repository *Repository) ByID(ctx context.Context, id domain.MonitorID) (domain.Monitor, error) {
	if err := ctx.Err(); err != nil {
		return domain.Monitor{}, err
	}

	var (
		rawID     string
		rawTarget string
		createdAt time.Time
	)

	err := repository.pool.QueryRow(
		ctx,
		`
			SELECT id::text, target_url, created_at
			FROM monitoring.monitors
			WHERE id = $1::uuid
		`,
		id.String(),
	).Scan(&rawID, &rawTarget, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Monitor{}, ports.ErrMonitorNotFound
	}
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("get monitor: %w", err)
	}

	monitorID, err := domain.ParseMonitorID(rawID)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map monitor id: %w", err)
	}
	target, err := domain.NewTargetURL(rawTarget)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map monitor target: %w", err)
	}
	monitor, err := domain.NewMonitor(monitorID, target, createdAt)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("map monitor: %w", err)
	}

	return monitor, nil
}
