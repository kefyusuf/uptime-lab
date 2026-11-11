//go:build integration

package postgres

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func TestTargetEditPersistsAndReturnsMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd601")
	createdAt := now.Add(-time.Hour)
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/old-target", createdAt)

	edited, err := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/new-target?region=eu"))
	if err != nil {
		t.Fatalf("EditTarget() error = %v", err)
	}
	if edited.ID() != monitorID {
		t.Fatalf("edited ID = %s, want %s", edited.ID(), monitorID)
	}
	if edited.TargetURL().String() != "https://example.com/new-target?region=eu" {
		t.Fatalf("edited target = %q", edited.TargetURL().String())
	}
	if !edited.CreatedAt().Equal(createdAt) {
		t.Fatalf("edited createdAt = %s, want unchanged %s", edited.CreatedAt(), createdAt)
	}

	var persisted string
	if err := pool.QueryRow(
		context.Background(),
		"SELECT target_url FROM monitoring.monitors WHERE id = $1::uuid",
		monitorID.String(),
	).Scan(&persisted); err != nil {
		t.Fatalf("query persisted target: %v", err)
	}
	if persisted != "https://example.com/new-target?region=eu" {
		t.Fatalf("persisted target = %q", persisted)
	}
}

func mustTargetURL(t *testing.T, raw string) domain.TargetURL {
	t.Helper()
	target, err := domain.NewTargetURL(raw)
	if err != nil {
		t.Fatalf("NewTargetURL(%q) error = %v", raw, err)
	}
	return target
}

func TestTargetEditUnknownMonitor(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	unknown := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd602")
	_, err := repository.EditTarget(context.Background(), unknown, mustTargetURL(t, "https://example.com/new"))
	if !errors.Is(err, ports.ErrMonitorNotFound) {
		t.Fatalf("EditTarget() error = %v, want ErrMonitorNotFound", err)
	}
}

func TestTargetEditSurvivesPauseResumeAndRetention(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 10, 14, 10, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd603")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/edit-paused", now.Add(-2*time.Hour))

	if _, err := repository.SetScheduling(context.Background(), monitorID, domain.SchedulingPaused); err != nil {
		t.Fatalf("SetScheduling(paused) error = %v", err)
	}
	if _, err := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/edited-while-paused")); err != nil {
		t.Fatalf("EditTarget() while paused error = %v", err)
	}
	state, err := repository.GetScheduling(context.Background(), monitorID)
	if err != nil {
		t.Fatalf("GetScheduling() error = %v", err)
	}
	if state != domain.SchedulingPaused {
		t.Fatalf("scheduling state after edit = %q, want paused", state)
	}
	edited, err := repository.Page(context.Background(), monitorID, nil, 50)
	if err != nil {
		t.Fatalf("history Page() error = %v", err)
	}
	if len(edited) != 0 {
		t.Fatalf("history rows = %d, want 0 (edit changes nothing else)", len(edited))
	}
}

func TestTargetEditRacingClaimProbesOldTarget(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 10, 14, 20, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd604")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/old-target", now.Add(-2*time.Hour))

	claimedCheck := mustExecutionCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c701")
	claimed, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   claimedCheck,
		Now:       now,
		DueBefore: now.Add(-60 * time.Second),
		Deadline:  now.Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("ClaimDueCheck() error = %v", err)
	}
	if claimed.TargetURL.String() != "https://example.com/old-target" {
		t.Fatalf("claimed target = %q, want the old target", claimed.TargetURL.String())
	}

	edited, err := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/edited-target"))
	if err != nil {
		t.Fatalf("EditTarget() after claim error = %v", err)
	}
	if edited.TargetURL().String() != "https://example.com/edited-target" {
		t.Fatalf("edited target = %q", edited.TargetURL().String())
	}

	// The in-flight run's completion is durable and unaffected by the edit.
	result, buildErr := domain.NewCheckFailureResult(domain.CheckResultTimeout, 1000)
	if buildErr != nil {
		t.Fatalf("build result: %v", buildErr)
	}
	if err := repository.CompleteCheck(context.Background(), ports.CompleteCheckInput{
		CheckID:     claimedCheck,
		CompletedAt: now.Add(time.Second),
		Result:      result,
	}); err != nil {
		t.Fatalf("CompleteCheck() after edit error = %v", err)
	}

	// The next claim probes the NEW target.
	nextClaim, err := repository.ClaimDueCheck(context.Background(), ports.ClaimDueCheckInput{
		CheckID:   mustExecutionCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c702"),
		Now:       now.Add(2 * time.Minute),
		DueBefore: now.Add(time.Minute),
		Deadline:  now.Add(2 * time.Minute).Add(20 * time.Second),
	})
	if err != nil {
		t.Fatalf("next ClaimDueCheck() error = %v", err)
	}
	if nextClaim.TargetURL.String() != "https://example.com/edited-target" {
		t.Fatalf("next claimed target = %q, want the edited target", nextClaim.TargetURL.String())
	}
}

