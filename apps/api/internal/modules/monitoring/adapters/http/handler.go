package monitoringhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
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

type getLatestCheckResult interface {
	Execute(context.Context, domain.MonitorID) (application.LatestCheckResult, error)
}

type getMonitorAvailability interface {
	Execute(context.Context, domain.MonitorID) (application.MonitorAvailability, error)
}

type deleteMonitor interface {
	Execute(context.Context, domain.MonitorID) error
}

type getMonitorHistory interface {
	Execute(context.Context, domain.MonitorID, application.HistoryInput) (application.HistoryCandidates, error)
}

type editMonitorTarget interface {
	Execute(context.Context, domain.MonitorID, string) (domain.Monitor, error)
}

// Handler adapts the public Monitoring HTTP contract to the existing application use cases.
type Handler struct {
	register        registerMonitor
	get             getMonitor
	getLatest       getLatestCheckResult
	getAvailability getMonitorAvailability
	list            listMonitors
	getScheduling   getMonitorScheduling
	setScheduling   setMonitorScheduling
	deletion        deleteMonitor
	history         getMonitorHistory
	targetEdit      editMonitorTarget
}

// NewHandler constructs the isolated public Monitoring HTTP adapter.
func NewHandler(register registerMonitor, get getMonitor) *Handler {
	return &Handler{
		register: register,
		get:      get,
	}
}

// NewHandlerWithLatestResult adds the isolated latest-terminal-result capability.
func NewHandlerWithLatestResult(
	register registerMonitor,
	get getMonitor,
	getLatest getLatestCheckResult,
) *Handler {
	handler := NewHandler(register, get)
	handler.getLatest = getLatest
	return handler
}

// NewHandlerWithAvailability adds the read-only product assessment capability.
func NewHandlerWithAvailability(register registerMonitor, get getMonitor, getLatest getLatestCheckResult, getAvailability getMonitorAvailability) *Handler {
	handler := NewHandlerWithLatestResult(register, get, getLatest)
	handler.getAvailability = getAvailability
	return handler
}

// ServeHTTP recognizes only configured contracted Monitoring resource shapes.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Recognize collection aliases only to reject their original raw target.
	if handler.list != nil && request.Method == http.MethodGet && path.Clean(request.URL.Path) == monitorsPath {
		handler.serveInventory(writer, request)
		return
	}
	if request.URL.Path == monitorsPath {
		handler.serveCollection(writer, request)
		return
	}

	if rawID, ok := availabilityMonitorIDPathSegment(request.URL.Path); ok && handler.getAvailability != nil {
		handler.serveAvailability(writer, request, rawID)
		return
	}
	if rawID, ok := schedulingMonitorIDPathSegment(path.Clean(strings.ReplaceAll(request.URL.Path, "\\", "/"))); ok && handler.getScheduling != nil && handler.setScheduling != nil {
		handler.serveScheduling(writer, request, rawID)
		return
	}

	if rawID, ok := latestResultMonitorIDPathSegment(request.URL.Path); ok && handler.getLatest != nil {
		handler.serveLatestResult(writer, request, rawID)
		return
	}
	if rawID, ok := historyMonitorIDPathSegment(request.URL.Path); ok && handler.history != nil {
		handler.serveHistory(writer, request, rawID)
		return
	}

	if rawID, ok := monitorIDPathSegment(request.URL.Path); ok {
		if handler.deletion != nil || handler.targetEdit != nil {
			writer.Header().Set("Cache-Control", "no-store")
			if !canonicalMonitorResourceTarget(request, rawID) {
				writeProblem(writer, 400, "Invalid monitor request.")
				return
			}
			switch {
			case request.Method == http.MethodPatch && handler.targetEdit != nil:
				handler.serveTargetEdit(writer, request, rawID)
				return
			case request.Method == http.MethodDelete && handler.deletion != nil:
				handler.serveDeletion(writer, request, rawID)
				return
			case request.Method == http.MethodGet:
				handler.serveResource(writer, request, rawID)
				return
			default:
				allow := "GET"
				if handler.deletion != nil && handler.targetEdit != nil {
					allow = "GET, PATCH, DELETE"
				} else if handler.deletion != nil {
					allow = "GET, DELETE"
				} else if handler.targetEdit != nil {
					allow = "GET, PATCH"
				}
				methodNotAllowed(writer, allow)
				return
			}
		}
		handler.serveResource(writer, request, rawID)
		return
	}
	// Recognize resource aliases only to reject their original raw target.
	if handler.deletion != nil {
		if rawID, ok := monitorIDPathSegment(path.Clean(strings.ReplaceAll(request.URL.Path, "\\", "/"))); ok {
			handler.serveDeletion(writer, request, rawID)
			return
		}
	}
	// Recognize history aliases only to reject their original raw target.
	if handler.history != nil {
		if rawID, ok := historyMonitorIDPathSegment(path.Clean(strings.ReplaceAll(request.URL.Path, "\\", "/"))); ok {
			handler.serveHistory(writer, request, rawID)
			return
		}
	}

	http.NotFound(writer, request)
}

