package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

var _ ports.MonitorSchedulingRepository = (*Repository)(nil)

func (repository *Repository) GetScheduling(ctx context.Context, id domain.MonitorID) (domain.SchedulingState, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var paused bool
	err := repository.pool.QueryRow(ctx, `SELECT paused FROM monitoring.monitors WHERE id=$1::uuid`, id.String()).Scan(&paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ports.ErrMonitorNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read monitor scheduling: %w", err)
	}
	return schedulingStateFromPaused(paused), nil
}

func (repository *Repository) SetScheduling(ctx context.Context, id domain.MonitorID, state domain.SchedulingState) (domain.SchedulingState, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !state.Valid() {
		return "", domain.ErrInvalidSchedulingState
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", fmt.Errorf("begin scheduling transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var paused bool
	err = tx.QueryRow(ctx, `UPDATE monitoring.monitors SET paused=$2 WHERE id=$1::uuid RETURNING paused`, id.String(), state == domain.SchedulingPaused).Scan(&paused)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ports.ErrMonitorNotFound
	}
	if err != nil {
		return "", fmt.Errorf("set monitor scheduling: %w", err)
	}
	persisted := schedulingStateFromPaused(paused)
	if persisted != state {
		return "", errors.New("scheduling state was not persisted")
	}
	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit scheduling transaction: %w", err)
	}
	return persisted, nil
}

func schedulingStateFromPaused(paused bool) domain.SchedulingState {
	if paused {
		return domain.SchedulingPaused
	}
	return domain.SchedulingActive
}
