package application

import (
	"encoding/base64"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const inventoryTimestampLayout = "2006-01-02T15:04:05.000000Z"

// EncodeInventoryCursor encodes a canonical persisted microsecond position.
func EncodeInventoryCursor(anchor ports.InventoryAnchor) (string, error) {
	at := anchor.CreatedAt.UTC()
	if at.IsZero() || at.Year() < 1 || at.Year() > 9999 || at.Nanosecond()%1000 != 0 {
		return "", ErrInvalidInventoryQuery
	}
	id, err := domain.ParseMonitorID(anchor.ID.String())
	if err != nil || id != anchor.ID {
		return "", ErrInvalidInventoryQuery
	}
	raw := "1|" + at.Format(inventoryTimestampLayout) + "|" + id.String()
	if len(raw) != 66 {
		return "", ErrInvalidInventoryQuery
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// DecodeInventoryCursor rejects alternate representations of the same position.
func DecodeInventoryCursor(token string) (ports.InventoryAnchor, error) {
	invalid := ports.InventoryAnchor{}
	if len(token) != 88 {
		return invalid, ErrInvalidInventoryQuery
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 66 || base64.RawURLEncoding.EncodeToString(raw) != token || string(raw[:2]) != "1|" || raw[29] != '|' {
		return invalid, ErrInvalidInventoryQuery
	}
	at, err := time.Parse(inventoryTimestampLayout, string(raw[2:29]))
	if err != nil {
		return invalid, ErrInvalidInventoryQuery
	}
	id, err := domain.ParseMonitorID(string(raw[30:]))
	if err != nil || id.String() != string(raw[30:]) {
		return invalid, ErrInvalidInventoryQuery
	}
	anchor := ports.InventoryAnchor{CreatedAt: at, ID: id}
	canonical, err := EncodeInventoryCursor(anchor)
	if err != nil || canonical != token {
		return invalid, ErrInvalidInventoryQuery
	}
	return anchor, nil
}
