package observability

import (
	"io"
	"log/slog"
)

// NewLogger constructs the service JSON logger for one runtime component.
func NewLogger(writer io.Writer, level slog.Level, component string) *slog.Logger {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With(
		"service", "api",
		"component", component,
	)
}
