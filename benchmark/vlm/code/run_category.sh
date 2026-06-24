#!/usr/bin/env bash
# Per-category self-consistency sweep: does multi-sampling (+ correlation) help,
# and does it differ by CATEGORY / MODEL? Fires each localvision tool prompt on
# one PROBLEMATIC image per category, at 3 temperatures x N repeats, in one
# warmed llama-server session per (temp). The scorer compares single vs union@N
# vs maj@N and tells you which categories benefit.
#
# Usage: ./run_category.sh <model> [<model> ...]    (default: qwen3-vl-8b)
#   REPS=3 ./run_category.sh glm-4.6v-flash-9b qwen3.5-9b   # override repeat count
#
# REPS defaults to 3 (the sweet spot -- see REPEAT-REPORT.md: union@3 ~= union@5,
# so 5 costs ~60% more time for ~0 extra quality).
# NOTE: no `set -e`; the harness handles per-cell errors and exits 0. Models whose
# GGUF is not on disk SKIP gracefully (re-download, then re-run just those).
cd "$(dirname "$0")/.."
M=/Volumes/ssd/llm-models
MAP=code/category_prompts.json
REPS=${REPS:-3}
TEMPS=(0.1 0.4 0.7)
MODELS=("$@"); [ ${#MODELS[@]} -eq 0 ] && MODELS=(qwen3-vl-8b)

# run_sweep <name> <gguf> <mmproj> <runid_prefix> <mode> <budget>
run_sweep() {
  local name="$1" gguf="$2" mmp="$3" rid="$4" mode="${5:-think}" budget="${6:-0}"
  if [ ! -f "$gguf" ] || [ ! -f "$mmp" ]; then
    echo "SKIP $name [$mode] — model files not on disk ($gguf)"; return 0
  fi
  local extra=()
  [ "$mode" = "nothink" ] && extra+=(--disable-thinking)
  [ "$budget" = "1" ] && extra+=(--max-vision-budget)   # Gemma 4 dynamic resolution
  for T in "${TEMPS[@]}"; do
    echo -e "\n===== $name [$mode]  temp=$T  ${REPS}x repeats  (rid=${rid}-t${T}) ====="
    python3 code/benchmark_llamaserver.py "$name" "$gguf" "$mmp" \
      --run-id "${rid}-t${T}" --prompt-map "$MAP" --repeat "$REPS" --temp "$T" "${extra[@]}"
  done
}

for MODEL in "${MODELS[@]}"; do
  case "$MODEL" in
    qwen3-vl-8b)       GGUF="$M/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf";       MMP="$M/qwen3-vl-8b/mmproj-F16.gguf";    RID=cat-qwen3vl-8b;       MODE=think;   BUDGET=0 ;;
    qwen3.5-4b)        GGUF="$M/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf";                MMP="$M/qwen3.5_4b/mmproj-F16.gguf";     RID=cat-qwen3.5-4b;        MODE=nothink; BUDGET=0 ;;
    glm-4.6v-flash-9b) GGUF="$M/glm-4.6v-flash-9b/GLM-4.6V-Flash-Q4_K_M.gguf";     MMP="$M/glm-4.6v-flash-9b/mmproj-F16.gguf"; RID=cat-glm-9b;          MODE=think;   BUDGET=0 ;;
    qwen3.5-4b-Q8)     GGUF="$M/qwen3.5_4b/Qwen3.5-4B-Q8_0.gguf";                  MMP="$M/qwen3.5_4b/mmproj-F16.gguf";     RID=cat-qwen3.5-4b-Q8;     MODE=nothink; BUDGET=0 ;;
    qwen3.5-9b)        GGUF="$M/qwen3.5_9b/Qwen3.5-9B-Q4_K_M.gguf";                MMP="$M/qwen3.5_9b/mmproj-F16.gguf";     RID=cat-qwen3.5-9b;        MODE=nothink; BUDGET=0 ;;
    gemma4-e4b)        GGUF="$M/gemma4-e4b/gemma-4-E4B-it-Q4_K_M.gguf";            MMP="$M/gemma4-e4b/mmproj-gemma-4-E4B-it-BF16.gguf"; RID=cat-gemma4-e4b;   MODE=think;   BUDGET=1 ;;
    qwen3.6-35b-a3b)   GGUF="$M/qwen3.6-35b-a3b/Qwen3.6-35B-A3B-UD-Q4_K_M.gguf";   MMP="$M/qwen3.6-35b-a3b/mmproj-F16.gguf"; RID=cat-qwen3.6-35b-a3b;  MODE=nothink; BUDGET=0 ;;
    *) echo "unknown model: $MODEL (see script for supported names)"; exit 1 ;;
  esac
  run_sweep "$MODEL" "$GGUF" "$MMP" "$RID" "$MODE" "$BUDGET"
done
echo -e "\n=== CATEGORY SWEEP COMPLETE (${MODELS[*]}) ==="
