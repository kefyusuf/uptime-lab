package application

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

func parseIncidentInstant(t *testing.T, raw string) time.Time {
	t.Helper()
	at, err := time.Parse(incidentTimestampLayout, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return at
}

func TestIncidentCursorVersion3Canonical(t *testing.T) {
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	incidentID := mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001")
	token, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: openedAt, IncidentID: incidentID})
	if err != nil {
		t.Fatalf("EncodeIncidentCursor() error = %v", err)
	}
	if len(token) != 88 {
		t.Fatalf("cursor length = %d, want 88", len(token))
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || string(raw[:2]) != "3|" {
		t.Fatalf("cursor version prefix = %q (error %v), want 3|", raw[:2], err)
	}
	decoded, err := DecodeIncidentCursor(token)
	if err != nil {
		t.Fatalf("DecodeIncidentCursor() error = %v", err)
	}
	if !decoded.OpenedAt.Equal(openedAt) || decoded.IncidentID != incidentID {
		t.Fatalf("decoded = %+v, want the original anchor", decoded)
	}
	reencoded, err := EncodeIncidentCursor(decoded)
	if err != nil || reencoded != token {
		t.Fatalf("re-encode = %q (error %v), want %q", reencoded, err, token)
	}
}

func TestIncidentCursorRejectsInventoryAndHistoryTokens(t *testing.T) {
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	incidentID := mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001")
	token, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: openedAt, IncidentID: incidentID})
	if err != nil {
		t.Fatalf("EncodeIncidentCursor() error = %v", err)
	}
	historyToken, err := EncodeHistoryCursor(ports.HistoryAnchor{
		CompletedAt: openedAt,
		CheckID:     mustCheckID(t, "7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675"),
	})
	if err != nil {
		t.Fatalf("EncodeHistoryCursor() error = %v", err)
	}
	inventoryToken := mustEncodeInventoryPosition(t, "2026-11-01T12:00:00.123456Z", "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2")

	for _, rejected := range []string{
		inventoryToken,
		historyToken,
		"1|" + openedAt.Format(incidentTimestampLayout) + "|" + incidentID.String(),
		"2|" + openedAt.Format(incidentTimestampLayout) + "|" + incidentID.String(),
		token[:87],
		token + "A",
		"3|" + openedAt.Format("2006-01-02T15:04:05Z") + "|" + incidentID.String() + "malformed",
	} {
		if _, err := DecodeIncidentCursor(rejected); err == nil {
			t.Fatalf("DecodeIncidentCursor(%q) error = nil, want rejection", rejected)
		}
	}
}

func TestIncidentCursorRejectsNonCanonicalPositions(t *testing.T) {
	openedAt := parseIncidentInstant(t, "2026-11-01T12:00:00.123456Z")
	incidentID := mustIncidentID(t, "018f22d3-1d6a-7cc0-a37b-46fc3faff001")

	if _, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: time.Time{}, IncidentID: incidentID}); err == nil {
		t.Fatal("EncodeIncidentCursor(zero instant) error = nil, want rejection")
	}
	if _, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: openedAt, IncidentID: domain.IncidentID{}}); err == nil {
		t.Fatal("EncodeIncidentCursor(zero incident id) error = nil, want rejection")
	}
	subMicro := openedAt.Add(100 * time.Nanosecond)
	if _, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: subMicro, IncidentID: incidentID}); err == nil {
		t.Fatal("EncodeIncidentCursor(sub-microsecond instant) error = nil, want rejection")
	}
}
