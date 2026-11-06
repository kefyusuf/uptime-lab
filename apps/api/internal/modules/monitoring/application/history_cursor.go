package application

import (
	"encoding/base64"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const historyTimestampLayout = "2006-01-02T15:04:05.000000Z"

// EncodeHistoryCursor encodes a canonical persisted microsecond position.
func EncodeHistoryCursor(anchor ports.HistoryAnchor) (string, error) {
	at := anchor.CompletedAt.UTC()
	if at.IsZero() || at.Year() < 1 || at.Year() > 9999 || at.Nanosecond()%1000 != 0 {
		return "", ErrInvalidHistoryQuery
	}
	id, err := domain.ParseCheckID(anchor.CheckID.String())
	if err != nil || id != anchor.CheckID {
		return "", ErrInvalidHistoryQuery
	}
	raw := "2|" + at.Format(historyTimestampLayout) + "|" + id.String()
	if len(raw) != 66 {
		return "", ErrInvalidHistoryQuery
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// DecodeHistoryCursor rejects alternate representations of the same position.
func DecodeHistoryCursor(token string) (ports.HistoryAnchor, error) {
	invalid := ports.HistoryAnchor{}
	if len(token) != 88 {
		return invalid, ErrInvalidHistoryQuery
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 66 || base64.RawURLEncoding.EncodeToString(raw) != token || string(raw[:2]) != "2|" || raw[29] != '|' {
		return invalid, ErrInvalidHistoryQuery
	}
	at, err := time.Parse(historyTimestampLayout, string(raw[2:29]))
	if err != nil || at.IsZero() || at.Nanosecond()%1000 != 0 {
		return invalid, ErrInvalidHistoryQuery
	}
	id, err := domain.ParseCheckID(string(raw[30:66]))
	if err != nil || id == (domain.CheckID{}) {
		return invalid, ErrInvalidHistoryQuery
	}
	canonical, err := EncodeHistoryCursor(ports.HistoryAnchor{CompletedAt: at, CheckID: id})
	if err != nil || canonical != token {
		return invalid, ErrInvalidHistoryQuery
	}
	return ports.HistoryAnchor{CompletedAt: at, CheckID: id}, nil
}
