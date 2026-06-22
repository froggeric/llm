package llama

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extra_test.go fills in coverage gaps not exercised by the focused
// lifecycle/client/binary test files. Keeps those files readable by
// grouping the misc coverage tests here.

// TestNewReturnsValidManager: New() with no options returns a usable
// manager. It will fail to spawn without a real binary, but the constructor
// itself must succeed.
func TestNewReturnsValidManager(t *testing.T) {
	m, err := New()
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, StateStopped, m.State())
}

// TestNewWithOptionsDefaultsApplied: zero-value Options get sane defaults.
func TestNewWithOptionsDefaultsApplied(t *testing.T) {
	m, err := NewWithOptions(Options{CacheDir: t.TempDir()})
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, defaultIdleTimeout, m.opts.IdleTimeout)
	assert.Equal(t, defaultStartupTimeout, m.opts.StartupTimeout)
	assert.Equal(t, defaultGracefulTimeout, m.opts.GracefulTimeout)
	assert.Equal(t, defaultSIGKILLAfter, m.opts.SIGKILLAfter)
	assert.Equal(t, defaultPortRetryAttempts, m.opts.PortRetryAttempts)
}

// TestNewWithOptionsCapsPortRetryAttempts: above-max value is capped.
func TestNewWithOptionsCapsPortRetryAttempts(t *testing.T) {
	m, err := NewWithOptions(Options{
		CacheDir:          t.TempDir(),
		PortRetryAttempts: 999,
	})
	require.NoError(t, err)
	assert.Equal(t, maxPortRetryAttempts, m.opts.PortRetryAttempts)
}

// TestBuildArgvFormat: the argv looks like the documented form.
func TestBuildArgvFormat(t *testing.T) {
	spec := models.ModelSpec{
		Ctx:       4096,
		GpuLayers: -1,
	}
	argv := buildArgv(spec, "/tmp/m.gguf", "/tmp/mmproj.gguf", 12345)
	require.Contains(t, argv, "-m")
	require.Contains(t, argv, "/tmp/m.gguf")
	require.Contains(t, argv, "--mmproj")
	require.Contains(t, argv, "/tmp/mmproj.gguf")
	require.Contains(t, argv, "--port")
	require.Contains(t, argv, "12345")
	require.Contains(t, argv, "--host")
	require.Contains(t, argv, "127.0.0.1")
	require.Contains(t, argv, "-ngl")
	require.Contains(t, argv, "-1")
	require.Contains(t, argv, "-c")
	require.Contains(t, argv, "4096")
	// Security: never 0.0.0.0.
	require.NotContains(t, argv, "0.0.0.0")
}

