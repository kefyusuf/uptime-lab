package application

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func inventoryAnchor(t *testing.T) ports.InventoryAnchor {
	t.Helper()
	id, err := domain.ParseMonitorID("018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")
	if err != nil {
		t.Fatal(err)
	}
	return ports.InventoryAnchor{ID: id, CreatedAt: time.Date(2026, 10, 5, 0, 0, 0, 123456000, time.UTC)}
}

func TestInventoryCursorCanonicalRoundtrip(t *testing.T) {
	anchor := inventoryAnchor(t)
	token, err := EncodeInventoryCursor(anchor)
	if err != nil || len(token) != 88 {
		t.Fatal(token, err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 66 {
		t.Fatal(string(raw), err)
	}
	got, err := DecodeInventoryCursor(token)
	if err != nil || got.ID != anchor.ID || !got.CreatedAt.Equal(anchor.CreatedAt) || got.CreatedAt.Nanosecond() != 123456000 {
		t.Fatal(got, err)
	}
}

func TestInventoryCursorRejectsAlternativeSpellings(t *testing.T) {
	valid := "1|2026-10-05T00:00:00.123456Z|018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"
	for _, raw := range []string{
		strings.Replace(valid, "1|", "2|", 1), strings.Replace(valid, "10-05", "02-30", 1),
		strings.Replace(valid, "00:00:00", "24:00:00", 1), strings.Replace(valid, "2026", "0000", 1),
		strings.Replace(valid, "2026-10-05T00:00:00.123456", "0001-01-01T00:00:00.000000", 1),
		strings.Replace(valid, ".123456", ".12345", 1), strings.ToUpper(valid),
		strings.Replace(valid, "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2", "00000000-0000-0000-0000-000000000000", 1),
		valid + "x", strings.Replace(valid, "|", "/", 1),
	} {
		if _, err := DecodeInventoryCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err != ErrInvalidInventoryQuery {
			t.Errorf("accepted invalid cursor: %q: %v", raw, err)
		}
	}
	token, _ := EncodeInventoryCursor(inventoryAnchor(t))
	for _, bad := range []string{token + "=", token[:87], "!" + token[1:], " " + token} {
		if _, err := DecodeInventoryCursor(bad); err != ErrInvalidInventoryQuery {
			t.Errorf("accepted spelling %q", bad)
		}
	}
	anchor := inventoryAnchor(t)
	for _, at := range []time.Time{time.Time{}, anchor.CreatedAt.Add(time.Nanosecond), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)} {
		anchor.CreatedAt = at
		if _, err := EncodeInventoryCursor(anchor); err != ErrInvalidInventoryQuery {
			t.Fatal(at, err)
		}
	}
	anchor = inventoryAnchor(t)
	anchor.ID = domain.MonitorID{}
	if _, err := EncodeInventoryCursor(anchor); err != ErrInvalidInventoryQuery {
		t.Fatal(err)
	}
}
