package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/tools"
)

// present.go adds terminal chrome (spinner, colors, symbols) to the one-shot
// CLI. All chrome goes to STDERR and is gated on stderr being a TTY, so stdout
// stays plain text for piping (`localvision img.png --type ocr > out.txt`).
//
// isTerminal is per-OS (term_{darwin,linux,windows,other}.go) so this package
// cross-compiles cleanly.

// stderrIsTTY gates color/animation so pipes and log files stay clean.
var stderrIsTTY = isTerminal(os.Stderr.Fd())

// ANSI color codes.
const (
	cCyan  = "\x1b[36m"
	cGreen = "\x1b[32m"
	cRed   = "\x1b[31m"
	cBold  = "\x1b[1m"
	cDim   = "\x1b[2m"
	cReset = "\x1b[0m"
)

// paint wraps s in color c when stderr is a TTY; otherwise returns s unchanged.
func paint(c, s string) string {
	if !stderrIsTTY {
		return s
	}
	return c + s + cReset
}

// toolGlyph maps a tool ID to a short human label for status lines.
var toolGlyph = map[string]string{
	tools.ToolReadImage:       "Describe",
	tools.ToolExtractText:     "OCR",
	tools.ToolExtractCode:     "Code",
	tools.ToolExtractTable:    "Table",
	tools.ToolDescribeUI:      "UI",
	tools.ToolDescribeDiagram: "Diagram",
	tools.ToolDescribeChart:   "Chart",
	tools.ToolDiagnoseError:   "Error",
	tools.ToolCompareImages:   "Compare",
}

// elapsed returns seconds since start as a short string (e.g. "42.1s").
func elapsed(start time.Time) string {
	return fmt.Sprintf("%.1fs", time.Since(start).Seconds())
}

// spinner shows an animated braille progress line on stderr (TTY only). The
// message is updatable via setMsg so phase transitions can update it in place.
type spinner struct {
	stop, done chan struct{}
	msg        atomic.Value // string
}

func newSpinner(msg string) *spinner {
	if !stderrIsTTY {
		fmt.Fprintln(os.Stderr, "localvision:", msg)
		return nil
	}
	s := &spinner{stop: make(chan struct{}), done: make(chan struct{})}
	s.msg.Store(msg)
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		defer close(s.done)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		i := 0
		m := s.msg.Load().(string)
		fmt.Fprintf(os.Stderr, "\r\033[K%s%s%s %s", cCyan, frames[i], cReset, m)
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-t.C:
				i = (i + 1) % len(frames)
				m = s.msg.Load().(string)
				fmt.Fprintf(os.Stderr, "\r\033[K%s%s%s %s", cCyan, frames[i], cReset, m)
			}
		}
	}()
	return s
}

func (s *spinner) setMsg(msg string) {
	if s == nil {
		return
	}
	s.msg.Store(msg)
}

func (s *spinner) halt() {
	if s == nil {
		return
	}
	close(s.stop)
	<-s.done
}

// phaseTime records one lifecycle phase's timing for the per-phase summary.
type phaseTime struct {
	phase, detail string
	start         time.Time
	elapsed       time.Duration
}

var phaseLabels = map[string]string{
	"downloading": "⬇ Downloading",
	"loading":     "⚙ Loading model",
	"ready":       "✓ Ready",
	"inferring":   "↻ Inferring",
}

func phaseLabel(phase string) string {
	if l, ok := phaseLabels[phase]; ok {
		return l
	}
	return phase
}

// printPhaseSummary prints the per-phase elapsed-time breakdown to stderr.
func printPhaseSummary(phases []phaseTime) {
	for _, p := range phases {
		fmt.Fprintf(os.Stderr, "  %s  %s\n",
			paint(cDim, phaseLabel(p.phase)),
			paint(cDim, fmt.Sprintf("%.1fs", p.elapsed.Seconds())))
	}
}

// llamaVersion queries llama-server --version and returns the first
// version/build/commit line, or "" if unavailable.
func llamaVersion() string {
	p, err := exec.LookPath("llama-server")
	if err != nil {
		return ""
	}
	out, err := exec.Command(p, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		l := strings.TrimSpace(line)
		lower := strings.ToLower(l)
		if strings.Contains(lower, "version") || strings.Contains(lower, "build") || strings.Contains(lower, "commit") {
			return l
		}
	}
	return ""
}

// stdoutIsTTY gates result styling so pipes/files get plain text.
var stdoutIsTTY = isTerminal(os.Stdout.Fd())

// paintOut wraps s in color c when STDOUT is a TTY (for the result); paint
// (above) is gated on stderr for status lines.
func paintOut(c, s string) string {
	if !stdoutIsTTY {
		return s
	}
	return c + s + cReset
}

// renderResult writes the model's output to stdout. When stdout is a TTY it
// applies lightweight markdown-ish styling (colored headings, dimmed code
// fences) for readability; when piped, it writes plain text so
// `localvision img.png > out.txt` stays clean.
func renderResult(raw string) {
	if !stdoutIsTTY {
		fmt.Println(raw)
		return
	}
	inFence := false
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			fmt.Printf("%s%s%s\n", cDim, line, cReset)
			continue
		}
		if inFence {
			fmt.Printf("%s%s%s\n", cDim, line, cReset)
			continue
		}
		fmt.Println(styleLine(line))
	}
}

// styleLine applies inline markdown-ish styling to a single (non-code) line.
func styleLine(line string) string {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "### ") {
		return paintOut(cCyan, line)
	}
	return line
}

// renderError prints an error with its ": "-separated cause chain broken into
// indented lines (instead of one long concatenated line), to stderr.
func renderError(label string, err error) {
	fmt.Fprintf(os.Stderr, "%s %s failed\n", paint(cRed, "✗"), label)
	for _, part := range strings.Split(err.Error(), ": ") {
		fmt.Fprintf(os.Stderr, "  %s\n", paint(cDim, "↳ "+part))
	}
}
