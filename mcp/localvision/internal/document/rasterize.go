// Package document rasterizes PDF (and other document) files into page images
// using an opportunistic, CLI-only, $PATH-discovered converter chain — the same
// approach internal/llama uses for HEIC/WEBP. No decoder is bundled (the same
// patent/freeware-redistribution rationale applies; PDF rasterizers are
// ubiquitous and "bring your own").
package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// pdfMagic is the leading signature of every PDF file. Detecting by magic
// bytes (not extension) means a data:-URI PDF written to a generic .bin temp
// by tools.ParseImageRef is still recognized.
const pdfMagic = "%PDF-"

// MaxPages caps how many pages are rasterized in one call, bounding the
// inference context window. Longer documents are truncated to the first N
// pages; the read_document tool notes the truncation in its output.
const MaxPages = 20

// ErrNoRasterizer means no PDF rasterizer was found on $PATH.
var ErrNoRasterizer = errors.New("no PDF rasterizer found")

// rasterizer is one candidate in the conversion chain.
type rasterizer struct {
	name  string
	avail func() bool
	run   func(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error)
}

// rasterizers is the first-wins chain. Order = most reliable / common first.
// poppler's pdftoppm and mupdf's mutool are dedicated, reliable PDF tools;
// ImageMagick (magick/convert) renders PDF via a ghostscript delegate that is
// frequently policy-disabled out of the box; ghostscript itself is the last,
// ubiquitous fallback.
var rasterizers = []rasterizer{
	{name: "pdftoppm", avail: avail("pdftoppm"), run: rasterizeWithPdftoppm},
	{name: "mutool", avail: avail("mutool"), run: rasterizeWithMutool},
	{name: "magick", avail: avail("magick"), run: rasterizeWithMagickLike("magick")},
	{name: "convert", avail: avail("convert"), run: rasterizeWithMagickLike("convert")},
	{name: "gs", avail: avail("gs"), run: rasterizeWithGhostscript},
}

// rasterizersVar is the testable seam (tests inject fakes).
var rasterizersVar = rasterizers

func avail(bin string) func() bool {
	return func() bool { _, err := exec.LookPath(bin); return err == nil }
}

// IsPDF reports whether path begins with the PDF magic bytes (%PDF-). Detecting
// by content (not extension) means data:-URI PDFs written to a .bin temp are
// recognized too.
func IsPDF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hdr := make([]byte, len(pdfMagic))
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false
	}
	return string(hdr) == pdfMagic
}

// Rasterize converts pdfPath into one PNG per page (<=maxPages) in outDir,
// returning the page paths sorted by page number. It uses the first available
// rasterizer on $PATH. maxPages<=0 uses MaxPages. Returns a wrapped
// ErrNoRasterizer (with install hints) if none is available, or an error
// naming the tools that were tried if they all failed.
func Rasterize(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error) {
	if maxPages <= 0 {
		maxPages = MaxPages
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir rasterize out dir: %w", err)
	}
	var tried []string
	for _, r := range rasterizersVar {
		if !r.avail() {
			continue
		}
		tried = append(tried, r.name)
		paths, err := r.run(ctx, pdfPath, outDir, maxPages)
		if err == nil && len(paths) > 0 {
			return paths, nil
		}
	}
	if len(tried) == 0 {
		return nil, fmt.Errorf("%w: install one of poppler (pdftoppm), mupdf (mutool), imagemagick (magick/convert), or ghostscript (gs)", ErrNoRasterizer)
	}
	return nil, fmt.Errorf("PDF rasterization failed (tried %s); check the file or install a different rasterizer", strings.Join(tried, ", "))
}

// rasterizeWithPdftoppm: `pdftoppm -png -r 150 -l <maxPages> <pdf> <prefix>`.
// Renders pages 1..maxPages as <prefix>-N.png.
func rasterizeWithPdftoppm(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error) {
	prefix := filepath.Join(outDir, "page")
	cmd := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", "150", "-l", strconv.Itoa(maxPages), pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return sortedPNGs(outDir)
}

// rasterizeWithMutool: `mutool draw -o <dir>/%d.png -r 150 <pdf> 1-<maxPages>`.
func rasterizeWithMutool(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error) {
	outPattern := filepath.Join(outDir, "%d.png")
	cmd := exec.CommandContext(ctx, "mutool", "draw", "-o", outPattern, "-r", "150", pdfPath, fmt.Sprintf("1-%d", maxPages))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("mutool: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return sortedPNGs(outDir)
}

// rasterizeWithMagickLike: `<bin> -density 150 <pdf>[0-<maxPages-1>] <dir>/page.png`.
// ImageMagick is 0-indexed; its PDF support often requires the ghostscript
// delegate and is frequently policy-disabled, so it is a middle-of-chain best
// effort. The glob picks up page.png, page-0.png, ... it emits.
func rasterizeWithMagickLike(bin string) func(context.Context, string, string, int) ([]string, error) {
	return func(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error) {
		rangeArg := fmt.Sprintf("%s[0-%d]", pdfPath, maxPages-1)
		out := filepath.Join(outDir, "page.png")
		cmd := exec.CommandContext(ctx, bin, "-density", "150", rangeArg, out)
		if o, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("%s: %w: %s", bin, err, strings.TrimSpace(string(o)))
		}
		return sortedPNGs(outDir)
	}
}

// rasterizeWithGhostscript: `gs -dBATCH -dNOPAUSE -dQUIET -sDEVICE=png16m -r150
// -dFirstPage=1 -dLastPage=<maxPages> -sOutputFile=<dir>/%d.png <pdf>`.
func rasterizeWithGhostscript(ctx context.Context, pdfPath, outDir string, maxPages int) ([]string, error) {
	out := filepath.Join(outDir, "%d.png")
	cmd := exec.CommandContext(ctx, "gs",
		"-dNOPAUSE", "-dBATCH", "-dQUIET",
		"-sDEVICE=png16m", "-r150",
		"-dFirstPage=1", fmt.Sprintf("-dLastPage=%d", maxPages),
		"-sOutputFile="+out, pdfPath)
	if o, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gs: %w: %s", err, strings.TrimSpace(string(o)))
	}
	return sortedPNGs(outDir)
}

// sortedPNGs globs *.png in dir and returns them sorted by the trailing page
// number embedded in each filename (numeric, so page-2 sorts before page-10).
func sortedPNGs(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		return pageNum(matches[i]) < pageNum(matches[j])
	})
	return matches, nil
}

// pageNum extracts the last run of digits in a filename as an int (0 if none),
// for numeric sorting of rasterized page files.
func pageNum(path string) int {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	end := -1
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] >= '0' && base[i] <= '9' {
			end = i
			break
		}
	}
	if end < 0 {
		return 0
	}
	start := end
	for start > 0 && base[start-1] >= '0' && base[start-1] <= '9' {
		start--
	}
	n, _ := strconv.Atoi(base[start : end+1])
	return n
}
