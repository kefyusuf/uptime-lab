package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

type inventoryFake struct {
	items              []ports.InventoryCandidate
	err                error
	calls, rows, bytes int
	anchor             *ports.InventoryAnchor
}

func (f *inventoryFake) Candidates(_ context.Context, a *ports.InventoryAnchor, rows, bytes int) ([]ports.InventoryCandidate, error) {
	f.calls++
	f.rows = rows
	f.bytes = bytes
	f.anchor = a
	return f.items, f.err
}
func inventoryCandidate(t *testing.T) ports.InventoryCandidate {
	a := inventoryAnchor(t)
	return ports.InventoryCandidate{ID: a.ID, CreatedAt: a.CreatedAt, TargetURL: "http://web/", TargetBytes: 11}
}
func TestListMonitorsBoundsBeforeRepository(t *testing.T) {
	f := &inventoryFake{items: []ports.InventoryCandidate{inventoryCandidate(t)}}
	useCase := NewListMonitors(f)
	for _, input := range []InventoryInput{{Limit: 0}, {Limit: -1}, {Limit: 51}, {Limit: 20, Cursor: "invalid"}} {
		if _, err := useCase.Execute(context.Background(), input); err != ErrInvalidInventoryQuery {
			t.Fatal(input, err)
		}
	}
	if f.calls != 0 {
		t.Fatal(f.calls)
	}
	got, err := useCase.Execute(context.Background(), InventoryInput{Limit: 20})
	if err != nil || got.Limit != 20 || len(got.Items) != 1 || f.rows != 21 || f.bytes != 245760 || f.anchor != nil {
		t.Fatal(got, err, f)
	}
	a := inventoryAnchor(t)
	a.CreatedAt = a.CreatedAt.Add(time.Microsecond)
	token, _ := EncodeInventoryCursor(a)
	if _, err := useCase.Execute(context.Background(), InventoryInput{Limit: 50, Cursor: token}); err != nil || f.rows != 51 || f.anchor == nil || !f.anchor.CreatedAt.Equal(a.CreatedAt) {
		t.Fatal(err, f)
	}
}
func TestListMonitorsRejectsInvalidCandidates(t *testing.T) {
	good := inventoryCandidate(t)
	bads := []ports.InventoryCandidate{good, good, good, good, good, good, good, good, good}
	bads[0].ID = domain.MonitorID{}
	bads[1].CreatedAt = time.Time{}
	bads[2].CreatedAt = good.CreatedAt.Add(time.Nanosecond)
	bads[3].TargetBytes++
	bads[4].TargetURL = "file:///private"
	bads[4].TargetBytes = int64(len(bads[4].TargetURL))
	bads[5].Oversized = true
	bads[6].TargetBytes = 245761
	bads[7].TargetURL = ""
	bads[7].TargetBytes = 245761
	bads[8].TargetBytes = -1
	for _, bad := range bads {
		f := &inventoryFake{items: []ports.InventoryCandidate{bad}}
		if _, err := NewListMonitors(f).Execute(context.Background(), InventoryInput{Limit: 20}); err != ErrPersistence {
			t.Fatal(bad, err)
		}
	}
	for _, items := range [][]ports.InventoryCandidate{{good, good}, make([]ports.InventoryCandidate, 22)} {
		if _, err := NewListMonitors(&inventoryFake{items: items}).Execute(context.Background(), InventoryInput{Limit: 20}); err != ErrPersistence {
			t.Fatal(err)
		}
	}
	token, _ := EncodeInventoryCursor(inventoryAnchor(t))
	if _, err := NewListMonitors(&inventoryFake{items: []ports.InventoryCandidate{good}}).Execute(context.Background(), InventoryInput{Limit: 20, Cursor: token}); err != ErrPersistence {
		t.Fatal(err)
	}
	newer := good
	newer.CreatedAt = newer.CreatedAt.Add(time.Microsecond)
	if _, err := NewListMonitors(&inventoryFake{items: []ports.InventoryCandidate{good, newer}}).Execute(context.Background(), InventoryInput{Limit: 20}); err != ErrPersistence {
		t.Fatal(err)
	}
	oversized := good
	oversized.Oversized = true
	oversized.TargetURL = ""
	oversized.TargetBytes = 245761
	if _, err := NewListMonitors(&inventoryFake{items: []ports.InventoryCandidate{oversized}}).Execute(context.Background(), InventoryInput{Limit: 20}); err != nil {
		t.Fatal(err)
	}
}
func TestListMonitorsCancellationAndSanitization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &inventoryFake{}
	if _, err := NewListMonitors(f).Execute(ctx, InventoryInput{Limit: 20}); err != context.Canceled || f.calls != 0 {
		t.Fatal(err, f.calls)
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		f = &inventoryFake{err: cause}
		if _, err := NewListMonitors(f).Execute(context.Background(), InventoryInput{Limit: 20}); err != cause {
			t.Fatal(err)
		}
	}
	f = &inventoryFake{err: errors.New("secret target and cursor")}
	if _, err := NewListMonitors(f).Execute(context.Background(), InventoryInput{Limit: 20}); err != ErrPersistence || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
}
