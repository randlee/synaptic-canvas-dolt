package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// logDir is the directory under the user's home where log files are stored.
	logDir = ".sc/logs"
	// logFile is the name of the log file.
	logFile = "sc.log"
	// logRetentionDays is the number of days to keep rotated log files.
	logRetentionDays = 7
)

// Setup creates and configures a structured logger based on verbosity settings.
//
// Logging behaviour:
//   - File output: always Info level (regardless of verbose/quiet flags)
//   - Console verbose=true  → Debug level on stderr
//   - Console quiet=true    → Warn level on stderr
//   - Console default       → Info level on stderr
//
// The logger always writes JSON-formatted entries to ~/.sc/logs/sc.log
// (creating the directory if needed). Log rotation occurs on startup: if
// sc.log was last modified on a different date, it is renamed to
// sc-YYYY-MM-DD.log and rotated log files older than 7 days are deleted.
//
// The returned logger is also installed as the slog package default.
func Setup(verbose, quiet bool) *slog.Logger {
	consoleLevel := resolveConsoleLevel(verbose, quiet)

	// Build the list of slog.Handler targets.
	handlers := make([]slog.Handler, 0, 2)

	// File handler — always enabled at Info level, JSON format.
	if fh, err := fileHandler(); err == nil {
		handlers = append(handlers, fh)
	}

	// Console handler — stderr, text format (suppressed when quiet).
	if !quiet {
		handlers = append(handlers, consoleHandler(consoleLevel))
	}

	var logger *slog.Logger
	switch len(handlers) {
	case 0:
		// Fallback: should not happen, but be safe.
		logger = slog.New(consoleHandler(consoleLevel))
	case 1:
		logger = slog.New(handlers[0])
	default:
		logger = slog.New(newMultiHandler(handlers...))
	}

	slog.SetDefault(logger)
	return logger
}

// WithContext returns a logger with standard component and operation attributes.
func WithContext(logger *slog.Logger, component, operation string) *slog.Logger {
	return logger.With("component", component, "operation", operation)
}

// resolveConsoleLevel maps the verbose/quiet flags to a slog.Level for console output.
func resolveConsoleLevel(verbose, quiet bool) slog.Level {
	switch {
	case verbose:
		return slog.LevelDebug
	case quiet:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// fileHandler returns a JSON handler that writes to the log file.
// The file handler always uses Info level regardless of verbosity settings.
func fileHandler() (slog.Handler, error) {
	dir, err := logDirPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	// Rotate logs before opening the file.
	if err := rotateLog(dir); err != nil {
		// Log rotation failure is not fatal; proceed with the current file.
		_ = err
	}

	path := filepath.Join(dir, logFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // path derived from os.UserHomeDir
	if err != nil {
		return nil, err
	}
	return slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}), nil
}

// rotateLog checks if sc.log was last modified on a different date than today.
// If so, it renames the file to sc-YYYY-MM-DD.log (using the file's mod date)
// and deletes any rotated log files older than logRetentionDays.
func rotateLog(dir string) error {
	return rotateLogWithTime(dir, time.Now())
}

// rotateLogWithTime is the testable version of rotateLog that accepts a "now" parameter.
func rotateLogWithTime(dir string, now time.Time) error {
	path := filepath.Join(dir, logFile)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No log file to rotate.
		}
		return fmt.Errorf("stat log file: %w", err)
	}

	modDate := info.ModTime().Format("2006-01-02")
	today := now.Format("2006-01-02")

	if modDate == today {
		return nil // Same day, no rotation needed.
	}

	// Rename current log to dated name.
	rotatedName := fmt.Sprintf("sc-%s.log", modDate)
	rotatedPath := filepath.Join(dir, rotatedName)
	if err := os.Rename(path, rotatedPath); err != nil {
		return fmt.Errorf("rotating log file: %w", err)
	}

	// Clean up old rotated log files.
	return cleanOldLogs(dir, now)
}

// cleanOldLogs deletes sc-YYYY-MM-DD.log files older than logRetentionDays.
func cleanOldLogs(dir string, now time.Time) error {
	// Truncate to start of day so comparisons with date-only parsed times are consistent.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	cutoff := today.AddDate(0, 0, -logRetentionDays)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading log directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "sc-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		// Extract date from sc-YYYY-MM-DD.log
		datePart := strings.TrimPrefix(name, "sc-")
		datePart = strings.TrimSuffix(datePart, ".log")

		fileDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue // Not a rotated log file we manage.
		}
		if fileDate.Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing old log %s: %w", name, err)
			}
		}
	}
	return nil
}

// consoleHandler returns a text handler writing to stderr.
func consoleHandler(level slog.Level) slog.Handler {
	return slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
}

// logDirPath returns the absolute path to the log directory.
func logDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, logDir), nil
}

// SetupWithWriter creates a logger that writes to the provided writer instead of
// the default file and stderr. This is useful for testing.
func SetupWithWriter(w io.Writer, verbose, quiet bool) *slog.Logger {
	level := resolveConsoleLevel(verbose, quiet)
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(context.Background(), level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return newMultiHandler(newHandlers...)
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return newMultiHandler(newHandlers...)
}
