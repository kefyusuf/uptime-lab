//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

// Documented copies of the adapter source queries (adapters/postgres/monitor_incidents.go
// firstPage / anchoredPage consts), verified for sync at whole-branch review. Any change
// to those queries must update these copies in the same commit.
const explainEvidenceQueryIncidentsFirst = "SELECT id::text, opened_at, resolved_at, opening_reason, opened_by_check_id::text, resolved_by_check_id::text FROM monitoring.incidents WHERE monitor_id = $1 ORDER BY opened_at DESC, id DESC LIMIT $2"

const explainEvidenceQueryIncidentsAnchored = "SELECT id::text, opened_at, resolved_at, opening_reason, opened_by_check_id::text, resolved_by_check_id::text FROM monitoring.incidents WHERE monitor_id = $1 AND (opened_at, id) < ($2, $3) ORDER BY opened_at DESC, id DESC LIMIT $4"

func insertIncidentRow(
	t *testing.T,
	pool *pgxpool.Pool,
	incidentID domain.IncidentID,
	monitorID domain.MonitorID,
	openedAt time.Time,
	resolvedAt *time.Time,
	reason string,
	openedBy, resolvedBy *domain.CheckID,
) {
	t.Helper()
	var openedByRaw, resolvedByRaw *string
	if openedBy != nil {
		raw := openedBy.String()
		openedByRaw = &raw
	}
	if resolvedBy != nil {
		raw := resolvedBy.String()
		resolvedByRaw = &raw
	}
	if _, err := pool.Exec(
		context.Background(),
		`INSERT INTO monitoring.incidents (
			id, monitor_id, opened_at, resolved_at, opening_reason, opened_by_check_id, resolved_by_check_id
		) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid, $7::uuid)`,
		incidentID.String(),
		monitorID.String(),
		openedAt,
		resolvedAt,
		reason,
		openedByRaw,
		resolvedByRaw,
	); err != nil {
		t.Fatalf("insert incident row: %v", err)
	}
}

func assertIncidentOrder(t *testing.T, entries []ports.IncidentRecord, want []domain.IncidentID) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("incident rows = %d, want %d", len(entries), len(want))
	}
	for index, entry := range entries {
		if entry.IncidentID != want[index] {
			t.Fatalf("incident row %d = %s, want %s", index, entry.IncidentID, want[index])
		}
	}
}

func TestIncidentPageOrdersByOpenedAtDescIDDesc(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd701")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-read", now.Add(-time.Hour))

	newest := incidentFixtureCandidateID(t, 1)
	middle := incidentFixtureCandidateID(t, 2)
	oldest := incidentFixtureCandidateID(t, 3)
	openedEvidence := incidentFixtureCheckID(t, 1)
	resolvedEvidence := incidentFixtureCheckID(t, 2)
	resolvedStatus := 200
	insertTerminalRun(t, pool, openedEvidence, monitorID, now.Add(-4*time.Minute), now.Add(-3*time.Minute), "timeout", nil, int64Ptr(1))
	insertTerminalRun(t, pool, resolvedEvidence, monitorID, now.Add(-2*time.Minute), now.Add(-time.Minute), "http_response", &resolvedStatus, int64Ptr(1))
	middleResolved := now.Add(-time.Minute)
	insertIncidentRow(t, pool, newest, monitorID, now.Add(-2*time.Minute), nil, "probe_failure", nil, nil)
	insertIncidentRow(t, pool, middle, monitorID, now.Add(-3*time.Minute), &middleResolved, "unexpected_http_status", &openedEvidence, &resolvedEvidence)
	insertIncidentRow(t, pool, oldest, monitorID, now.Add(-4*time.Minute), &middleResolved, "probe_failure", nil, nil)

	entries, err := repository.PageIncidents(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("PageIncidents() error = %v", err)
	}
	assertIncidentOrder(t, entries, []domain.IncidentID{newest, middle, oldest})
	if entries[0].ResolvedAt != nil || entries[0].OpeningReason != domain.IncidentProbeFailure {
		t.Fatalf("open window = resolved %v reason %q, want nil/probe_failure", entries[0].ResolvedAt, entries[0].OpeningReason)
	}
	if entries[0].OpenedByCheckID != nil || entries[0].ResolvedByCheckID != nil {
		t.Fatalf("open window exposed null evidence links: %+v", entries[0])
	}
	if entries[1].ResolvedAt == nil || !entries[1].ResolvedAt.Equal(middleResolved) || entries[1].OpeningReason != domain.IncidentUnexpectedHTTPStatus {
		t.Fatalf("resolved window = %+v, want resolved instant and unexpected_http_status", entries[1])
	}
	if entries[1].OpenedByCheckID == nil || *entries[1].OpenedByCheckID != openedEvidence {
		t.Fatalf("resolved window opening evidence = %v, want %s", entries[1].OpenedByCheckID, openedEvidence)
	}
	if entries[1].ResolvedByCheckID == nil || *entries[1].ResolvedByCheckID != resolvedEvidence {
		t.Fatalf("resolved window resolving evidence = %v, want %s", entries[1].ResolvedByCheckID, resolvedEvidence)
	}
	if !entries[0].OpenedAt.Equal(now.Add(-2*time.Minute)) || entries[0].OpenedAt.Location() != time.UTC {
		t.Fatalf("opened instant = %v, want UTC %v", entries[0].OpenedAt, now.Add(-2*time.Minute))
	}
}

