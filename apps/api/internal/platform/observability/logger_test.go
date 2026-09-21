package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLoggerEmitsStructuredBaselineFields(t *testing.T) {
	var output bytes.Buffer

	logger := NewLogger(&output, slog.LevelInfo, "runtime")
	logger.Info("runtime started", "event", "startup")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("logger output is not valid JSON: %v; output=%q", err, output.String())
	}

	if got := record["service"]; got != "api" {
		t.Fatalf("service = %#v, want api", got)
	}
	if got := record["component"]; got != "runtime" {
		t.Fatalf("component = %#v, want runtime", got)
	}
	if got := record["event"]; got != "startup" {
		t.Fatalf("event = %#v, want startup", got)
	}
	if got := record["msg"]; got != "runtime started" {
		t.Fatalf("msg = %#v, want runtime started", got)
	}
	if got := record["level"]; got != "INFO" {
		t.Fatalf("level = %#v, want INFO", got)
	}
}

func TestLoggerDoesNotEmitAmbientDatabaseSecrets(t *testing.T) {
	const secret = "logger-must-not-see-this"

	t.Setenv("PGPASSWORD", secret)

	var output bytes.Buffer
	logger := NewLogger(&output, slog.LevelInfo, "runtime")
	logger.Info("runtime started", "event", "startup", "http_addr", ":8080")

	if strings.Contains(output.String(), secret) {
		t.Fatalf("logger output leaked ambient PGPASSWORD: %q", output.String())
	}
}
