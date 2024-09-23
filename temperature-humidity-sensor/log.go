package main

import (
	"log/slog"
	"machine"
)

// The logger we'll use to, well, log stuff
var logger *slog.Logger

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
