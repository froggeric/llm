package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/froggeric/llm/mcp/localvision/internal/config"
	"github.com/froggeric/llm/mcp/localvision/internal/models"
	"github.com/froggeric/llm/mcp/localvision/internal/setup"
)

// runSetup is the interactive first-run configuration wizard. It detects
// hardware, recommends a model, checks for llama-server, and writes
// ~/.localvision/config.toml. Framework-free (stdlib only): numbered menus +
// the ANSI helpers in present.go, so it adds zero dependencies.
//
// It reads from os.Stdin and writes to os.Stdout. A Ctrl-C / EOF at any prompt
// cancels without writing anything.
func runSetup(args []string) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var cf commonFlags
	cf.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitBadArgs
	}

	cfg, _, err := loadAndConfigure(cf, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		return exitUnsetConfig
	}

	w := os.Stdout
	r := bufio.NewReader(os.Stdin)

	fmt.Fprintf(w, "%s\n", paintOut(cCyan+cBold, "localvision setup"))
	fmt.Fprintln(w, "Guided first-run configuration. Press Ctrl-C at any time to cancel.")
	fmt.Fprintln(w)

	// 1. Hardware detection.
	fmt.Fprintf(w, "%s\n", paintOut(cBold, "Detected hardware"))
	hw, hwErr := models.DetectHardware()
	if hwErr != nil {
		fmt.Fprintf(w, "  Detection failed: %v\n", hwErr)
	} else {
		fmt.Fprintf(w, "  Memory:  %.1f GB\n", hw.TotalMemoryGB)
		fmt.Fprintf(w, "  Tier:    %s\n", hw.Tier)
		if hw.DetectNote != "" {
			fmt.Fprintf(w, "  Note:    %s\n", hw.DetectNote)
		}
	}
	fmt.Fprintln(w)

	// 2. Model selection.
	catalog, err := models.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: load catalog: %v\n", err)
		return exitGeneric
	}
	opts := setup.ModelOptions(catalog, hw)
	if len(opts) == 0 {
		fmt.Fprintf(os.Stderr, "setup: catalog has no models\n")
		return exitGeneric
	}

	fmt.Fprintf(w, "%s\n", paintOut(cBold, "Select a default model"))
	defaultIdx := 1
	for i, o := range opts {
		bullet := " "
		if o.Recommended {
			bullet = paintOut(cGreen, "★")
			defaultIdx = i + 1
		}
		fit := ""
		if !o.Fits {
			fit = paintOut(cRed, " (may not fit)")
		}
		fmt.Fprintf(w, "  %s %d) %-16s %s%s%s\n",
			bullet, i+1, o.ID, o.DisplayName, paintOut(cDim, " ("+string(o.Tier)+")"), fit)
	}
	choice, ok := readMenu(r, w, len(opts), defaultIdx)
	if !ok {
		fmt.Fprintln(w, "\nSetup canceled.")
		return exitGeneric
	}
	picked := opts[choice-1]
	if !picked.Fits {
		fmt.Fprintf(w, "%s\n", paintOut(cDim, "  note: that model may not fit your detected hardware"))
	}
	fmt.Fprintln(w)

	// 2b. Per-tool routing (v0.7): the benchmark crowns a different best model
	// per tool. Offer to write the recommended per-tool routing explicitly.
	fmt.Fprintf(w, "%s\n", paintOut(cBold, "Per-tool model routing (optional)"))
	fmt.Fprintf(w, "  The benchmark crowns a different best model per tool, e.g.\n")
	fmt.Fprintf(w, "  Qwen3.5-4B-Q8 for code/UI/diagram/error and Qwen3-VL-8B-Q8 for\n")
	fmt.Fprintf(w, "  the rest. Routing each tool to its best model improves per-task\n")
	fmt.Fprintf(w, "  quality, but a mixed-tool session then switches models (a cold\n")
	fmt.Fprintf(w, "  reload per switch); the default keeps one warm model for all tools.\n")
	useRouting, ok := readYesNo(r, w, "Write the benchmark's recommended per-tool routing?", false)
	if !ok {
		fmt.Fprintln(w, "\nSetup canceled.")
		return exitGeneric
	}
	fmt.Fprintln(w)

	// 3. llama-server status.
	fmt.Fprintf(w, "%s\n", paintOut(cBold, "llama-server binary"))
	if path, found := setup.DetectLLAMAServer(); found {
		fmt.Fprintf(w, "  Found: %s\n", path)
		fmt.Fprintf(w, "  %s\n", paintOut(cDim, "(used as-is; integrity not verified)"))
	} else {
		fmt.Fprintln(w, "  Not found on $PATH.")
		fmt.Fprintln(w, "  Recommended: `brew install llama.cpp`. Otherwise the first tool")
		fmt.Fprintln(w, "  call downloads a pinned release and verifies its SHA256.")
	}
	fmt.Fprintln(w)

	// 4. Storage paths (defaults; flags/TOML override later).
	fmt.Fprintf(w, "%s\n", paintOut(cBold, "Storage paths (defaults)"))
	fmt.Fprintf(w, "  Config:  %s\n", config.DefaultPath())
	fmt.Fprintf(w, "  Cache:   %s\n", cfg.CacheDir)
	fmt.Fprintf(w, "  Models:  %s (%s)\n", cfg.ModelsDir, models.DiskFreeHuman(cfg.ModelsDir))
	fmt.Fprintf(w, "  Bin:     %s\n", cfg.BinDir)
	fmt.Fprintf(w, "  %s\n", paintOut(cDim, "(edit the TOML or use --cache-dir/--models-dir to change these)"))
	fmt.Fprintln(w)

	// 5. Review + confirm.
	fmt.Fprintf(w, "%s\n", paintOut(cBold, "Review"))
	fmt.Fprintf(w, "  Default model: %s\n", picked.ID)
	fmt.Fprintf(w, "  Config file:   %s\n", config.DefaultPath())
	confirm, ok := readYesNo(r, w, "Write this configuration?", true)
	if !ok || !confirm {
		fmt.Fprintln(w, "\nSetup canceled. No changes written.")
		return exitGeneric
	}

	// Build + persist.
	final, err := setup.BuildConfig(cfg, catalog, hw, setup.Choices{Model: picked.ID, PerToolRouting: useRouting})
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		return exitGeneric
	}
	path := config.DefaultPath()
	if err := config.Save(path, final); err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
		return exitGeneric
	}

	fmt.Fprintf(w, "\n%s Wrote %s\n", paintOut(cGreen, "✓"), path)
	fmt.Fprintln(w, "\nNext steps:")
	fmt.Fprintln(w, "  • Run `localvision doctor` to verify the install.")
	fmt.Fprintln(w, "  • Point an MCP client at `localvision run` (see README).")
	fmt.Fprintln(w, "  • Or run a one-shot query: `localvision img.png --type ocr`.")
	return exitOK
}

