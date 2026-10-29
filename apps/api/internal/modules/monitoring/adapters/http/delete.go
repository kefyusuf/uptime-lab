package monitoringhttp

import (
	"errors"
	"net/http"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

// serveDeletion answers the shared Monitor resource once the irreversible
// removal capability is composed: raw-path canonicality precedes the method
// check, the UUID precedes body framing, and exactly one use case runs.
func (handler *Handler) serveDeletion(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	if rawTarget != monitorsPath+"/"+rawID || request.URL.Fragment != "" {
		writeProblem(writer, http.StatusBadRequest, "Invalid monitor request.")
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodDelete {
		methodNotAllowed(writer, "GET, DELETE")
		return
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "monitorId is not a valid UUID.")
		return
	}
	if request.Method != http.MethodDelete {
		handler.serveResourceRead(writer, request, id)
		return
	}
	if request.ContentLength > 0 || len(request.TransferEncoding) > 0 {
		writeProblem(writer, http.StatusBadRequest, "Delete requests must not contain a body.")
		return
	}

	if err := handler.deletion.Execute(request.Context(), id); err != nil {
		switch {
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, http.StatusNotFound, "The monitor was not found.")
		default:
			writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		}
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) serveResourceRead(writer http.ResponseWriter, request *http.Request, id domain.MonitorID) {
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
