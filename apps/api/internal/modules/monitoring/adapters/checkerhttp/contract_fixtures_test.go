package checkerhttp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
)

func TestCrossRuntimeInternalFixtures(t *testing.T) {
	work := decodeFixtureObject(t, "check_work.json")
	assertExactKeys(t, work, "checkId", "monitorId", "targetUrl", "timeoutMs", "maxRedirects")
	if work["checkId"] != "018f22d3-1d6a-7cc0-a37b-46fc3fafd101" {
		t.Fatalf("checkId = %#v", work["checkId"])
	}
	if work["monitorId"] != "018f22d3-1d6a-7cc0-a37b-46fc3fafd001" {
		t.Fatalf("monitorId = %#v", work["monitorId"])
	}
	if work["targetUrl"] != "https://example.com/health?region=eu" {
		t.Fatalf("targetUrl = %#v", work["targetUrl"])
	}
	if work["timeoutMs"] != json.Number("10000") || work["maxRedirects"] != json.Number("3") {
		t.Fatalf("work policy = timeout %#v redirects %#v", work["timeoutMs"], work["maxRedirects"])
	}

	httpResult := decodeFixtureValue(t, "http_response_result.json")
	httpInput, err := parseSubmitCheckResultInput(httpResult)
	if err != nil {
		t.Fatalf("parse HTTP response fixture: %v", err)
	}
	if httpInput.Kind != domain.CheckResultHTTPResponse || httpInput.DurationMS != 125 {
		t.Fatalf("HTTP fixture input = %#v", httpInput)
	}
	if httpInput.HTTPStatus == nil || *httpInput.HTTPStatus != 204 {
		t.Fatalf("HTTP fixture status = %#v", httpInput.HTTPStatus)
	}

	failure := decodeFixtureValue(t, "failure_result.json")
	failureInput, err := parseSubmitCheckResultInput(failure)
	if err != nil {
		t.Fatalf("parse failure fixture: %v", err)
	}
	if failureInput.Kind != domain.CheckResultTimeout || failureInput.DurationMS != 10000 {
		t.Fatalf("failure fixture input = %#v", failureInput)
	}
	if failureInput.HTTPStatus != nil {
		t.Fatalf("failure fixture unexpectedly has status %#v", failureInput.HTTPStatus)
	}
}

func decodeFixtureObject(t *testing.T, name string) map[string]any {
	t.Helper()
	value := decodeFixtureValue(t, name)
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("fixture %s root = %T, want object", name, value)
	}
	return object
}

func decodeFixtureValue(t *testing.T, name string) any {
	t.Helper()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(
		filepath.Dir(source),
		"..", "..", "..", "..", "..", "..", "..",
	))
	path := filepath.Join(root, "contracts", "fixtures", "internal", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture %s: %v", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		t.Fatalf("fixture %s contains multiple JSON documents", name)
	}
	return value
}