func TestTargetEditRacingDelete(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 10, 14, 30, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd605")

	t.Run("edit then delete", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/edit-delete", now.Add(-2*time.Hour))
		if _, err := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/edited-first")); err != nil {
			t.Fatalf("EditTarget() error = %v", err)
		}
		if err := repository.Delete(context.Background(), monitorID); err != nil {
			t.Fatalf("Delete() after edit error = %v", err)
		}
		assertMonitorRowCount(t, pool, monitorID, 0)
	})

	t.Run("delete then edit", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/delete-edit", now.Add(-2*time.Hour))
		if err := repository.Delete(context.Background(), monitorID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/racing")); !errors.Is(err, ports.ErrMonitorNotFound) {
			t.Fatalf("EditTarget() after delete error = %v, want ErrMonitorNotFound", err)
		}
		assertMonitorRowCount(t, pool, monitorID, 0)
	})

	t.Run("edit serializes on a concurrently held monitor row", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/edit-held", now.Add(-2*time.Hour))

		held, err := pool.Begin(context.Background())
		if err != nil {
			t.Fatalf("begin parking transaction: %v", err)
		}
		if _, err := held.Exec(
			context.Background(),
			"SELECT id FROM monitoring.monitors WHERE id = $1::uuid FOR UPDATE",
			monitorID.String(),
		); err != nil {
			t.Fatalf("park monitor row lock: %v", err)
		}

		edits := make(chan error, 1)
		go func() {
			_, editErr := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/edit-held"))
			edits <- editErr
		}()
		waitForTransactionLockWait(t, pool)

		if err := held.Commit(context.Background()); err != nil {
			t.Fatalf("release parked lock: %v", err)
		}
		select {
		case editErr := <-edits:
			if editErr != nil {
				t.Fatalf("blocked EditTarget() error = %v", editErr)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("blocked EditTarget() did not finish after release")
		}
		var persisted string
		if err := pool.QueryRow(
			context.Background(),
			"SELECT target_url FROM monitoring.monitors WHERE id = $1::uuid",
			monitorID.String(),
		).Scan(&persisted); err != nil {
			t.Fatalf("query edited target: %v", err)
		}
		if persisted != "https://example.com/edit-held" {
			t.Fatalf("persisted target = %q", persisted)
		}
	})

	t.Run("edit blocked by a delete observes not found after the delete commits", func(t *testing.T) {
		pool := openIntegrationPool(t)
		resetMonitors(t, pool)
		insertExecutionMonitor(t, pool, monitorID, "https://example.com/edit-vs-delete-held", now.Add(-2*time.Hour))

		held, err := pool.Begin(context.Background())
		if err != nil {
			t.Fatalf("begin deleting transaction: %v", err)
		}
		if _, err := held.Exec(
			context.Background(),
			"DELETE FROM monitoring.monitors WHERE id = $1::uuid",
			monitorID.String(),
		); err != nil {
			t.Fatalf("delete monitor row under lock: %v", err)
		}

		edits := make(chan error, 1)
		go func() {
			_, editErr := repository.EditTarget(context.Background(), monitorID, mustTargetURL(t, "https://example.com/racing-edit"))
			edits <- editErr
		}()
		waitForTransactionLockWait(t, pool)

		if err := held.Commit(context.Background()); err != nil {
			t.Fatalf("commit racing delete: %v", err)
		}
		select {
		case editErr := <-edits:
			if !errors.Is(editErr, ports.ErrMonitorNotFound) {
				t.Fatalf("EditTarget() after committed delete error = %v, want ErrMonitorNotFound", editErr)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("blocked EditTarget() did not finish after the delete committed")
		}
		assertMonitorRowCount(t, pool, monitorID, 0)
	})
}

func TestTargetEditConcurrentLastCommittedWins(t *testing.T) {
	pool := openIntegrationPool(t)
	resetMonitors(t, pool)
	repository := NewRepository(pool)

	now := time.Date(2026, 11, 10, 14, 40, 0, 0, time.UTC)
	monitorID := mustMonitorID(t, "018f22d3-1d6a-7cc0-a37b-46fc3fafd606")
	insertExecutionMonitor(t, pool, monitorID, "https://example.com/last-wins", now.Add(-2*time.Hour))

	const attempts = 6
	start := make(chan struct{})
	results := make(chan string, attempts)
	var wait sync.WaitGroup
	for index := 0; index < attempts; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			target := mustTargetURL(t, "https://example.com/edit-"+itoa(index))
			edited, err := repository.EditTarget(context.Background(), monitorID, target)
			if err != nil {
				results <- "ERROR:" + err.Error()
				return
			}
			results <- edited.TargetURL().String()
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)

	editedCount := 0
	for outcome := range results {
		if len(outcome) > 5 && outcome[:5] == "ERROR" {
			t.Fatalf("racing edit error = %s", outcome)
		}
		editedCount++
	}
	if editedCount != attempts {
		t.Fatalf("edited outcomes = %d, want %d", editedCount, attempts)
	}
	var persisted string
	if err := pool.QueryRow(
		context.Background(),
		"SELECT target_url FROM monitoring.monitors WHERE id = $1::uuid",
		monitorID.String(),
	).Scan(&persisted); err != nil {
		t.Fatalf("query final target: %v", err)
	}
	if persisted == "" {
		t.Fatal("final target empty")
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
