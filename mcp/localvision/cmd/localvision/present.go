package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/froggeric/llm/mcp/localvision/internal/tools"
	"golang.org/x/sys/unix"
)

// present.go adds terminal chrome (spinner, colors, symbols) to the one-shot
// CLI. All chrome goes to STDERR and is gated on stderr being a TTY, so stdout
// stays plain text for piping (`localvision img.png --type ocr > out.txt`).

// stderrIsTTY gates color/animation so pipes and log files stay clean.
var stderrIsTTY = isTerminal(os.Stderr.Fd())

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
}

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

// imageLabel renders the positional image args as a short label.
func imageLabel(positionals []string) string {
	switch len(positionals) {
	case 0:
		return ""
	case 1:
		return filepath.Base(positionals[0])
	case 2:
		return filepath.Base(positionals[0]) + " + " + filepath.Base(positionals[1])
	default:
		return fmt.Sprintf("%d images", len(positionals))
	}
}

// elapsed returns seconds since start as a short string (e.g. "42.1s").
func elapsed(start time.Time) string {
	return fmt.Sprintf("%.1fs", time.Since(start).Seconds())
}

// spinner shows an animated braille progress line on stderr (TTY only). When
// stderr is not a TTY, it prints a single plain line and stop() is a no-op.
type spinner struct{ stop, done chan struct{} }

func newSpinner(msg string) *spinner {
	if !stderrIsTTY {
		fmt.Fprintln(os.Stderr, "localvision:", msg)
		return nil
	}
	s := &spinner{stop: make(chan struct{}), done: make(chan struct{})}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		defer close(s.done)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		i := 0
		fmt.Fprintf(os.Stderr, "\r%s%s%s %s", cCyan, frames[i], cReset, msg)
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(os.Stderr, "\r\033[K") // clear the spinner line
				return
			case <-t.C:
				i = (i + 1) % len(frames)
				fmt.Fprintf(os.Stderr, "\r%s%s%s %s", cCyan, frames[i], cReset, msg)
			}
		}
	}()
	return s
}

func (s *spinner) halt() {
	if s == nil {
		return
	}
	close(s.stop)
	<-s.done
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
