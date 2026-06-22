package llama

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lifecycle_test.go exercises the lifecycle state machine using fake
// subprocesses. We never spawn a real llama-server here; instead, the
// SpawnHook injects a long-running `sleep` (happy path) or a false-equivalent
// (crash) to drive the state machine deterministically.
//
// Required coverage (exit criteria):
//   - concurrent Acquire serialization
//   - idle-timer-gated-by-refcount
//   - crash-then-respawn
//   - SHA256-mismatch rejection
//   - port-binding race retry
//   - Shutdown idempotency

// lookPathOrDefault finds an executable by name via PATH lookup, falling
// back to fallback if not found. Used so tests work on Linux and macOS
// where sleep/false live in different /bin or /usr/bin paths.
func lookPathOrDefault(name, fallback string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return fallback
}

// sleepBin / falseBin are resolved at init via PATH lookup so tests don't
// hard-code /bin vs /usr/bin.
var (
	sleepBin = lookPathOrDefault("sleep", "/bin/sleep")
	falseBin = lookPathOrDefault("false", "/usr/bin/false")
	trueBin  = lookPathOrDefault("true", "/usr/bin/true")
)

// testSpec returns a ModelSpec suitable for tests: placeholder URLs with
// real SHA256 placeholders, only the fields the lifecycle actually reads.
func testSpec() models.ModelSpec {
	return models.ModelSpec{
		DisplayName:    "Test VLM",
		GGUF:           "https://huggingface.co/froggeric/test-gguf/resolve/main/x.gguf",
		Mmproj:         "https://huggingface.co/froggeric/test-gguf/resolve/main/x-mmproj.gguf",
		GGUFSha256:     stringsRepeat("a", 64),
		MmprojSha256:   stringsRepeat("b", 64),
		Ctx:            4096,
		GpuLayers:      -1,
		MinVramGb:      4,
		MinSystemRamGb: 8,
		License:        "Apache-2.0",
		HardwareTier:   models.TierConstrained,
	}
}

// stringsRepeat is a tiny local helper so we don't add a "strings" import
// to this test file. (Keeps the import list focused on test packages.)
func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// testCatalog returns a Catalog with two models ("alpha", "beta").
func testCatalog() *models.Catalog {
	c := &models.Catalog{
		SchemaVersion: 1,
		Models:        map[string]models.ModelSpec{},
	}
	c.Models["alpha"] = testSpec()
	c.Models["beta"] = testSpec()
	return c
}

// fakeSpawnSleep returns a SpawnHook that spawns `sleep 30` (a long-lived
// process that won't exit on its own). The accompanying HealthHook inspects
// process liveness, treating an alive process as healthy. The returned
// cleanup function kills any leftover sleep processes.
func fakeSpawnSleep(t *testing.T) (spawn func(ctx context.Context, opts spawnOptions) (*spawnResult, error), health func(ctx context.Context, port int, timeout time.Duration) error, cleanup func()) {
	t.Helper()
	var mu sync.Mutex
	cmds := []*exec.Cmd{}
	spawn = func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		cmd := exec.CommandContext(ctx, sleepBin, "30")
		stderr := newLimitedBuffer(1024)
		cmd.Stderr = stderr
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		mu.Lock()
		cmds = append(cmds, cmd)
		mu.Unlock()
		port, _ := sampleFreePort()
		return &spawnResult{cmd: cmd, port: port, stderr: stderr}, nil
	}
	health = func(ctx context.Context, port int, timeout time.Duration) error {
		// In tests, we can't actually hit /health because we spawn `sleep`,
		// not a real HTTP server. Instead, the test injects a different
		// health hook to simulate failure. This default always-succeeds
		// hook is only used by happy-path tests; crash tests override it.
		return nil
	}
	cleanup = func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range cmds {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
		}
	}
	return spawn, health, cleanup
}

