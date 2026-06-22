// Package llama wraps a local llama.cpp subprocess: lifecycle management,
// binary discovery/download, OpenAI-compatible HTTP client, and integrity
// verification.
//
// This file implements the lifecycle state machine. The interface (struct
// names, method signatures, exported types) was pre-defined in the contract
// phase and is locked; Track C only fills in the bodies and adds unexported
// fields/helpers.
//
// State machine summary (see PLAN-v2.md "Corrected subprocess lifecycle"):
//
//	stopped ──Acquire──▶ loading ──health ok──▶ ready
//	   ▲                   │                       │
//	   │                   └──spawn fail──▶ crashed
//	   │                                           │
//	   └────Shutdown / idle-kill ◀────refcount=0──┘
//	               │
//	               └──watcher sees exit──▶ crashed (if not Shutdown)
//
// All state transitions happen under m.mu. Concurrent Acquire calls serialize
// via m.cond.Wait() / m.cond.Broadcast().
package llama

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/models"
)

// State is the lifecycle state of the managed subprocess.
type State int

const (
	StateStopped State = iota
	StateLoading
	StateReady
	StateCrashed
)

// String returns a human-readable state name for logs and the doctor command.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateLoading:
		return "loading"
	case StateReady:
		return "ready"
	case StateCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

// ErrNotImplemented is returned by stub functions during the contract phase.
// Retained for backwards compatibility; production code does not return it.
var ErrNotImplemented = errors.New("not implemented")

// ErrIntegrityFail is returned when a model file fails SHA256 verification.
// Track C's structured form is *ErrIntegrityFailStruct; this sentinel is
// kept for callers that prefer errors.Is checks.
var ErrIntegrityFail = errors.New("model integrity check failed")

// ErrSpawnTimeout is returned when the subprocess does not become healthy
// within the configured startup window.
var ErrSpawnTimeout = errors.New("subprocess failed to become healthy in time")

// ErrModelNotFound is returned by Acquire when the requested model ID is not
// in the catalog.
var ErrModelNotFound = errors.New("model not found in catalog")

// ErrShuttingDown is returned by Acquire when Shutdown has been called.
var ErrShuttingDown = errors.New("lifecycle manager is shutting down")

// ErrBinaryUnavailable is returned when no usable llama-server binary can
// be found or downloaded.
var ErrBinaryUnavailable = errors.New("llama-server binary unavailable")

// Default config knobs. The lifecycle accepts an Options struct so tests
// can shrink these.
const (
	defaultIdleTimeout       = 5 * time.Minute
	defaultStartupTimeout    = 2 * time.Minute
	defaultGracefulTimeout   = 10 * time.Second
	defaultSIGKILLAfter      = 5 * time.Second
	defaultPortRetryAttempts = 3
	maxPortRetryAttempts     = 5
	defaultModelsSubdir      = "models"
	defaultBinSubdir         = "bin"
	defaultCacheDirName      = ".localvision"
)

// Options configures the LifecycleManager. The zero value is not usable;
// use New() which fills in defaults, or NewWithOptions for tests.
type Options struct {
	// Catalog supplies ModelSpec by ID. Required.
	Catalog *models.Catalog
	// CacheDir is the root for models/ and bin/. Defaults to ~/.localvision.
	CacheDir string
	// IdleTimeout is how long after refcount hits 0 to keep the subprocess
	// alive. Default 5m.
	IdleTimeout time.Duration
	// StartupTimeout is how long to wait for /health after spawn. Default 2m.
	StartupTimeout time.Duration
	// GracefulTimeout is the SIGTERM grace window during Shutdown. Default 10s.
	GracefulTimeout time.Duration
	// SIGKILLAfter is how long after SIGTERM to wait before SIGKILL. Default 5s.
	SIGKILLAfter time.Duration
	// PortRetryAttempts is how many times to re-sample a port on bind
	// failure. Default 3.
	PortRetryAttempts int
	// BinaryPath overrides binary discovery (tests inject fakes).
	BinaryPath string
	// SkipBinaryDiscovery short-circuits findOrDownloadBinary. Used by
	// tests that inject BinaryPath directly.
	SkipBinaryDiscovery bool
	// BinarySHA256 pins the binary's expected hash. Defaults to
	// pinnedLLAMAServerSHA256 in binary.go. Set to "" to disable.
	BinarySHA256 string
	// Logger is the slog.Logger used for diagnostic output. Defaults to
	// slog.Default().
	Logger *slog.Logger

	// Test hooks. When non-nil, these replace the production spawn / health
	// / binary-resolution code paths. Tests use them to inject fakes that
	// don't require an actual llama-server binary or model file on disk.
	// Production code leaves them nil.

	// SpawnHook, if non-nil, replaces spawnSubprocess. Must return a started
	// subprocess (or an error). Used by tests to inject `sleep` / `false`.
	SpawnHook func(ctx context.Context, opts spawnOptions) (*spawnResult, error)
	// HealthHook, if non-nil, replaces waitForHealth. Used by tests to skip
	// the HTTP probing (the fake subprocess doesn't serve /health).
	HealthHook func(ctx context.Context, port int, timeout time.Duration) error
	// ResolveBinaryHook, if non-nil, replaces findOrDownloadBinary.
	// Returns the path to use as the binary. The path is NOT validated
	// against BinDir when this hook is set.
	ResolveBinaryHook func(ctx context.Context, pinnedSHA256, binDir string) (string, error)
	// VerifyHashHook, if non-nil, replaces models.VerifySHA256 for the
	// GGUF and mmproj files. Used by tests to skip the actual hashing.
	VerifyHashHook func(ctx context.Context, path, expectedHex string) error
}

