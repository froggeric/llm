// Package logging configures the structured logger (log/slog) used by every
// component of localvision.
//
// Two knobs are surfaced via the CLI:
//
//   - --verbose: switches to debug-level logging (default is info)
//   - --log-file /path/to/log.jsonl: writes structured JSON logs to a file
//     in addition to stderr (useful for filing bug reports)
//
// All log output is structured (key=value or JSON). Stderr uses the
// human-readable text handler so console users get readable output; file
// output uses the JSON handler so log aggregators and `jq` can parse it.
//
// Per F3.5: the logger is plumbed through every component (lifecycle,
// executor, MCP server). Subsystems take a *slog.Logger via their
// constructor; passing nil falls back to slog.Default().
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// ErrInvalidLevel is returned by Setup when the supplied level string is
// not one of: debug, info, warn, error.
var ErrInvalidLevel = errors.New("invalid log level")

// ValidLevels is the set of level strings accepted by ParseLevel.
var ValidLevels = []string{"debug", "info", "warn", "error"}

// ParseLevel converts a case-insensitive level name ("debug", "info",
// "warn", "error") to the corresponding slog.Level. The empty string
// resolves to LevelInfo (the safe default).
//
// Returns ErrInvalidLevel (wrapped) for unknown names.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug", "verbose": // "verbose" is the --verbose flag alias
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q (expected one of %s)",
			ErrInvalidLevel, s, strings.Join(ValidLevels, ", "))
	}
}

// Setup constructs the configured *slog.Logger and installs it as the
// package-level default via slog.SetDefault (so library code that calls
// slog.Info directly also picks up the configuration).
//
// Parameters:
//
//   - level: one of "debug", "info", "warn", "error" (case-insensitive).
//     Empty string falls back to "info". The special value "verbose" is
//     accepted as an alias for "debug" so the --verbose flag can be passed
//     through unchanged.
//   - logFile: optional path. If non-empty, logs are tee'd: structured
//     JSON goes to the file, human-readable text goes to stderr. If the
//     file cannot be opened for append/create, Setup returns an error.
//
// The returned logger is also installed as slog.Default() so any
// slog.Info-style calls throughout the codebase use it.
//
// Setup does NOT close the file when the program exits; the OS reaps it.
// Long-running tests that call Setup repeatedly should call CloseLogFile
// between invocations to avoid leaking file handles.
func Setup(level, logFile string) (*slog.Logger, error) {
	lvl, err := ParseLevel(normalizeLevel(level))
	if err != nil {
		return nil, err
	}

	// Stderr handler: human-readable text. Stderr is the canonical MCP
	// log surface (stdout is reserved for JSON-RPC frames).
	stderrHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
	})

	var handler slog.Handler
	if logFile == "" {
		handler = stderrHandler
	} else {
		fh, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open log file %q: %w", logFile, err)
		}
		// JSON handler for the file: machine-parseable, one record per line.
		jsonHandler := slog.NewJSONHandler(fh, &slog.HandlerOptions{Level: lvl})
		handler = newTeeHandler(jsonHandler, stderrHandler)
		// Remember the handle so CloseLogFile can close it. We do this via
		// a package-level variable because Setup's caller (main) does not
		// have access to the file handle directly.
		logFileHandle = fh
	}

	logger := slog.New(handler)
	logger = logger.With(slog.String("component", "localvision"))
	slog.SetDefault(logger)
	return logger, nil
}

// logFileHandle is the file (if any) opened by the most recent Setup call.
// It is closed by CloseLogFile. Tests use this to avoid leaking handles
// when they call Setup repeatedly.
var logFileHandle io.Closer

// CloseLogFile closes any file handle opened by the previous Setup call.
// It is safe to call when no file is open (no-op) and safe to call multiple
// times. Useful for tests; production code never needs to call it (the OS
// reaps the handle at process exit).
func CloseLogFile() error {
	if logFileHandle == nil {
		return nil
	}
	err := logFileHandle.Close()
	logFileHandle = nil
	return err
}

// normalizeLevel maps the --verbose flag's natural string ("verbose") to
// "debug", and passes everything else through.
func normalizeLevel(s string) string {
	if strings.EqualFold(s, "verbose") {
		return "debug"
	}
	return s
}

// teeHandler fans out each log record to multiple underlying handlers.
// Used so file output and stderr output can coexist.
type teeHandler struct {
	handlers []slog.Handler
}

func newTeeHandler(handlers ...slog.Handler) slog.Handler {
	return &teeHandler{handlers: handlers}
}

func (t *teeHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range t.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		// Each handler gets its own copy of the record because Handle
		// mutates internal state during formatting.
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &teeHandler{handlers: make([]slog.Handler, len(t.handlers))}
	for i, h := range t.handlers {
		clone.handlers[i] = h.WithAttrs(attrs)
	}
	return clone
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	clone := &teeHandler{handlers: make([]slog.Handler, len(t.handlers))}
	for i, h := range t.handlers {
		clone.handlers[i] = h.WithGroup(name)
	}
	return clone
}