// readMenu prompts for a 1..n choice, defaulting to def on blank input. Returns
// the 1-based choice, or ok=false on EOF/cancel.
func readMenu(r *bufio.Reader, w io.Writer, n, def int) (int, bool) {
	for {
		fmt.Fprintf(w, "Choice [%d]: ", def)
		line, ok := readLine(r)
		if !ok {
			return 0, false
		}
		if line == "" {
			return def, true
		}
		i, err := strconv.Atoi(line)
		if err == nil && i >= 1 && i <= n {
			return i, true
		}
		fmt.Fprintf(w, "  %s\n", paintOut(cDim, fmt.Sprintf("enter a number 1-%d (or blank for %d)", n, def)))
	}
}

// readYesNo prompts for a yes/no answer with a default. Returns (answer, ok);
// ok=false on EOF/cancel.
func readYesNo(r *bufio.Reader, w io.Writer, prompt string, def bool) (bool, bool) {
	hint := "Y/n"
	if !def {
		hint = "y/N"
	}
	for {
		fmt.Fprintf(w, "%s [%s]: ", prompt, hint)
		line, ok := readLine(r)
		if !ok {
			return false, false
		}
		switch strings.ToLower(line) {
		case "":
			return def, true
		case "y", "yes":
			return true, true
		case "n", "no":
			return false, true
		}
		fmt.Fprintf(w, "  %s\n", paintOut(cDim, "please answer y or n"))
	}
}

// readLine reads one line from r, trimming the trailing newline. Returns
// ok=false when input ends with no data (Ctrl-D / closed stdin).
func readLine(r *bufio.Reader) (string, bool) {
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", false // EOF with no data → cancel
	}
	return strings.TrimRight(line, "\r\n"), true
}
