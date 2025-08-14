package logging

import (
	"log/slog"
	"os"
)

func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})

	handlerWithRequestID := &RequestIDHandler{Handler: handler}

	logger := slog.New(handlerWithRequestID)

	return logger
}
