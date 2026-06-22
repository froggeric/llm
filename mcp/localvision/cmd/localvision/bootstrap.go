package main

import (
	"fmt"
	"log/slog"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/llama"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
)

// runtime holds the shared, fully-constructed runtime used by both the MCP
// server (run) and the one-shot CLI query path. bootstrap performs the common
// setup: hardware detection → catalog load → lifecycle manager.
type runtime struct {
	cfg       *config.Config
	logger    *slog.Logger
	hw        models.HardwareInfo
	catalog   *models.Catalog
	lifecycle *llama.LifecycleManager
}

// bootstrap constructs the hardware/catalog/lifecycle from an already-loaded
// config + logger. Non-fatal issues (hardware detection, catalog validation)
// are logged and the runtime is still returned; fatal issues (catalog load,
// lifecycle construction) return an error for the caller to map to an exit
// code. Callers must have already called loadAndConfigure (which carries the
// config-load → exitUnsetConfig semantics).
func bootstrap(cfg *config.Config, logger *slog.Logger) (*runtime, error) {
	hw, err := models.DetectHardware()
	if err != nil {
		// Non-fatal: proceed with an empty HardwareInfo; the catalog's
		// DefaultModel will surface ErrNoFittingModel on first use.
		logger.Warn("hardware detection failed; falling back to empty hardware info", "error", err)
	}

	catalog, err := models.Load("")
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	if err := catalog.Validate(); err != nil {
		// Non-fatal: tools/list and one-shot queries still work; a tool call
		// that needs a model surfaces a clear error.
		logger.Error("catalog validation failed", "error", err)
	}

	lm, err := llama.NewWithOptions(llama.Options{
		Catalog:        catalog,
		CacheDir:       cfg.CacheDir,
		IdleTimeout:    cfg.IdleTimeout,
		StartupTimeout: cfg.StartupTimeout,
		Logger:         logger,
		BinarySHA256:   cfg.LLAMAServerPinnedSHA256,
	})
	if err != nil {
		return nil, fmt.Errorf("construct lifecycle manager: %w", err)
	}

	return &runtime{
		cfg:       cfg,
		logger:    logger,
		hw:        hw,
		catalog:   catalog,
		lifecycle: lm,
	}, nil
}