// TestSpawnSubprocessValidatesPaths: spawnSubprocess rejects paths outside
// the configured dirs.
func TestSpawnSubprocessValidatesPaths(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	modelsDir := filepath.Join(dir, "models")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.MkdirAll(modelsDir, 0o755))

	// Place a stub binary inside the bin dir.
	binPath := filepath.Join(binDir, "stub")
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755))

	// GGUF outside models dir.
	outside := filepath.Join(dir, "evil.gguf")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o600))

	_, err := spawnSubprocess(context.Background(), spawnOptions{
		BinaryPath: binPath,
		BinDir:     binDir,
		GGUFPath:   outside,
		ModelsDir:  modelsDir,
		Spec:       models.ModelSpec{Ctx: 100, GpuLayers: 0},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside models cache dir")
}

// TestSpawnSubprocessRejectsOutsideBinary: binary path outside bin dir.
func TestSpawnSubprocessRejectsOutsideBinary(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Place a stub binary OUTSIDE the bin dir.
	outside := filepath.Join(dir, "evil")
	require.NoError(t, os.WriteFile(outside, []byte("#!/bin/sh\n"), 0o755))

	_, err := spawnSubprocess(context.Background(), spawnOptions{
		BinaryPath: outside,
		BinDir:     binDir,
		GGUFPath:   filepath.Join(dir, "models", "x.gguf"),
		ModelsDir:  filepath.Join(dir, "models"),
		Spec:       models.ModelSpec{Ctx: 100, GpuLayers: 0},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside bin cache dir")
}

// TestSpawnSubprocessHappyPath: spawnSubprocess can actually start a
// subprocess (sleep) and capture its PID.
func TestSpawnSubprocessHappyPath(t *testing.T) {
	sleepPath, err := exec.LookPath("sleep")
	require.NoError(t, err)

	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	modelsDir := filepath.Join(dir, "models")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	require.NoError(t, os.MkdirAll(modelsDir, 0o755))

	// Place a stub binary inside the bin dir by copying sleep.
	binPath := filepath.Join(binDir, "stub")
	data, err := os.ReadFile(sleepPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(binPath, data, 0o755))

	// GGUF inside models dir.
	gguf := filepath.Join(modelsDir, "x.gguf")
	require.NoError(t, os.WriteFile(gguf, []byte("x"), 0o600))

	result, err := spawnSubprocess(context.Background(), spawnOptions{
		BinaryPath: binPath,
		BinDir:     binDir,
		GGUFPath:   gguf,
		ModelsDir:  modelsDir,
		Spec:       models.ModelSpec{Ctx: 100, GpuLayers: 0},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.cmd)
	require.NotNil(t, result.cmd.Process)
	defer func() {
		_ = result.cmd.Process.Kill()
		_, _ = result.cmd.Process.Wait()
	}()
}

// TestSampleFreePortSucceeds: a port is sampled and is a valid TCP port number.
func TestSampleFreePortSucceeds(t *testing.T) {
	port, err := sampleFreePort()
	require.NoError(t, err)
	assert.Greater(t, port, 0)
	assert.LessOrEqual(t, port, 65535)
}

// TestPathInside: covers the boundary conditions.
func TestPathInside(t *testing.T) {
	assert.True(t, pathInside("/a/b", "/a/b"))
	assert.True(t, pathInside("/a/b/c", "/a/b"))
	assert.False(t, pathInside("/a/bc", "/a/b"))
	assert.False(t, pathInside("a/b", "/a"))
	assert.False(t, pathInside("/a/b", ""))
}

// TestValidateModelPath: covers both branches.
func TestValidateModelPath(t *testing.T) {
	inside, err := validateModelPath("/a/b/m.gguf", "/a/b")
	require.NoError(t, err)
	assert.Equal(t, "/a/b/m.gguf", inside)

	_, err = validateModelPath("relative.gguf", "/a/b")
	require.Error(t, err)
}

// TestHTTPErrorMessage: covers (*httpError).Error() and (transportError).
func TestHTTPErrorMessage(t *testing.T) {
	he := &httpError{Status: 503, Body: "service unavailable"}
	s := he.Error()
	assert.Contains(t, s, "503")
	assert.Contains(t, s, "service unavailable")

	te := &transportError{err: errors.New("connection reset")}
	s = te.Error()
	assert.Contains(t, s, "connection reset")
}

// TestErrNotRunningFormatting covers (*ErrNotRunning).Error and IsErrNotRunning.
func TestErrNotRunningFormatting(t *testing.T) {
	e := &ErrNotRunning{State: StateCrashed, ModelID: "alpha"}
	assert.Contains(t, e.Error(), "crashed")
	assert.Contains(t, e.Error(), "alpha")
	assert.True(t, IsErrNotRunning(e))
	assert.False(t, IsErrNotRunning(errors.New("other")))

	var nilE *ErrNotRunning
	assert.Contains(t, nilE.Error(), "not running")
}

// TestErrTimeoutFormatting covers (*ErrTimeout).Error.
func TestErrTimeoutFormatting(t *testing.T) {
	e := &ErrTimeout{Op: "shutdown", ModelID: "alpha"}
	assert.Contains(t, e.Error(), "shutdown")
	assert.Contains(t, e.Error(), "alpha")

	e2 := &ErrTimeout{Op: "spawn"}
	assert.Contains(t, e2.Error(), "spawn")

	var nilE *ErrTimeout
	assert.Contains(t, nilE.Error(), "timed out")
}

// TestErrPortInUseFormatting covers (*ErrPortInUse).Error and IsErrPortInUse.
func TestErrPortInUseFormatting(t *testing.T) {
	e := &ErrPortInUse{Port: 8080, Attempts: 3}
	assert.Contains(t, e.Error(), "8080")
	assert.Contains(t, e.Error(), "3")
	assert.True(t, IsErrPortInUse(e))

	var nilE *ErrPortInUse
	assert.Contains(t, nilE.Error(), "port in use")
}

// TestErrCrashedIsFunc: IsErrCrashed matches a wrapped *ErrCrashed.
func TestErrCrashedIsFunc(t *testing.T) {
	e := &ErrCrashed{ModelID: "x"}
	assert.True(t, IsErrCrashed(e))
	assert.False(t, IsErrCrashed(errors.New("other")))

	var nilE *ErrCrashed
	assert.Contains(t, nilE.Error(), "crashed")
}

// TestErrCrashedNoStderr covers the empty-tail formatting branch.
func TestErrCrashedNoStderr(t *testing.T) {
	e := &ErrCrashed{ModelID: "x", ExitErr: errors.New("boom")}
	s := e.Error()
	assert.Contains(t, s, "no stderr captured")
}

// TestErrIntegrityFailStructFormat: structured error string contains both hashes.
func TestErrIntegrityFailStructFormat(t *testing.T) {
	e := &ErrIntegrityFailStruct{Path: "/p", Expected: "abc", Actual: "def"}
	s := e.Error()
	assert.Contains(t, s, "/p")
	assert.Contains(t, s, "abc")
	assert.Contains(t, s, "def")

	var nilE *ErrIntegrityFailStruct
	assert.Contains(t, nilE.Error(), ErrIntegrityFail.Error())
}

// TestLimitedBufferReset covers Reset().
func TestLimitedBufferReset(t *testing.T) {
	b := newLimitedBuffer(16)
	_, _ = b.Write([]byte("hello"))
	b.Reset()
	assert.Equal(t, "", b.String())
}

// TestLimitedBufferEmptyWrite: writing zero bytes is a no-op.
func TestLimitedBufferEmptyWrite(t *testing.T) {
	b := newLimitedBuffer(16)
	n, err := b.Write(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// TestLimitedBufferDefaultMax: passing max <= 0 uses the documented default.
func TestLimitedBufferDefaultMax(t *testing.T) {
	b := newLimitedBuffer(0)
	assert.Equal(t, 1024, b.max)
	b2 := newLimitedBuffer(-1)
	assert.Equal(t, 1024, b2.max)
}

// TestFindOrDownloadBinaryFallsBackToDownload: when the cache is empty and
// the server returns a valid binary, the download succeeds.
func TestFindOrDownloadBinaryFallsBackToDownload(t *testing.T) {
	payload := []byte("#!/bin/sh\necho stub\n")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })
	// Override the URL so we don't try to reach GitHub.
	restore := setDownloadURLOverride(srv.URL + "/llama-server")
	defer restore()

	dir := t.TempDir()
	got, err := findOrDownloadBinary(t.Context(), "TODO-PHASE3", dir)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(got, "llama-server"))

	info, err := os.Stat(got)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100)
}

// TestFindOrDownloadBinaryCreatesBinDir: a missing bin dir is created.
func TestFindOrDownloadBinaryCreatesBinDir(t *testing.T) {
	payload := []byte("#!/bin/sh\necho stub\n")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })
	restore := setDownloadURLOverride(srv.URL + "/llama-server")
	defer restore()

	dir := t.TempDir()
	binDir := filepath.Join(dir, "nested", "bin")
	_, err := findOrDownloadBinary(t.Context(), "TODO-PHASE3", binDir)
	require.NoError(t, err)
	_, err = os.Stat(binDir)
	require.NoError(t, err)
}

