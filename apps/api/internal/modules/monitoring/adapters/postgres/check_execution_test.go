package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsPendingClaimConflict(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "matching unique constraint",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "check_runs_one_pending_per_monitor_idx",
			},
			want: true,
		},
		{
			name: "other unique constraint",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "other_unique_idx",
			},
		},
		{
			name: "same constraint non unique violation",
			err: &pgconn.PgError{
				Code:           "23514",
				ConstraintName: "check_runs_one_pending_per_monitor_idx",
			},
		},
		{
			name: "wrapped matching violation",
			err: errors.Join(errors.New("insert failed"), &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "check_runs_one_pending_per_monitor_idx",
			}),
			want: true,
		},
		{name: "non PostgreSQL error", err: errors.New("boom")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := isPendingClaimConflict(test.err); got != test.want {
				t.Fatalf("isPendingClaimConflict() = %v, want %v", got, test.want)
			}
		})
	}
}
