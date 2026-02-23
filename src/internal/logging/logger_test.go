package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupVerbose(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := SetupWithWriter(&buf, true, false)

	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Error("verbose mode should log debug messages")
	}
}

func TestSetupQuiet(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := SetupWithWriter(&buf, false, true)

	logger.Info("info message")
	if strings.Contains(buf.String(), "info message") {
		t.Error("quiet mode should suppress info messages on console")
	}

	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("quiet mode should log warn messages on console")
	}

	logger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("quiet mode should still log error messages")
	}
}

func TestSetupDefault(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := SetupWithWriter(&buf, false, false)

	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("default mode should log info messages")
	}

	buf.Reset()
	logger.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("default mode should not log debug messages")
	}
}

func TestResolveConsoleLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		verbose bool
		quiet   bool
		want    slog.Level
	}{
		{"verbose", true, false, slog.LevelDebug},
		{"quiet", false, true, slog.LevelWarn},
		{"default", false, false, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveConsoleLevel(tt.verbose, tt.quiet)
			if got != tt.want {
				t.Errorf("resolveConsoleLevel(%v, %v) = %v, want %v", tt.verbose, tt.quiet, got, tt.want)
			}
		})
	}
}

func TestLogFileCreation(t *testing.T) {
	t.Parallel()

	// Use a temp directory to avoid polluting the real home directory.
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, ".sc", "logs")
	logPath := filepath.Join(logDir, "sc.log")

	// Create the directory and file manually to simulate what Setup does.
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // test file in temp dir
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	logger.Info("test log entry")
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close log file: %v", err)
	}

	// Verify the file was created and contains content.
	data, err := os.ReadFile(logPath) //nolint:gosec // test file in temp dir
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(data), "test log entry") {
		t.Error("log file should contain the test log entry")
	}
}

func TestWithContext(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	logger = WithContext(logger, "cli", "init")
	logger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "component=cli") {
		t.Errorf("expected component=cli in output, got: %s", output)
	}
	if !strings.Contains(output, "operation=init") {
		t.Errorf("expected operation=init in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("expected 'test message' in output, got: %s", output)
	}
}

func TestRotateLog_NoFileNoError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No sc.log exists — rotation should be a no-op.
	if err := rotateLogWithTime(dir, time.Now()); err != nil {
		t.Fatalf("rotateLog should not error when no log file exists: %v", err)
	}
}

func TestRotateLog_SameDayNoRotation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sc.log")

	// Create a log file "today".
	if err := os.WriteFile(logPath, []byte("today's log"), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := rotateLogWithTime(dir, now); err != nil {
		t.Fatalf("rotateLog should not error for same-day file: %v", err)
	}

	// sc.log should still exist.
	if _, err := os.Stat(logPath); err != nil {
		t.Error("sc.log should still exist when no rotation is needed")
	}
}

func TestRotateLog_DifferentDayRotates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sc.log")

	// Create a log file and backdate its mod time to yesterday.
	if err := os.WriteFile(logPath, []byte("yesterday's log"), 0o600); err != nil {
		t.Fatal(err)
	}
	yesterday := time.Now().AddDate(0, 0, -1)
	if err := os.Chtimes(logPath, yesterday, yesterday); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := rotateLogWithTime(dir, now); err != nil {
		t.Fatalf("rotateLog failed: %v", err)
	}

	// sc.log should no longer exist (it was renamed).
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("sc.log should have been renamed during rotation")
	}

	// The rotated file should exist.
	expectedName := "sc-" + yesterday.Format("2006-01-02") + ".log"
	rotatedPath := filepath.Join(dir, expectedName)
	data, err := os.ReadFile(rotatedPath) //nolint:gosec // test file in temp dir
	if err != nil {
		t.Fatalf("rotated log file should exist at %s: %v", expectedName, err)
	}
	if string(data) != "yesterday's log" {
		t.Errorf("rotated file content mismatch: got %q", string(data))
	}
}

func TestCleanOldLogs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create rotated logs at various ages.
	now := time.Now()
	files := map[string]bool{
		"sc-" + now.AddDate(0, 0, -1).Format("2006-01-02") + ".log":  true,  // 1 day ago — keep
		"sc-" + now.AddDate(0, 0, -6).Format("2006-01-02") + ".log":  true,  // 6 days ago — keep
		"sc-" + now.AddDate(0, 0, -8).Format("2006-01-02") + ".log":  false, // 8 days ago — delete
		"sc-" + now.AddDate(0, 0, -30).Format("2006-01-02") + ".log": false, // 30 days ago — delete
		"other.log": true, // Not a rotated log — keep
	}

	for name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("log"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := cleanOldLogs(dir, now); err != nil {
		t.Fatalf("cleanOldLogs failed: %v", err)
	}

	for name, shouldExist := range files {
		_, err := os.Stat(filepath.Join(dir, name))
		exists := err == nil
		if shouldExist && !exists {
			t.Errorf("file %q should exist after cleanup but doesn't", name)
		}
		if !shouldExist && exists {
			t.Errorf("file %q should have been deleted but still exists", name)
		}
	}
}

func TestFileHandlerAlwaysInfoLevel(t *testing.T) {
	t.Parallel()

	// Create a temp dir and a log file to write to, simulating fileHandler behavior.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "sc.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // test file in temp dir
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("failed to close log file: %v", err)
		}
	}()

	// Create handler at Info level (as fileHandler does).
	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})

	// Debug should NOT be enabled.
	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("file handler should not log debug messages")
	}

	// Info should be enabled.
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("file handler should log info messages")
	}
}
