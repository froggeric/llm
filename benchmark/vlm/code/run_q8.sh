#!/bin/bash
# Q8 quantization benchmark — comprehensive version.
#
# Tests BOTH think and nothink modes for Qwen hybrid thinkers at Q8.
# Timeouts are RECORDED as valid data (they tell us if Q8 makes thinking
# runaway worse). Per user direction: we need to compare modes at Q8 too.
#
# Run ID scheme:
#   - Non-thinker models (gemma4-*, glm-*, qwen3-vl-*): "q8-1/2/3"
#   - Qwen hybrid thinkers (qwen3.5-*) THINK mode: "q8-think-1/2/3"
#   - Qwen hybrid thinkers (qwen3.5-*) NOTHINK mode: "q8-nothink-1/2/3"
#
# GLM-9B-Q8 excluded if not fully downloaded (check below).
#
# Estimated cells:
#   - 5 non-thinker models × 30 images × 3 passes = 450 cells
#   - 2 Qwen thinkers × 30 images × 3 passes × 2 modes = 360 cells
#   - Total: ~810 cells, est ~12-15 hours

set -e
cd "$(dirname "$0")/.."

# Non-thinker Q8 models (test once each)
NON_THINKERS=()
for entry in \
  "gemma4-12b-Q8|/Volumes/ssd/llm-models/gemma4-12b/gemma-4-12b-it-Q8_0.gguf|/Volumes/ssd/llm-models/gemma4-12b/mmproj-F16.gguf" \
  "gemma4-e4b-Q8|/Volumes/ssd/llm-models/gemma4-e4b/gemma-4-E4B-it-Q8_0.gguf|/Volumes/ssd/llm-models/gemma4-e4b/mmproj-F16.gguf" \
  "qwen3-vl-4b-Q8|/Volumes/ssd/llm-models/qwen3-vl-4b/Qwen3-VL-4B-Instruct-Q8_0.gguf|/Volumes/ssd/llm-models/qwen3-vl-4b/mmproj-F16.gguf" \
  "qwen3-vl-8b-Q8|/Volumes/ssd/llm-models/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf|/Volumes/ssd/llm-models/qwen3-vl-8b/mmproj-F16.gguf"
do
  IFS='|' read -r name gguf mmproj <<< "$entry"
  if [ -f "$gguf" ]; then
    NON_THINKERS+=("$entry")
  else
    echo "SKIP $name: $gguf not found"
  fi
done

# GLM-9B-Q8: only include if fully downloaded (>9GB)
GLM_SIZE=$(stat -f%z /Volumes/ssd/llm-models/GLM-4.6V-Flash-9B_latest/model-Q8_0.gguf 2>/dev/null || echo 0)
if [ "$GLM_SIZE" -gt 9000000000 ]; then
  NON_THINKERS+=("glm-4.6v-flash-9b-Q8|/Volumes/ssd/llm-models/GLM-4.6V-Flash-9B_latest/model-Q8_0.gguf|/Volumes/ssd/llm-models/GLM-4.6V-Flash-9B_latest/mmproj.gguf")
  echo "GLM-9B-Q8 included ($(( GLM_SIZE / 1024 / 1024 ))MB)"
else
  echo "SKIP glm-4.6v-flash-9b-Q8: only $(( GLM_SIZE / 1024 / 1024 ))MB downloaded (need >9000MB)"
fi

# Qwen hybrid thinkers (test BOTH modes)
THINKERS=()
for entry in \
  "qwen3.5-4b-Q8|/Volumes/ssd/llm-models/qwen3.5_4b/Qwen3.5-4B-Q8_0.gguf|/Volumes/ssd/llm-models/qwen3.5_4b/mmproj-F16.gguf" \
  "qwen3.5-9b-Q8|/Volumes/ssd/llm-models/qwen3.5_9b/Qwen3.5-9B-Q8_0.gguf|/Volumes/ssd/llm-models/qwen3.5_9b/mmproj-F16.gguf"
do
  IFS='|' read -r name gguf mmproj <<< "$entry"
  if [ -f "$gguf" ]; then
    THINKERS+=("$entry")
  else
    echo "SKIP $name: $gguf not found"
  fi
done

if [ ${#NON_THINKERS[@]} -eq 0 ] && [ ${#THINKERS[@]} -eq 0 ]; then
  echo "ERROR: No Q8 GGUFs available. Exiting."
  exit 1
fi

echo "=============================================================="
echo "Q8 Benchmark (comprehensive)"
echo "  Non-thinkers: ${#NON_THINKERS[@]} models × 3 passes"
echo "  Thinkers:     ${#THINKERS[@]} models × 3 passes × 2 modes (think+nothink)"
echo "Started: $(date)"
echo "=============================================================="

PASS_START=$(date +%s)

run_model() {
  local name="$1" gguf="$2" mmproj="$3" run_id="$4" extra_args="$5"
  echo ""
  echo "--------------------------------------------------------------"
  echo "[$(date +%H:%M:%S)] $name × run_id=$run_id $extra_args"
  echo "--------------------------------------------------------------"
  python3 benchmark_llamaserver.py "$name" "$gguf" "$mmproj" \
    --port 8842 --run-id "$run_id" --max-tokens 16384 $extra_args
}

for pass in 1 2 3; do
  echo ""
  echo "##############################################################"
  echo "# Q8 PASS $pass"
  echo "##############################################################"

  # Non-thinkers: single mode
  for entry in "${NON_THINKERS[@]}"; do
    IFS='|' read -r name gguf mmproj <<< "$entry"
    EXTRA_ARGS=""
    [[ "$name" == gemma4-* ]] && EXTRA_ARGS="--max-vision-budget"
    run_model "$name" "$gguf" "$mmproj" "q8-$pass" "$EXTRA_ARGS"
  done

  # Thinkers: BOTH modes
  for entry in "${THINKERS[@]}"; do
    IFS='|' read -r name gguf mmproj <<< "$entry"
    # Think mode (will produce timeouts on dense images — that's valid data)
    run_model "$name" "$gguf" "$mmproj" "q8-think-$pass" ""
    # Nothink mode
    run_model "$name" "$gguf" "$mmproj" "q8-nothink-$pass" "--disable-thinking"
  done
done

PASS_END=$(date +%s)
ELAPSED=$((PASS_END - PASS_START))
echo ""
echo "=============================================================="
echo "Q8 benchmark complete."
echo "Duration: $((ELAPSED / 60))m $((ELAPSED % 60))s"
echo "Ended: $(date)"
echo "=============================================================="