// fakeSpawnCrash returns a SpawnHook that spawns `false`, which exits
// immediately with code 1. Used to simulate a crash.
//
// The accompanying HealthHook inspects the cmd's process liveness; if the
// process has already exited, it returns an error (mimicking what real
// waitForHealth would see). The watcher goroutine will also flip state to
// Crashed.
func fakeSpawnCrash(t *testing.T) (spawn func(ctx context.Context, opts spawnOptions) (*spawnResult, error), health func(ctx context.Context, port int, timeout time.Duration) error) {
	t.Helper()
	var mu sync.Mutex
	var lastCmd *exec.Cmd
	spawn = func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		cmd := exec.CommandContext(ctx, falseBin)
		stderr := newLimitedBuffer(1024)
		// Pre-populate stderr so the crash capture has something to show.
		_, _ = stderr.Write([]byte("simulated crash for test\n"))
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		mu.Lock()
		lastCmd = cmd
		mu.Unlock()
		port, _ := sampleFreePort()
		return &spawnResult{cmd: cmd, port: port, stderr: stderr}, nil
	}
	health = func(ctx context.Context, port int, timeout time.Duration) error {
		// Wait briefly for the false process to actually exit (so the
		// watcher has flipped state). Then check liveness; we expect it
		// to be dead.
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			mu.Lock()
			c := lastCmd
			mu.Unlock()
			if c == nil || !processAlive(c) {
				return errors.New("simulated: subprocess not healthy")
			}
			time.Sleep(10 * time.Millisecond)
		}
		return errors.New("simulated: subprocess not healthy")
	}
	return spawn, health
}

// newTestManager constructs a LifecycleManager with all the test hooks
// wired up. Returns the manager; t.Cleanup registers a Shutdown.
func newTestManager(t *testing.T, opts Options) *LifecycleManager {
	t.Helper()
	dir := t.TempDir()
	opts.CacheDir = dir
	if opts.Catalog == nil {
		opts.Catalog = testCatalog()
	}
	// Default the test hooks: sleep spawn, immediate-health, no SHA check.
	if opts.SpawnHook == nil {
		spawn, health, cleanup := fakeSpawnSleep(t)
		opts.SpawnHook = spawn
		if opts.HealthHook == nil {
			opts.HealthHook = health
		}
		t.Cleanup(cleanup)
	}
	if opts.VerifyHashHook == nil {
		opts.VerifyHashHook = func(ctx context.Context, path, expectedHex string) error { return nil }
	}
	if opts.BinaryPath == "" && opts.ResolveBinaryHook == nil {
		opts.BinaryPath = filepath.Join(dir, "bin", "llama-server")
		opts.SkipBinaryDiscovery = true
	}
	// Shrink idle and shutdown timeouts so tests run quickly.
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 100 * time.Millisecond
	}
	if opts.StartupTimeout == 0 {
		opts.StartupTimeout = 5 * time.Second
	}
	if opts.GracefulTimeout == 0 {
		opts.GracefulTimeout = 2 * time.Second
	}
	if opts.SIGKILLAfter == 0 {
		opts.SIGKILLAfter = 500 * time.Millisecond
	}

	m, err := NewWithOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	return m
}

// TestAcquireHappyPath: a single Acquire loads the model, returns a Client,
// and release() decrements the refcount.
func TestAcquireHappyPath(t *testing.T) {
	m := newTestManager(t, Options{})

	require.Equal(t, StateStopped, m.State())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, release, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, release)
	defer release()

	require.Equal(t, StateReady, m.State())
	assert.Equal(t, "alpha", m.loadedModelID)
	assert.Equal(t, 1, m.activeRefcount)
}

