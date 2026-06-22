package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/froggeric/llm/mcp/localvision/internal/tools"
	"github.com/froggeric/llm/mcp/localvision/internal/tools/format"
)

// outputSink writes the formatted result of each inference to its destination:
// stdout (default), a single file (--output), or one file per input
// (--output-dir). It also writes optional --meta sidecars next to file targets.
//
// The presentational single-unit case (stdout, no explicit --format, one
// image) is handled directly by runOneShot via renderResult; the sink is only
// used for raw-byte emission (explicit --format, file/dir output, or batch).
type outputSink struct {
	mode     sinkMode
	filePath string // mode == sinkFile
	dir      string // mode == sinkDir
	fmt      format.Format
	meta     bool
	batch    bool           // >1 unit: prefix stdout blocks with a header
	used     map[string]int // dir mode: basename stems already written → disambiguate collisions
}

type sinkMode int

const (
	sinkStdout sinkMode = iota
	sinkFile
	sinkDir
)

// newOutputSink validates the output flags and returns a sink. --output and
// --output-dir are mutually exclusive.
func newOutputSink(output, outputDir string, f format.Format, meta bool, batch bool) (*outputSink, error) {
	switch {
	case output != "" && outputDir != "":
		return nil, errors.New("--output and --output-dir are mutually exclusive")
	case output != "":
		return &outputSink{mode: sinkFile, filePath: output, fmt: f, meta: meta, batch: batch, used: map[string]int{}}, nil
	case outputDir != "":
		return &outputSink{mode: sinkDir, dir: outputDir, fmt: f, meta: meta, batch: batch, used: map[string]int{}}, nil
	default:
		return &outputSink{mode: sinkStdout, fmt: f, meta: meta, batch: batch, used: map[string]int{}}, nil
	}
}

// write emits one unit's formatted result. srcLabel is the human-readable
// source (stdout headers + meta); srcBase is the primary input's stem (per-file
// naming in dir mode). toolID + parsed are converted via the format package.
func (s *outputSink) write(srcLabel, srcBase, toolID string, parsed any, stats tools.Stats) error {
	content, err := format.Convert(toolID, parsed, s.fmt)
	if err != nil {
		return fmt.Errorf("format result: %w", err)
	}

	// Compute the file target ONCE. target() has a side effect — in dir mode it
	// reserves the (possibly disambiguated) name in s.used — so calling it twice
	// (once for the result, once for the sidecar) would disambiguate twice and
	// orphan the .meta.json. stdout has no file target.
	target := ""
	if s.mode != sinkStdout {
		target = s.target(srcBase)
	}

	switch s.mode {
	case sinkStdout:
		if s.batch {
			fmt.Printf("=== %s ===\n", srcLabel)
		}
		if _, err := os.Stdout.Write(content); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		if len(content) == 0 || content[len(content)-1] != '\n' {
			fmt.Println()
		}
	default:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", target, err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}

	// Meta sidecar (best-effort) anchors to the same file target computed above.
	if s.meta && target != "" {
		if err := writeMetaSidecar(target, srcLabel, toolID, stats); err != nil {
			fmt.Fprintf(os.Stderr, "localvision: --meta sidecar for %s: %v\n", srcLabel, err)
		}
	}
	return nil
}

// target derives the output path for file/dir modes. stdout returns "" (the
// caller writes directly). In dir mode, basename collisions are disambiguated
// (a.png + screenshots/a.png → a.json, a_1.json) so batch runs never silently
// overwrite a previous result.
func (s *outputSink) target(srcBase string) string {
	switch s.mode {
	case sinkFile:
		return s.filePath
	case sinkDir:
		name := srcBase + "." + format.Suffix(s.fmt)
		if _, dup := s.used[name]; dup {
			// Basename collision (e.g. photos/a.png + shots/a.png): pick the
			// first free a_<k>.<ext> so neither result is silently overwritten.
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			for k := 1; ; k++ {
				cand := fmt.Sprintf("%s_%d%s", base, k, ext)
				if _, exists := s.used[cand]; !exists {
					s.used[cand] = 1
					return filepath.Join(s.dir, cand)
				}
			}
		}
		s.used[name] = 1
		return filepath.Join(s.dir, name)
	default:
		return ""
	}
}

// metaDoc is the --meta sidecar payload.
type metaDoc struct {
	Source    string `json:"source"`
	Tool      string `json:"tool"`
	Model     string `json:"model"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

// writeMetaSidecar writes <target>.meta.json next to an output file.
func writeMetaSidecar(target, srcLabel, toolID string, stats tools.Stats) error {
	doc := metaDoc{
		Source:    srcLabel,
		Tool:      toolID,
		Model:     stats.Model,
		TokensIn:  stats.TokensIn,
		TokensOut: stats.TokensOut,
		ElapsedMs: stats.ElapsedMs,
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(target+".meta.json", b, 0o644)
}

// unitLabel renders a unit's paths as a short label for stdout headers and
// summary lines.
func unitLabel(paths []string) string {
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return filepath.Base(paths[0])
	default:
		return filepath.Base(paths[0]) + " + " + filepath.Base(paths[1])
	}
}

// unitFileStem derives the filename stem (no extension) for --output-dir naming.
// A compare pair becomes "a_vs_b".
func unitFileStem(paths []string) string {
	stem := func(p string) string {
		return strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	switch len(paths) {
	case 0:
		return "result"
	case 1:
		return stem(paths[0])
	default:
		return stem(paths[0]) + "_vs_" + stem(paths[1])
	}
}
