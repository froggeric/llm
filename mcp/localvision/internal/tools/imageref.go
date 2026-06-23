package tools

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrUnsupportedImageSource is returned by ParseImageRef when the input
// is an http(s):// URL (rejected per F1.10) or otherwise unparseable.
//
// llama-server is bound to 127.0.0.1 and cannot fetch remote URLs, so a
// remote image URL is a hard error rather than a fallback.
var ErrUnsupportedImageSource = errors.New("unsupported image source")

// ErrInvalidImageInput is returned for malformed input — a missing comma in
// a data: URI, a non-base64 payload, an empty path, etc.
var ErrInvalidImageInput = errors.New("invalid image input")

// tempRegistry tracks which LocalPath values were created from data: URIs
// so CleanupImageRef can remove them after inference. The locked ImageRef
// struct in tool.go cannot grow fields, so we keep this sidecar.
//
// Keyed by absolute temp file path. The value's *sync.Once makes Cleanup
// idempotent per path even if multiple refs point to the same temp file
// (rare but possible if a caller reuses the same data: URI).
var tempRegistry sync.Map // map[string]*sync.Once

// ParseImageRef converts a single user-supplied image argument into a
// normalized ImageRef whose LocalPath is always an absolute filesystem path.
//
// Three input formats are accepted (F1.10):
//
//  1. **File path** (primary). May be relative or absolute; resolved to
//     absolute via filepath.Abs so subprocess argv is unambiguous regardless
//     of the llama-server working directory. The file is NOT checked for
//     existence here — callers can surface a clearer "not found" error at
//     use time.
//
//  2. **data:image/<mime>;base64,<payload>** (fallback). The base64 payload
//     is decoded and written to a fresh temp file under os.TempDir(). The
//     temp file path is registered for cleanup via CleanupImageRef; the
//     caller (or the executor) MUST invoke CleanupImageRef after inference
//     to avoid leaking files. Cleanup is safe to call multiple times.
//
//  3. **file://** URI. The path component is extracted and returned as the
//     LocalPath. file:// URIs are the form some MCP clients emit when
//     surfacing local files.
//
// Remote http(s):// URLs are REJECTED with a wrapped ErrUnsupportedImageSource.
// The error message guides the caller toward a supported alternative
// (download the image locally and pass via image_path, or embed the bytes
// via image_data).
//
// F1.10 / F3.4: image bytes are never logged; the Source field on the
// returned ImageRef is redacted for data: URIs to keep diagnostics free of
// base64 payloads.
func ParseImageRef(raw string) (ImageRef, error) {
	if raw == "" {
		return ImageRef{}, fmt.Errorf("%w: empty input", ErrInvalidImageInput)
	}

	switch {
	case strings.HasPrefix(raw, "data:"):
		path, err := writeDataURIToTemp(raw)
		if err != nil {
			return ImageRef{}, err
		}
		tempRegistry.Store(path, &sync.Once{})
		return ImageRef{
			LocalPath: path,
			Source:    redactDataURI(raw),
		}, nil

	case strings.HasPrefix(raw, "file://"):
		u, err := url.Parse(raw)
		if err != nil {
			return ImageRef{}, fmt.Errorf("%w: malformed file:// URI: %v", ErrInvalidImageInput, err)
		}
		if u.Path == "" {
			return ImageRef{}, fmt.Errorf("%w: file:// URI has no path component", ErrInvalidImageInput)
		}
		abs, err := filepath.Abs(u.Path)
		if err != nil {
			return ImageRef{}, fmt.Errorf("resolving file:// path %q: %w", u.Path, err)
		}
		return ImageRef{LocalPath: abs, Source: raw}, nil

	case strings.HasPrefix(raw, "http://"), strings.HasPrefix(raw, "https://"):
		return ImageRef{}, fmt.Errorf(
			"%w: remote URLs are not supported (llama-server is localhost-only "+
				"and cannot fetch %q). Download the image locally and pass it "+
				"via image_path, or embed the bytes via image_data as a data: URI",
			ErrUnsupportedImageSource, raw,
		)

	default:
		// Treat as a file path. Make absolute.
		abs, err := filepath.Abs(raw)
		if err != nil {
			return ImageRef{}, fmt.Errorf("resolving path %q: %w", raw, err)
		}
		return ImageRef{LocalPath: abs, Source: raw}, nil
	}
}