// TestAcquireSerializeConcurrent: 50 concurrent Acquire calls all succeed
// and serialize correctly — only ONE subprocess spawn happens.
func TestAcquireSerializeConcurrent(t *testing.T) {
	var spawnCount int32
	spawn, health, cleanup := fakeSpawnSleep(t)
	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		atomic.AddInt32(&spawnCount, 1)
		return spawn(ctx, opts)
	}
	t.Cleanup(cleanup)
	m := newTestManager(t, Options{
		SpawnHook:  wrappedSpawn,
		HealthHook: health,
	})

	const N = 50
	var wg sync.WaitGroup
	errs := make([]error, N)
	clients := make([]*Client, N)
	releases := make([]func(), N)
	start := make(chan struct{})
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c, rel, err := m.Acquire(ctx, "alpha")
			errs[i] = err
			clients[i] = c
			releases[i] = rel
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d failed", i)
		require.NotNil(t, clients[i], "goroutine %d got nil client", i)
		require.NotNil(t, releases[i], "goroutine %d got nil release", i)
	}

	assert.Equal(t, N, m.activeRefcount, "all N Acquires should have incremented refcount")
	assert.Equal(t, int32(1), atomic.LoadInt32(&spawnCount),
		"all N concurrent Acquires must share a single subprocess spawn")

	// Release them all; refcount should hit 0.
	for _, rel := range releases {
		if rel != nil {
			rel()
		}
	}
	assert.Equal(t, 0, m.activeRefcount)
}

// TestAcquireDifferentModelSwitches: requesting a different model unloads
// the current one and spawns a new subprocess.
func TestAcquireDifferentModelSwitches(t *testing.T) {
	var spawnCount int32
	spawn, health, cleanup := fakeSpawnSleep(t)
	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		atomic.AddInt32(&spawnCount, 1)
		return spawn(ctx, opts)
	}
	t.Cleanup(cleanup)
	m := newTestManager(t, Options{
		SpawnHook:  wrappedSpawn,
		HealthHook: health,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c1, rel1, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c1)
	rel1()

	c2, rel2, err := m.Acquire(ctx, "beta")
	require.NoError(t, err)
	require.NotNil(t, c2)
	defer rel2()

	assert.Equal(t, "beta", m.loadedModelID)
	assert.Equal(t, int32(2), atomic.LoadInt32(&spawnCount))
}

// TestIdleTimerGatedByRefcount: with refcount > 0, the idle timer does not
// kill the subprocess even after IdleTimeout has elapsed.
func TestIdleTimerGatedByRefcount(t *testing.T) {
	m := newTestManager(t, Options{
		IdleTimeout: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, release, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)
	defer release()

	// Sleep long enough for the idle timer to fire (if it could).
	time.Sleep(400 * time.Millisecond)

	// The state should still be ready because refcount > 0.
	require.Equal(t, StateReady, m.State(), "refcount must gate the idle timer")
	require.Equal(t, 1, m.activeRefcount)
}

// TestIdleTimerFiresAtZero: when refcount hits 0, the idle timer fires and
// kills the subprocess. This is the positive control for the gate above.
func TestIdleTimerFiresAtZero(t *testing.T) {
	m := newTestManager(t, Options{
		IdleTimeout:     100 * time.Millisecond,
		SIGKILLAfter:    500 * time.Millisecond,
		GracefulTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, release, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)

	// Release; idle timer armed. Wait for it to fire and the kill to settle.
	release()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m.State() == StateStopped {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, StateStopped, m.State(), "idle timer should have killed the subprocess")
}

// TestCrashThenRespawn: spawn crashes (exit code 1); the next Acquire
// observes Crashed and respawns successfully.
func TestCrashThenRespawn(t *testing.T) {
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
	// Health: route to the appropriate hook based on spawn count.
	wrappedHealth := func(ctx context.Context, port int, timeout time.Duration) error {
		n := atomic.LoadInt32(&spawnCount)
		if n <= 1 {
			return crashHealth(ctx, port, timeout)
		}
		return sleepHealth(ctx, port, timeout)
	}

	m := newTestManager(t, Options{
		SpawnHook:      wrappedSpawn,
		HealthHook:     wrappedHealth,
		StartupTimeout: 500 * time.Millisecond, // short so we fail fast
	})

	// First Acquire should fail because the subprocess crashed.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err, "first Acquire should fail due to crash")
	// State should be crashed (or stopped, since unload may have run).
	state := m.State()
	assert.True(t, state == StateCrashed || state == StateStopped,
		"expected crashed or stopped, got %s", state)

	// Second Acquire should succeed.
	c2, rel2, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err, "second Acquire should respawn and succeed")
	require.NotNil(t, c2)
	defer rel2()

	assert.Equal(t, StateReady, m.State())
	assert.Equal(t, "alpha", m.loadedModelID)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&spawnCount), int32(2))
}

