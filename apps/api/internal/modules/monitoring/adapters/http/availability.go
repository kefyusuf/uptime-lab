package monitoringhttp

import (
	"encoding/json"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"net/http"
	"strings"
	"time"
)

func availabilityMonitorIDPathSegment(path string) (string, bool) {
	const prefix = monitorsPath + "/"
	const suffix = "/availability"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	segment := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if segment == "" || strings.Contains(segment, "/") {
		return "", false
	}
	return segment, true
}

type availabilityResponse struct {
	Status      domain.AvailabilityStatus     `json:"status"`
	Reason      domain.AvailabilityReason     `json:"reason"`
	EvaluatedAt time.Time                     `json:"evaluatedAt"`
	Evidence    *availabilityEvidenceResponse `json:"evidence,omitempty"`
}
type availabilityEvidenceResponse struct {
	CheckID     string    `json:"checkId"`
	CompletedAt time.Time `json:"completedAt"`
}

func (handler *Handler) serveAvailability(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, http.MethodGet)
		return
	}
	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, http.StatusBadRequest, "monitorId is not a valid UUID.")
		return
	}
	assessment, err := handler.getAvailability.Execute(request.Context(), id)
	if err != nil {
		if errors.Is(err, application.ErrMonitorNotFound) {
			writeProblem(writer, http.StatusNotFound, "The monitor was not found.")
		} else {
			writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		}
		return
	}
	response := availabilityResponse{Status: assessment.Status(), Reason: assessment.Reason(), EvaluatedAt: assessment.EvaluatedAt()}
	if evidence, ok := assessment.Evidence(); ok {
		response.Evidence = &availabilityEvidenceResponse{CheckID: evidence.CheckID.String(), CompletedAt: evidence.CompletedAt}
	}
	payload, err := json.Marshal(response)
	if err != nil {
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}
