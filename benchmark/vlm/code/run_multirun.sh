#!/bin/bash
# Multi-run benchmark: run all 11 models × 20 images × 2 additional passes (run_id=2 and run_id=3)
# Each pass is serial (one model at a time) to avoid resource contention.
# Expected duration: ~4-5 hours total.

set -e
cd "$(dirname "$0")/.."

MODELS=(
  "gemma4-12b|/Volumes/ssd/llm-models/gemma4-12b/gemma-4-12b-it-Q4_K_M.gguf|/Volumes/ssd/llm-models/gemma4-12b/mmproj-F16.gguf"
  "gemma4-26b-a4b|/Volumes/ssd/llm-models/gemma4-26b-a4b/gemma-4-26B-A4B-it-UD-Q4_K_M.gguf|/Volumes/ssd/llm-models/gemma4-26b-a4b/mmproj-F16.gguf"
  "gemma4-31b|/Volumes/ssd/llm-models/gemma4-31b/gemma-4-31B-it-Q4_K_M.gguf|/Volumes/ssd/llm-models/gemma4-31b/mmproj-F16.gguf"
  "gemma4-e4b|/Volumes/ssd/llm-models/gemma4-e4b/gemma-4-E4B-it-Q4_K_M.gguf|/Volumes/ssd/llm-models/gemma4-e4b/mmproj-F16.gguf"
  "glm-4.6v-flash-9b|/Volumes/ssd/llm-models/GLM-4.6V-Flash-9B_latest/model.gguf|/Volumes/ssd/llm-models/GLM-4.6V-Flash-9B_latest/mmproj.gguf"
  "qwen3-vl-4b|/Volumes/ssd/llm-models/qwen3-vl-4b/Qwen3-VL-4B-Instruct-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3-vl-4b/mmproj-F16.gguf"
  "qwen3-vl-8b|/Volumes/ssd/llm-models/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3-vl-8b/mmproj-F16.gguf"
  "qwen3.5-4b|/Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.5_4b/mmproj-F16.gguf"
  "qwen3.5-9b|/Volumes/ssd/llm-models/qwen3.5_9b/Qwen3.5-9B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.5_9b/mmproj-F16.gguf"
  "qwen3.6-27b|/Volumes/ssd/llm-models/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.6-27b/mmproj-F16.gguf"
  "qwen3.6-35b-a3b|/Volumes/ssd/llm-models/qwen3.6-35b-a3b/Qwen3.6-35B-A3B-UD-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.6-35b-a3b/mmproj-F16.gguf"
)

RUN_ID="${1:-2}"  # pass as arg; default 2
echo "=============================================================="
echo "Multi-run benchmark pass: run_id=$RUN_ID"
echo "Models: ${#MODELS[@]}"
echo "Started: $(date)"
echo "=============================================================="

PASS_START=$(date +%s)

for entry in "${MODELS[@]}"; do
  IFS='|' read -r name gguf mmproj <<< "$entry"
  echo ""
  echo "--------------------------------------------------------------"
  echo "[$(date +%H:%M:%S)] Running $name (run_id=$RUN_ID)"
  echo "--------------------------------------------------------------"
  python3 benchmark_llamaserver.py "$name" "$gguf" "$mmproj" \
    --port 8842 --run-id "$RUN_ID" --max-tokens 16384
done

PASS_END=$(date +%s)
ELAPSED=$((PASS_END - PASS_START))
echo ""
echo "=============================================================="
echo "Pass $RUN_ID complete. Duration: $((ELAPSED / 60))m $((ELAPSED % 60))s"
echo "Ended: $(date)"
echo "=============================================================="