// TestSHA256MismatchRejected: when VerifyHashHook returns an integrity
// error, Acquire returns a wrapped ErrIntegrityFail and does NOT spawn.
func TestSHA256MismatchRejected(t *testing.T) {
	var spawnCount int32
	spawn, health, cleanup := fakeSpawnSleep(t)
	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		atomic.AddInt32(&spawnCount, 1)
		return spawn(ctx, opts)
	}
	t.Cleanup(cleanup)

	verifyErr := &ErrIntegrityFailStruct{
		Path:     "/path/to/model.gguf",
		Expected: stringsRepeat("a", 64),
		Actual:   stringsRepeat("b", 64),
	}
	m := newTestManager(t, Options{
		SpawnHook:  wrappedSpawn,
		HealthHook: health,
		VerifyHashHook: func(ctx context.Context, path, expectedHex string) error {
			return verifyErr
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrIntegrityFail),
		"err should wrap ErrIntegrityFail, got %v", err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&spawnCount),
		"spawn must NOT happen when integrity check fails")
}

// TestPortBindingRaceRetry: the spawn hook fails N-1 times then succeeds,
// simulating a transient "address already in use". The lifecycle should
// retry up to PortRetryAttempts and eventually succeed.
func TestPortBindingRaceRetry(t *testing.T) {
	var spawnCount int32
	sleepSpawn, health, cleanup := fakeSpawnSleep(t)
	t.Cleanup(cleanup)

	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		n := atomic.AddInt32(&spawnCount, 1)
		if n < 3 {
			// Simulate a port-binding failure.
			return nil, errors.New("listen: bind: address already in use")
		}
		return sleepSpawn(ctx, opts)
	}

	m := newTestManager(t, Options{
		SpawnHook:         wrappedSpawn,
		HealthHook:        health,
		PortRetryAttempts: 3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, rel, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)
	defer rel()

	assert.Equal(t, StateReady, m.State())
	assert.Equal(t, int32(3), atomic.LoadInt32(&spawnCount),
		"should have retried 2 times then succeeded on the 3rd")
}

// TestPortBindingRaceExhausted: the spawn hook always fails; the lifecycle
// retries PortRetryAttempts times then surfaces an error.
func TestPortBindingRaceExhausted(t *testing.T) {
	var spawnCount int32
	wrappedSpawn := func(ctx context.Context, opts spawnOptions) (*spawnResult, error) {
		atomic.AddInt32(&spawnCount, 1)
		return nil, errors.New("listen: bind: address already in use")
	}

	m := newTestManager(t, Options{
		SpawnHook:         wrappedSpawn,
		HealthHook:        func(ctx context.Context, port int, timeout time.Duration) error { return nil },
		PortRetryAttempts: 3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&spawnCount),
		"should have retried exactly PortRetryAttempts times")
}

// TestShutdownIdempotent: calling Shutdown twice is a no-op.
func TestShutdownIdempotent(t *testing.T) {
	m := newTestManager(t, Options{})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, m.Shutdown(ctx))
	require.NoError(t, m.Shutdown(ctx))
	require.NoError(t, m.Shutdown(ctx))
	assert.True(t, m.shuttingDown)
}

// TestShutdownKillsSubprocess: a running subprocess is killed by Shutdown.
func TestShutdownKillsSubprocess(t *testing.T) {
	m := newTestManager(t, Options{
		IdleTimeout: 30 * time.Second, // don't let idle timer race us
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, release, err := m.Acquire(ctx, "alpha")
	require.NoError(t, err)
	require.NotNil(t, c)

	cmd := m.cmd
	require.NotNil(t, cmd)
	require.True(t, processAlive(cmd), "subprocess should be alive before shutdown")

	release()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	require.NoError(t, m.Shutdown(shutdownCtx))

	// Subprocess should not be alive.
	assert.False(t, processAlive(cmd), "subprocess should be killed by Shutdown")
	assert.Equal(t, StateStopped, m.State())
}

// TestAcquireAfterShutdownReturnsErrShuttingDown: once Shutdown is called,
// further Acquire calls fail fast.
func TestAcquireAfterShutdownReturnsErrShuttingDown(t *testing.T) {
	m := newTestManager(t, Options{})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, m.Shutdown(ctx))

	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrShuttingDown))
}