// LifecycleManager owns a single llama-server subprocess and serializes
// load/unload transitions. At most one model is loaded at a time.
//
// Implementations must be safe for concurrent use: multiple goroutines may
// call Acquire simultaneously, and the manager must serialize model switches
// without deadlocking. The active-inference refcount prevents the idle timer
// from killing the process mid-request.
type LifecycleManager struct {
	opts Options

	mu    sync.Mutex
	cond  *sync.Cond // broadcast on state change; wraps m.mu

	state          State
	loadedModelID  string
	activeRefcount int
	cmd            *exec.Cmd
	port           int
	stderr         *limitedBuffer

	// spawnCtx / spawnCancel are the ctx passed to exec.CommandContext.
	// They are independent of any individual Acquire ctx: subprocess lifetime
	// spans multiple Acquire calls; only Shutdown cancels them. This is
	// important because exec.CommandContext sends SIGKILL on ctx cancel,
	// and we don't want individual Acquire returns to kill the subprocess.
	spawnCtx    context.Context
	spawnCancel context.CancelFunc

	idleTimer *time.Timer

	// shuttingDown is set by Shutdown to gate further Acquire calls.
	shuttingDown bool

	// crashErr is the most recent crash error captured by the watcher.
	crashErr error

	// loadErr is the error from the most recent spawn attempt; cleared on
	// the next successful Acquire.
	loadErr error

	// binaryPath is the resolved llama-server path, cached after first
	// discovery so we don't re-stat it on every Acquire.
	binaryPath     string
	binaryResolved bool

	// modelsDir and binDir are resolved from opts.CacheDir.
	modelsDir string
	binDir    string

	logger *slog.Logger
}

// New creates a LifecycleManager with default options and an empty catalog.
// Tests should use NewWithOptions to inject fakes; production code should
// use NewWithCatalog.
func New() (*LifecycleManager, error) {
	return NewWithOptions(Options{})
}

// NewWithCatalog is the production constructor. catalog must be non-nil.
func NewWithCatalog(catalog *models.Catalog) (*LifecycleManager, error) {
	if catalog == nil {
		return nil, errors.New("catalog is required")
	}
	return NewWithOptions(Options{Catalog: catalog})
}

// NewWithOptions constructs a LifecycleManager from explicit Options.
// Default values are applied for zero-valued fields.
func NewWithOptions(opts Options) (*LifecycleManager, error) {
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = defaultIdleTimeout
	}
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = defaultStartupTimeout
	}
	if opts.GracefulTimeout <= 0 {
		opts.GracefulTimeout = defaultGracefulTimeout
	}
	if opts.SIGKILLAfter <= 0 {
		opts.SIGKILLAfter = defaultSIGKILLAfter
	}
	if opts.PortRetryAttempts <= 0 {
		opts.PortRetryAttempts = defaultPortRetryAttempts
	}
	if opts.PortRetryAttempts > maxPortRetryAttempts {
		opts.PortRetryAttempts = maxPortRetryAttempts
	}
	if opts.BinarySHA256 == "" {
		opts.BinarySHA256 = pinnedLLAMAServerSHA256
	}

	cacheDir := opts.CacheDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		cacheDir = filepath.Join(home, defaultCacheDirName)
	}
	cacheDir = filepath.Clean(cacheDir)
	if !filepath.IsAbs(cacheDir) {
		return nil, fmt.Errorf("cache dir %q must be absolute", cacheDir)
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	m := &LifecycleManager{
		opts:      opts,
		state:     StateStopped,
		modelsDir: filepath.Join(cacheDir, defaultModelsSubdir),
		binDir:    filepath.Join(cacheDir, defaultBinSubdir),
		logger:    logger,
	}
	m.cond = sync.NewCond(&m.mu)
	// spawnCtx is rooted at the manager's lifetime, not at any individual
	// Acquire. Cancelled only by Shutdown.
	m.spawnCtx, m.spawnCancel = context.WithCancel(context.Background())
	return m, nil
}

