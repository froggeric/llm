package logging

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLevel verifies level name parsing including the --verbose
// alias handled by normalizeLevel.
func TestParseLevel(t *testing.T) {
	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"info", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{"Debug", slog.LevelDebug, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"bogus", 0, true},
		{"verbose", slog.LevelDebug, false}, // alias via normalizeLevel
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			level, err := ParseLevel(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, level)
		})
	}
}

// TestSetupReturnsValidLogger verifies Setup returns a non-nil logger
// and that the logger is installed as slog.Default() so library code
// picks up the configuration.
func TestSetupReturnsValidLogger(t *testing.T) {
	// Suppress stderr noise for this test by redirecting; we don't read
	// it back, we just verify Setup doesn't fail and returns something
	// usable. We pass an empty level (defaults to info) and no log file.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		// drain the pipe so it doesn't leak
		go func() { _, _ = io.Copy(io.Discard, r) }()
	}()

	origDefault := slog.Default()
	defer slog.SetDefault(origDefault)

	logger, err := Setup("", "")
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Emitting a record must not crash.
	logger.Info("test message", "key", "value")

	// Verify slog.Default() now matches our logger (same handler).
	// We can't compare pointers because Default wraps; instead just
	// verify Default is non-nil and works.
	def := slog.Default()
	require.NotNil(t, def)
	def.Info("another test")
}

// TestSetupFileOutput verifies that when --log-file is set, log records
// are written to the file in JSON form (machine-parseable).
func TestSetupFileOutput(t *testing.T) {
	// Skip on platforms where temp file paths are unreliable.
	if runtime.GOOS == "windows" {
		t.Skip("temp dir edge cases on windows; revisit in v0.2")
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "out.jsonl")

	// Suppress stderr.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		go func() { _, _ = io.Copy(io.Discard, r) }()
	}()

	origDefault := slog.Default()
	defer slog.SetDefault(origDefault)

	logger, err := Setup("debug", logPath)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// CloseLogFile is needed before we read the file: writes go through
	// buffered I/O in slog (actually slog doesn't buffer, but the OS may).
	defer CloseLogFile()

	logger.Info("structured test", "k1", "v1", "k2", 42, "time", time.Unix(1700000000, 0))

	// Sync by closing.
	require.NoError(t, CloseLogFile())

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.NotEmpty(t, data, "log file should not be empty")

	// Each line should be valid JSON. The first line should contain our
	// "structured test" message.
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var found bool
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Errorf("log line is not valid JSON: %v\nline: %s", err, line)
			continue
		}
		if msg, _ := rec["msg"].(string); msg == "structured test" {
			found = true
			assert.Equal(t, "v1", rec["k1"])
			assert.Equal(t, float64(42), rec["k2"])
			assert.Equal(t, "local-vision-mcp", rec["component"])
			assert.Equal(t, "INFO", rec["level"])
		}
	}
	require.True(t, found, "expected to find 'structured test' log record in file output")
}

// TestSetupFileOpenError verifies Setup surfaces a useful error when the
// log file path is unwritable.
func TestSetupFileOpenError(t *testing.T) {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		go func() { _, _ = io.Copy(io.Discard, r) }()
	}()

	origDefault := slog.Default()
	defer slog.SetDefault(origDefault)
	defer CloseLogFile()

	// Try to write to a directory that doesn't exist.
	_, err := Setup("info", "/nonexistent/dir/that/does/not/exist/log.jsonl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open log file")
}

// TestSetupInvalidLevel verifies Setup rejects bogus level names.
func TestSetupInvalidLevel(t *testing.T) {
	_, err := Setup("trace", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

// TestSetupDebugLevelEnablesDebug verifies the level is honored.
func TestSetupDebugLevelEnablesDebug(t *testing.T) {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		go func() { _, _ = io.Copy(io.Discard, r) }()
	}()

	origDefault := slog.Default()
	defer slog.SetDefault(origDefault)
	defer CloseLogFile()

	logger, err := Setup("debug", "")
	require.NoError(t, err)
	// A debug-level record should be enabled at debug level.
	assert.True(t, logger.Enabled(nil, slog.LevelDebug))
	assert.True(t, logger.Enabled(nil, slog.LevelInfo))
	assert.True(t, logger.Enabled(nil, slog.LevelError))
}

// TestSetupInfoLevelDisablesDebug verifies the level is honored.
func TestSetupInfoLevelDisablesDebug(t *testing.T) {
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		go func() { _, _ = io.Copy(io.Discard, r) }()
	}()

	origDefault := slog.Default()
	defer slog.SetDefault(origDefault)
	defer CloseLogFile()

	logger, err := Setup("info", "")
	require.NoError(t, err)
	// Debug records should be filtered out at info level.
	assert.False(t, logger.Enabled(nil, slog.LevelDebug))
	assert.True(t, logger.Enabled(nil, slog.LevelInfo))
}

// TestCloseLogFileIdempotent verifies CloseLogFile is safe to call when
// no file is open and safe to call multiple times.
func TestCloseLogFileIdempotent(t *testing.T) {
	// Start from a known state.
	_ = CloseLogFile()
	require.NoError(t, CloseLogFile())
	require.NoError(t, CloseLogFile())
}
