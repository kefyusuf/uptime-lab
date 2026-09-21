package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestLoadUsesApplicationDefaults(t *testing.T) {
	unsetEnv(t, "UPTIME_LAB_HTTP_ADDR", "UPTIME_LAB_LOG_LEVEL")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", got.HTTPAddr)
	}
	if got.LogLevel != slog.LevelInfo {
		t.Fatalf("LogLevel = %v, want %v", got.LogLevel, slog.LevelInfo)
	}
}

func TestLoadAcceptsSupportedLogLevels(t *testing.T) {
	unsetEnv(t, "UPTIME_LAB_HTTP_ADDR")
	t.Setenv("UPTIME_LAB_HTTP_ADDR", ":8080")

	tests := []struct {
		raw  string
		want slog.Level
	}{
		{raw: "debug", want: slog.LevelDebug},
		{raw: "info", want: slog.LevelInfo},
		{raw: "warn", want: slog.LevelWarn},
		{raw: "error", want: slog.LevelError},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			t.Setenv("UPTIME_LAB_LOG_LEVEL", test.raw)
			got, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.LogLevel != test.want {
				t.Fatalf("LogLevel = %v, want %v", got.LogLevel, test.want)
			}
		})
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	t.Setenv("UPTIME_LAB_HTTP_ADDR", ":8080")
	t.Setenv("UPTIME_LAB_LOG_LEVEL", "verbose")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid log level error")
	}
}

func TestLoadRejectsEmptyHTTPAddress(t *testing.T) {
	t.Setenv("UPTIME_LAB_HTTP_ADDR", "")
	t.Setenv("UPTIME_LAB_LOG_LEVEL", "info")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want empty HTTP address error")
	}
}

func TestLoadRejectsInvalidHTTPAddresses(t *testing.T) {
	t.Setenv("UPTIME_LAB_LOG_LEVEL", "info")

	for _, value := range []string{
		"localhost",
		"localhost:not-a-port",
		":0",
		":70000",
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("UPTIME_LAB_HTTP_ADDR", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil for %q, want invalid HTTP address error", value)
			}
		})
	}
}

func TestConfigRenderingDoesNotExposePostgresSecrets(t *testing.T) {
	const secret = "task6-super-secret-password"

	t.Setenv("UPTIME_LAB_HTTP_ADDR", ":8080")
	t.Setenv("UPTIME_LAB_LOG_LEVEL", "info")
	t.Setenv("PGPASSWORD", secret)
	t.Setenv("PGHOST", "db.internal.example")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rendered := fmt.Sprintf("%+v", got)
	if strings.Contains(rendered, secret) {
		t.Fatalf("rendered config leaked PGPASSWORD: %q", rendered)
	}
}

func unsetEnv(t *testing.T, names ...string) {
	t.Helper()

	for _, name := range names {
		name := name
		value, ok := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("os.Unsetenv(%q) error = %v", name, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(name, value)
				return
			}
			_ = os.Unsetenv(name)
		})
	}
}
