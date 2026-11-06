package application

import (
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func parseHistoryInstant(t *testing.T, raw string) time.Time {
	t.Helper()
	at, err := time.Parse(historyTimestampLayout, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return at
}

func mustEncodeInventoryPosition(t *testing.T, raw string, monitorID string) string {
	t.Helper()
	id, err := domain.ParseMonitorID(monitorID)
	if err != nil {
		t.Fatalf("ParseMonitorID(%q) error = %v", monitorID, err)
	}
	token, err := EncodeInventoryCursor(ports.InventoryAnchor{CreatedAt: parseHistoryInstant(t, raw), ID: id})
	if err != nil {
		t.Fatalf("EncodeInventoryCursor() error = %v", err)
	}
	return token
}

func TestHistoryCursorRoundTripAndRejections(t *testing.T) {
	completedAt := parseHistoryInstant(t, "2026-11-01T12:00:00.123456Z")
	checkID := mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675")
	token, err := EncodeHistoryCursor(ports.HistoryAnchor{CompletedAt: completedAt, CheckID: checkID})
	if err != nil {
		t.Fatalf("EncodeHistoryCursor() error = %v", err)
	}
	if len(token) != 88 {
		t.Fatalf("cursor length = %d, want 88", len(token))
	}
	decoded, err := DecodeHistoryCursor(token)
	if err != nil {
		t.Fatalf("DecodeHistoryCursor() error = %v", err)
	}
	if !decoded.CompletedAt.Equal(completedAt) || decoded.CheckID != checkID {
		t.Fatalf("decoded = %+v, want the original anchor", decoded)
	}
	reencoded, err := EncodeHistoryCursor(decoded)
	if err != nil || reencoded != token {
		t.Fatalf("re-encode = %q (error %v), want %q", reencoded, err, token)
	}

	for _, rejected := range []string{
		mustEncodeInventoryPosition(t, "2026-11-01T12:00:00.123456Z", "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"),
		"1|" + completedAt.Format("2006-01-02T15:04:05.000000Z") + "|" + checkID.String(),
		token[:87],
		token + "A",
		"2|" + completedAt.Format("2006-01-02T15:04:05Z") + "|" + checkID.String() + "malformed",
	} {
		if _, err := DecodeHistoryCursor(rejected); err == nil {
			t.Fatalf("DecodeHistoryCursor(%q) error = nil, want rejection", rejected)
		}
	}
}