// Acquire ensures the given model is loaded and returns a Client that can
// serve requests for it.
//
// Concurrent calls block on an internal mutex until the right model is
// loaded. If a different model is currently loaded, it is unloaded first.
// The returned release function MUST be called when the caller is done; it
// decrements the active-inference refcount. When refcount reaches zero, the
// idle timer becomes eligible to fire.
//
// ctx cancellation is propagated: if the ctx is cancelled while waiting for
// a model to load, Acquire returns ctx.Err(). Note that ctx cancellation
// does NOT kill the underlying subprocess — only Shutdown does that.
func (m *LifecycleManager) Acquire(ctx context.Context, modelID string) (c *Client, release func(), err error) {
	if m == nil {
		return nil, nil, errors.New("nil lifecycle manager")
	}

	// Watch for ctx cancellation while we hold the lock. We do this with a
	// goroutine that broadcasts on the cond, waking us up.
	ctxCancelled := make(chan struct{})
	defer close(ctxCancelled)
	stopCtxWatch := m.startCtxWatch(ctx, ctxCancelled)
	defer stopCtxWatch()

	m.mu.Lock()
	defer m.mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if m.shuttingDown {
			return nil, nil, ErrShuttingDown
		}

		// Crash recovery: clear crashed state, respawn on next iteration.
		if m.state == StateCrashed {
			m.state = StateStopped
			m.cmd = nil
			m.loadedModelID = ""
			m.crashErr = nil
		}

		// Same model already loaded?
		if m.state == StateReady && m.loadedModelID == modelID {
			m.activeRefcount++
			m.stopIdleTimerLocked()
			port := m.port
			return newClient(port), m.releaseFn(), nil
		}

		// Someone else is loading — wait.
		if m.state == StateLoading {
			m.cond.Wait()
			continue
		}

		// Different model loaded, or stopped — load the requested one.
		if m.state == StateReady && m.loadedModelID != modelID {
			// Unload the currently-loaded model first.
			if err := m.unloadLocked(ctx); err != nil {
				m.logger.Warn("unload previous model failed", "model", m.loadedModelID, "err", err)
				// Continue: we'll try to spawn the new one anyway.
			}
		}

		// Load it. We pass m.spawnCtx (not ctx) so the subprocess survives
		// this Acquire returning.
		if err := m.loadLocked(ctx, m.spawnCtx, modelID); err != nil {
			m.loadErr = err
			return nil, nil, err
		}

		// Loop back; state should be StateReady now.
	}
}

// startCtxWatch spawns a goroutine that broadcasts on m.cond when ctx is
// cancelled. Returns a stop function that must be deferred by the caller.
// This is how a ctx cancel wakes an Acquire blocked on cond.Wait().
func (m *LifecycleManager) startCtxWatch(ctx context.Context, done <-chan struct{}) func() {
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			// Wake anyone blocked on cond.Wait so they re-check ctx.Err().
			m.mu.Lock()
			m.cond.Broadcast()
			m.mu.Unlock()
		case <-done:
		case <-stop:
		}
	}()
	return func() { close(stop) }
}

