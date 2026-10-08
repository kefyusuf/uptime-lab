package monitoringhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

type getMonitorScheduling interface {
	Execute(context.Context, domain.MonitorID) (domain.SchedulingState, error)
}
type setMonitorScheduling interface {
	Execute(context.Context, domain.MonitorID, domain.SchedulingState) (domain.SchedulingState, error)
}

// NewHandlerWithScheduling adds explicit durable scheduling reads and writes.
func NewHandlerWithScheduling(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling) *Handler {
	handler := NewHandlerWithInventory(register, get, latest, availability, list)
	handler.getScheduling = schedulingGet
	handler.setScheduling = schedulingSet
	return handler
}

func schedulingMonitorIDPathSegment(path string) (string, bool) {
	const prefix = monitorsPath + "/"
	const suffix = "/scheduling"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return id, id != "" && !strings.Contains(id, "/")
}

type schedulingResponse struct {
	State domain.SchedulingState `json:"state"`
}

func (handler *Handler) serveScheduling(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	if rawTarget != monitorsPath+"/"+rawID+"/scheduling" || request.URL.Fragment != "" {
		writeProblem(writer, 400, "Invalid scheduling path.")
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPut {
		methodNotAllowed(writer, "GET, PUT")
		return
	}
	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, 400, "monitorId is not a valid UUID.")
		return
	}
	var state domain.SchedulingState
	if request.Method == http.MethodGet && (request.ContentLength > 0 || len(request.TransferEncoding) > 0) {
		writeProblem(writer, 400, "GET scheduling requests must not contain a body.")
		return
	}
	if request.Method == http.MethodPut {
		if request.ContentLength > 1024 {
			writeProblem(writer, 413, "Scheduling requests must not exceed 1024 bytes.")
			return
		}
		media, parameters, mediaErr := mime.ParseMediaType(request.Header.Get("Content-Type"))
		validMedia := mediaErr == nil && strings.EqualFold(media, "application/json") && request.Header.Get("Content-Encoding") == ""
		for key, value := range parameters {
			if key != "charset" || !strings.EqualFold(value, "utf-8") {
				validMedia = false
			}
		}
		if !validMedia {
			writeProblem(writer, 415, "Content-Type must be application/json with optional UTF-8 charset and no content encoding.")
			return
		}
		body, readErr := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1024))
		if readErr != nil {
			var overflow *http.MaxBytesError
			if errors.As(readErr, &overflow) {
				writeProblem(writer, 413, "Scheduling requests must not exceed 1024 bytes.")
			} else {
				writeProblem(writer, 400, "Invalid scheduling request.")
			}
			return
		}
		desired, decodeErr := decodeSchedulingBody(bytes.NewReader(body))
		if decodeErr != nil {
			writeProblem(writer, 400, "Invalid scheduling request.")
			return
		}
		state, err = handler.setScheduling.Execute(request.Context(), id, desired)
	} else {
		state, err = handler.getScheduling.Execute(request.Context(), id)
	}
	if err != nil {
		switch {
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, 404, "The monitor was not found.")
		case errors.Is(err, domain.ErrInvalidSchedulingState):
			writeProblem(writer, 400, "Invalid scheduling request.")
		default:
			writeProblem(writer, 500, "The server could not complete the request.")
		}
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(200)
	_ = json.NewEncoder(writer).Encode(schedulingResponse{State: state})
}

func decodeSchedulingBody(reader io.Reader) (domain.SchedulingState, error) {
	decoder := json.NewDecoder(reader)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return "", domain.ErrInvalidSchedulingState
	}
	var state domain.SchedulingState
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil || key != "state" || seen {
			return "", domain.ErrInvalidSchedulingState
		}
		seen = true
		value, err := decoder.Token()
		text, ok := value.(string)
		if err != nil || !ok {
			return "", domain.ErrInvalidSchedulingState
		}
		state, err = domain.ParseSchedulingState(text)
		if err != nil {
			return "", err
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || !seen {
		return "", domain.ErrInvalidSchedulingState
	}
	if _, err := decoder.Token(); err != io.EOF {
		return "", domain.ErrInvalidSchedulingState
	}
	return state, nil
}
