// slog_basic verifies that log/slog.* functions are correctly intercepted.
//
// All log calls are noops for violation analysis. The test confirms they
// return without crashing so code that follows them executes normally.
//
// Expected: 0 violations.
package main

import (
	"log/slog"
	"os"
)

func main() {
	// Package-level logging — all noops.
	slog.Info("hello", "key", "value")
	slog.Debug("debug message")
	slog.Warn("warning")
	slog.Error("error")

	// Structured handler construction.
	handler := slog.NewTextHandler(os.Stdout, nil)
	logger := slog.New(handler)

	// Logger method calls.
	logger.Info("structured log", "count", 42)
	logger.With("req_id", "abc123").Info("with attrs")
}