// loadLocked spawns the subprocess for modelID, waits for /health, and flips
// state to StateReady. Caller MUST hold m.mu.
//
// acquireCtx is the caller's ctx (used for health probing and integrity
// verification; cancelled when the caller gives up). spawnCtx is the
// subprocess's lifetime ctx (cancelled only by Shutdown).
func (m *LifecycleManager) loadLocked(acquireCtx, spawnCtx context.Context, modelID string) error {
	if m.opts.Catalog == nil {
		return errors.New("no catalog configured")
	}
	spec, ok := m.opts.Catalog.Models[modelID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrModelNotFound, modelID)
	}

	m.state = StateLoading
	m.loadedModelID = modelID
	m.cond.Broadcast()

	// Resolve the binary path once.
	if !m.binaryResolved {
		if m.opts.SkipBinaryDiscovery || m.opts.BinaryPath != "" {
			m.binaryPath = m.opts.BinaryPath
		} else if m.opts.ResolveBinaryHook != nil {
			path, err := m.opts.ResolveBinaryHook(acquireCtx, m.opts.BinarySHA256, m.binDir)
			if err != nil {
				m.state = StateCrashed
				m.cond.Broadcast()
				return fmt.Errorf("%w: %v", ErrBinaryUnavailable, err)
			}
			m.binaryPath = path
		} else {
			path, err := findOrDownloadBinary(acquireCtx, m.opts.BinarySHA256, m.binDir)
			if err != nil {
				m.state = StateCrashed
				m.cond.Broadcast()
				return fmt.Errorf("%w: %v", ErrBinaryUnavailable, err)
			}
			m.binaryPath = path
		}
		m.binaryResolved = true
	}

	// Resolve local file paths from the URLs in the spec. Downloader
	// convention: destPath = modelsDir/<basename>.
	ggufLocal := filepath.Join(m.modelsDir, filepath.Base(spec.GGUF))
	mmprojLocal := ""
	if spec.Mmproj != "" {
		mmprojLocal = filepath.Join(m.modelsDir, filepath.Base(spec.Mmproj))
	}

	// Ensure model files are present. The Downloader is a no-op if the
	// file already exists with the correct SHA256, so this is safe to call
	// on every Acquire (it caches the hash result internally).
	if m.opts.VerifyHashHook == nil {
		// Real download path (skipped in tests via VerifyHashHook).
		if err := os.MkdirAll(m.modelsDir, 0o755); err != nil {
			m.state = StateCrashed
			m.cond.Broadcast()
			return fmt.Errorf("mkdir models dir %s: %w", m.modelsDir, err)
		}
		d := &models.Downloader{}
		m.logger.Info("ensuring model files present", "model", modelID, "gguf", ggufLocal)
		if err := d.Download(acquireCtx, spec.GGUF, ggufLocal, spec.GGUFSha256, nil); err != nil {
			m.state = StateCrashed
			m.cond.Broadcast()
			return fmt.Errorf("download gguf: %w", err)
		}
		if mmprojLocal != "" {
			if err := d.Download(acquireCtx, spec.Mmproj, mmprojLocal, spec.MmprojSha256, nil); err != nil {
				m.state = StateCrashed
				m.cond.Broadcast()
				return fmt.Errorf("download mmproj: %w", err)
			}
		}
	}

	// SHA256 verification on every load (F1.5).
	if m.opts.VerifyHashHook != nil {
		if err := m.opts.VerifyHashHook(acquireCtx, ggufLocal, spec.GGUFSha256); err != nil {
			m.state = StateCrashed
			m.cond.Broadcast()
			return fmt.Errorf("verify gguf: %w", err)
		}
		if mmprojLocal != "" {
			if err := m.opts.VerifyHashHook(acquireCtx, mmprojLocal, spec.MmprojSha256); err != nil {
				m.state = StateCrashed
				m.cond.Broadcast()
				return fmt.Errorf("verify mmproj: %w", err)
			}
		}
	} else {
		if err := models.VerifySHA256(acquireCtx, ggufLocal, spec.GGUFSha256); err != nil {
			m.state = StateCrashed
			m.cond.Broadcast()
			return fmt.Errorf("verify gguf: %w", err)
		}
		if mmprojLocal != "" {
			if err := models.VerifySHA256(acquireCtx, mmprojLocal, spec.MmprojSha256); err != nil {
				m.state = StateCrashed
				m.cond.Broadcast()
				return fmt.Errorf("verify mmproj: %w", err)
			}
		}
	}

	// Spawn with port-retry loop (F4.7).
	spawn := spawnSubprocess
	if m.opts.SpawnHook != nil {
		spawn = m.opts.SpawnHook
	}
	health := waitForHealth
	if m.opts.HealthHook != nil {
		health = m.opts.HealthHook
	}

	var result *spawnResult
	var lastErr error
	for attempt := 0; attempt < m.opts.PortRetryAttempts; attempt++ {
		result, lastErr = spawn(spawnCtx, spawnOptions{
			BinaryPath: m.binaryPath,
			BinDir:     m.binDir,
			GGUFPath:   ggufLocal,
			MmprojPath: mmprojLocal,
			ModelsDir:  m.modelsDir,
			Spec:       spec,
		})
		if lastErr == nil {
			break
		}
		m.logger.Warn("llama-server spawn failed; retrying",
			"attempt", attempt+1, "model", modelID, "err", lastErr)
	}

	if result == nil || lastErr != nil {
		m.state = StateCrashed
		m.cond.Broadcast()
		return fmt.Errorf("spawn llama-server after %d attempts: %w",
			m.opts.PortRetryAttempts, lastErr)
	}

	m.cmd = result.cmd
	m.port = result.port
	m.stderr = result.stderr

	// Start the watcher goroutine immediately after Start. F3.7.
	watcherDone := make(chan struct{})
	go m.watchSubprocess(result.cmd, watcherDone)

	// Poll /health. F3.6: ctx-aware.
	if err := health(acquireCtx, result.port, m.opts.StartupTimeout); err != nil {
		// Health failed. Kill the subprocess, flip to crashed.
		m.killSubprocessLocked(spawnCtx, result.cmd)
		// Wait for the watcher to observe the exit and reap the process.
		m.mu.Unlock()
		<-watcherDone
		m.mu.Lock()
		m.state = StateCrashed
		m.cond.Broadcast()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrSpawnTimeout, err)
	}

	m.state = StateReady
	m.cond.Broadcast()
	return nil
}

