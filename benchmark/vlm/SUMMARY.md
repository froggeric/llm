# VLM Benchmark — Quick Reference

**Authoritative report**: `BENCHMARK-REPORT-v5.md` — single consolidated report covering all 24 tested variants (Q4 + Q8 × think + nothink) across 11 base models.

## Headline

- **Champion**: **Qwen3.6-27B-nothink @ Q4_K_M** (79.6/100, σ 0.24, **0 failures**, ~70s/img, 16.9GB)
- **Best small model**: **Qwen3.5-4B-nothink @ Q4_K_M** (75.5/100, σ 0.48, **0 failures**, ~20s/img, 3.2GB) — best quality-per-GB (23.6 pts/GB)
- **Best mid-tier (revised)**: **Qwen3-VL-8B @ Q8_0** (74.4/100, σ 0.33, **0 failures across 90 cells**, 26s/img, 8.1GB) — strict win over its Q4 version, the only Q8 with zero timeouts
- **Gemma 4 12B cautionary tale**: Raw +12.5 at Q8 looks great but **22% timeout rate** makes effective score ~59.7 (worse than Q4 with 0 failures). Exclude from production at both quants.

## Hardware picks (Apple Silicon unified memory — GPU gets ~70-80% of total RAM)

| VRAM tier | Pick |
|---|---|
| **4-8 GB** | Q3.5-4B-nothink @ Q4 |
| **12-16 GB** | Q3VL-8B @ Q8 ★ (only perfect Q8) |
| **24+ GB** | Q3.6-27B-nothink @ Q4 |

Three tiers, three picks. Beyond 24GB there is no model in the study that justifies the larger footprint — Q3.6-27B-nothink remains the answer at 32/48/64+ GB.

**Specialty swap**: GLM-9B @ Q4 (7.4GB) for **Chinese-signage-heavy workloads only**. Head-to-head with Q3VL-8B-Q8 it's statistically tied on quality but slower (37s vs 26s), less stable (σ 0.53 vs 0.33), and wins on fewer image types. Its only genuine edge: +3 to +4 on the 3 images with Chinese characters (banner, motion-blur, Hawaii stations). For ~5% of MCP workloads; everyone else should use the tier default.

**MoE reality check**: Q3.6-35B-A3B (21.9GB, "35B" total / 3B active per token) **ties Q3VL-8B-Q8 and Q3.5-4B-nothink on quality** despite being 2.7-7× larger. The big parameter count buys knowledge breadth, not perception depth.

(See `BENCHMARK-REPORT-v5.md` § Hardware Recommendations and § Qwen3.6-35B-A3B for full reasoning.)

## Always exclude from deployment

- **G4-12B @ both quants**: Q4 hallucination flips; Q8 22% failure rate
- **G4-E4B** (any quant): wrong counts, perception failures
- **Q3VL-4B** (any quant): degeneration loops on dense scenes
- **Q3.5-9B @ Q8**: 28-40% timeout rate, unusable
- **G4-26B**: outclassed by Q3.6-27B-nothink at the same footprint
- **Q3.6-35B-A3B**: MoE with only 3B active per token — ties much smaller dense models on vision despite 21.9GB footprint
- **G4-31B**: slow (93s/img) AND not actually better than Q3.6-27B on photos (loses by 0.71 mean on photo/art images)

## Use-case picks (the right model for the right job)

| Use case | Pick |
|---|---|
| Code screenshots (most common MCP use) | Q3.6-27B-nothink @ Q4 |
| Error messages / stack traces | Q3.6-27B-nothink @ Q4 |
| UI screenshots | Q3.6-27B-nothink @ Q4 |
| Chinese signage OCR (storefronts, banners with CJK) | GLM-9B @ Q4 (specialty swap — only genuine edge) |
| Other multilingual (French, German, Japanese) | Q3.6-27B-nothink @ Q4 |
| Photos / general scenes | Q3.6-27B-nothink @ Q4 (beats G4-31B on photos by +0.71 mean) |
| Dense OCR (catalogs, schedules) | Q3.6-27B-nothink @ Q4 |
| Medical images | **None — all 24 variants missed the rib fracture** |

Only **GLM-9B @ Q4** is worth swapping in — and only for Chinese signage OCR. On French, German, and general OCR, Q3.6-27B-nothink matches or beats it.

## Universal configuration

- **All Qwen hybrid thinkers** (Qwen3.5/3.6): `chat_template_kwargs.enable_thinking=false` for vision tasks
- **Gemma 4 vision**: `--image-min-tokens 560 --image-max-tokens 2240` (default 280 is "essentially blind")
- **max_tokens=16384**, batch sizes `-b 4096 -ub 4096`
- **300s call timeout + 360s watchdog** (fail fast on thinking runaways)
- **WebP images**: harness converts to PNG via `sips` (llama-server silently drops webp)

## Q4 vs Q8 guidance (in brief)

- **Default for new models**: Q4_K_M (Q4 is the canonical scorecard)
- **Q8 worth testing for**: non-thinking architectures under 13B parameters where Q4 score is below ~75
- **Skip Q8 for**: any hybrid thinker, any model already above 75 at Q4, anything over 13B
- **Always report effective score** (raw × completion rate) when comparing quants — raw scores hide reliability problems

## Files

| file | purpose |
|---|---|
| `BENCHMARK-REPORT-v5.md` | **Authoritative consolidated report** (24 variants, all categories) |
| `FINAL-ANALYSIS.md` | Per-model qualitative analysis (input to v5) |
| `benchmark-results/FINAL-RESULTS-v5.md` | Q4 aggregated tables |
| `benchmark-results/FINAL-RESULTS-q8.md` | Q8 aggregated tables |
| `score_v5.py` | deterministic probes for all 30 images |
| `score_v5_multirun.py` | Q4 multi-run aggregation |
| `score_q8_multirun.py` | Q8 aggregation |
| `benchmark_llamaserver.py` | test harness (webp-aware, configurable timeouts) |
| `test-images/GROUND-TRUTH.md` | owner-verified GT for all 30 images |
| `benchmark-results/raw.jsonl` | all raw responses (Q4 + Q8) |
| `benchmark-results/judgments_v5/` | 30 Q4 LLM-judge outputs |
| `benchmark-results/judgments_q8/` | 30 Q8 LLM-judge outputs |