func TestIncidentPageTieBreaksEqualOpenedInstants(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd702")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-ties", now.Add(-time.Hour))
	instant := now.Add(-time.Minute)
	resolved := now.Add(-30 * time.Second)
	ids := make([]domain.IncidentID, 0, 3)
	for index := 0; index < 3; index++ {
		id := incidentFixtureCandidateID(t, 21+index)
		ids = append(ids, id)
		insertIncidentRow(t, pool, id, monitorID, instant, &resolved, "probe_failure", nil, nil)
	}

	entries, err := repository.PageIncidents(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("PageIncidents() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("incident rows = %d, want 3", len(entries))
	}
	ordered := make([]string, 0, 3)
	for _, entry := range entries {
		ordered = append(ordered, entry.IncidentID.String())
	}
	if !sort.StringsAreSorted(reverse(ordered)) {
		t.Fatalf("tie-break order = %v, want incident id descending", ordered)
	}
}

func TestIncidentPageRespectsLimitLookahead(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd703")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-bounds", now.Add(-time.Hour))
	ids := make([]domain.IncidentID, 0, 22)
	for index := 0; index < 22; index++ {
		id := incidentFixtureCandidateID(t, index)
		ids = append(ids, id)
		openedAt := now.Add(-time.Duration(index+1) * time.Minute)
		resolvedAt := openedAt.Add(30 * time.Second)
		insertIncidentRow(t, pool, id, monitorID, openedAt, &resolvedAt, "probe_failure", nil, nil)
	}

	page, err := repository.PageIncidents(context.Background(), monitorID, nil, 21)
	if err != nil {
		t.Fatalf("PageIncidents(21) error = %v", err)
	}
	if len(page) != 21 {
		t.Fatalf("PageIncidents(21) rows = %d, want exactly 21", len(page))
	}
	assertIncidentOrder(t, page, ids[:21])

	full, err := repository.PageIncidents(context.Background(), monitorID, nil, 51)
	if err != nil {
		t.Fatalf("PageIncidents(51) error = %v", err)
	}
	assertIncidentOrder(t, full, ids)

	if _, err := repository.PageIncidents(context.Background(), monitorID, nil, 0); err == nil {
		t.Fatal("PageIncidents(0) error = nil, want bounds rejection")
	}
	if _, err := repository.PageIncidents(context.Background(), monitorID, nil, 52); err == nil {
		t.Fatal("PageIncidents(52) error = nil, want bounds rejection")
	}
}

func TestIncidentPageAnchorKeyset(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd704")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-continuation", now.Add(-time.Hour))
	newest := incidentFixtureCandidateID(t, 11)
	middle := incidentFixtureCandidateID(t, 12)
	oldest := incidentFixtureCandidateID(t, 13)
	resolved := now.Add(-30 * time.Second)
	insertIncidentRow(t, pool, newest, monitorID, now.Add(-time.Minute), &resolved, "probe_failure", nil, nil)
	insertIncidentRow(t, pool, middle, monitorID, now.Add(-2*time.Minute), &resolved, "probe_failure", nil, nil)
	insertIncidentRow(t, pool, oldest, monitorID, now.Add(-3*time.Minute), &resolved, "probe_failure", nil, nil)

	anchor := ports.IncidentAnchor{OpenedAt: now.Add(-time.Minute), IncidentID: newest}
	older, err := repository.PageIncidents(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("anchored PageIncidents() error = %v", err)
	}
	assertIncidentOrder(t, older, []domain.IncidentID{middle, oldest})

	oldestAnchor := ports.IncidentAnchor{OpenedAt: now.Add(-3 * time.Minute), IncidentID: oldest}
	none, err := repository.PageIncidents(context.Background(), monitorID, &oldestAnchor, 50)
	if err != nil {
		t.Fatalf("anchored PageIncidents() at oldest error = %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("anchored PageIncidents() at oldest = %d rows, want 0", len(none))
	}

	between := ports.IncidentAnchor{OpenedAt: now.Add(-90 * time.Second), IncidentID: incidentFixtureCandidateID(t, 99)}
	gap, err := repository.PageIncidents(context.Background(), monitorID, &between, 50)
	if err != nil {
		t.Fatalf("between-rows PageIncidents() error = %v", err)
	}
	assertIncidentOrder(t, gap, []domain.IncidentID{middle, oldest})
}

func TestIncidentPageUnknownMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	unknown := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd705")
	if _, err := repository.PageIncidents(context.Background(), unknown, nil, 50); !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("unknown PageIncidents() error = %v, want ErrMonitorNotFound", err)
	}
}

