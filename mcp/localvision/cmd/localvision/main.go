// localvision is a Claude Code (and any-MCP-client) plugin that wraps a
// local llama.cpp subprocess to provide vision-language model tools to text-only
// coding LLMs.
//
// Usage:
//
//	localvision run [--config PATH] [--verbose] [--log-file PATH]
//	localvision doctor [--config PATH] [--verbose]
//	localvision version
//	localvision --help
//
// The default subcommand is `run`, which starts the MCP server on stdio and
// blocks until the client disconnects or the process is signalled.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/llama"
	"github.com/froggeric/llm/mcp/localvision/internal/logging"
	"github.com/froggeric/llm/mcp/localvision/internal/mcpserver"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/version"
)

const (
	exitOK          = 0
	exitGeneric     = 1
	exitBadArgs     = 2
	exitUnsetConfig = 3
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		args = []string{"run"}
	}

	switch args[0] {
	case "-h", "--help", "help":
		printHelp(os.Stdout)
		return exitOK
	case "version":
		// `version` ignores other flags so it works without a config.
		fmt.Printf("localvision %s (commit %s, built %s)\n", version.Version, version.GitCommit, version.BuildDate)
		return exitOK
	case "run":
		return runSubcommand(args[1:])
	case "doctor":
		return doctorSubcommand(args[1:])
	default:
		// Not a known subcommand: treat the arg list as a one-shot image query
		// (positional image(s) + flags like --type). Supports `localvision img.png`
		// and `localvision img.png --type ocr`. (No-args still runs the MCP server
		// for now; the TUI setup lands in Phase 4.)
		return runOneShot(args)
	}
}

// commonFlags carries the flags shared by `run` and `doctor`.
type commonFlags struct {
	configPath string
	verbose    bool
	logFile    string
	cacheDir   string
	modelsDir  string
}

func (f *commonFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.configPath, "config", "", "path to config.toml (default: ~/.localvision/config.toml)")
	fs.BoolVar(&f.verbose, "verbose", false, "enable debug-level logging")
	fs.StringVar(&f.logFile, "log-file", "", "also write structured logs to this path")
	fs.StringVar(&f.cacheDir, "cache-dir", "", "override the cache dir (config cache_dir); redirects ALL storage including multi-GB models")
	fs.StringVar(&f.modelsDir, "models-dir", "", "override the models dir (config models_dir); where model files are stored")
}

func runSubcommand(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitBadArgs
	}

	cfg, logger, err := loadAndConfigure(cf, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitUnsetConfig
	}

	rt, err := bootstrap(cfg, logger, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitGeneric
	}

	srv, err := mcpserver.NewServer(mcpserver.Dependencies{
		Logger:    rt.logger,
		Lifecycle: rt.lifecycle,
		Catalog:   rt.catalog,
		Hardware:  rt.hw,
	})
	if err != nil {
		logger.Error("failed to construct MCP server", "error", err)
		return exitGeneric
	}

	// Wire process-level signals to server shutdown. The server installs
	// its own signal handler too, but this catches the case where the
	// caller's ctx is never cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case sig := <-sigCh:
			logger.Info("signal received", "signal", sig.String())
			cancel()
		case <-ctx.Done():
		}
	}()

	logger.Info("starting MCP server on stdio",
		"hardware_total_memory_gb", rt.hw.TotalMemoryGB,
		"hardware_tier", rt.hw.Tier,
		"hardware_backend", rt.hw.Backend,
		"cache_dir", cfg.CacheDir,
	)
	if err := srv.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("server exited with error", "error", err)
		return exitGeneric
	}
	return exitOK
}

