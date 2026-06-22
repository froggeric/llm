package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/froggeric/llm/mcp/localvision/internal/llama"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/tools"
	"github.com/froggeric/llm/mcp/localvision/internal/version"
)

// shutdownTimeout bounds how long Run waits for in-flight tool calls to
// drain before forcing shutdown. Per F3.8: 10 seconds.
const shutdownTimeout = 10 * time.Second

// Server is the localvision MCP server. It owns a single underlying
// *mcp.Server from the official Go SDK plus a snapshot of the dependencies
// (registry, executor, lifecycle) used to dispatch tools/call.
//
// Construct via NewServer; run via Run.
type Server struct {
	mcp       *mcp.Server
	logger    *slog.Logger
	tools     []tools.Tool
	executor  tools.Executor
	lifecycle *llama.LifecycleManager

	// inFlight tracks the number of tool handlers currently running.
	// Graceful shutdown waits for this to reach zero (or the shutdown
	// timeout to fire) before tearing down the subprocess.
	inFlightWG sync.WaitGroup
}

// Dependencies are the collaborators NewServer wires together.
//
// All fields are optional except as noted:
//
//   - Logger: if nil, slog.Default() is used.
//   - Lifecycle: if nil, llama.New() is called. Required for the
//     production CatalogExecutor to function, but optional if Executor
//     is supplied directly (e.g. tests).
//   - Catalog / Hardware: passed to the default CatalogExecutor.
//     Ignored if Executor is non-nil.
//   - Executor: if non-nil, used as the tools.Executor for every tool
//     call. Use this in tests to inject a mock; in production, leave
//     nil and the server builds a CatalogExecutor from Lifecycle +
//     Catalog + Hardware.
//   - Registry: if non-nil, its All() is used as the tool list.
//   - Tools: if non-empty, used as the tool list directly. Use this in
//     tests where the registry hasn't been populated yet. If both
//     Registry and Tools are set, Tools wins.
//
// If neither Registry nor Tools is set, NewServer calls tools.NewRegistry()
// itself; until Track E lands this returns an empty list, which is fine
// for first-run (tools/list returns zero tools, no calls can succeed).
type Dependencies struct {
	Logger    *slog.Logger
	Lifecycle *llama.LifecycleManager
	Catalog   *models.Catalog
	Hardware  models.HardwareInfo
	Executor  tools.Executor
	Registry  *tools.Registry
	Tools     []tools.Tool
}

// NewServer wires together the SDK server, tool registry, executor, and
// lifecycle. It registers every tool from the registry/Tools against the
// SDK so they show up in tools/list.
//
// Returns an error only if the underlying mcp.NewServer call would fail
// (currently it never does) or if Logger / Lifecycle resolution fails.
func NewServer(deps Dependencies) (*Server, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Resolve the lifecycle manager. In production, deps.Lifecycle comes
	// from main.go (which calls llama.New() once at startup). For tests
	// and the first-run path, we leave it nil — the executor surfaces a
	// clear "first-run setup required" error if any tool call is attempted.
	lifecycle := deps.Lifecycle

	// Resolve the tool list. Tests pass Tools directly; production passes
	// a Registry whose All() returns 9 tools once Track E lands.
	toolList := deps.Tools
	if len(toolList) == 0 {
		if deps.Registry != nil {
			toolList = deps.Registry.All()
		} else {
			toolList = tools.NewRegistry().All()
		}
	}

	// Resolve the executor. Production: CatalogExecutor. Tests: a mock.
	executor := deps.Executor
	if executor == nil {
		executor = NewCatalogExecutor(deps.Catalog, lifecycle, deps.Hardware, logger)
	}

	// Build the underlying SDK server. The Implementation's Version comes
	// from internal/version (set via -ldflags at release time).
	impl := &mcp.Implementation{
		Name:    "localvision",
		Version: version.Version,
	}
	opts := &mcp.ServerOptions{
		Logger:       logger,
		Instructions: "Local vision tools backed by a local llama.cpp server. Each call loads a vision-language model and runs one inference. Tool descriptions include expected latency; budget 30-60 seconds per call.",
		// KeepAlive: leave zero — stdio transport doesn't need pings.
	}
	mcpServer := mcp.NewServer(impl, opts)

	srv := &Server{
		mcp:       mcpServer,
		logger:    logger,
		tools:     toolList,
		executor:  executor,
		lifecycle: lifecycle,
	}

	// Register every tool. Each tool gets its own ToolHandler closure
	// that captures the tool reference and dispatches to srv.callTool.
	for _, t := range toolList {
		srv.registerTool(t)
	}

	logger.Info("mcp server constructed",
		"tool_count", len(toolList),
		"version", version.Version,
	)
	return srv, nil
}

// ToolCount returns the number of tools currently registered. Mainly for
// diagnostics and tests.
func (s *Server) ToolCount() int { return len(s.tools) }

// Run starts the server on the stdio transport and blocks until the
// context is cancelled, the client disconnects, or an unrecoverable
// error occurs.
//
// On shutdown (ctx cancellation, SIGTERM, or SIGINT), Run performs the
// graceful-shutdown sequence from F3.8:
//
//  1. The SDK stops accepting new requests (its Run returns).
//  2. We wait up to shutdownTimeout (10s) for in-flight tool calls to
//     finish; in parallel, we send SIGTERM to the llama-server
//     subprocess via lifecycle.Shutdown.
//  3. We return.
//
// SIGTERM/SIGINT handling: Run installs its own signal handler so the
// process exits cleanly even if the caller did not set up signal
// handling. The handler cancels the internal run context. If the caller
// passes a ctx that is already wired to signals, that's fine — both
// paths converge.
func (s *Server) Run(ctx context.Context) error {
	// Install SIGTERM/SIGINT handler. We cancel runCtx on signal.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go func() {
		select {
		case sig := <-sigCh:
			s.logger.Info("signal received, initiating graceful shutdown", "signal", sig.String())
			cancel()
		case <-runCtx.Done():
			// Normal cancellation path; nothing to do.
		}
	}()

	s.logger.Info("mcp server starting on stdio transport")
	err := s.mcp.Run(runCtx, &mcp.StdioTransport{})
	if err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("mcp server run returned error", "error", err)
	}

	// Graceful shutdown: wait for in-flight tool calls, then stop the
	// subprocess. We run these in parallel because the lifecycle shutdown
	// may need to interrupt a stuck subprocess while tool calls drain.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	drained := make(chan struct{})
	go func() {
		s.inFlightWG.Wait()
		close(drained)
	}()

	lifecycleDone := make(chan error, 1)
	if s.lifecycle != nil {
		go func() {
			lifecycleDone <- s.lifecycle.Shutdown(shutdownCtx)
		}()
	} else {
		close(lifecycleDone) // already closed: no-op send below
	}

	select {
	case <-drained:
		s.logger.Info("in-flight tool calls drained")
	case <-shutdownCtx.Done():
		s.logger.Warn("graceful shutdown timeout reached; some in-flight calls may have been interrupted",
			"timeout", shutdownTimeout)
	}

	if s.lifecycle != nil {
		if lcErr := <-lifecycleDone; lcErr != nil {
			s.logger.Error("lifecycle shutdown returned error", "error", lcErr)
			// Don't clobber the run error if there was one.
			if err == nil {
				err = fmt.Errorf("lifecycle shutdown: %w", lcErr)
			}
		}
	}

	s.logger.Info("mcp server shutdown complete")
	return err
}
