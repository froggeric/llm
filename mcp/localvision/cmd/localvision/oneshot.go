package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/mcpserver"
	"github.com/froggeric/llm/mcp/localvision/internal/tools"
)

// typeToToolID maps the --type flag value to a registry tool ID.
var typeToToolID = map[string]string{
	"ocr":      tools.ToolExtractText,
	"code":     tools.ToolExtractCode,
	"table":    tools.ToolExtractTable,
	"ui":       tools.ToolDescribeUI,
	"diagram":  tools.ToolDescribeDiagram,
	"chart":    tools.ToolDescribeChart,
	"error":    tools.ToolDiagnoseError,
	"compare":  tools.ToolCompareImages,
	"describe": tools.ToolReadImage,
	"read":     tools.ToolReadImage, // alias for describe
}

// resolveToolID maps a --type value to a tool ID. Empty returns read_image
// (the generic describe tool, the least-surprising default for a shell user).
func resolveToolID(typeFlag string) (string, error) {
	if typeFlag == "" {
		return tools.ToolReadImage, nil
	}
	id, ok := typeToToolID[strings.ToLower(typeFlag)]
	if !ok {
		return "", fmt.Errorf("unknown --type %q (valid: ocr, code, table, ui, diagram, chart, error, compare, describe)", typeFlag)
	}
	return id, nil
}

// splitArgs separates one-shot args into flag args (for flag.Parse) and
// positional image args, so flags and images may appear in any order:
// `localvision img.png --type ocr` works, not only `--type`-first. It knows
// which flags are boolean (consume no following value).
func splitArgs(args []string) (flagArgs, positionals []string) {
	boolFlags := map[string]bool{"--verbose": true, "-verbose": true, "-h": true, "--help": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-" { // stdin sentinel (Phase 3)
			positionals = append(positionals, a)
			continue
		}
		if strings.HasPrefix(a, "-") {
			flagArgs = append(flagArgs, a)
			// Consume the next arg as the value unless this is a bool flag or
			// the value is attached with "=".
			if !boolFlags[a] && !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return flagArgs, positionals
}

// runOneShot handles the positional-image query form:
//
//	localvision <image...> [--type T] [--model M] [--question Q] [common flags]
//
// It maps --type to one of the 9 tools, runs a single inference through the
// same CatalogExecutor the MCP server uses, and prints the result to stdout.
func runOneShot(args []string) int {
	flagArgs, positionals := splitArgs(args)

	fs := flag.NewFlagSet("oneshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cf commonFlags
	cf.register(fs)
	var typeFlag, modelFlag, questionFlag string
	fs.StringVar(&typeFlag, "type", "", "query type: ocr|code|table|ui|diagram|chart|error|compare|describe (default describe)")
	fs.StringVar(&modelFlag, "model", "", "override the auto-selected model (a catalog ID)")
	fs.StringVar(&questionFlag, "question", "", "specific question about the image (describe/read only)")
	if err := fs.Parse(flagArgs); err != nil {
		return exitBadArgs
	}

	toolID, err := resolveToolID(typeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}

	if len(positionals) == 0 {
		fmt.Fprintln(os.Stderr, "localvision: provide at least one image path")
		return exitBadArgs
	}
	if toolID == tools.ToolCompareImages && len(positionals) != 2 {
		fmt.Fprintf(os.Stderr, "localvision: --type compare needs exactly 2 images, got %d\n", len(positionals))
		return exitBadArgs
	}

	cfg, logger, err := loadAndConfigure(cf, stderrIsTTY && !cf.verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitUnsetConfig
	}

	// Fast-fail on missing local files before the slow model load. (Skips
	// data:/file: URIs, which ParseImageRef resolves.)
	for _, p := range positionals {
		if strings.HasPrefix(p, "data:") || strings.HasPrefix(p, "file://") {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
			return exitBadArgs
		}
	}

	rt, err := bootstrap(cfg, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitGeneric
	}

	registry := tools.NewRegistry()
	tool, ok := registry.Get(toolID)
	if !ok {
		fmt.Fprintf(os.Stderr, "localvision: tool %q not registered (internal error)\n", toolID)
		return exitGeneric
	}

	refs := make([]tools.ImageRef, 0, len(positionals))
	for _, p := range positionals {
		ref, err := tools.ParseImageRef(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
			return exitBadArgs
		}
		refs = append(refs, ref)
	}
	defer tools.CleanupImageRefs(refs)

	extra := map[string]any{}
	if questionFlag != "" {
		extra["question"] = questionFlag
	}
	system, user, _, err := tool.BuildRequest(tools.ToolInput{Images: refs, Extra: extra})
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: build request: %v\n", err)
		return exitGeneric
	}

	exec := mcpserver.NewCatalogExecutor(rt.catalog, rt.lifecycle, rt.hw, rt.logger)
	// C5: --model overrides; else config default_model; else catalog selection.
	override := modelFlag
	if override == "" {
		override = cfg.DefaultModel
	}
	if override != "" {
		if _, ok := rt.catalog.Models[override]; !ok {
			fmt.Fprintf(os.Stderr, "localvision: model %q is not in the catalog\n", override)
			return exitBadArgs
		}
		exec.SetOverrideModel(override)
	}

	// Cancel inference on SIGINT/SIGTERM so Ctrl-C during a 30-70s cold start
	// exits promptly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Presentation: an animated spinner on stderr while the model loads and
	// infers (TTY only), so the 30-70s wait isn't silent. stdout stays plain
	// for piping.
	label := toolGlyph[toolID]
	if label == "" {
		label = toolID
	}
	msg := label + " · " + imageLabel(positionals)
	start := time.Now()
	sp := newSpinner(msg)
	raw, err := exec.Run(ctx, toolID, system, user, refs, tool.MaxTokens())
	sp.halt()
	if err != nil {
		renderError(label, err)
		return exitGeneric
	}
	fmt.Fprintf(os.Stderr, "%s %s (%s)\n", paint(cGreen, "✓"), msg, elapsed(start))

	parsed, _ := tool.ParseOutput(raw)
	if s, ok := parsed.(string); ok {
		renderResult(s)
	} else {
		// Structured output (e.g. extract_code returns a map). Phase 2's
		// --format handles proper encoding; for now print the raw text.
		renderResult(raw)
	}
	return exitOK
}
