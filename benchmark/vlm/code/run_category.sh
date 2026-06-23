#!/usr/bin/env bash
# Per-category self-consistency sweep: does multi-sampling (+ correlation) help,
# and does it differ by CATEGORY? Fires each localvision tool prompt on one
# PROBLEMATIC image per category, at 3 temperatures × 5 repeats, in one warmed
# llama-server session per (temp). The scorer compares single vs union@5 vs
# maj@5 and tells you which categories benefit.
#
# Model scope = the DEFAULT recommendation (Q3VL-8B-Q8). Category benefit is the
# question; add Q3.5-4B per-category afterward for any category that looks promising.
# NOTE: no `set -e` — the harness handles per-cell errors and exits 0; we don't
# want one python hiccup to abort the other temperatures in an unattended run.
cd "$(dirname "$0")/.."
M=/Volumes/ssd/llm-models
MAP=code/category_prompts.json
REPS=5
TEMPS=(0.1 0.4 0.7)

# run_sweep <name> <gguf> <mmproj> <runid_prefix> [nothink]
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

# NOTE: no `set -e` — the harness handles per-cell errors and exits 0; we don't
# want one python hiccup to abort the other temperatures in an unattended run.

run_sweep qwen3-vl-8b "$M/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf" "$M/qwen3-vl-8b/mmproj-F16.gguf" cat-qwen3vl-8b think

echo -e "\n=== CATEGORY SWEEP COMPLETE (default model) ==="
