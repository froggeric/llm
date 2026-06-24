#!/bin/bash
# HuggingFace upload reference for the localvision catalog models (v0.7: 5 models).
#
# Mirror of record: huggingface.co/froggeric/<repo>. After uploading, the SHA256
# in internal/models/builtin.toml must match the file (verify with
# `shasum -a 256 <local-file>`). The lifecycle verifies the SHA on every load.
#
# Requires: the `hf` CLI (pip install huggingface_hub) + auth (`hf auth login`,
# a token with write scope on the froggeric namespace). `hf upload` auto-creates
# the repo if it doesn't exist.
#
# This is a reference of what was uploaded; re-running is idempotent (HF rejects
# identical content). Run the large MoE upload in the background — it's ~22 GB.

set -e

HF_USER="froggeric"

echo "============================================================"
echo "1. Qwen3-VL-8B-Instruct (Q8_0) — qwen3-vl-8b"
echo "============================================================"
hf upload $HF_USER/Qwen3-VL-8B-Instruct-GGUF \
  /Volumes/ssd/llm-models/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf \
  Qwen3-VL-8B-Instruct-Q8_0.gguf
hf upload $HF_USER/Qwen3-VL-8B-Instruct-GGUF \
  /Volumes/ssd/llm-models/qwen3-vl-8b/mmproj-F16.gguf \
  mmproj-F16.gguf

echo ""
echo "============================================================"
echo "2. Qwen3.5-4B (Q4_K_M + Q8_0) — qwen3.5-4b / qwen3.5-4b-q8"
echo "============================================================"
hf upload $HF_USER/Qwen3.5-4B-GGUF \
  /Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf \
  Qwen3.5-4B-Q4_K_M.gguf
hf upload $HF_USER/Qwen3.5-4B-GGUF \
  /Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q8_0.gguf \
  Qwen3.5-4B-Q8_0.gguf          # v0.7: routed for code/ui/diagram/error
hf upload $HF_USER/Qwen3.5-4B-GGUF \
  /Volumes/ssd/llm-models/qwen3.5_4b/mmproj-F16.gguf \
  mmproj-F16.gguf               # shared by the Q4 + Q8 variants

echo ""
echo "============================================================"
echo "3. Qwen3.6-27B (Q4_K_M) — qwen3.6-27b (opt-in)"
echo "============================================================"
hf upload $HF_USER/Qwen3.6-27B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf \
  Qwen3.6-27B-Q4_K_M.gguf
hf upload $HF_USER/Qwen3.6-27B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-27b/mmproj-F16.gguf \
  mmproj-F16.gguf

echo ""
echo "============================================================"
echo "4. Qwen3.6-35B-A3B MoE (UD-Q4_K_M) — qwen3.6-35b-a3b (opt-in; ~22 GB)"
echo "============================================================"
hf upload $HF_USER/Qwen3.6-35B-A3B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-35b-a3b/Qwen3.6-35B-A3B-UD-Q4_K_M.gguf \
  Qwen3.6-35B-A3B-UD-Q4_K_M.gguf
hf upload $HF_USER/Qwen3.6-35B-A3B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-35b-a3b/mmproj-F16.gguf \
  mmproj-F16.gguf

echo ""
echo "Done. Verify reachability + the catalog SHAs:"
echo "  curl -sIL -o /dev/null -w '%{http_code}\\n' https://huggingface.co/$HF_USER/<repo>/resolve/main/<file>"
echo "  go test ./internal/models/..."
