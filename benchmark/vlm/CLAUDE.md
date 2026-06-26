# CLAUDE.md — benchmark/vlm

Benchmarking open-weights VLMs for a local vision-MCP server for Claude Code. Catalog at [`mcp/localvision`](../../mcp/localvision).

## Current state (2026-06-26)

- **v5/v6 is authoritative** — `code/score_v5.py` / `code/score_v5_multirun.py` / `code/score_q8_multirun.py`. v4 frozen for historical comparison (had a think/nothink conflation bug, deleted during cleanup).
- 15 model variants × 30 images × 3 runs aggregated for Q4_K_M; Q8_0 comparison complete on 7 small/mid models.
- Top recommendation: `Q3.6-27B-nothink` (~79.6/100, σ=0.24) — see [`BENCHMARK-REPORT-v5.md`](./BENCHMARK-REPORT-v5.md).
- **Multi-sampling investigation done** (2026-06-23/24) — does re-querying a warm model help? [`REPEAT-REPORT.md`](./REPEAT-REPORT.md) (latency of warm re-queries + single/union/majority correlation at temp 0.1, with the temp sweep folded in) and [`CATEGORY-REPORT.md`](./CATEGORY-REPORT.md) (7-model × 8-category × 3-temp sweep: **temperature is the gate**, aggregator must match the task). Raw data: `run_id=repeat-*` (30 calls, temp 0.1) and `run_id=cat-*-t{0.1,0.4,0.7}` (the temp sweep); scorers `code/score_repeat.py` / `score_category.py`; orchestrators `code/run_repeat.sh` / `run_category.sh`. Now also **LLM-judged** (2026-06-26): `code/prepare_judge.py` → Claude-judge subagents → `code/aggregate_judgments.py` over `benchmark-results/judgments_repeat/` (30 responses) + `judgments_cat/` (600 responses), adding `holistic_score` + free-form hallucinations alongside the deterministic scorers.

## Key files

- `code/benchmark_llamaserver.py` — test harness (webp-aware, `--disable-thinking`, `--image-pattern`, `--call-timeout`, `--watchdog-timeout`). Uses `Path(__file__).parent.parent` so it runs from any cwd.
- `code/score_v5.py` — deterministic probes for all 30 images + failure mode detector
- `code/score_v5_multirun.py` — multi-run aggregation (median-run judged); keys cells as `model|think` / `model|nothink`. Uses cwd-relative paths — run from `benchmark/vlm/`.
- `code/score_q8_multirun.py` — Q8 aggregator (loads only `q8-*` run IDs)
- `code/score_repeat.py` / `score_category.py` — multi-sampling scorers (`repeat-*` latency/correlation; `cat-*` temp sweep with P/R/F1 + hallucination). See [`REPEAT-REPORT.md`](./REPEAT-REPORT.md) / [`CATEGORY-REPORT.md`](./CATEGORY-REPORT.md).
- `code/prepare_judge.py` / `dispatch_multirun_judges.py` / `aggregate_judgments.py` — LLM-judge pipeline for repeat/cat (prepare inputs → Claude-judge subagents → aggregate holistic/hallucinations into `judgments_repeat/` + `judgments_cat/`).
- `code/run_q8.sh` / `run_multirun.sh` / `run_nothink.sh` / `run_nothink_ext.sh` / `run_repeat.sh` / `run_category.sh` — orchestrators (all `cd "$(dirname "$0")/.."` themselves)
- `test-images/GROUND-TRUTH.md` — owner-verified truth for all 30 images
- `benchmark-results/raw.jsonl` — append-only raw responses (~2,200 lines, 12 MB)
- `benchmark-results/judgments_v5/` — 60 LLM-judge outputs
- `benchmark-results/judgments_q8/` — Q8 judge outputs

## Operational gotchas

- **Qwen hybrid thinkers (Q3.5/Q3.6)**: always run with `--disable-thinking` for vision tasks. v5 confirmed all 4 benefit.
- **Gemma 4 vision**: must set `--max-vision-budget` (560/2240) or default 280 tokens is essentially blind.
- **WebP**: llama-server silently drops webp; harness converts via `sips`. Don't put webp images directly.
- **Dense images + thinking = timeouts**. 300s call timeout + 360s watchdog catch this fast.
- **LLM judges flag correct OCR as hallucination if GT is incomplete**. Dense scenes (Waldo-style) must enumerate every identifiable object category.

## Adding new test images

1. Drop file in `test-images/` (PNG/JPG preferred; webp works via sips conversion)
2. Add owner-verified entry to `test-images/GROUND-TRUTH.md` (use existing entries as template; include "verified YYYY-MM-DD" markers)
3. Add probes for the image to `code/score_v5.py` (`probes_for_image()` + weights in `deterministic_score()`)
4. Run `python3 code/benchmark_llamaserver.py <name> <gguf> <mmproj> --run-id <id>` for each model

## Downloading models from HuggingFace

- `hf` CLI stalls on large files via xet/cas-bridge CDN (~800MB then 0 B/s)
- Use direct curl: `curl -L -C - --retry 5 -o <dest> https://huggingface.co/{repo}/resolve/main/{file}`
- Repo locations: Qwen3.5/3.6 GGUFs at `unsloth/` (NOT `Qwen/`), GLM-4.6V-Flash GGUF at `unsloth/GLM-4.6V-Flash-GGUF` (no "9B" in repo name)
- All repos public; no HF auth needed

## Running the benchmark

All commands assume cwd = `benchmark/vlm/`. The shell orchestrators handle the cd themselves.

```bash
# Q4 single model
python3 code/benchmark_llamaserver.py <name> <gguf> <mmproj> --run-id <id> [--disable-thinking] [--max-vision-budget]

# Multi-model orchestrators
./code/run_multirun.sh <pass>         # 11 base Q4 models
./code/run_nothink.sh                 # Q3.5-9B + Q3.6-27B with --disable-thinking
./code/run_q8.sh                      # 7 models × 3 passes at Q8, both modes for Qwen thinkers

# Aggregation
python3 code/score_v5_multirun.py --prepare-median    # generate judge inputs
# dispatch LLM-judge subagents (one per image, judges all 15 variants)
python3 code/score_v5_multirun.py                      # re-aggregate after judges complete
```
