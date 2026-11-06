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

const historyBodyLimit = 245760

var historyLimitPattern = regexp.MustCompile(`^[1-9][0-9]?$`)

func parseHistoryQuery(raw string) (application.HistoryInput, error) {
	input := application.HistoryInput{Limit: 20}
	if len(raw) > 128 {
		return input, application.ErrInvalidHistoryQuery
	}
	if raw == "" {
		return input, nil
	}
	for _, c := range []byte(raw) {
		if c > 127 {
			return input, application.ErrInvalidHistoryQuery
		}
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || seen[key] || value == "" {
			return input, application.ErrInvalidHistoryQuery
		}
		seen[key] = true
		switch key {
		case "limit":
			if !historyLimitPattern.MatchString(value) {
				return input, application.ErrInvalidHistoryQuery
			}
			limit, err := strconv.Atoi(value)
			if err != nil || limit > 50 {
				return input, application.ErrInvalidHistoryQuery
			}
			input.Limit = limit
		case "cursor":
			if _, err := application.DecodeHistoryCursor(value); err != nil {
				return input, application.ErrInvalidHistoryQuery
			}
			input.Cursor = value
		default:
			return input, application.ErrInvalidHistoryQuery
		}
	}
	return input, nil
}

type historyEntryResponse struct {
	CheckID     string                 `json:"checkId"`
	ResultKind  domain.CheckResultKind `json:"resultKind"`
	HTTPStatus  *int                   `json:"httpStatus,omitempty"`
	DurationMS  *int64                 `json:"durationMs,omitempty"`
	CompletedAt time.Time              `json:"completedAt"`
}

func encodeHistoryPage(candidates []application.LatestCheckResult, limit int) ([]byte, error) {
	if limit < 1 || limit > 50 || len(candidates) > limit+1 {
		return nil, application.ErrPersistence
	}
	items := make([]json.RawMessage, 0, limit)
	used := 128
	for _, candidate := range candidates {
		if len(items) == limit {
			break
		}
		encoded, err := json.Marshal(historyEntryResponse{
			CheckID:     candidate.CheckID().String(),
			ResultKind:  candidate.ResultKind(),
			HTTPStatus:  httpStatusPointer(candidate),
			DurationMS:  durationPointer(candidate),
			CompletedAt: candidate.CompletedAt(),
		})
		if err != nil {
			return nil, application.ErrPersistence
		}
		cost := len(encoded)
		if len(items) > 0 {
			cost++
		}
		if used+cost > historyBodyLimit {
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
		token, err := application.EncodeHistoryCursor(ports.HistoryAnchor{CompletedAt: last.CompletedAt(), CheckID: last.CheckID()})
		if err != nil {
			return nil, application.ErrPersistence
		}
		next = &token
	}
	payload, err := json.Marshal(struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor *string           `json:"nextCursor"`
	}{items, next})
	if err != nil || len(payload) > historyBodyLimit {
		return nil, application.ErrPersistence
	}
	return payload, nil
}

func httpStatusPointer(candidate application.LatestCheckResult) *int {
	if status, ok := candidate.HTTPStatus(); ok {
		return &status
	}
	return nil
}

func durationPointer(candidate application.LatestCheckResult) *int64 {
	if duration, ok := candidate.DurationMS(); ok {
		return &duration
	}
	return nil
}

func historyMonitorIDPathSegment(path string) (string, bool) {
	const (
		prefix = monitorsPath + "/"
		suffix = "/history"
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

// serveHistory answers the bounded terminal-history sub-resource with the
// inventory's query grammar and budget conventions.
func (handler *Handler) serveHistory(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	path, query, hasQuery := strings.Cut(rawTarget, "?")
	if path != monitorsPath+"/"+rawID+"/history" || (hasQuery && query == "") || request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		writeProblem(writer, 400, "The monitor history query is invalid.")
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
	input, err := parseHistoryQuery(query)
	if err != nil {
		writeProblem(writer, 400, "The monitor history query is invalid.")
		return
	}
	page, err := handler.history.Execute(request.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidHistoryQuery):
			writeProblem(writer, 400, "The monitor history query is invalid.")
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, 404, "The monitor was not found.")
		default:
			writeProblem(writer, 500, "The server could not complete the request.")
		}
		return
	}
	payload, err := encodeHistoryPage(page.Items, page.Limit)
	if err != nil {
		writeProblem(writer, 500, "The server could not complete the request.")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}
