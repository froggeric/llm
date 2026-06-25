package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/froggeric/llm/mcp/localvision/internal/document"
)

// read_document ingests a PDF by rasterizing its pages and sending them to the
// model in one inference for a document summary + per-page highlights. It
// implements Expander so a PDF becomes N page ImageRefs; a non-PDF input passes
// through unchanged (lenient — read_document on a single image still works).
//
// Requires a PDF rasterizer on $PATH (poppler/mupdf/imagemagick/ghostscript);
// none is bundled. Served by the default qwen3-vl-8b.
type readDocumentTool struct{}

// rasterize is the document.Rasterize entry point, hoisted to a package var so
// tests can swap it for a fake (CI has no real PDF rasterizer). Production code
// leaves it as document.Rasterize.
var rasterize = document.Rasterize

func (readDocumentTool) ID() string { return idReadDocument }

func (readDocumentTool) Description() string {
	return "Read a PDF document: rasterize its pages and summarize the content with per-page highlights, transcribed key text, and any tables or figures called out. Pass a PDF via image_path (or image_data/image_url). Useful for papers, reports, slides, and scans. Requires a PDF rasterizer on $PATH (poppler/mupdf/imagemagick/ghostscript)." + latencyHint
}

func (readDocumentTool) InputSchema() map[string]any {
	props := commonSchemaProperties()
	props["question"] = map[string]any{
		"type":        "string",
		"description": "Optional: a specific question to answer using the document's content.",
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"oneOf":      commonOneOf(),
	}
}

// MaxTokens is 2048 — a document summary across many pages needs the room.
func (readDocumentTool) MaxTokens() int { return 2048 }

func (readDocumentTool) SystemPrompt() string { return promptReadDocument }

// BuildRequest runs after ExpandInput, so for a PDF input.Images already holds
// the rasterized page refs. The schema accepts one document input (oneOf); the
// page count after expansion is lenient (>= 1). Returns SystemPrompt() so the
// default path satisfies TestBuildRequestSanity's invariant.
func (t readDocumentTool) BuildRequest(input ToolInput) (systemPrompt, userPrompt string, imagePaths []string, err error) {
	if len(input.Images) == 0 {
		return "", "", nil, fmt.Errorf("read_document requires one document (PDF) or image, got 0 (supply image_path, image_data, or image_url)")
	}
	paths := make([]string, 0, len(input.Images))
	for _, ref := range input.Images {
		paths = append(paths, ref.LocalPath)
	}
	return t.SystemPrompt(), singleImageUserPrompt(input.Extra, true), paths, nil
}

func (readDocumentTool) ParseOutput(_ ToolInput, raw string) (any, error) {
	return passthroughOutput(raw)
}

// ExpandImages implements Expander. If the single input ref points at a PDF
// (by magic bytes, so data:-URI PDFs written to a .bin temp are detected too),
// it rasterizes up to document.MaxPages pages into temp PNGs — each registered
// via RegisterTemp so CleanupImageRefs reaps them (and their now-empty out dir)
// after inference — and returns one ImageRef per page. A non-PDF input is
// returned unchanged (lenient).
func (readDocumentTool) ExpandImages(ctx context.Context, input ToolInput) ([]ImageRef, error) {
	if len(input.Images) != 1 {
		return input.Images, nil // not a single-document input; leave as-is
	}
	ref := input.Images[0]
	if !document.IsPDF(ref.LocalPath) {
		return input.Images, nil // not a PDF — process as a plain image
	}
	outDir, err := os.MkdirTemp("", "lvm-doc-*")
	if err != nil {
		return nil, fmt.Errorf("create rasterize temp dir: %w", err)
	}
	pages, err := rasterize(ctx, ref.LocalPath, outDir, document.MaxPages)
	if err != nil {
		_ = os.RemoveAll(outDir)
		return nil, fmt.Errorf("rasterize PDF %q: %w", ref.LocalPath, err)
	}
	refs := make([]ImageRef, 0, len(pages))
	for _, p := range pages {
		RegisterTemp(p)
		refs = append(refs, ImageRef{LocalPath: p, Source: ref.Source})
	}
	return refs, nil
}