// CleanupImageRef removes the temp file backing ref, if it was created from
// a data: URI by ParseImageRef. Safe to call multiple times; the second and
// subsequent calls are no-ops. Safe to call on refs that point at user
// files (those are not registered and are left alone).
//
// Errors from os.Remove are swallowed: by the time Cleanup runs, the
// inference has already completed and the temp file's existence is no
// longer load-bearing. A failure here indicates either "already removed"
// (common when a test calls Cleanup twice) or "permission issue" (rare;
// there's nothing useful the caller can do).
func CleanupImageRef(ref ImageRef) {
	v, ok := tempRegistry.LoadAndDelete(ref.LocalPath)
	if !ok {
		return // not a temp file we created
	}
	once := v.(*sync.Once)
	once.Do(func() {
		_ = os.Remove(ref.LocalPath)
		// Best-effort: remove a now-empty parent temp dir (e.g. a PDF
		// rasterizer's out dir, created by read_document's Expander) without
		// ever touching the shared os.TempDir() itself. os.Remove is a no-op on
		// a non-empty dir, so this only succeeds once the page files inside it
		// are gone.
		if parent := filepath.Dir(ref.LocalPath); parent != os.TempDir() {
			_ = os.Remove(parent)
		}
	})
}

// CleanupImageRefs is the slice form of CleanupImageRef. Convenience for
// callers working with a ToolInput.Images slice.
func CleanupImageRefs(refs []ImageRef) {
	for _, r := range refs {
		CleanupImageRef(r)
	}
}

// RegisterTemp registers path for cleanup by CleanupImageRef/CleanupImageRefs,
// using the same tempRegistry that data: URIs go through. Used by document
// Expanders (e.g. read_document) to mark rasterized page temp files for reaping
// after inference. Safe to call on an empty path (no-op), on a path already
// registered, or on a real user file — Cleanup is a no-op if the path wasn't
// registered, so registering a non-temp path is harmless (it just makes it
// eligible for removal, so only ever call this on paths you own).
func RegisterTemp(path string) {
	if path == "" {
		return
	}
	tempRegistry.Store(path, &sync.Once{})
}

// redactDataURI returns the header portion of a data: URI with the payload
// replaced by "<...bytes...>" so logs and diagnostics don't carry image bytes.
func redactDataURI(raw string) string {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return raw
	}
	return raw[:comma+1] + "<...bytes...>"
}

// writeDataURIToTemp decodes a data: URI (base64-encoded) and writes the
// bytes to a new file under os.TempDir() with a random name. Returns the
// absolute path of the temp file.
//
// The temp file is created with mode 0600 and is removed on any error
// path inside this function. The returned path is registered with
// tempRegistry by ParseImageRef so CleanupImageRef will remove it after
// inference completes.
func writeDataURIToTemp(dataURI string) (string, error) {
	// data:[<mediatype>][;base64],<data>
	comma := strings.IndexByte(dataURI, ',')
	if comma < 0 {
		return "", fmt.Errorf("%w: data URI missing comma separator", ErrInvalidImageInput)
	}
	header := dataURI[:comma] // e.g. "data:image/png;base64"
	payload := dataURI[comma+1:]

	var decoded []byte
	if strings.Contains(header, ";base64") || strings.HasSuffix(header, ";base64") {
		b, err := decodeBase64Loose(payload)
		if err != nil {
			return "", fmt.Errorf("%w: decoding base64 payload: %v", ErrInvalidImageInput, err)
		}
		decoded = b
	} else {
		// URL-encoded raw data (rare). Best-effort decode as ASCII bytes.
		decoded = []byte(payload)
	}

	ext := extFromDataURIHeader(header)
	f, err := os.CreateTemp("", "lvm-image-*"+ext)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := f.Write(decoded); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return f.Name(), nil
}

// decodeBase64Loose decodes a standard base64 string, tolerating missing
// padding. Some MCP clients strip trailing '=' for compactness; we restore
// it before handing off to StdEncoding.
func decodeBase64Loose(s string) ([]byte, error) {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.StdEncoding.DecodeString(s)
}

// extFromDataURIHeader maps the media-type portion of a data: URI header to
// a file extension. Unknown types default to ".bin" — llama-server usually
// sniffs the actual format from the file header anyway.
func extFromDataURIHeader(header string) string {
	// header is like "data:image/png;base64" or "data:image/jpeg"
	media := strings.TrimPrefix(header, "data:")
	media = strings.Split(media, ";")[0]
	switch strings.ToLower(media) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tif"
	case "":
		return ".bin"
	default:
		return ".bin"
	}
}
