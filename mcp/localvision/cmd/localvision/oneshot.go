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
	"github.com/froggeric/llm/mcp/localvision/internal/tools/format"
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
// `localvision img.png --type ocr` works, not only `--type`-first. It detects
// boolean flags (which consume no following value) by querying fs, so it tracks
// flag registration automatically — no hand-maintained name list to keep in sync.
func splitArgs(args []string, fs *flag.FlagSet) (flagArgs, positionals []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-" { // stdin sentinel
			positionals = append(positionals, a)
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			// Consume the next arg as the value unless this is a bool flag, the
			// value is attached with "=", or there is no next arg.
			if strings.Contains(a, "=") || isBoolFlagArg(a, fs) || i+1 >= len(args) {
				continue
			}
			i++
			flagArgs = append(flagArgs, args[i])
			continue
		}
		positionals = append(positionals, a)
	}
	return flagArgs, positionals
}

// boolFlag is the interface the standard library's bool flag values satisfy
// (their IsBoolFlag reports true). Type-asserting on it is how we tell a flag
// consumes no following value.
type boolFlag interface{ IsBoolFlag() bool }

// isBoolFlagArg reports whether arg names a registered boolean flag (so
// splitArgs knows it consumes no following value). -h/--help are treated as
// bool: the flag package handles them itself and we don't register them.
func isBoolFlagArg(arg string, fs *flag.FlagSet) bool {
	name := strings.TrimLeft(arg, "-")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	if name == "" || name == "h" || name == "help" {
		return true
	}
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	if bf, ok := f.Value.(boolFlag); ok {
		return bf.IsBoolFlag()
	}
	return false
}

