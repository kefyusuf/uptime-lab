package monitoringhttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

const targetEditBodyLimit = 16384

var (
	errTargetEditMalformed = errors.New("target edit body malformed")
	errTargetEditContract  = errors.New("target edit body violates the monitor contract")
)

// decodeTargetEditDocument strictly decodes the closed edit body: exactly one
// "targetUrl" member with a string value. Duplicate keys, unknown keys,
// non-string values, non-object documents and trailing content are rejected.
// An empty string is passed through so the domain owns target validation.
func decodeTargetEditDocument(request *http.Request) (string, error) {
	decoder := json.NewDecoder(request.Body)
	open, err := decoder.Token()
	if err != nil {
		return "", errTargetEditMalformed
	}
	if open != json.Delim('{') {
		return "", errTargetEditContract
	}
	seen := false
	var target string
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return "", errTargetEditMalformed
		}
		key, ok := keyToken.(string)
		if !ok || key != "targetUrl" || seen {
			return "", errTargetEditContract
		}
		seen = true
		valueToken, err := decoder.Token()
		if err != nil {
			return "", errTargetEditMalformed
		}
		value, ok := valueToken.(string)
		if !ok {
			return "", errTargetEditContract
		}
		target = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return "", errTargetEditMalformed
	}
	if !seen {
		return "", errTargetEditContract
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", errTargetEditMalformed
	}
	return target, nil
}

// serveTargetEdit answers PATCH on the shared Monitor resource with the
// creation-mirrored body grammar, editing the target on the immutable
// identity through one use-case call.
func (handler *Handler) serveTargetEdit(writer http.ResponseWriter, request *http.Request, rawID string) {
	writer.Header().Set("Cache-Control", "no-store")
	rawTarget := request.RequestURI
	if rawTarget == "" {
		rawTarget = request.URL.RequestURI()
	}
	if rawTarget != monitorsPath+"/"+rawID || len(request.TransferEncoding) != 0 {
		writeProblem(writer, 400, "The monitor target edit request is invalid.")
		return
	}

	id, err := domain.ParseMonitorID(rawID)
	if err != nil {
		writeProblem(writer, 400, "monitorId is not a valid UUID.")
		return
	}

	if request.ContentLength > targetEditBodyLimit {
		writeProblem(writer, 400, "The monitor target edit request is invalid.")
		return
	}

	mediaType, _, _ := strings.Cut(request.Header.Get("Content-Type"), ";")
	mediaType = strings.TrimSpace(mediaType)
	if request.ContentLength != 0 && mediaType != "application/json" {
		writeProblem(writer, 415, "Content-Type must be application/json.")
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, targetEditBodyLimit)
	raw, err := decodeTargetEditDocument(request)
	if err != nil {
		if errors.Is(err, errTargetEditContract) {
			writeProblem(writer, 422, "The request content violates the monitor contract.")
			return
		}
		writeProblem(writer, 400, "The request content is malformed.")
		return
	}

	monitor, err := handler.targetEdit.Execute(request.Context(), id, raw)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidTargetURL):
			writeProblem(writer, 422, "The request content violates the monitor contract.")
		case errors.Is(err, application.ErrMonitorNotFound):
			writeProblem(writer, 404, "The monitor was not found.")
		default:
			writeProblem(writer, 500, "The server could not complete the request.")
		}
		return
	}
	writeMonitor(writer, http.StatusOK, monitor, "")
}