func doctorSubcommand(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitBadArgs
	}

	cfg, logger, err := loadAndConfigure(cf, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
		return exitUnsetConfig
	}

	w := os.Stdout
	fmt.Fprintf(w, "localvision %s (commit %s, built %s)\n\n", version.Version, version.GitCommit, version.BuildDate)

	fmt.Fprintln(w, "=== Configuration ===")
	fmt.Fprintf(w, "Config file:    %s\n", orDefault(cfgPathDisplay(cf.configPath), "(default)"))
	fmt.Fprintf(w, "Cache dir:      %s\n", cfg.CacheDir)
	fmt.Fprintf(w, "Models dir:     %s (%s)\n", cfg.ModelsDir, models.DiskFreeHuman(cfg.ModelsDir))
	fmt.Fprintf(w, "Bin dir:        %s\n", cfg.BinDir)
	fmt.Fprintf(w, "Idle timeout:   %s\n", cfg.IdleTimeout)
	fmt.Fprintf(w, "Startup:        %s\n", cfg.StartupTimeout)
	fmt.Fprintf(w, "Safety margin:  %.1f GB\n", cfg.SafetyMarginGB)
	fmt.Fprintf(w, "HF user:        %s\n", cfg.HFUser)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== Hardware ===")
	hw, hwErr := models.DetectHardware()
	if hwErr != nil {
		fmt.Fprintf(w, "Detection FAILED: %v\n", hwErr)
	} else {
		fmt.Fprintf(w, "Total memory:   %.1f GB\n", hw.TotalMemoryGB)
		fmt.Fprintf(w, "Tier:           %s\n", hw.Tier)
		fmt.Fprintf(w, "Backend:        %s\n", hw.Backend)
		if hw.DetectNote != "" {
			fmt.Fprintf(w, "Note:           %s\n", hw.DetectNote)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== Catalog ===")
	catalog, catErr := models.Load("")
	if catErr != nil {
		fmt.Fprintf(w, "Load FAILED: %v\n", catErr)
	} else {
		fmt.Fprintf(w, "Schema version: %d\n", catalog.SchemaVersion)
		fmt.Fprintf(w, "Models:         %d\n", len(catalog.Models))
		if vErr := catalog.Validate(); vErr != nil {
			fmt.Fprintf(w, "Validation:     FAILED: %v\n", vErr)
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Catalog validation failures block tool execution. Fix the")
			fmt.Fprintln(w, "issues above (typically: missing SHA256 hashes, which are")
			fmt.Fprintln(w, "populated by the Phase 3 lead by downloading and hashing each")
			fmt.Fprintln(w, "model file) before reporting issues.")
		} else {
			fmt.Fprintln(w, "Validation:     OK")
		}
		if hwErr == nil {
			if def, err := catalog.DefaultModel(hw); err != nil {
				fmt.Fprintf(w, "Default model:  NONE FITS: %v\n", err)
			} else {
				fmt.Fprintf(w, "Default model:  %s\n", def)
			}
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Models:")
		for id, m := range catalog.Models {
			star := ""
			if m.Preferred {
				star = " *"
			}
			fmt.Fprintf(w, "  %s%s\n    %s — min_vram=%dGB, released=%s\n",
				id, star, m.DisplayName, m.MinVramGb, m.Released)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== llama-server binary ===")
	if path, err := lookPathLLAMAServer(); err == nil {
		fmt.Fprintf(w, "Discovered on $PATH: %s\n", path)
		fmt.Fprintln(w, "(used as-is; integrity not verified)")
	} else {
		fmt.Fprintln(w, "Not found on $PATH. Recommended: install llama.cpp")
		fmt.Fprintln(w, "(`brew install llama.cpp`) so `llama-server` is on $PATH.")
		fmt.Fprintln(w, "Otherwise the first tool call downloads a pinned release from")
		fmt.Fprintln(w, "https://github.com/ggml-org/llama.cpp/releases into the bin dir")
		fmt.Fprintln(w, "and verifies its SHA256 before extracting.")
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== Tools ===")
	// Use the same construction as the server so the count matches.
	srv, srvErr := mcpserver.NewServer(mcpserver.Dependencies{
		Logger:  logger,
		Catalog: catalog,
		Hardware: hw,
	})
	if srvErr != nil {
		fmt.Fprintf(w, "Construction FAILED: %v\n", srvErr)
	} else {
		fmt.Fprintf(w, "Registered:     %d\n", srv.ToolCount())
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "=== Logging ===")
	fmt.Fprintf(w, "Level:          %s\n", cfg.LogLevel)
	if cfg.LogFile != "" {
		fmt.Fprintf(w, "File:           %s (in addition to stderr)\n", cfg.LogFile)
	} else {
		fmt.Fprintln(w, "File:           (stderr only)")
	}
	if err := logging.CloseLogFile(); err != nil {
		logger.Warn("failed to close log file cleanly", "error", err)
	}

	return exitOK
}

// loadAndConfigure is shared by run and doctor. It loads the config (with
// sensible defaults if missing), sets up structured logging, and returns
// both. The returned logger writes to stderr (and an optional file) and is
// also installed as the slog default so libraries that call slog.Info
// without a logger argument use it.
func loadAndConfigure(cf commonFlags, quiet bool) (*config.Config, *slog.Logger, error) {
	cfg, err := config.Load(cf.configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	// CLI --cache-dir / --models-dir override the config (and re-derive
	// subordinate dirs). Lets users redirect multi-GB model storage off the
	// system drive without editing the TOML.
	cfg.ApplyDirOverrides(cf.cacheDir, cf.modelsDir, "")

	level := cfg.LogLevel
	if cf.verbose {
		level = "debug"
	}
	logFile := cfg.LogFile
	if cf.logFile != "" {
		logFile = cf.logFile
	}

	var logger *slog.Logger
	if quiet {
		logger, err = logging.SetupQuiet(level, logFile)
	} else {
		logger, err = logging.Setup(level, logFile)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("setup logging: %w", err)
	}

	// Surface the version on every log line so triage from a user-supplied
	// log file is unambiguous.
	logger = logger.With("version", version.Version, "commit", version.GitCommit)
	slog.SetDefault(logger)

	return cfg, logger, nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `localvision — local vision-language model tools for MCP clients

Usage:
  localvision [run] [--config PATH] [--verbose] [--log-file PATH]
  localvision doctor [--config PATH] [--verbose]
  localvision version

Subcommands:
  run (default)   Start the MCP server on stdio. Blocks until the client
                  disconnects or the process receives SIGTERM/SIGINT.
  doctor          Print diagnostics: hardware detection, catalog validation,
                  default model selection, binary availability, tool count.
                  Exits 0 regardless of issues; the report itself is the output.
  version         Print version, commit, and build date.

Flags:
  --config PATH   Override the config file location. Default:
                  $XDG_CONFIG_HOME/localvision/config.toml or
                  ~/.localvision/config.toml.
  --verbose       Set log level to debug.
  --log-file PATH Also write structured JSON logs to PATH (in addition to stderr).

Environment:
  XDG_CONFIG_HOME  Override the config lookup root.

Exit codes:
  0  success
  1  runtime error
  2  bad arguments
  3  configuration / first-run setup failure`)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func cfgPathDisplay(p string) string {
	if p == "" {
		return config.DefaultPath()
	}
	return p
}

// lookPathLLAMAServer searches $PATH for a usable `llama-server` binary.
// Returns the resolved path, or os/exec.ErrNotFound if not present.
//
// We deliberately don't verify the SHA256 here — the lifecycle manager
// does that on real spawn. The doctor command is informational only.
func lookPathLLAMAServer() (string, error) {
	return exec.LookPath("llama-server")
}

// Compile-time guarantee that the binary still references internal/llama
// (so go vet doesn't flag the import as unused when the doctor command
// changes shape in the future). Remove if doctor grows a real llama call.
var _ = llama.StateStopped
