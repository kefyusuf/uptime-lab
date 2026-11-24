package application

import (
	"encoding/base64"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const incidentTimestampLayout = "2006-01-02T15:04:05.000000Z"

// EncodeIncidentCursor encodes a canonical persisted microsecond position.
func EncodeIncidentCursor(anchor ports.IncidentAnchor) (string, error) {
	at := anchor.OpenedAt.UTC()
	if at.IsZero() || at.Year() < 1 || at.Year() > 9999 || at.Nanosecond()%1000 != 0 {
		return "", ErrInvalidIncidentQuery
	}
	id, err := domain.ParseIncidentID(anchor.IncidentID.String())
	if err != nil || id != anchor.IncidentID {
		return "", ErrInvalidIncidentQuery
	}
	raw := "3|" + at.Format(incidentTimestampLayout) + "|" + id.String()
	if len(raw) != 66 {
		return "", ErrInvalidIncidentQuery
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

// DecodeIncidentCursor rejects alternate representations of the same position.
func DecodeIncidentCursor(token string) (ports.IncidentAnchor, error) {
	invalid := ports.IncidentAnchor{}
	if len(token) != 88 {
		return invalid, ErrInvalidIncidentQuery
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 66 || base64.RawURLEncoding.EncodeToString(raw) != token || string(raw[:2]) != "3|" || raw[29] != '|' {
		return invalid, ErrInvalidIncidentQuery
	}
	at, err := time.Parse(incidentTimestampLayout, string(raw[2:29]))
	if err != nil || at.IsZero() || at.Nanosecond()%1000 != 0 {
		return invalid, ErrInvalidIncidentQuery
	}
	id, err := domain.ParseIncidentID(string(raw[30:66]))
	if err != nil || id == (domain.IncidentID{}) {
		return invalid, ErrInvalidIncidentQuery
	}
	canonical, err := EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: at, IncidentID: id})
	if err != nil || canonical != token {
		return invalid, ErrInvalidIncidentQuery
	}
	return ports.IncidentAnchor{OpenedAt: at, IncidentID: id}, nil
}
