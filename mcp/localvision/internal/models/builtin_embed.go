package models

import _ "embed"

// builtinToml is the built-in model catalog, embedded at compile time so the
// binary ships with a complete catalog even before any user overlay files
// exist. Phase 3 (lead) replaces the placeholder SHA256s in builtin.toml
// with real values; Track D treats the file as immutable.
//
//go:embed builtin.toml
var builtinToml []byte

// BuiltinCatalog returns the raw TOML bytes of the built-in catalog. The
// caller is responsible for parsing it (typically via toml.Decode).
//
// The bytes are a snapshot of builtin.toml at compile time; changes to the
// file require a rebuild.
func BuiltinCatalog() ([]byte, error) {
	if len(builtinToml) == 0 {
		// Should be impossible with go:embed, but guard for safety.
		return nil, ErrInvalidCatalog
	}
	// Return a defensive copy so callers can't mutate the embedded slice.
	out := make([]byte, len(builtinToml))
	copy(out, builtinToml)
	return out, nil
}