// TestAcquireUnknownModelReturnsErrModelNotFound.
func TestAcquireUnknownModelReturnsErrModelNotFound(t *testing.T) {
	m := newTestManager(t, Options{})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := m.Acquire(ctx, "nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrModelNotFound))
}

// TestCtxCancelledWhileLoading: a cancelled ctx surfaced during the load
// returns ctx.Err().
func TestCtxCancelledWhileLoading(t *testing.T) {
	// Slow spawn that never finishes health.
	slowSpawn, _, cleanup := fakeSpawnSleep(t)
	t.Cleanup(cleanup)
	health := func(ctx context.Context, port int, timeout time.Duration) error {
		// Honor ctx — return immediately when cancelled.
		<-ctx.Done()
		return ctx.Err()
	}
	m := newTestManager(t, Options{
		SpawnHook:      slowSpawn,
		HealthHook:     health,
		StartupTimeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := m.Acquire(ctx, "alpha")
	require.Error(t, err)
	// Either ctx.DeadlineExceeded or the wrapped spawn timeout.
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrSpawnTimeout),
		"got %v", err)
}

// TestStateStrings covers the State.String() method.
func TestStateStrings(t *testing.T) {
	assert.Equal(t, "stopped", StateStopped.String())
	assert.Equal(t, "loading", StateLoading.String())
	assert.Equal(t, "ready", StateReady.String())
	assert.Equal(t, "crashed", StateCrashed.String())
	assert.Equal(t, "unknown", State(99).String())
}

// TestLimitedBufferBehavior covers the stderr ring buffer.
func TestLimitedBufferBehavior(t *testing.T) {
	b := newLimitedBuffer(16)
	_, _ = b.Write([]byte("abc"))
	assert.Equal(t, "abc", b.String())

	_, _ = b.Write([]byte("defghijklmnopqrstuvwxyz"))
	// Total written = 3+23 = 26 bytes; keep last 16.
	assert.Equal(t, "klmnopqrstuvwxyz", b.String())

	// Single big write.
	b2 := newLimitedBuffer(8)
	_, _ = b2.Write([]byte("0123456789ABCDEF"))
	assert.Equal(t, "89ABCDEF", b2.String())
}

// TestErrCrashedFormatting: a populated ErrCrashed has both exit and tail.
func TestErrCrashedFormatting(t *testing.T) {
	e := &ErrCrashed{
		ModelID:    "alpha",
		ExitErr:    errors.New("exit status 1"),
		StderrTail: "address already in use\n",
	}
	s := e.Error()
	assert.Contains(t, s, "alpha")
	assert.Contains(t, s, "exit status 1")
	assert.Contains(t, s, "address already in use")
	assert.Equal(t, "exit status 1", errors.Unwrap(e).Error())
}

// TestErrIntegrityFailStructIsSentinel: errors.Is(err, ErrIntegrityFail)
// works for the structured form.
func TestErrIntegrityFailStructIsSentinel(t *testing.T) {
	e := &ErrIntegrityFailStruct{Path: "/x", Expected: "a", Actual: "b"}
	assert.True(t, errors.Is(e, ErrIntegrityFail))
}

// TestNewRequiresCatalog: production constructor rejects nil catalog.
func TestNewRequiresCatalog(t *testing.T) {
	_, err := NewWithCatalog(nil)
	require.Error(t, err)
}

// TestNewWithOptionsRejectsBadCacheDir: cache dir must be absolute.
func TestNewWithOptionsRejectsBadCacheDir(t *testing.T) {
	_, err := NewWithOptions(Options{CacheDir: "relative/path"})
	require.Error(t, err)
}