// canonicalMonitorResourceTarget reports whether the raw request target is the
// canonical form of the shared Monitor resource: no query (including an empty
// trailing question mark), no fragment and no alternate spelling.
func canonicalMonitorResourceTarget(request *http.Request, rawID string) bool {
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	return rawTarget == monitorsPath+"/"+rawID
}

// NewHandlerWithTargetEdit adds the target edit capability to the shared
// Monitor resource.
func NewHandlerWithTargetEdit(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling, deletion deleteMonitor, history getMonitorHistory, targetEdit editMonitorTarget) *Handler {
	handler := NewHandlerWithHistory(register, get, latest, availability, list, schedulingGet, schedulingSet, deletion, history)
	handler.targetEdit = targetEdit
	return handler
}

// NewHandlerWithDeletion adds the irreversible local removal capability.
func NewHandlerWithDeletion(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling, deletion deleteMonitor) *Handler {
	handler := NewHandlerWithScheduling(register, get, latest, availability, list, schedulingGet, schedulingSet)
	handler.deletion = deletion
	return handler
}

// NewHandlerWithHistory adds the bounded terminal-history read capability.
func NewHandlerWithHistory(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling, deletion deleteMonitor, history getMonitorHistory) *Handler {
	handler := NewHandlerWithDeletion(register, get, latest, availability, list, schedulingGet, schedulingSet, deletion)
	handler.history = history
	return handler
}

// NewHandlerWithInventory adds the bounded local collection read capability.
func NewHandlerWithInventory(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors) *Handler {
	handler := NewHandlerWithAvailability(register, get, latest, availability)
	handler.list = list
	return handler
}

func (handler *Handler) serveCollection(writer http.ResponseWriter, request *http.Request) {
	if handler.list != nil && request.Method == http.MethodGet {
		handler.serveInventory(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		if handler.list != nil {
			writer.Header().Set("Cache-Control", "no-store")
			methodNotAllowed(writer, "GET, POST")
			return
		}
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

func (handler *Handler) serveLatestResult(
	writer http.ResponseWriter,
	request *http.Request,
	rawID string,
) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "monitorId is not a valid UUID.")
		return
	}

	result, err := handler.getLatest.Execute(request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrNoTerminalCheckResult):
			writer.WriteHeader(http.StatusNoContent)
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, http.StatusNotFound, "The monitor was not found.")
		default:
			writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		}
		return
	}

	writeLatestCheckResult(writer, result)
}

func latestResultMonitorIDPathSegment(path string) (string, bool) {
	const (
		prefix = monitorsPath + "/"
		suffix = "/latest-result"
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

type latestCheckResultResponse struct {
	CheckID     string                 `json:"checkId"`
	ResultKind  domain.CheckResultKind `json:"resultKind"`
	HTTPStatus  *int                   `json:"httpStatus,omitempty"`
	DurationMS  *int64                 `json:"durationMs,omitempty"`
	CompletedAt time.Time              `json:"completedAt"`
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

func writeLatestCheckResult(writer http.ResponseWriter, result application.LatestCheckResult) {
	var httpStatus *int
	if value, ok := result.HTTPStatus(); ok {
		httpStatus = &value
	}

	var durationMS *int64
	if value, ok := result.DurationMS(); ok {
		durationMS = &value
	}

	payload, err := json.Marshal(latestCheckResultResponse{
		CheckID:     result.CheckID().String(),
		ResultKind:  result.ResultKind(),
		HTTPStatus:  httpStatus,
		DurationMS:  durationMS,
		CompletedAt: result.CompletedAt(),
	})
	if err != nil {
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
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
