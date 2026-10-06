package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
)

const inventoryBodyLimit = 245760

var errInventoryItemTooLarge = errors.New("inventory item exceeds response limit")
var inventoryLimitPattern = regexp.MustCompile(`^[1-9][0-9]?$`)

type listMonitors interface {
	Execute(context.Context, application.InventoryInput) (application.InventoryCandidates, error)
}

func parseInventoryQuery(raw string) (application.InventoryInput, error) {
	input := application.InventoryInput{Limit: 20}
	if len(raw) > 128 {
		return input, application.ErrInvalidInventoryQuery
	}
	if raw == "" {
		return input, nil
	}
	for _, c := range []byte(raw) {
		if c > 127 {
			return input, application.ErrInvalidInventoryQuery
		}
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || seen[key] || value == "" {
			return input, application.ErrInvalidInventoryQuery
		}
		seen[key] = true
		switch key {
		case "limit":
			if !inventoryLimitPattern.MatchString(value) {
				return input, application.ErrInvalidInventoryQuery
			}
			limit, err := strconv.Atoi(value)
			if err != nil || limit > 50 {
				return input, application.ErrInvalidInventoryQuery
			}
			input.Limit = limit
		case "cursor":
			if _, err := application.DecodeInventoryCursor(value); err != nil {
				return input, application.ErrInvalidInventoryQuery
			}
			input.Cursor = value
		default:
			return input, application.ErrInvalidInventoryQuery
		}
	}
	return input, nil
}

func encodeInventoryPage(candidates []ports.InventoryCandidate, limit int) ([]byte, error) {
	if limit < 1 || limit > 50 || len(candidates) > limit+1 {
		return nil, application.ErrPersistence
	}
	items := make([]json.RawMessage, 0, limit)
	used := 128
	for _, candidate := range candidates {
		if len(items) == limit {
			break
		}
		if candidate.Oversized {
			if len(items) == 0 {
				return nil, errInventoryItemTooLarge
			}
			break
		}
		encoded, err := json.Marshal(monitorResponse{ID: candidate.ID.String(), TargetURL: candidate.TargetURL, CreatedAt: candidate.CreatedAt})
		if err != nil {
			return nil, application.ErrPersistence
		}
		cost := len(encoded)
		if len(items) > 0 {
			cost++
		}
		if used+cost > inventoryBodyLimit {
			if len(items) == 0 {
				return nil, errInventoryItemTooLarge
			}
			break
		}
		items = append(items, encoded)
		used += cost
	}
	var next *string
	if len(items) < len(candidates) {
		last := candidates[len(items)-1]
		token, err := application.EncodeInventoryCursor(ports.InventoryAnchor{ID: last.ID, CreatedAt: last.CreatedAt})
		if err != nil {
			return nil, application.ErrPersistence
		}
		next = &token
	}
	payload, err := json.Marshal(struct {
		Items      []json.RawMessage `json:"items"`
		NextCursor *string           `json:"nextCursor"`
	}{items, next})
	if err != nil || len(payload) > inventoryBodyLimit {
		return nil, application.ErrPersistence
	}
	return payload, nil
}

func (handler *Handler) serveInventory(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	path, query, hasQuery := strings.Cut(rawTarget, "?")
	if path != monitorsPath || (hasQuery && query == "") || request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		writeProblem(writer, 400, "The monitor inventory query is invalid.")
		return
	}
	input, err := parseInventoryQuery(query)
	if err != nil {
		writeProblem(writer, 400, "The monitor inventory query is invalid.")
		return
	}
	page, err := handler.list.Execute(request.Context(), input)
	if err != nil {
		if errors.Is(err, application.ErrInvalidInventoryQuery) {
			writeProblem(writer, 400, "The monitor inventory query is invalid.")
		} else {
			writeProblem(writer, 500, "The server could not complete the request.")
		}
		return
	}
	payload, err := encodeInventoryPage(page.Items, page.Limit)
	if err != nil {
		detail := "The server could not complete the request."
		if errors.Is(err, errInventoryItemTooLarge) {
			detail = "A registered monitor exceeds the inventory response limit."
		}
		writeProblem(writer, 500, detail)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}
