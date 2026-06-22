#!/bin/bash
# Extension: also benchmark Q3.5-4B and Q3.6-35B-A3B with thinking disabled.
# These are also thinking models; comparison data will be interesting.
# Waits for the existing nothink experiment to finish first.

set -e
cd "$(dirname "$0")/.."

MODELS=(
  "qwen3.5-4b|/Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.5_4b/mmproj-F16.gguf"
  "qwen3.6-35b-a3b|/Volumes/ssd/llm-models/qwen3.6-35b-a3b/Qwen3.6-35B-A3B-UD-Q4_K_M.gguf|/Volumes/ssd/llm-models/qwen3.6-35b-a3b/mmproj-F16.gguf"
)

# Wait for existing nothink (120 cells = Q3.5-9B + Q3.6-27B × 3 passes each) to complete
echo "[$(date)] Waiting for existing nothink experiment (120 cells) to finish..."
while true; do
  count=$(python3 -c "
import json
n = 0
for line in open('benchmark-results/raw.jsonl'):
    r = json.loads(line)
    if r.get('type') == 'result' and r.get('ok') and r.get('thinking_disabled'):
        n += 1
print(n)
")
  if [ "$count" -ge 120 ]; then
    echo "[$(date)] Existing nothink complete ($count cells). Starting extension."
    break
  fi
  sleep 60
done

echo "=============================================================="
echo "Extension: thinking-disabled for Q3.5-4B + Q3.6-35B-A3B"
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
echo "Extension complete. Duration: $((ELAPSED / 60))m $((ELAPSED % 60))s"
echo "Ended: $(date)"
echo "=============================================================="
