//go:build integration

package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// The four exercised query shapes mirror the adapter sources one-to-one:
//   - inventory first page:        adapters/postgres/monitor_inventory.go (inventoryPageFirst)
//   - history first/anchored page: adapters/postgres/monitor_history.go (firstPage / anchoredPage)
//   - latest terminal read:        adapters/postgres/latest_check_result.go
//   - claim candidate:             adapters/postgres/check_execution.go (claimCandidate)
//   - retention:                   adapters/postgres/retention.go (retainHistoryWindow)
// Any change to those queries must update these copies and the committed
// evidence document (docs/testing/populated-explain-evidence.md) in the same
// commits.
const explainEvidenceQueryInventory = `SELECT id::text, created_at, octet_length(target_url),
 CASE WHEN octet_length(target_url)<=$1 THEN target_url ELSE NULL END
 FROM monitoring.monitors ORDER BY created_at DESC,id DESC LIMIT $2`

const explainEvidenceQueryHistoryFirst = "SELECT id::text, result_kind, http_status, duration_ms, completed_at FROM monitoring.check_runs WHERE monitor_id = $1 AND completed_at IS NOT NULL ORDER BY completed_at DESC, id DESC LIMIT $2"

const explainEvidenceQueryHistoryAnchored = "SELECT id::text, result_kind, http_status, duration_ms, completed_at FROM monitoring.check_runs WHERE monitor_id = $1 AND completed_at IS NOT NULL AND (completed_at, id) < ($2, $3) ORDER BY completed_at DESC, id DESC LIMIT $4"

const explainEvidenceQueryLatest = "SELECT id::text, result_kind, http_status, duration_ms, completed_at FROM monitoring.check_runs WHERE monitor_id = $1 AND completed_at IS NOT NULL ORDER BY completed_at DESC LIMIT 1"

const explainEvidenceQueryClaim = `
			SELECT
				monitor.id::text,
				monitor.target_url
			FROM monitoring.monitors AS monitor
			LEFT JOIN LATERAL (
				SELECT run.completed_at AS latest_completed_at
				FROM monitoring.check_runs AS run
				WHERE run.monitor_id = monitor.id
				  AND run.completed_at IS NOT NULL
				ORDER BY run.completed_at DESC
				LIMIT 1
			) AS terminal ON true
			WHERE monitor.paused = false AND NOT EXISTS (
				SELECT 1
				FROM monitoring.check_runs AS pending
				WHERE pending.monitor_id = monitor.id
				  AND pending.completed_at IS NULL
			)
			  AND (
				terminal.latest_completed_at IS NULL
				OR terminal.latest_completed_at <= $1
			  )
			ORDER BY
				COALESCE(terminal.latest_completed_at, monitor.created_at) ASC,
				monitor.id ASC
			FOR UPDATE OF monitor SKIP LOCKED
			LIMIT 1
		`

const explainEvidenceQueryRetention = `
		DELETE FROM monitoring.check_runs
		WHERE completed_at IS NOT NULL
		  AND id IN (
		    SELECT id FROM (
		      SELECT id,
		             row_number() OVER (PARTITION BY monitor_id
		                                ORDER BY completed_at DESC, id DESC) AS rank
		      FROM monitoring.check_runs
		      WHERE completed_at IS NOT NULL
		    ) ranked
		    WHERE rank > $1
		  )
		`

const explainEvidenceMonitors = 25
const explainEvidenceRowsPerMonitor = 50

func explainPool(t *testing.T, pool *pgxpool.Pool, query string, args ...any) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "EXPLAIN (ANALYZE, FORMAT TEXT) "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN failed: %v", err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan line: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate plan lines: %v", err)
	}
	return plan.String()
}

func assertPlanUsesIndex(t *testing.T, plan string, indexName string, label string) {
	t.Helper()
	if !strings.Contains(plan, indexName) {
		t.Fatalf("%s plan does not use index %s:\n%s", label, indexName, plan)
	}
}

func TestExplainEvidenceOverPopulatedDataset(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	monitors := make([]domain.MonitorID, 0, explainEvidenceMonitors)
	for monitorIndex := 0; monitorIndex < explainEvidenceMonitors; monitorIndex++ {
		monitorID := mustMonitorID(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3faf6%03x", monitorIndex))
		monitors = append(monitors, monitorID)
		insertExecutionMonitor(t, pool, monitorID, fmt.Sprintf("https://example.com/explain-%02d", monitorIndex), now.Add(-24*time.Hour))
		base := now.Add(-2 * time.Hour)
		for rowIndex := 0; rowIndex < explainEvidenceRowsPerMonitor; rowIndex++ {
			checkID := historyCheckID(t, monitorIndex*100+rowIndex)
			completedAt := base.Add(-time.Duration(rowIndex) * time.Second)
			insertTerminalRun(t, pool, checkID, monitorID, completedAt.Add(-20*time.Second), completedAt, "timeout", nil, int64Ptr(1))
		}
	}
	pending := historyCheckID(t, 999)
	insertPendingRun(t, pool, pending, monitors[0], now.Add(-5*time.Second), now.Add(15*time.Second))

	plans := map[string]string{}
	plans["inventory"] = explainPool(t, pool, explainEvidenceQueryInventory, 245760, 21)
	plans["history-first"] = explainPool(t, pool, explainEvidenceQueryHistoryFirst, monitors[0].String(), 21)
	plans["history-anchored"] = explainPool(t, pool, explainEvidenceQueryHistoryAnchored, monitors[0].String(), now.Add(-90*time.Minute), historyCheckID(t, 999), 21)
	plans["latest-terminal"] = explainPool(t, pool, explainEvidenceQueryLatest, monitors[0].String())
	plans["claim-candidate"] = explainPool(t, pool, explainEvidenceQueryClaim, now.Add(-time.Minute))
	plans["retention"] = explainPool(t, pool, explainEvidenceQueryRetention, 50)

	assertPlanUsesIndex(t, plans["inventory"], "monitors_inventory_order_idx", "inventory")
	assertPlanUsesIndex(t, plans["history-first"], "check_runs_latest_terminal_idx", "history-first")
	assertPlanUsesIndex(t, plans["history-anchored"], "check_runs_latest_terminal_idx", "history-anchored")
	assertPlanUsesIndex(t, plans["latest-terminal"], "check_runs_latest_terminal_idx", "latest-terminal")

	// The index-usage assertion is load-bearing: dropping the terminal index
	// (test-owned drop, restored immediately) must make the asserted index
	// disappear from the plan.
	if _, err := pool.Exec(context.Background(), "DROP INDEX monitoring.check_runs_latest_terminal_idx"); err != nil {
		t.Fatalf("drop terminal index: %v", err)
	}
	broken := explainPool(t, pool, explainEvidenceQueryHistoryFirst, monitors[0].String(), 21)
	if strings.Contains(broken, "check_runs_latest_terminal_idx") {
		t.Fatal("terminal index still used after drop")
	}
	if _, err := pool.Exec(context.Background(), "CREATE INDEX check_runs_latest_terminal_idx ON monitoring.check_runs (monitor_id, completed_at DESC) WHERE completed_at IS NOT NULL"); err != nil {
		t.Fatalf("recreate terminal index: %v", err)
	}
	restored := explainPool(t, pool, explainEvidenceQueryHistoryFirst, monitors[0].String(), 21)
	assertPlanUsesIndex(t, restored, "check_runs_latest_terminal_idx", "history-first restored")
}
