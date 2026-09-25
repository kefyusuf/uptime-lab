package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

const monitorsPath = "/monitors"

type registerMonitor interface {
	Execute(context.Context, string) (domain.Monitor, error)
}

type getMonitor interface {
	Execute(context.Context, domain.MonitorID) (domain.Monitor, error)
}

// Handler adapts the public Monitoring HTTP contract to the existing application use cases.
type Handler struct {
	register registerMonitor
	get      getMonitor
}

// NewHandler constructs the isolated public Monitoring HTTP adapter.
func NewHandler(register registerMonitor, get getMonitor) *Handler {
	return &Handler{
		register: register,
		get:      get,
	}
}

// ServeHTTP recognizes only the two contracted Monitoring resource shapes.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == monitorsPath {
		handler.serveCollection(writer, request)
		return
	}

	if rawID, ok := monitorIDPathSegment(request.URL.Path); ok {
		handler.serveResource(writer, request, rawID)
		return
	}

	http.NotFound(writer, request)
}

func (handler *Handler) serveCollection(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, http.MethodPost)
		return
	}

	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		writeProblem(writer, http.StatusUnsupportedMediaType, "Content-Type must be application/json.")
		return
	}

	value, err := decodeSingleJSONDocument(request)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "The request content is malformed.")
		return
	}

	object, ok := value.(map[string]any)
	if !ok || len(object) != 1 {
		writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the monitor contract.")
		return
	}

	rawValue, exists := object["targetUrl"]
	if !exists {
		writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the monitor contract.")
		return
	}
	rawTarget, ok := rawValue.(string)
	if !ok {
		writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the monitor contract.")
		return
	}

	monitor, err := handler.register.Execute(request.Context(), rawTarget)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidTargetURL) {
			writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the monitor contract.")
			return
		}
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}

	writeMonitor(writer, http.StatusCreated, monitor, "/monitors/"+monitor.ID().String())
}

func (handler *Handler) serveResource(writer http.ResponseWriter, request *http.Request, rawID string) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "monitorId is not a valid UUID.")
		return
	}

	monitor, err := handler.get.Execute(request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, http.StatusNotFound, "The monitor was not found.")
		default:
			writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		}
		return
	}

	writeMonitor(writer, http.StatusOK, monitor, "")
}

func monitorIDPathSegment(path string) (string, bool) {
	const prefix = monitorsPath + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	segment := strings.TrimPrefix(path, prefix)
	if segment == "" || strings.Contains(segment, "/") {
		return "", false
	}
	return segment, true
}

func decodeSingleJSONDocument(request *http.Request) (any, error) {
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple JSON documents")
		}
		return nil, err
	}

	return value, nil
}

type monitorResponse struct {
	ID        string    `json:"id"`
	TargetURL string    `json:"targetUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

type problemResponse struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func writeMonitor(writer http.ResponseWriter, status int, monitor domain.Monitor, location string) {
	payload, err := json.Marshal(monitorResponse{
		ID:        monitor.ID().String(),
		TargetURL: monitor.TargetURL().String(),
		CreatedAt: monitor.CreatedAt(),
	})
	if err != nil {
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	if location != "" {
		writer.Header().Set("Location", location)
	}
	writer.WriteHeader(status)
	_, _ = writer.Write(payload)
}

func writeProblem(writer http.ResponseWriter, status int, detail string) {
	payload, err := json.Marshal(problemResponse{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
	})
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(status)
	_, _ = writer.Write(payload)
}

func methodNotAllowed(writer http.ResponseWriter, allow string) {
	writer.Header().Set("Allow", allow)
	writer.WriteHeader(http.StatusMethodNotAllowed)
}
