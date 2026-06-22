#!/bin/bash
# HuggingFace upload script for localvision v0.2 models.
#
# Run these commands to upload the 3 models to the froggeric namespace.
# Requires: huggingface-cli (pip install huggingface_hub) and HF auth
# (huggingface-cli login).
#
# After uploads complete, the SHA256 values in builtin.toml should match
# the files on HF. Verify with:
#   shasum -a 256 <local-file>  →  should match the gguf_sha256 field

set -e

HF_USER="froggeric"

echo "============================================================"
echo "1/3: Upload Qwen3-VL-8B-Instruct-Q8_0.gguf + mmproj"
echo "============================================================"
huggingface-cli upload $HF_USER/Qwen3-VL-8B-Instruct-GGUF \
  /Volumes/ssd/llm-models/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf \
  Qwen3-VL-8B-Instruct-Q8_0.gguf \
  --repo-type model

# mmproj already exists if the Q4 was uploaded previously; re-upload to be safe
huggingface-cli upload $HF_USER/Qwen3-VL-8B-Instruct-GGUF \
  /Volumes/ssd/llm-models/qwen3-vl-8b/mmproj-F16.gguf \
  mmproj-F16.gguf \
  --repo-type model

echo ""
echo "============================================================"
echo "2/3: Upload Qwen3.5-4B Q4_K_M + mmproj (new repo)"
echo "============================================================"
huggingface-cli repo create $HF_USER/Qwen3.5-4B-GGUF --type model || true

huggingface-cli upload $HF_USER/Qwen3.5-4B-GGUF \
  /Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf \
  Qwen3.5-4B-Q4_K_M.gguf \
  --repo-type model

huggingface-cli upload $HF_USER/Qwen3.5-4B-GGUF \
  /Volumes/ssd/llm-models/qwen3.5_4b/mmproj-F16.gguf \
  mmproj-F16.gguf \
  --repo-type model

echo ""
echo "============================================================"
echo "3/3: Upload Qwen3.6-27B Q4_K_M + mmproj (new repo)"
echo "============================================================"
huggingface-cli repo create $HF_USER/Qwen3.6-27B-GGUF --type model || true

huggingface-cli upload $HF_USER/Qwen3.6-27B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf \
  Qwen3.6-27B-Q4_K_M.gguf \
  --repo-type model

huggingface-cli upload $HF_USER/Qwen3.6-27B-GGUF \
  /Volumes/ssd/llm-models/qwen3.6-27b/mmproj-F16.gguf \
  mmproj-F16.gguf \
  --repo-type model

echo ""
echo "============================================================"
echo "Optional: Delete deprecated Qwen3-VL-4B repo"
echo "============================================================"
echo "To remove the old Qwen3-VL-4B repo (no longer recommended):"
echo "  Go to https://huggingface.co/$HF_USER/Qwen3-VL-4B-Instruct-GGUF/settings"
echo "  Click 'Delete this repository'"
echo ""
echo "Or keep it for one release as a deprecation period."

echo ""
echo "Done. Verify the catalog with:"
echo "  go test ./internal/models/ -run TestLoad_NoOverlayDir -v"
