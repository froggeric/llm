#!/usr/bin/env bash
# Per-category self-consistency sweep: does multi-sampling (+ correlation) help,
# and does it differ by CATEGORY? Fires each localvision tool prompt on one
# PROBLEMATIC image per category, at 3 temperatures × 5 repeats, in one warmed
# llama-server session per (temp). The scorer compares single vs union@5 vs
# maj@5 and tells you which categories benefit.
#
# Usage: ./run_category.sh [qwen3-vl-8b|qwen3.5-4b]   (default: qwen3-vl-8b)
#
# Model scope note: category benefit is the question. Run the default first,
# then the small/fast model on any category that looked promising. The scorer
# filters by model so the two never mix.
# NOTE: no `set -e` — the harness handles per-cell errors and exits 0; we don't
# want one python hiccup to abort the other temperatures in an unattended run.
cd "$(dirname "$0")/.."
M=/Volumes/ssd/llm-models
MAP=code/category_prompts.json
REPS=5
TEMPS=(0.1 0.4 0.7)
MODEL="${1:-qwen3-vl-8b}"

case "$MODEL" in
  qwen3-vl-8b) GGUF="$M/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf"; MMP="$M/qwen3-vl-8b/mmproj-F16.gguf"; RID=cat-qwen3vl-8b; MODE=think ;;
  qwen3.5-4b)  GGUF="$M/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf";          MMP="$M/qwen3.5_4b/mmproj-F16.gguf";      RID=cat-qwen3.5-4b;  MODE=nothink ;;
  *) echo "unknown model: $MODEL (use qwen3-vl-8b or qwen3.5-4b)"; exit 1 ;;
esac

# run_sweep <name> <gguf> <mmproj> <runid_prefix> <mode>
run_sweep() {
  local name="$1" gguf="$2" mmp="$3" rid="$4" mode="${5:-think}"
  if [ ! -f "$gguf" ] || [ ! -f "$mmp" ]; then
    echo "SKIP $name [$mode] — model files not present ($gguf)"; return 0
  fi
  local extra=(); [ "$mode" = "nothink" ] && extra=(--disable-thinking)
  for T in "${TEMPS[@]}"; do
    echo -e "\n===== $name [$mode]  temp=$T  ${REPS}x repeats  (rid=${rid}-t${T}) ====="
    python3 code/benchmark_llamaserver.py "$name" "$gguf" "$mmp" \
      --run-id "${rid}-t${T}" --prompt-map "$MAP" --repeat "$REPS" --temp "$T" "${extra[@]}"
  done
}

run_sweep "$MODEL" "$GGUF" "$MMP" "$RID" "$MODE"
echo -e "\n=== CATEGORY SWEEP COMPLETE ($MODEL) ==="
