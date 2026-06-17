// Package config loads the user configuration file (default location:
// ~/.local-vision-mcp/config.toml) and exposes typed settings to the rest
// of the MCP.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ErrNotImplemented is retained for backwards compatibility; production code
// does not return it.
var ErrNotImplemented = errors.New("not implemented")

// Defaults applied when the config file is missing or partial.
const (
	DefaultIdleTimeout    = 5 * time.Minute
	DefaultStartupTimeout = 2 * time.Minute
	DefaultLogLevel       = "info"
	// DefaultCacheDirName is appended to the user's home directory.
	DefaultCacheDirName = ".local-vision-mcp"
	DefaultModelsSubdir = "models"
	DefaultBinSubdir    = "bin"
	DefaultSafetyMarginGB  = 4.0
	DefaultDownloadRetries = 3
	// DefaultHFUser is the HuggingFace namespace model files are downloaded
	// from. Defaults to the project maintainer's account.
	DefaultHFUser = "froggeric"
)

// Valid LogLevels.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Config is the parsed user configuration. All fields have safe defaults
// except where noted.
type Config struct {
	IdleTimeout    time.Duration `toml:"idle_timeout"`
	StartupTimeout time.Duration `toml:"startup_timeout"`
	LogLevel       string        `toml:"log_level"`
	LogFile        string        `toml:"log_file"`
	CacheDir       string        `toml:"cache_dir"`
	ModelsDir      string        `toml:"models_dir"`
	BinDir         string        `toml:"bin_dir"`
	DefaultModel   string        `toml:"default_model"`
	SafetyMarginGB float64       `toml:"safety_margin_gb"`
	HFUser         string        `toml:"hf_user"`

	// LLAMAServerPinnedSHA256 is populated from internal/llama at link time
	// (so the binary pins the hash it expects). Not set via TOML.
	LLAMAServerPinnedSHA256 string `toml:"-"`
}

// Load reads config from path, applies defaults for missing fields, and
// validates basic invariants (paths are absolute after expansion, log level
// is recognized, etc.).
//
// If path is empty, DefaultPath() is used. If that file does not exist,
// pure defaults are returned without error.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}

	c := &Config{
		IdleTimeout:    DefaultIdleTimeout,
		StartupTimeout: DefaultStartupTimeout,
		LogLevel:       DefaultLogLevel,
		SafetyMarginGB: DefaultSafetyMarginGB,
		HFUser:         DefaultHFUser,
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, c); err != nil {
				return nil, fmt.Errorf("decode %s: %w", path, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}
		// Missing file is fine — fall through to defaults.
	}

	// Resolve CacheDir. Default to ~/.local-vision-mcp.
	if c.CacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		c.CacheDir = filepath.Join(home, DefaultCacheDirName)
	}
	c.CacheDir = expandPath(c.CacheDir)

	if c.ModelsDir == "" {
		c.ModelsDir = filepath.Join(c.CacheDir, DefaultModelsSubdir)
	}
	c.ModelsDir = expandPath(c.ModelsDir)

	if c.BinDir == "" {
		c.BinDir = filepath.Join(c.CacheDir, DefaultBinSubdir)
	}
	c.BinDir = expandPath(c.BinDir)

	if c.LogFile != "" {
		c.LogFile = expandPath(c.LogFile)
	}

	// Validate log level.
	switch strings.ToLower(c.LogLevel) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, "":
	default:
		return nil, fmt.Errorf("invalid log_level %q (want debug|info|warn|error)", c.LogLevel)
	}
	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}

	// Validate timeouts.
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}
	if c.StartupTimeout <= 0 {
		c.StartupTimeout = DefaultStartupTimeout
	}

	// Validate safety margin.
	if c.SafetyMarginGB <= 0 {
		c.SafetyMarginGB = DefaultSafetyMarginGB
	}

	// Validate HF user.
	if c.HFUser == "" {
		c.HFUser = DefaultHFUser
	}

	return c, nil
}

// DefaultPath returns the canonical config file location for the current
// user: $XDG_CONFIG_HOME/local-vision-mcp/config.toml if XDG_CONFIG_HOME is
// set, otherwise ~/.local-vision-mcp/config.toml.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "local-vision-mcp", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, DefaultCacheDirName, "config.toml")
}

// expandPath resolves leading ~ and environment variables, then converts to
// absolute. Relative paths are resolved against the user's home directory
// (config files commonly use ~/.foo paths).
func expandPath(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			if strings.HasPrefix(p, "~/") {
				return filepath.Join(home, p[2:])
			}
		}
	}
	p = os.ExpandEnv(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