// TestDownloadAndVerifyCtxCancelled: ctx cancelled mid-download cleans up
// the .tmp file.
func TestDownloadAndVerifyCtxCancelled(t *testing.T) {
	// Slow server.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setDownloadClient(srv.Client())
	t.Cleanup(func() { setDownloadClient(&http.Client{Timeout: 0}) })

	dir := t.TempDir()
	finalPath := filepath.Join(dir, "llama-server")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := downloadAndVerify(ctx, srv.URL+"/x", finalPath, "TODO-PHASE3")
	require.Error(t, err)

	// .tmp must be cleaned up.
	_, statErr := os.Stat(finalPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(finalPath)
	assert.True(t, os.IsNotExist(statErr))
}

// TestStateAfterCrashAndRespawn: regression test for the watcher firing
// between spawn and health, then a follow-up Acquire respawning.
func TestStateAfterCrashAndRespawn(t *testing.T) {
	var spawnCount int32
	crashSpawn, crashHealth := fakeSpawnCrash(t)
	sleepSpawn, sleepHealth, cleanup := fakeSpawnSleep(t)
	t.Cleanup(cleanup)

	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		n := atomic.AddInt32(&spawnCount, 1)
		if n == 1 {
			return crashSpawn(ctx, opts)
		}
		return sleepSpawn(ctx, opts)
	}
	wrappedHealth := func(ctx context.Context, port int, timeout time.Duration) error {
		if atomic.LoadInt32(&spawnCount) <= 1 {
			return crashHealth(ctx, port, timeout)
		}
		return sleepHealth(ctx, port, timeout)
	}
	m := newTestManager(t, Options{
		SpawnHook:      wrappedSpawn,
		HealthHook:     wrappedHealth,
		StartupTimeout: 500 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err)

	// Re-acquire; should respawn.
	c, rel, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)
	defer rel()
	assert.Equal(t, StateReady, m.State())
}

// TestSigkillOnProcess: cover sigkill() by sending SIGKILL to a sleep
// subprocess and verifying via Wait() that the process exits.
func TestSigkillOnProcess(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	require.NoError(t, cmd.Start())

	// signal 0 should succeed before kill.
	require.True(t, processAlive(cmd))

	require.NoError(t, sigkill(cmd))

	// Wait reaps the process; once reaped, signal 0 returns ESRCH.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cmd.Wait() did not return after SIGKILL")
	}
	// After Wait has reaped, processAlive should report false.
	assert.False(t, processAlive(cmd), "process should be dead after Wait+reap")
}

// TestSigtermOnProcess: cover sigterm() by sending SIGTERM to sleep and
// verifying it exits.
func TestSigtermOnProcess(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	require.NoError(t, cmd.Start())
	defer func() {
		_ = sigkill(cmd)
		_, _ = cmd.Process.Wait()
	}()

	require.True(t, processAlive(cmd))
	require.NoError(t, sigterm(cmd))
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cmd.Wait() did not return after SIGTERM")
	}
}

// TestSigtermNilDoesntPanic: defensive nil handling for sigterm/sigkill/
// processAlive.
func TestSigtermNilDoesntPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_ = sigterm(nil)
		_ = sigkill(nil)
		assert.False(t, processAlive(nil))
	})
}
