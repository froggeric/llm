// Package format encodes a tool's parsed output into the representation
// requested via the CLI --format flag (text/markdown/json/yaml/xml).
//
// It is a CLI-layer post-processor: the 9 tool prompts and the MCP server
// path are untouched. The model always produces its natural output; this
// package converts that into the requested encoding for machine consumption
// (scripting, piping to jq, batch sidecar files).
//
// # Honest limitation
//
// Without constrained decoding (ROADMAP Theme F4), the machine formats wrap
// the model's natural output rather than imposing a per-tool JSON schema.
// The wrapping is always structurally valid JSON/YAML/XML; deeply-structured
// per-tool output (e.g. a chart described as {"chart_type": ...}) is a future
// enhancement gated on grammar-constrained sampling. The one structured tool
// today is extract_code, whose {language, code} result is emitted as-is.
package format

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format is an output encoding requested via --format.
type Format string

const (
	// Auto means "no explicit format"; the caller uses its presentational
	// default (colored markdown in a TTY, plain text when piped). Convert is
	// not called in this case.
	Auto Format = ""
	// Text is plain text: the model's output with no markdown chrome. For
	// extract_code this is the code body alone (no fence).
	Text Format = "text"
	// Markdown is the model's natural output, which is markdown for 8 of 9
	// tools. For extract_code the code is wrapped in a fenced block.
	Markdown Format = "markdown"
	// JSON emits a wrapped object: {"tool": <id>, "result": <parsed>}. Always
	// valid JSON. result is a string for most tools; {language, code} for
	// extract_code.
	JSON Format = "json"
	// YAML emits the same structure as JSON, YAML-encoded.
	YAML Format = "yaml"
	// XML emits the same structure as JSON, XML-encoded.
	XML Format = "xml"
)

// Parse resolves a (case-insensitive) format name. Empty (or "auto") returns
// Auto with no error, meaning "no format requested".
func Parse(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return Auto, nil
	case "text", "txt":
		return Text, nil
	case "markdown", "md":
		return Markdown, nil
	case "json":
		return JSON, nil
	case "yaml", "yml":
		return YAML, nil
	case "xml":
		return XML, nil
	default:
		return Auto, fmt.Errorf("unknown --format %q (valid: text, markdown, json, yaml, xml)", s)
	}
}

// Suffix returns the file extension (without dot) to use for a format, e.g.
// "json". Used by --output-dir to name per-image result files. Auto and the
// human formats map to a readable extension.
func Suffix(f Format) string {
	switch f {
	case JSON:
		return "json"
	case YAML:
		return "yaml"
	case XML:
		return "xml"
	case Markdown:
		return "md"
	case Text:
		return "txt"
	default:
		return "txt"
	}
}

// Convert encodes a tool's parsed output in the requested format.
//
// toolID is the source tool; parsed is the value returned by
// tools.Tool.ParseOutput (a string for most tools, a {language, code} map for
// extract_code). Text and Markdown produce human-readable output; JSON/YAML/XML
// produce machine-readable, always-valid encodings that wrap the result.
//
// Auto is handled by the caller (presentational rendering); passing it here
// behaves like Text.
func Convert(toolID string, parsed any, requested Format) ([]byte, error) {
	switch requested {
	case Text:
		return []byte(renderHuman(toolID, parsed, false)), nil
	case Auto, Markdown:
		return []byte(renderHuman(toolID, parsed, true)), nil
	case JSON:
		return encodeJSON(toolID, parsed)
	case YAML:
		return encodeYAML(toolID, parsed)
	case XML:
		return encodeXML(toolID, parsed)
	default:
		return nil, fmt.Errorf("unsupported format %q", requested)
	}
}

// resultEnvelope is the structure emitted for the machine formats. It always
// carries the source tool ID and the tool's parsed result so a consumer can
// route on tool without re-parsing prose.
type resultEnvelope struct {
	Tool   string `json:"tool" yaml:"tool"`
	Result any    `json:"result" yaml:"result"`
}

// renderHuman returns the human-readable form. When fenced is true, extract_code
// output is wrapped in a fenced code block (Markdown); when false, the code
// body is emitted bare (Text). All other tools emit their parsed string
// unchanged in both modes.
func renderHuman(toolID string, parsed any, fenced bool) string {
	if lang, code, ok := asCode(parsed); ok {
		if !fenced {
			return code
		}
		if lang != "" {
			return "```" + lang + "\n" + code + "\n```"
		}
		return "```\n" + code + "\n```"
	}
	switch s := parsed.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		// Defensive: a future tool could return another type. Fall back to
		// indented JSON so we never emit nothing.
		b, err := json.MarshalIndent(parsed, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", parsed)
		}
		return string(b)
	}
}

// encodeJSON wraps the parsed result and encodes as pretty-printed JSON.
func encodeJSON(toolID string, parsed any) ([]byte, error) {
	env := resultEnvelope{Tool: toolID, Result: envelopeValue(parsed)}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return nil, fmt.Errorf("encode json: %w", err)
	}
	// json.Encoder appends a newline; trim it so callers control termination.
	return []byte(strings.TrimRight(b.String(), "\n")), nil
}

// encodeYAML wraps the parsed result and encodes as YAML.
func encodeYAML(toolID string, parsed any) ([]byte, error) {
	env := resultEnvelope{Tool: toolID, Result: envelopeValue(parsed)}
	out, err := yaml.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}
	return out, nil
}

// envelopeValue projects parsed into a JSON/YAML/XML-safe value. extract_code's
// map is kept as-is (both libraries marshal it); nil becomes an empty string so
// JSON null never appears in output.
func envelopeValue(parsed any) any {
	if parsed == nil {
		return ""
	}
	return parsed
}

// asCode reports whether parsed is an extract_code result ({language, code})
// and returns its parts. Detection keys on the presence of a "code" key, which
// only extract_code produces, so this package stays decoupled from the tools
// package's ID constants.
func asCode(parsed any) (lang, code string, ok bool) {
	m, isMap := parsed.(map[string]any)
	if !isMap {
		return "", "", false
	}
	c, has := m["code"]
	if !has {
		return "", "", false
	}
	code, _ = c.(string)
	lang, _ = m["language"].(string)
	return lang, code, true
}

// --- XML ---
//
// encoding/xml cannot marshal map[string]any, so the code result is projected
// onto a fixed structure rather than flowing through resultEnvelope.

// xmlTextResult is the XML shape for string-output tools.
type xmlTextResult struct {
	XMLName xml.Name `xml:"localvision"`
	Tool    string   `xml:"tool,attr"`
	Result  string   `xml:"result"`
}

// xmlCodeResult is the XML shape for extract_code.
type xmlCodeResult struct {
	XMLName  xml.Name `xml:"localvision"`
	Tool     string   `xml:"tool,attr"`
	Language string   `xml:"language,attr"`
	Code     string   `xml:"code"`
}

func encodeXML(toolID string, parsed any) ([]byte, error) {
	var v any
	if lang, code, ok := asCode(parsed); ok {
		v = xmlCodeResult{Tool: toolID, Language: lang, Code: code}
	} else {
		s, _ := parsed.(string)
		v = xmlTextResult{Tool: toolID, Result: s}
	}
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode xml: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}
