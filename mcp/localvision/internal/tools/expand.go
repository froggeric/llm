package tools

import "context"

// Expander is implemented by tools that expand a single document input (e.g. a
// PDF) into multiple image refs (one per rasterized page) before inference.
//
// It is an OPTIONAL interface: the tools that don't implement it (all the
// single-image perception tools) pay nothing, and ExpandInput is a no-op for
// them. Keeping it separate from the Tool interface means adding the document
// modality doesn't grow every tool's method set.
type Expander interface {
	// ExpandImages turns the tool's input into the image refs the model should
	// actually see. For a non-document input the Expander may return the input
	// refs unchanged (lenient); for a document it rasterizes and returns one
	// ref per page, registering any temp files via RegisterTemp so
	// CleanupImageRefs reaps them after inference. ctx propagates to the
	// rasterizer subprocess so a hung conversion is cancellable.
	ExpandImages(ctx context.Context, input ToolInput) ([]ImageRef, error)
}

// ExpandInput expands a tool's image input iff the tool implements Expander
// (e.g. read_document rasterizing a PDF into page images). Non-Expanders get
// input back unchanged. The expanded refs REPLACE input.Images; the caller
// uses them for both BuildRequest and the executor, and the existing
// CleanupImageRefs defer reaps any temp files the Expander registered.
func ExpandInput(ctx context.Context, tool Tool, input ToolInput) (ToolInput, error) {
	ex, ok := tool.(Expander)
	if !ok {
		return input, nil
	}
	refs, err := ex.ExpandImages(ctx, input)
	if err != nil {
		return ToolInput{}, err
	}
	if refs != nil {
		input.Images = refs
	}
	return input, nil
}
