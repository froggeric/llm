// Package config loads the user configuration file (default location:
// ~/.local-vision-mcp/config.toml) and exposes typed settings to the rest
// of the MCP.
//
// This file defines the public Config struct. Loading and validation are
// implemented by Track A.
package config

import (
	"errors"
	"time"
)

// ErrNotImplemented is returned by stub functions during the contract phase.
var ErrNotImplemented = errors.New("not implemented")

// Defaults applied when the config file is missing or partial.
const (
	DefaultIdleTimeout     = 5 * time.Minute
	DefaultStartupTimeout  = 2 * time.Minute
	DefaultLogLevel        = "info"
	DefaultCacheDirName    = ".local-vision-mcp"
	DefaultModelsSubdir    = "models"
	DefaultBinSubdir       = "bin"
	DefaultSafetyMarginGB  = 4.0
	DefaultDownloadRetries = 3
)

// Config is the parsed user configuration. All fields have safe defaults
// except where noted.
type Config struct {
	// IdleTimeout is how long a fully-unloaded model is kept resident
	// before the subprocess is killed. Default 5m.
	IdleTimeout time.Duration `toml:"idle_timeout"`

	// StartupTimeout is how long we wait for llama-server to become healthy
	// before declaring the spawn a failure. Default 2m.
	StartupTimeout time.Duration `toml:"startup_timeout"`

	// LogLevel controls slog verbosity. One of: debug, info, warn, error.
	LogLevel string `toml:"log_level"`

	// LogFile, if non-empty, writes structured logs to this path in addition
	// to stderr. Useful for filing bug reports.
	LogFile string `toml:"log_file"`

	// CacheDir is the root for all local-vision-mcp state. Defaults to
	// ~/.local-vision-mcp.
	CacheDir string `toml:"cache_dir"`

	// ModelsDir overrides the model-file cache location. Defaults to
	// CacheDir/models.
	ModelsDir string `toml:"models_dir"`

	// BinDir overrides the llama-server binary cache. Defaults to CacheDir/bin.
	BinDir string `toml:"bin_dir"`

	// DefaultModel, if set, overrides the auto-selected default for the
	// detected hardware tier. Useful for testing or power users.
	DefaultModel string `toml:"default_model"`

	// SafetyMarginGB is subtracted from total memory when computing the
	// "available" pool for model loading. Default 4 GB; raise if you run
	// other memory-hungry apps alongside.
	SafetyMarginGB float64 `toml:"safety_margin_gb"`

	// HFUser is the HuggingFace username model files are downloaded from.
	// Defaults to "froggeric"; can be overridden for forks/enterprises.
	HFUser string `toml:"hf_user"`

	// LLAMAServerPinnedSHA256 is the SHA256 of the llama-server binary we
	// expect to download. Pinned in source by Track C; user can override
	// only at their own risk.
	LLAMAServerPinnedSHA256 string `toml:"-"`
}

// Load reads config from path, applies defaults for missing fields, and
// validates basic invariants (paths are absolute after expansion, log level
// is recognized, etc.).
//
// path may be empty; in that case defaults are used entirely.
func Load(path string) (*Config, error) {
	return nil, ErrNotImplemented
}

// DefaultPath returns the canonical config file location for the current
// user: $XDG_CONFIG_HOME/local-vision-mcp/config.toml or
// ~/.local-vision-mcp/config.toml if XDG is not set.
func DefaultPath() string {
	return ""
}