// watchSubprocess calls cmd.Wait() and updates state on exit. F3.7.
// It is the single reaper of the subprocess.
//
// On unexpected exit (state != StateStopped at exit time), flips state to
// StateCrashed and captures the last 1KB of stderr.
func (m *LifecycleManager) watchSubprocess(cmd *exec.Cmd, done chan<- struct{}) {
	defer close(done)
	waitErr := cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	// If state is already StateStopped, we initiated the shutdown; nothing
	// to do. If it's StateLoading or StateReady, this was an unexpected
	// exit — capture crash context.
	if m.state == StateStopped {
		return
	}

	var tail string
	if m.stderr != nil {
		tail = m.stderr.String()
	}
	m.crashErr = &ErrCrashed{
		ModelID:    m.loadedModelID,
		ExitErr:    waitErr,
		StderrTail: tail,
	}
	m.state = StateCrashed
	m.loadedModelID = ""
	m.cmd = nil
	m.cond.Broadcast()
}

// unloadLocked gracefully kills the current subprocess. Caller MUST hold m.mu.
// Releases m.mu while waiting for the watcher goroutine, then re-acquires.
func (m *LifecycleManager) unloadLocked(ctx context.Context) error {
	if m.cmd == nil || m.state == StateStopped {
		m.state = StateStopped
		return nil
	}
	cmd := m.cmd
	// Flip to StateStopped BEFORE signaling so the watcher knows this is
	// an expected exit.
	m.state = StateStopped
	m.killSubprocessLocked(ctx, cmd)
	return nil
}

// killSubprocessLocked sends SIGTERM, waits up to SIGKILLAfter, then SIGKILL.
// Caller MUST hold m.mu (we briefly release during the wait). State must
// already be flipped to StateStopped (for graceful) or the caller accepts
// the crashed interpretation by the watcher.
func (m *LifecycleManager) killSubprocessLocked(ctx context.Context, cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// SIGTERM
	_ = sigterm(cmd)

	// Wait up to SIGKILLAfter in a goroutine-safe way. We release the lock
	// so the watcher goroutine can run and reap.
	done := make(chan struct{})
	go func() {
		// We don't cmd.Wait() here — the watcher is the single reaper.
		// Poll process liveness instead.
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if !processAlive(cmd) {
				close(done)
				return
			}
			select {
			case <-ctx.Done():
				close(done)
				return
			default:
			}
		}
	}()

	// Wait briefly (SIGKILLAfter) for the process to exit on SIGTERM.
	m.mu.Unlock()
	select {
	case <-done:
	case <-time.After(m.opts.SIGKILLAfter):
	case <-ctx.Done():
	}
	m.mu.Lock()

	if processAlive(cmd) {
		_ = sigkill(cmd)
		// Give the watcher a moment to observe the SIGKILL.
		m.mu.Unlock()
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
		}
		m.mu.Lock()
	}
}

// releaseFn returns a closure that decrements the active-inference refcount
// and arms the idle timer when refcount reaches zero. The returned function
// is safe to call multiple times (subsequent calls are no-ops).
func (m *LifecycleManager) releaseFn() func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.activeRefcount > 0 {
				m.activeRefcount--
			}
			if m.activeRefcount == 0 && m.state == StateReady && !m.shuttingDown {
				m.startIdleTimerLocked()
			}
		})
	}
}

