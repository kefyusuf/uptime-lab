package checkerhttp

import (
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

const (
	claimPath       = "/internal/checks/claim"
	checkPathPrefix = "/internal/checks/"
	resultSuffix    = "/result"
)

type claimDueCheck interface {
	Execute(context.Context) (application.CheckWork, error)
}

type submitCheckResult interface {
	Execute(context.Context, string, application.SubmitCheckResultInput) error
}

// Handler adapts the internal Checker HTTP contract to Monitoring application use cases.
type Handler struct {
	claim  claimDueCheck
	submit submitCheckResult
}

// NewHandler constructs the isolated internal Checker HTTP adapter.
func NewHandler(claim claimDueCheck, submit submitCheckResult) *Handler {
	return &Handler{
		claim:  claim,
		submit: submit,
	}
}

// ServeHTTP recognizes only the two internal Checker contract resource shapes.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == claimPath {
		handler.serveClaim(writer, request)
		return
	}

	if rawCheckID, ok := resultCheckIDPathSegment(request.URL.Path); ok {
		handler.serveResult(writer, request, rawCheckID)
		return
	}

	http.NotFound(writer, request)
}

func (handler *Handler) serveClaim(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeMethodNotAllowed(writer, http.MethodPost)
		return
	}

	work, err := handler.claim.Execute(request.Context())
	if err != nil {
		if errors.Is(err, application.ErrNoDueCheck) {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}

	payload, err := json.Marshal(checkWorkResponse{
		CheckID:      work.CheckID.String(),
		MonitorID:    work.MonitorID.String(),
		TargetURL:    work.TargetURL.String(),
		TimeoutMS:    work.Timeout.Milliseconds(),
		MaxRedirects: work.MaxRedirects,
	})
	if err != nil {
		writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}

func (handler *Handler) serveResult(writer http.ResponseWriter, request *http.Request, rawCheckID string) {
	if request.Method != http.MethodPut {
		writeMethodNotAllowed(writer, http.MethodPut)
		return
	}

	if _, err := domain.ParseCheckID(rawCheckID); err != nil {
		writeProblem(writer, http.StatusBadRequest, "checkId is not a valid UUID.")
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

	input, err := parseSubmitCheckResultInput(value)
	if err != nil {
		writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the check result contract.")
		return
	}

	err = handler.submit.Execute(request.Context(), rawCheckID, input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCheckID):
			writeProblem(writer, http.StatusBadRequest, "checkId is not a valid UUID.")
		case errors.Is(err, application.ErrCheckRunNotFound):
			writeProblem(writer, http.StatusNotFound, "The check run was not found.")
		case errors.Is(err, application.ErrCheckRunConflict):
			writeProblem(writer, http.StatusConflict, "The check run cannot accept this result.")
		case errors.Is(err, domain.ErrInvalidCheckResult):
			writeProblem(writer, http.StatusUnprocessableEntity, "The request content violates the check result contract.")
		default:
			writeProblem(writer, http.StatusInternalServerError, "The server could not complete the request.")
		}
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func resultCheckIDPathSegment(path string) (string, bool) {
	if !strings.HasPrefix(path, checkPathPrefix) || !strings.HasSuffix(path, resultSuffix) {
		return "", false
	}

	segment := strings.TrimSuffix(strings.TrimPrefix(path, checkPathPrefix), resultSuffix)
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

func parseSubmitCheckResultInput(value any) (application.SubmitCheckResultInput, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
	}

	rawKind, ok := object["kind"].(string)
	if !ok {
		return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
	}
	durationMS, ok := integerField(object, "durationMs")
	if !ok {
		return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
	}

	kind := domain.CheckResultKind(rawKind)
	if kind == domain.CheckResultHTTPResponse {
		if len(object) != 3 {
			return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
		}
		httpStatus64, ok := integerField(object, "httpStatus")
		if !ok {
			return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
		}
		httpStatus := int(httpStatus64)
		if _, err := domain.NewHTTPResponseResult(durationMS, httpStatus); err != nil {
			return application.SubmitCheckResultInput{}, err
		}
		return application.SubmitCheckResultInput{
			Kind:       kind,
			DurationMS: durationMS,
			HTTPStatus: &httpStatus,
		}, nil
	}

	if len(object) != 2 {
		return application.SubmitCheckResultInput{}, domain.ErrInvalidCheckResult
	}
	if _, err := domain.NewCheckFailureResult(kind, durationMS); err != nil {
		return application.SubmitCheckResultInput{}, err
	}
	return application.SubmitCheckResultInput{
		Kind:       kind,
		DurationMS: durationMS,
	}, nil
}

func integerField(object map[string]any, name string) (int64, bool) {
	raw, exists := object[name]
	if !exists {
		return 0, false
	}
	number, ok := raw.(json.Number)
	if !ok {
		return 0, false
	}
	value, err := number.Int64()
	if err != nil {
		return 0, false
	}
	return value, true
}

type checkWorkResponse struct {
	CheckID      string `json:"checkId"`
	MonitorID    string `json:"monitorId"`
	TargetURL    string `json:"targetUrl"`
	TimeoutMS    int64  `json:"timeoutMs"`
	MaxRedirects int    `json:"maxRedirects"`
}

type problemResponse struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
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

func writeMethodNotAllowed(writer http.ResponseWriter, allow string) {
	writer.Header().Set("Allow", allow)
	writeProblem(writer, http.StatusMethodNotAllowed, "The method is not allowed for this resource.")
}