func TestIncidentPageEmptyWhenNoIncidents(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 12, 50, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd706")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-empty", now.Add(-time.Hour))

	entries, err := repository.PageIncidents(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("empty PageIncidents() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty PageIncidents() rows = %d, want 0", len(entries))
	}
}

func TestIncidentPageSnapshotUnderConcurrentCompletion(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 1, 13, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd707")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/incident-stable", now.Add(-time.Hour))
	newest := incidentFixtureCandidateID(t, 51)
	older := incidentFixtureCandidateID(t, 52)
	resolved := now.Add(-30 * time.Second)
	insertIncidentRow(t, pool, newest, monitorID, now.Add(-time.Minute), &resolved, "probe_failure", nil, nil)
	insertIncidentRow(t, pool, older, monitorID, now.Add(-2*time.Minute), &resolved, "probe_failure", nil, nil)

	anchor := ports.IncidentAnchor{OpenedAt: now.Add(-time.Minute), IncidentID: newest}
	first, err := repository.PageIncidents(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("first anchored PageIncidents() error = %v", err)
	}

	fresher := incidentFixtureCandidateID(t, 53)
	insertIncidentRow(t, pool, fresher, monitorID, now.Add(-30*time.Second), &resolved, "probe_failure", nil, nil)

	second, err := repository.PageIncidents(context.Background(), monitorID, &anchor, 50)
	if err != nil {
		t.Fatalf("second anchored PageIncidents() error = %v", err)
	}
	assertIncidentOrder(t, first, []domain.IncidentID{older})
	assertIncidentOrder(t, second, []domain.IncidentID{older})
}

func TestIncidentPageUsesOpenedIndex(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	const monitors = 12
	const rowsPerMonitor = 80
	ids := make([]domain.MonitorID, 0, monitors)
	for monitorIndex := 0; monitorIndex < monitors; monitorIndex++ {
		monitorID := mustMonitorID(t, fmt.Sprintf("018f22d3-1d6a-7cc0-a37b-46fc3faf7%03x", monitorIndex))
		ids = append(ids, monitorID)
		insertExecutionMonitor(t, pool, monitorID, fmt.Sprintf("https://example.com/incident-explain-%02d", monitorIndex), now.Add(-24*time.Hour))
		base := now.Add(-2 * time.Hour)
		for rowIndex := 0; rowIndex < rowsPerMonitor; rowIndex++ {
			incidentID := incidentFixtureCandidateID(t, monitorIndex*rowsPerMonitor+rowIndex)
			openedAt := base.Add(-time.Duration(rowIndex) * time.Second)
			resolvedAt := openedAt.Add(30 * time.Second)
			insertIncidentRow(t, pool, incidentID, monitorID, openedAt, &resolvedAt, "probe_failure", nil, nil)
		}
	}
	if _, err := pool.Exec(context.Background(), "ANALYZE monitoring.incidents"); err != nil {
		t.Fatalf("analyze incidents: %v", err)
	}

	first := explainPool(t, pool, explainEvidenceQueryIncidentsFirst, ids[0].String(), 21)
	assertPlanUsesIndex(t, first, "incidents_monitor_opened_idx", "incidents-first")
	anchored := explainPool(t, pool, explainEvidenceQueryIncidentsAnchored, ids[0].String(), now.Add(-90*time.Minute), incidentFixtureCandidateID(t, 999), 21)
	assertPlanUsesIndex(t, anchored, "incidents_monitor_opened_idx", "incidents-anchored")

	// Load-bearing mutation: dropping the opened index must make it disappear from
	// the plan. The drop is test-owned and restored via t.Cleanup so a failure cannot
	// leave the schema without it.
	if _, err := pool.Exec(context.Background(), "DROP INDEX monitoring.incidents_monitor_opened_idx"); err != nil {
		t.Fatalf("drop opened index: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "CREATE INDEX incidents_monitor_opened_idx ON monitoring.incidents (monitor_id, opened_at DESC, id DESC)")
	})
	broken := explainPool(t, pool, explainEvidenceQueryIncidentsFirst, ids[0].String(), 21)
	if strings.Contains(broken, "incidents_monitor_opened_idx") {
		t.Fatal("opened index still used after drop")
	}
	if _, err := pool.Exec(context.Background(), "CREATE INDEX incidents_monitor_opened_idx ON monitoring.incidents (monitor_id, opened_at DESC, id DESC)"); err != nil {
		t.Fatalf("recreate opened index: %v", err)
	}
	restored := explainPool(t, pool, explainEvidenceQueryIncidentsFirst, ids[0].String(), 21)
	assertPlanUsesIndex(t, restored, "incidents_monitor_opened_idx", "incidents-first restored")

	t.Logf("EXPLAIN-EVIDENCE-BEGIN incidents-first\n%sEXPLAIN-EVIDENCE-END incidents-first", first)
	t.Logf("EXPLAIN-EVIDENCE-BEGIN incidents-anchored\n%sEXPLAIN-EVIDENCE-END incidents-anchored", anchored)
}
