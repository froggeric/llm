# Third-Party Licenses

`local-vision-mcp` includes, downloads, or depends on the following
third-party software. This document lists each component and its license so
that redistributors can satisfy notice requirements.

## Go module dependencies

These dependencies are declared in `go.mod` and compiled into the
`local-vision-mcp` binary at build time.

### `github.com/modelcontextprotocol/go-sdk`

- **Purpose:** Official Go SDK for the Model Context Protocol. Used by
  `internal/mcpserver` for JSON-RPC framing, tool registration, and the
  stdio transport.
- **License:** MIT
- **Repository:** <https://github.com/modelcontextprotocol/go-sdk>
- **Version pinned in `go.mod`:** v1.6.1

> Permission is hereby granted, free of charge, to any person obtaining a
> copy of this software and associated documentation files (the
> "Software"), to deal in the Software without restriction ...

### `github.com/BurntSushi/toml`

- **Purpose:** TOML parser used by `internal/models` to load the model
  catalog and user overlays.
- **License:** MIT
- **Repository:** <https://github.com/BurntSushi/toml>
- **Version pinned in `go.mod`:** v1.6.0

### `github.com/stretchr/testify`

- **Purpose:** Test assertions and mocks. Used only in test files
  (`*_test.go`); not linked into the shipped binary.
- **License:** MIT
- **Repository:** <https://github.com/stretchr/testify>
- **Version pinned in `go.mod`:** v1.11.1

The full text of the MIT license is reproduced below.

## Bundled / auto-downloaded binaries

### `llama.cpp` (`llama-server`)

- **Purpose:** Inference engine. `local-vision-mcp` does **not** ship the
  `llama-server` binary in this repository; instead, it downloads a pinned
  build at first run from `huggingface.co/froggeric/`. The SHA256 of the
  downloaded binary is hardcoded in `internal/llama/binary.go`.
- **License:** MIT
- **Upstream:** <https://github.com/ggml-org/llama.cpp>
- **Authors:** Georgi Gerganov and contributors.

When `local-vision-mcp` downloads `llama-server`, it also places a copy of
the upstream MIT `LICENSE` file alongside the binary as
`LICENSE-llama.cpp.txt` (per Track C). The Go binary's `--license` /
`license` subcommand prints both the PolyForm Noncommercial license that
covers the wrapper code and this MIT notice for the bundled inference
engine.

## Model weights (informational)

The model GGUF and mmproj files downloaded from
`huggingface.co/froggeric/` are **not** part of this source distribution and
are **not** covered by the PolyForm Noncommercial license that covers
`local-vision-mcp` itself. Each model is governed by its own upstream
license (typically Apache-2.0 or MIT for the v0.1 model set). The catalog
in `internal/models/builtin.toml` records the `license` field per model
entry; consult the model card on Hugging Face for authoritative terms.

---

## MIT License (full text)

For the dependencies listed above, the following terms apply:

```
MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