// startIdleTimerLocked arms the idle kill timer. Caller MUST hold m.mu.
// The timer fires only when refcount is still zero at fire time.
func (m *LifecycleManager) startIdleTimerLocked() {
	m.stopIdleTimerLocked()
	m.idleTimer = time.AfterFunc(m.opts.IdleTimeout, m.killIfIdle)
}

// stopIdleTimerLocked disarms the idle kill timer if armed. Caller MUST hold m.mu.
func (m *LifecycleManager) stopIdleTimerLocked() {
	if m.idleTimer != nil {
		m.idleTimer.Stop()
		m.idleTimer = nil
	}
}

// killIfIdle is the idle-timer callback. It checks refcount under the lock
// before killing — F1.9: an Acquire that arrives between timer-fire and
// process-kill must abort the kill.
func (m *LifecycleManager) killIfIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown {
		return
	}
	if m.activeRefcount > 0 {
		// New Acquire raced with us; abort.
		return
	}
	if m.state != StateReady {
		return
	}
	m.logger.Debug("idle timer fired; killing llama-server",
		"model", m.loadedModelID)
	ctx, cancel := context.WithTimeout(context.Background(), m.opts.SIGKILLAfter*2)
	defer cancel()
	m.unloadLocked(ctx)
}

// Shutdown gracefully stops the active-inference watcher, sends SIGTERM to
// the subprocess, waits up to the configured timeout, escalates to SIGKILL
// if still alive, and returns.
//
// Safe to call multiple times; subsequent calls are no-ops. F3.8 ordering:
//  1. Stop accepting new Acquire calls (set shuttingDown)
//  2. Send SIGTERM
//  3. Wait up to GracefulTimeout
//  4. SIGKILL if still alive
//  5. Return
//
// In-flight Acquire/ChatVision calls are NOT cancelled by Shutdown — they
// hold a release() that the caller must invoke. The lifecycle just stops
// spawning new subprocesses.
func (m *LifecycleManager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	// Idempotent: subsequent calls are no-ops once shuttingDown is set.
	if m.shuttingDown {
		m.mu.Unlock()
		return nil
	}
	m.shuttingDown = true
	m.stopIdleTimerLocked()

	if m.cmd == nil || m.state == StateStopped {
		m.state = StateStopped
		// Cancel the spawn ctx so any in-flight exec.CommandContext kills
		// the subprocess (defensive; usually cmd is nil here).
		if m.spawnCancel != nil {
			m.spawnCancel()
		}
		m.mu.Unlock()
		return nil
	}

	cmd := m.cmd
	// Flip state to StateStopped so the watcher treats the exit as expected.
	m.state = StateStopped
	m.cond.Broadcast()
	// Cancel the spawn ctx as well; this is a defensive belt-and-suspenders
	// in case the SIGTERM doesn't take.
	if m.spawnCancel != nil {
		m.spawnCancel()
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, m.opts.GracefulTimeout)
	defer cancel()

	m.killSubprocessLocked(shutdownCtx, cmd)
	m.mu.Unlock()
	return nil
}

// State returns the current lifecycle state for observability/diagnostics.
// Used by the doctor command.
func (m *LifecycleManager) State() State {
	if m == nil {
		return StateStopped
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Client is a thin HTTP client that talks to one running llama-server
// subprocess. A Client is only valid between the Acquire that returned it
// and the corresponding release call.
type Client struct {
	port int
	base string
}

// ChatRequest is a single vision-language chat completion request.
type ChatRequest struct {
	Model        string         // model ID as known to the registry (informational)
	SystemPrompt string         // task-tuned system prompt from the tool
	UserPrompt   string         // user-turn prompt from the tool
	ImagePaths   []string       // absolute local paths to image files
	MaxTokens    int            // per-tool output budget
	Temperature  float64        // usually 0.1 for deterministic output
	// ChatTemplateKwargs is forwarded as `chat_template_kwargs` in the
	// request body. Populated from ModelSpec.ChatTemplateKwargs by the
	// executor before calling ChatVision. Empty = no kwargs sent.
	ChatTemplateKwargs map[string]any
}

// ChatResponse holds the model's reply plus accounting fields.
type ChatResponse struct {
	Content   string // raw text from the model
	TokensIn  int    // prompt tokens (incl. image tokens) reported by server
	TokensOut int    // generated tokens reported by server
	ElapsedMs int64  // wall-clock inference time
}
