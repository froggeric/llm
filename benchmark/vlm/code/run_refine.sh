#!/usr/bin/env bash
# Refinement benchmark: 9 recommended variants × 3 runs on the 7 new UI/OCR/code
# images, using the actual localvision per-tool prompts (--tool-prompts).
# Hybrids run in BOTH think and nothink; non-hybrids (Q3VL-8B-Q8, GLM Q4/Q8) think only.
set -e
cd "$(dirname "$0")/.."
M=/Volumes/ssd/llm-models
PAT='^(ui-test|ocr-test|extract-code-test)'

# run_variant <model_name> <gguf> <mmproj> <runid_prefix> [nothink]
run_variant() {
  local name="$1" gguf="$2" mmp="$3" rid="$4" mode="${5:-think}"
  if [ ! -f "$gguf" ] || [ ! -f "$mmp" ]; then
    echo "SKIP $name [$mode] — model files not present yet ($gguf)"
    return 0
  fi
  local extra=()
  [ "$mode" = "nothink" ] && extra=(--disable-thinking)
  for r in 1 2 3; do
    echo -e "\n=========== $name [$mode]  run $r  (rid=${rid}-${r}) ==========="
    python3 code/benchmark_llamaserver.py "$name" "$gguf" "$mmp" \
      --run-id "${rid}-${r}" --image-pattern "$PAT" --tool-prompts "${extra[@]}"
  done
}

# ---- think variants (run-id refine-*) ----
run_variant qwen3.5-4b        "$M/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf"            "$M/qwen3.5_4b/mmproj-F16.gguf"      refine
run_variant qwen3-vl-8b-Q8    "$M/qwen3-vl-8b/Qwen3-VL-8B-Instruct-Q8_0.gguf"   "$M/qwen3-vl-8b/mmproj-F16.gguf"     refine
run_variant glm-4.6v-flash-9b    "$M/glm-4.6v-flash-9b/GLM-4.6V-Flash-Q4_K_M.gguf" "$M/glm-4.6v-flash-9b/mmproj-F16.gguf" refine
run_variant glm-4.6v-flash-9b-Q8 "$M/glm-4.6v-flash-9b/GLM-4.6V-Flash-Q8_0.gguf"  "$M/glm-4.6v-flash-9b/mmproj-F16.gguf" refine
run_variant qwen3.5-9b        "$M/qwen3.5_9b/Qwen3.5-9B-Q4_K_M.gguf"            "$M/qwen3.5_9b/mmproj-F16.gguf"      refine
run_variant qwen3.6-27b       "$M/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf"          "$M/qwen3.6-27b/mmproj-F16.gguf"     refine
# ---- nothink variants (run-id refine-nt-*; hybrids only) ----
run_variant qwen3.5-4b        "$M/qwen3.5_4b/Qwen3.5-4B-Q4_K_M.gguf"            "$M/qwen3.5_4b/mmproj-F16.gguf"      refine-nt  nothink
run_variant qwen3.5-9b        "$M/qwen3.5_9b/Qwen3.5-9B-Q4_K_M.gguf"            "$M/qwen3.5_9b/mmproj-F16.gguf"      refine-nt  nothink
run_variant qwen3.6-27b       "$M/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf"          "$M/qwen3.6-27b/mmproj-F16.gguf"     refine-nt  nothink

echo -e "\n=== REFINEMENT BENCHMARK COMPLETE ==="
