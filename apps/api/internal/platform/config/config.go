package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPAddr = ":8080"
	defaultLogLevel = "info"
)

// Config contains application runtime settings only.
type Config struct {
	HTTPAddr string
	LogLevel slog.Level
}

// Load reads and validates application runtime settings.
func Load() (Config, error) {
	httpAddr := envOrDefault("UPTIME_LAB_HTTP_ADDR", defaultHTTPAddr)
	if err := validateHTTPAddr(httpAddr); err != nil {
		return Config{}, err
	}

	rawLogLevel := envOrDefault("UPTIME_LAB_LOG_LEVEL", defaultLogLevel)
	logLevel, err := parseLogLevel(rawLogLevel)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr: httpAddr,
		LogLevel: logLevel,
	}, nil
}

// String renders only non-secret application settings.
func (config Config) String() string {
	return fmt.Sprintf("http_addr=%s log_level=%s", config.HTTPAddr, config.LogLevel)
}

func envOrDefault(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return value
}

func validateHTTPAddr(value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("invalid UPTIME_LAB_HTTP_ADDR")
	}

	_, rawPort, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("invalid UPTIME_LAB_HTTP_ADDR")
	}

	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid UPTIME_LAB_HTTP_ADDR")
	}

	return nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid UPTIME_LAB_LOG_LEVEL")
	}
}
