package main

import (
	"log/slog"
	"machine"
)

// logLevel is the log level we'll use.
const logLevel = slog.LevelInfo

// createLogger creates our logger.
func createLogger() *slog.Logger {
	logger := slog.New(slog.NewTextHandler(
		machine.Serial,
		&slog.HandlerOptions{Level: logLevel},
	))

	return logger
}
