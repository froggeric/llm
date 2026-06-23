#!/usr/bin/env bash
# Multi-sampling experiment: instead of one query per image, fire N IDENTICAL
# queries back-to-back in ONE warmed llama-server session, then correlate.
# Questions this answers:
#   (1) Latency: how much cheaper are calls 2..N once the model is loaded+warm?
#   (2) Quality: does correlating N runs (majority / union) beat a single run?
#
# Scope = small + medium recommendations only (Q3.5-4B-nothink, Q3VL-8B-Q8).
# Images chosen by RUN-VARIANCE (not just struggle): at temp 0.1 these models
# are near-deterministic on extraction (OCR/code) but highly variable on
# open-ended UI description, so multi-sampling only has room to help on UI.
# extract-code-test-1 is the low-variance CONTRAST (correlation can't fix
# systematic errors). 5 repeats; the scorer derives 3x/4x/5x from prefixes.
set -e
cd "$(dirname "$0")/.."
M=/Volumes/ssd/llm-models
PAT='^(ui-test-1|ui-test-2|extract-code-test-1)'
REPS=5

# run_one <name> <gguf> <mmproj> <runid> [nothink]
run_one() {
  local name="$1" gguf="$2" mmp="$3" rid="$4" mode="${5:-think}"
  if [ ! -f "$gguf" ] || [ ! -f "$mmp" ]; then
    echo "SKIP $name [$mode] — model files not present ($gguf)"
    return 0
  fi
  local extra=()
  [ "$mode" = "nothink" ] && extra=(--disable-thinking)
  echo -e "\n=========== $name [$mode]  ${REPS}x repeats  (rid=$rid) ==========="
  python3 code/benchmark_llamaserver.py "$name" "$gguf" "$mmp" \
    --run-id "$rid" --image-pattern "$PAT" --tool-prompts \
    --repeat "$REPS" --temp 0.1 "${extra[@]}"
}

# small / fast recommendation (hybrid → nothink only)
run_one qwen3.5-4b  "$M/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf"          "$M/qwen3.5_4b/mmproj-F16.gguf"   repeat-qwen3.5-4b-nt  nothink
# default / medium recommendation (non-hybrid → think)
run_one qwen3-vl-8b "$M/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf" "$M/qwen3-vl-8b/mmproj-F16.gguf"  repeat-qwen3vl-8b    think

echo -e "\n=== MULTI-SAMPLING BENCHMARK COMPLETE ==="