// runOneShot handles the positional-image query form:
//
//	localvision <image...> [--type T] [--model M] [--format F]
//	    [--output FILE | --output-dir DIR] [--recursive] [--meta] [--question Q]
//
// It expands positional args (literal files, globs, directories, stdin) into a
// list of image paths, groups them into inference units (consecutive pairs for
// --type compare), and runs each through the same CatalogExecutor the MCP
// server uses. Output goes to stdout, a single file (--output), or one file per
// input (--output-dir); --meta writes a per-output .meta.json telemetry sidecar.
func runOneShot(args []string) int {
	// Register flags first so splitArgs can detect bool flags from the FlagSet
	// itself (no hand-synced name list). Then split, then parse.
	fs := flag.NewFlagSet("oneshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cf commonFlags
	cf.register(fs)
	var typeFlag, modelFlag, questionFlag, formatFlag, outputFlag, outputDirFlag string
	var recursiveFlag, metaFlag bool
	fs.StringVar(&typeFlag, "type", "", "query type: ocr|code|table|ui|diagram|chart|error|compare|describe (default describe)")
	fs.StringVar(&modelFlag, "model", "", "override the auto-selected model (a catalog ID)")
	fs.StringVar(&questionFlag, "question", "", "specific question about the image (describe/read only)")
	fs.StringVar(&formatFlag, "format", "", "output format: text|markdown|json|yaml|xml (default: presentational)")
	fs.StringVar(&outputFlag, "output", "", "write the result to this file (single image only)")
	fs.StringVar(&outputDirFlag, "output-dir", "", "write one result file per input into this directory")
	fs.BoolVar(&recursiveFlag, "recursive", false, "recurse into directories when expanding inputs")
	fs.BoolVar(&metaFlag, "meta", false, "write a .meta.json sidecar (tokens/timing) next to each --output/--output-dir result")

	flagArgs, positionals := splitArgs(args, fs)
	if err := fs.Parse(flagArgs); err != nil {
		return exitBadArgs
	}

	// Validate --format early, before the slow model load. The config default
	// (if --format is absent) is resolved and validated after config load.
	requestedFormat, err := format.Parse(formatFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}

	toolID, err := resolveToolID(typeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}

	if len(positionals) == 0 {
		fmt.Fprintln(os.Stderr, "localvision: provide at least one image path (or '-' for stdin)")
		return exitBadArgs
	}

	// Expand positional args (literal / glob / dir / stdin) into image paths.
	paths, err := expandInputs(positionals, recursiveFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}

	// Group into inference units (pairs for compare).
	units, err := groupUnits(paths, toolID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}
	batch := len(units) > 1

	// --output is single-result only.
	if outputFlag != "" && batch {
		fmt.Fprintln(os.Stderr, "localvision: --output requires a single image; use --output-dir for batch")
		return exitBadArgs
	}

	cfg, logger, err := loadAndConfigure(cf, stderrIsTTY && !cf.verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitUnsetConfig
	}
	// If --format was absent, fall back to the config default_format.
	if formatFlag == "" && cfg.DefaultFormat != "" {
		requestedFormat, err = format.Parse(cfg.DefaultFormat)
		if err != nil {
			fmt.Fprintf(os.Stderr, "localvision: invalid config default_format: %v\n", err)
			return exitUnsetConfig
		}
	}

	// emitFormat is the concrete encoding for raw byte emission (files, batch,
	// or explicit --format). Presentational (Auto) only applies to a single
	// stdout result; otherwise default to the model's natural markdown.
	emitFormat := requestedFormat
	if emitFormat == format.Auto {
		emitFormat = format.Markdown
	}
	presentational := requestedFormat == format.Auto && outputFlag == "" && outputDirFlag == "" && !batch

	sink, err := newOutputSink(outputFlag, outputDirFlag, emitFormat, metaFlag, batch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitBadArgs
	}
	if metaFlag && sink.mode == sinkStdout {
		fmt.Fprintln(os.Stderr, "localvision: --meta requires --output or --output-dir; ignoring")
	}

	// Fast-fail on missing local files before the slow model load. (Skips
	// data:/file: URIs, which ParseImageRef resolves.)
	for _, p := range paths {
		if strings.HasPrefix(p, "data:") || strings.HasPrefix(p, "file://") {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
			return exitBadArgs
		}
	}

	// Per-phase progress tracking for the CLI presentation.
	var sp *spinner
	var phaseTimes []phaseTime
	phaseCallback := func(phase, detail string) {
		now := time.Now()
		if n := len(phaseTimes); n > 0 {
			phaseTimes[n-1].elapsed = now.Sub(phaseTimes[n-1].start)
		}
		phaseTimes = append(phaseTimes, phaseTime{phase: phase, detail: detail, start: now})
		if sp != nil {
			sp.setMsg(phaseLabel(phase) + " · " + detail)
		}
	}

	rt, err := bootstrap(cfg, logger, phaseCallback)
	if err != nil {
		fmt.Fprintf(os.Stderr, "localvision: %v\n", err)
		return exitGeneric
	}
	// Fire-and-exit: shut down llama-server so it doesn't linger as an orphan
	// consuming RAM. (The MCP `run` command keeps it warm for the idle window;
	// the one-shot is fire-and-exit, so it cleans up immediately. Warm starts
	// between one-shot calls need the daemon/HTTP API planned for Theme F.)
	defer rt.lifecycle.Shutdown(context.Background())

	registry := tools.NewRegistry()
	tool, ok := registry.Get(toolID)
	if !ok {
		fmt.Fprintf(os.Stderr, "localvision: tool %q not registered (internal error)\n", toolID)
		return exitGeneric
	}

	exec := mcpserver.NewCatalogExecutor(rt.catalog, rt.lifecycle, rt.hw, rt.logger)
	// --model overrides; else config default_model; else catalog selection.
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

	// SIGINT/SIGTERM cancels in-flight inference so Ctrl-C during a 30-70s
	// cold start exits promptly.
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

	ver := llamaVersion()
	llamaSuffix := ""
	if ver != "" {
		llamaSuffix = " · llama.cpp " + strings.TrimSpace(strings.TrimPrefix(ver, "version:"))
	}
	glyph := toolGlyph[toolID]
	if glyph == "" {
		glyph = toolID
	}

	runStart := time.Now()
	failures := 0
	completed := 0
	interrupted := false
	for _, unitPaths := range units {
		// Stop the batch promptly on cancellation (e.g. SIGINT) instead of
		// spinning a spinner + error line for every remaining unit.
		if ctx.Err() != nil {
			interrupted = true
			fmt.Fprintln(os.Stderr, paint(cRed, "✗ interrupted"))
			break
		}
		label := glyph + " · " + unitLabel(unitPaths)
		unitStart := time.Now()
		phaseStart := len(phaseTimes)
		sp = newSpinner(label)

		res, runErr := runUnit(ctx, exec, tool, toolID, unitPaths, questionFlag)

		sp.halt()
		// Close out the final phase's elapsed time for this unit.
		if n := len(phaseTimes); n > phaseStart {
			phaseTimes[n-1].elapsed = time.Since(phaseTimes[n-1].start)
		}

		if runErr != nil {
			renderError(glyph+" · "+unitLabel(unitPaths), runErr)
			failures++
			if !batch {
				return exitGeneric
			}
			continue
		}

		summary := label + " · " + elapsed(unitStart)
		if res.stats.Model != "" {
			summary += " · " + res.stats.Model
		}
		summary += llamaSuffix
		fmt.Fprintf(os.Stderr, "%s %s\n", paint(cGreen, "✓"), summary)
		if !batch {
			printPhaseSummary(phaseTimes[phaseStart:])
		}

		if presentational {
			// Single stdout result, no explicit format: styled markdown.
			out, _ := format.Convert(toolID, res.parsed, format.Markdown)
			renderResult(string(out))
			completed++
		} else if werr := sink.write(unitLabel(unitPaths), unitFileStem(unitPaths), toolID, res.parsed, res.stats); werr != nil {
			fmt.Fprintf(os.Stderr, "localvision: %v\n", werr)
			failures++
			if !batch {
				return exitGeneric
			}
		} else {
			completed++
		}
	}

	if batch {
		mark := paint(cGreen, "✓")
		if failures > 0 || interrupted {
			mark = paint(cRed, "✗")
		}
		fmt.Fprintf(os.Stderr, "%s %d/%d done · %d failed · %s%s\n",
			mark, completed, len(units), failures, elapsed(runStart), llamaSuffix)
	}

	// An interrupted or partially-failed run is non-zero so scripts chained
	// with `&&` don't proceed after Ctrl-C or errors.
	if interrupted || failures > 0 {
		return exitGeneric
	}
	return exitOK
}

// groupUnits splits expanded paths into inference units. compare_images takes
// consecutive pairs (an even count is required); every other tool takes one
// path per unit.
func groupUnits(paths []string, toolID string) ([][]string, error) {
	if toolID == tools.ToolCompareImages {
		if len(paths) < 2 {
			return nil, fmt.Errorf("--type compare needs at least 2 images, got %d", len(paths))
		}
		if len(paths)%2 != 0 {
			return nil, fmt.Errorf("--type compare needs an even number of images (consecutive pairs), got %d", len(paths))
		}
		units := make([][]string, 0, len(paths)/2)
		for i := 0; i < len(paths); i += 2 {
			units = append(units, paths[i:i+2])
		}
		return units, nil
	}
	units := make([][]string, len(paths))
	for i, p := range paths {
		units[i] = []string{p}
	}
	return units, nil
}

// unitResult is the outcome of a single inference.
type unitResult struct {
	raw    string
	parsed any
	stats  tools.Stats
}

// runUnit runs a single inference over the given paths and returns the parsed
// result + telemetry. It owns image-ref construction and cleanup (data: URIs
// are written to temp files that must exist for the duration of Run).
func runUnit(ctx context.Context, exec tools.Executor, tool tools.Tool, toolID string, unitPaths []string, question string) (unitResult, error) {
	refs, err := buildImageRefs(unitPaths)
	if err != nil {
		return unitResult{}, err
	}
	defer tools.CleanupImageRefs(refs)

	extra := map[string]any{}
	if question != "" {
		extra["question"] = question
	}
	system, user, _, err := tool.BuildRequest(tools.ToolInput{Images: refs, Extra: extra})
	if err != nil {
		return unitResult{}, fmt.Errorf("build request: %w", err)
	}
	raw, stats, err := exec.Run(ctx, toolID, system, user, refs, tool.MaxTokens())
	if err != nil {
		return unitResult{}, err
	}
	parsed, _ := tool.ParseOutput(raw)
	return unitResult{raw: raw, parsed: parsed, stats: stats}, nil
}

// buildImageRefs parses each path into an ImageRef (file path, data: URI, or
// file:// URI). The caller must tools.CleanupImageRefs the result.
func buildImageRefs(paths []string) ([]tools.ImageRef, error) {
	refs := make([]tools.ImageRef, 0, len(paths))
	for _, p := range paths {
		ref, err := tools.ParseImageRef(p)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}
