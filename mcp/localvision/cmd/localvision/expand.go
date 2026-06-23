package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// imageExts are the file extensions (lowercase, no dot) treated as images
// when expanding a directory. Matches the formats llama-server accepts plus
// HEIC/HEIF (converted upstream via sips on macOS; cross-platform converter
// chain planned — see ROADMAP D5).
var imageExts = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "webp": true,
	"heic": true, "heif": true, "gif": true, "bmp": true,
	"tiff": true, "tif": true,
}

// hasGlobChars reports whether s contains shell-glob metacharacters.
func hasGlobChars(s string) bool { return strings.ContainsAny(s, "*?[") }

// isImageExt reports whether path has a recognized image extension.
func isImageExt(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	return imageExts[ext]
}

// isAcceptedExt reports whether a directory entry should be collected for the
// given tool: always an image extension, plus ".pdf" when the tool is
// read_document (so `localvision papers/ --type doc` picks up PDFs). Keeping
// PDF out of imageExts keeps that set semantically honest.
func isAcceptedExt(path string, includePDF bool) bool {
	if isImageExt(path) {
		return true
	}
	if includePDF {
		return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) == "pdf"
	}
	return false
}

// expandInputs resolves positional args into a flat, de-duplicated list of
// image references, supporting:
//   - literal file paths
//   - glob patterns (filepath.Glob; zero matches is an error)
//   - directories (image files within; recursive descends subdirectories)
//   - "-" (stdin: one path per line, for `find . -name '*.png' | localvision -`)
//
// data: and file:// URIs pass through verbatim (they are not filesystem paths
// and are resolved later by tools.ParseImageRef). Remote http(s):// URLs are
// rejected because the underlying llama-server is localhost-only.
//
// includePDF extends directory expansion to .pdf files (for read_document).
//
// Expansion is deterministic: WalkDir yields sorted paths, and dedup preserves
// first-seen order so batch output is stable.
func expandInputs(args []string, recursive, includePDF bool) ([]string, error) {
	var out []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "data:"), strings.HasPrefix(a, "file://"):
			add(a)
		case strings.HasPrefix(a, "http://"), strings.HasPrefix(a, "https://"):
			return nil, fmt.Errorf("remote URL %q rejected: llama-server is localhost-only; download the image and pass a local path", a)
		case a == "-":
			paths, err := readStdinPaths(os.Stdin)
			if err != nil {
				return nil, err
			}
			for _, p := range paths {
				add(p)
			}
		default:
			paths, err := expandOne(a, recursive, includePDF)
			if err != nil {
				return nil, err
			}
			for _, p := range paths {
				add(p)
			}
		}
	}
	return out, nil
}

// expandOne resolves a single non-URI, non-stdin argument: a literal file, a
// glob pattern, or a directory.
func expandOne(a string, recursive, includePDF bool) ([]string, error) {
	info, statErr := os.Stat(a)
	if statErr == nil {
		if info.IsDir() {
			paths, err := expandDir(a, recursive, includePDF)
			if err != nil {
				return nil, err
			}
			if len(paths) == 0 {
				noun := "image"
				if includePDF {
					noun = "image/PDF"
				}
				return nil, fmt.Errorf("directory %q contains no %s files", a, noun)
			}
			return paths, nil
		}
		// Literal file: accept any extension (the model/normalizer will reject
		// unreadable inputs with a clearer error than we can produce here).
		return []string{a}, nil
	}
	// Not stat-able. If it looks like a glob, try matching; otherwise surface
	// the original stat error so a missing literal file reads naturally.
	if hasGlobChars(a) {
		matches, err := filepath.Glob(a)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", a, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("glob %q matched no files", a)
		}
		return matches, nil
	}
	return nil, statErr
}

// expandDir returns the accepted files in dir (images, plus PDFs when
// includePDF). When recursive is false, only top-level files are returned
// (subdirectories are skipped); when true, the whole tree is walked. WalkDir
// yields a sorted, deterministic order.
func expandDir(dir string, recursive, includePDF bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == dir {
				return nil
			}
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if isAcceptedExt(path, includePDF) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", dir, err)
	}
	return out, nil
}

// readStdinPaths reads whitespace-trimmed, non-empty, non-comment lines from r
// as image paths. Used for the "-" sentinel so `find … | localvision -` works.
func readStdinPaths(r io.Reader) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("stdin contained no paths")
	}
	return out, nil
}
