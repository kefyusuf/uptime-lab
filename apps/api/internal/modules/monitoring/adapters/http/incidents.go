package monitoringhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const incidentsBodyLimit = 245760

var incidentsLimitPattern = regexp.MustCompile(`^[1-9][0-9]?$`)

func parseIncidentsQuery(raw string) (application.IncidentInput, error) {
	input := application.IncidentInput{Limit: 20}
	if len(raw) > 128 {
		return input, application.ErrInvalidIncidentQuery
	}
	if raw == "" {
		return input, nil
	}
	for _, c := range []byte(raw) {
		if c > 127 {
			return input, application.ErrInvalidIncidentQuery
		}
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || seen[key] || value == "" {
			return input, application.ErrInvalidIncidentQuery
		}
		seen[key] = true
		switch key {
		case "limit":
			if !incidentsLimitPattern.MatchString(value) {
				return input, application.ErrInvalidIncidentQuery
			}
			limit, err := strconv.Atoi(value)
			if err != nil || limit > 50 {
				return input, application.ErrInvalidIncidentQuery
			}
			input.Limit = limit
		case "cursor":
			if _, err := application.DecodeIncidentCursor(value); err != nil {
				return input, application.ErrInvalidIncidentQuery
			}
			input.Cursor = value
		default:
			return input, application.ErrInvalidIncidentQuery
		}
	}
	return input, nil
}

type incidentResponse struct {
	IncidentID        string                `json:"incidentId"`
	OpenedAt          time.Time             `json:"openedAt"`
	ResolvedAt        *time.Time            `json:"resolvedAt,omitempty"`
	OpeningReason     domain.IncidentReason `json:"openingReason"`
	OpenedByCheckID   *string               `json:"openedByCheckId,omitempty"`
	ResolvedByCheckID *string               `json:"resolvedByCheckId,omitempty"`
}

func encodeIncidentPage(candidates []application.Incident, limit int) ([]byte, error) {
	if limit < 1 || limit > 50 || len(candidates) > limit+1 {
		return nil, application.ErrPersistence
	}
	items := make([]json.RawMessage, 0, limit)
	used := 128
	for _, candidate := range candidates {
		if len(items) == limit {
			break
		}
		openedBy, hasOpenedBy := candidate.OpenedByCheckID()
		resolvedBy, hasResolvedBy := candidate.ResolvedByCheckID()
		encoded, err := json.Marshal(incidentResponse{
			IncidentID:        candidate.IncidentID().String(),
			OpenedAt:          candidate.OpenedAt(),
			ResolvedAt:        incidentResolvedAtPointer(candidate),
			OpeningReason:     candidate.OpeningReason(),
			OpenedByCheckID:   incidentEvidencePointer(openedBy, hasOpenedBy),
			ResolvedByCheckID: incidentEvidencePointer(resolvedBy, hasResolvedBy),
		})
		if err != nil {
			return nil, application.ErrPersistence
		}
		cost := len(encoded)
		if len(items) > 0 {
			cost++
		}
		if used+cost > incidentsBodyLimit {
			if len(items) == 0 {
				return nil, application.ErrPersistence
			}
			break
		}
		items = append(items, encoded)
		used += cost
	}
	var next *string
	if len(items) < len(candidates) {
		last := candidates[len(items)-1]
		token, err := application.EncodeIncidentCursor(ports.IncidentAnchor{OpenedAt: last.OpenedAt(), IncidentID: last.IncidentID()})
		if err != nil {
			return nil, application.ErrPersistence
		}
		next = &token
	}
	payload, err := json.Marshal(struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor *string           `json:"nextCursor"`
	}{items, next})
	if err != nil || len(payload) > incidentsBodyLimit {
		return nil, application.ErrPersistence
	}
	return payload, nil
}

func incidentResolvedAtPointer(incident application.Incident) *time.Time {
	if resolvedAt, ok := incident.ResolvedAt(); ok {
		return &resolvedAt
	}
	return nil
}

func incidentEvidencePointer(id domain.CheckID, present bool) *string {
	if !present {
		return nil
	}
	value := id.String()
	return &value
}

func incidentsMonitorIDPathSegment(path string) (string, bool) {
	const (
		prefix = monitorsPath + "/"
		suffix = "/incidents"
	)
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	segment := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if segment == "" || strings.Contains(segment, "/") {
		return "", false
	}
	return segment, true
}

// serveIncidents answers the bounded durable-incident sub-resource with the
// inventory's query grammar and budget conventions.
func (handler *Handler) serveIncidents(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	path, query, hasQuery := strings.Cut(rawTarget, "?")
	if path != monitorsPath+"/"+rawID+"/incidents" || (hasQuery && query == "") || request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		writeProblem(writer, 400, "The monitor incident query is invalid.")
		return
	}
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, 400, "monitorId is not a valid UUID.")
		return
	}
	input, err := parseIncidentsQuery(query)
	if err != nil {
		writeProblem(writer, 400, "The monitor incident query is invalid.")
		return
	}
	page, err := handler.incidents.Execute(request.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidIncidentQuery):
			writeProblem(writer, 400, "The monitor incident query is invalid.")
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, 404, "The monitor was not found.")
		default:
			writeProblem(writer, 500, "The server could not complete the request.")
		}
		return
	}
	payload, err := encodeIncidentPage(page.Items, page.Limit)
	if err != nil {
		writeProblem(writer, 500, "The server could not complete the request.")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}
