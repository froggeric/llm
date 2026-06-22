#!/bin/bash
# Re-run Q3.5-9B and Q3.6-27B with thinking disabled (3 passes each).
# These two models had runaway thinking on dense images.
# Uses run_id=nothink-N to distinguish from original thinking-enabled runs.

set -e
cd "$(dirname "$0")/.."

MODELS=(
  "qwen3.5-9b|/Volumes/ssd/llm-models/qwen3.5_9b/Qwen3.5-9B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.5_9b/mmproj-F16.gguf"
  "qwen3.6-27b|/Volumes/ssd/llm-models/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.6-27b/mmproj-F16.gguf"
)

echo "=============================================================="
echo "Thinking-disabled re-run: Q3.5-9B + Q3.6-27B (3 passes each)"
echo "Started: $(date)"
echo "=============================================================="

PASS_START=$(date +%s)

for pass in 1 2 3; do
  echo ""
  echo "=============================================================="
  echo "PASS $pass (run_id=nothink-$pass)"
  echo "=============================================================="
  for entry in "${MODELS[@]}"; do
    IFS='|' read -r name gguf mmproj <<< "$entry"
    echo ""
    echo "--------------------------------------------------------------"
    echo "[$(date +%H:%M:%S)] $name × pass $pass (thinking DISABLED)"
    echo "--------------------------------------------------------------"
    python3 benchmark_llamaserver.py "$name" "$gguf" "$mmproj" \
      --port 8842 --run-id "nothink-$pass" --max-tokens 16384 --disable-thinking
  done
done

PASS_END=$(date +%s)
ELAPSED=$((PASS_END - PASS_START))
echo ""
echo "=============================================================="
echo "All thinking-disabled passes complete. Duration: $((ELAPSED / 60))m $((ELAPSED % 60))s"
echo "Ended: $(date)"
echo "=============================================================="
