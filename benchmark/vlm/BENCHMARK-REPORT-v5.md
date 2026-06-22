# Local VLM Benchmark — Final Consolidated Report

**Date**: 2026-06-21
**Engine**: llama.cpp b9690 via llama-server on Apple Silicon (unified memory)
**Scope**: 11 base models × {Q4_K_M, Q8_0 where tested} × {think, nothink where applicable} × 30 images × 3 runs = **24 tested variants**, ~2,000 cells collected and judged
**Scoring**: v5 hybrid — 40% deterministic probes + 60% LLM-judge on median run, image-weighted by judge spread, failure modes cap final cell score

This report consolidates Q4 and Q8 results, think and nothink variants, into a single ranking. There are no split sub-reports — every variant lives in one table and is judged on the same image set with the same methodology.

---

## Executive summary

- **Champion**: **Qwen3.6-27B with `enable_thinking=false` at Q4_K_M** — 79.6/100, σ 0.24, **zero failures** across 90 cells, ~70s/img. The right default for any Mac with 24GB+ of usable VRAM. The only model with the trifecta: highest score AND lowest σ AND 0 failures.
- **Best small model**: **Qwen3.5-4B with `enable_thinking=false` at Q4_K_M** — 75.5/100 at 3.2GB and ~20s/img gives the best quality-per-GB in the study (23.6 pts/GB). The realistic choice for resource-constrained hardware.
- **Best mid-tier (revised)**: **Qwen3-VL-8B @ Q8_0** — 74.4/100, σ 0.33, **0 failures across 90 cells**, 26s/img, 8.1GB. The only model that's 100% reliable at Q8. Strict win over its Q4 version. The right default for 12-16GB Macs.
- **Q8 is asymmetric and unreliable as a blanket upgrade**: it can rescue Gemma 4 (G4-12B σ drops 0.66 → 0.28) but introduces failure-rate issues (G4-12B-Q8 still times out on 22% of cells) and cripples Qwen hybrid thinkers. Test Q8 selectively, never as default.
- **Universal failure**: every variant missed the rib fracture on image 28 (X-ray) — these models are not clinical-grade.
- **Reliability matters more than peak score** for an MCP: users can tolerate a slightly-less-capable model that always returns something over a more-capable model that returns nothing 22% of the time.

---

## Master ranking — all 24 variants tested

Sorted by **effective score** (raw score × completion rate). This penalizes models with high timeout/failure rates — a model that returns nothing on 22% of images is not actually a 76.6-score model in production.

| # | Variant | Quant | Mode | Raw score | Effective* | σ | Latency | Size | Reliability | Tier |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | Qwen3.6-27B | Q4 | nothink | **79.6** | **79.6** | 0.24 | ~70s | 16.9GB | 90/90 ★ | **Champion** |
| 2 | Qwen3.6-35B-A3B | Q4 | nothink | 76.4 | 76.4 | 0.55 | 30s | 21.9GB | 90/90 | Top-tier (large) |
| 3 | Qwen3.5-4B | Q4 | nothink | 75.5 | **75.5** | 0.48 | ~20s | 3.2GB | 90/90 ★ | **Best small** |
| 4 | GLM-4.6V-Flash-9B | Q4 | think | 75.1 | **75.1** | 0.53 | 37s | 7.4GB | 90/90 | Strong mid |
| 5 | Qwen3.6-35B-A3B | Q4 | think | 75.0 | 75.0 | 0.31 | ~43s | 21.9GB | 90/90 | Top-tier (stable) |
| 6 | Gemma 4 31B | Q4 | think | 74.6 | 74.6 | 0.45 | 93s | 18.1GB | 90/90 | Skip — slow, not better than Q3.6-27B on photos |
| 7 | Qwen3-VL-8B | Q8 | think | 74.4 | **74.4** | 0.33 | 26s | 8.1GB | 90/90 ★★ | **Best mid-tier** |
| 8 | Qwen3-VL-8B | Q4 | think | 73.1 | 73.1 | 0.52 | 19s | 5.8GB | 90/90 | Sweet spot |
| 9 | Qwen3.5-9B | Q4 | nothink | 73.1 | 73.1 | 0.58 | ~27s | 6.2GB | 90/90 | Solid mid |
| 10 | Gemma 4 26B-A4B | Q4 | think | 72.7 | 72.7 | 0.51 | 30s | 17.1GB | 90/90 | Outclassed |
| 11 | Qwen3.5-9B | Q4 | think | 72.7 | 72.7 | 0.52 | ~60s | 6.2GB | 90/90 | Solid mid |
| 12 | Qwen3.6-27B | Q4 | think | 78.2 | 70.4 | 0.26 | ~115s | 16.9GB | 81/90 | Champion (slower, less reliable) |
| 13 | GLM-4.6V-Flash-9B | Q8 | think | 73.4 | 68.5 | 0.51 | 43s | 10GB | 84/90 | Drop vs Q4 |
| 14 | Qwen3-VL-4B | Q4 | think | 65.9 | 65.9 | 0.76 | 14s | 3.1GB | 90/90 | Degeneration-prone |
| 15 | Gemma 4 12B | Q4 | think | 64.1 | 64.1 | 0.66 | 47s | 6.8GB | 90/90 | **Avoid** (hallucination flips) |
| 16 | Qwen3-VL-4B | Q8 | think | 65.3 | 61.0 | 1.03 | 17s | 4.0GB | 87/93 | Unstable |
| 17 | **Gemma 4 12B** | **Q8** | think | 76.6 | **59.7** ⚠ | 0.28 | 68s | 13GB | 74/95 | **Q8 looks good on paper, fails 22%** |
| 18 | Gemma 4 E4B | Q4 | think | 58.8 | 58.8 | 0.60 | 22s | 5.6GB | 90/90 | **Avoid** (perception fails) |
| 19 | Gemma 4 E4B | Q8 | think | 63.9 | 55.4 | 0.46 | 27s | 7.6GB | 78/90 | Q8 helps, still weak |
| 20 | Qwen3.5-4B | Q8 | think | 66.1 | ~50 | 0.51 | 26s | 4.2GB | partial | Drop vs Q4 |
| 21 | Qwen3.5-4B | Q8 | nothink | 65.7 | ~50 | 0.51 | 26s | 4.2GB | partial | Drop vs Q4 |
| 22 | Qwen3.5-9B | Q8 | nothink | partial | ~45 | — | — | 8.9GB | ~85% fail | **Unusable** |
| 23 | Qwen3.5-9B | Q8 | think | partial | ~35 | — | — | 8.9GB | ~60% fail | **Unusable** |

*\*Effective score = raw_score × (successful_cells / total_attempted). Approximates the score a user would experience accounting for "model returned nothing" failures.*

**Tier markers**: ★ = zero failures AND σ ≤ 0.5. ★★ = zero failures across both Q4 and Q8.

**Reading the table**: Gemma 4 12B-Q8 ranks #17 by effective score despite raw score of 76.6, because 22% of cells timed out. For an MCP, the effective score is what matters.

⚠ = caution — high raw score masks reliability problem.

---

## Per-model analysis

All variants of each model are combined under one heading. **Ranked by best-variant effective score, best model first.** Recommendations are based on actual outputs reviewed in `raw.jsonl`, not just score deltas.

### 1. Qwen3.6-27B (Q4: 16.9GB / Q8: not tested) — hybrid thinker

**Verdict**: **The benchmark champion.** Best quality, best stability, no failures with thinking disabled.

- **Q4 nothink** (score 79.6, σ 0.24, ~70s/img, 0/90 failures): **#1 overall.** Zero failures, lowest σ in the study. Captured the rare "125scratch" credit on spritesheet (22) where G4-12B hallucinated. Got the "AL" marker on the MRI (25). Best on dense scenes — its directness avoids the runaway-reasoning trap.
- **Q4 think** (score 78.2, σ 0.26, ~115s/img, 81/90 successful): Same quality as nothink but 1.6× slower and produces 9 truncated/empty cells when dense images trigger thinking runaways. No reason to use this over nothink for vision tasks.

**Recommended**: **Q4_K_M with `enable_thinking=false`.** This is the default for any Mac with 24GB+ of usable VRAM (model + KV cache + overhead ≈ 22GB).

---

### 2. Qwen3.6-35B-A3B (Q4: 21.9GB / Q8: not tested) — hybrid thinker

**Verdict**: Sparse MoE that runs fast despite its size — but the "35B" parameter count is misleading. **On vision tasks it ties much smaller dense models despite being 2.7-7× larger.**

**MoE reality check (per-image judge scores, mean across 30 images)**:
| Variant | Mean | σ | Latency | Size |
|---|---|---|---|---|
| Q3.6-27B-nothink Q4 | **7.62** | 1.96 | ~70s | 16.9GB |
| Q3VL-8B Q8 | 7.43 | 1.92 | 26s | 8.1GB |
| **Q3.6-35B-A3B nothink Q4** | **7.40** | **2.17** | 30s | 21.9GB |
| Q3.5-4B nothink Q4 | 7.38 | 1.96 | ~20s | 3.2GB |

Q3.6-35B-A3B is **statistically tied** with Q3VL-8B-Q8 and Q3.5-4B-nothink (within 0.05 points), despite being 2.7-7× larger. It also has the **highest variance** of the four (σ 2.17) — wins big on banner (+5 vs Q3.6-27B) but crashes on error_trace (4/10, hallucinated "psycopg" for "psycopg2"), animation (3/10, missed the run cycle), and MRI (4/10).

**Why**: A3B = "3B Active" — only 3B parameters are active per inference token. For vision tasks where the bottleneck is perception (not knowledge retrieval), the active-parameter count dominates, and 3B is the same league as Q3.5-4B and Q3VL-8B. The 35B total parameter count buys breadth of knowledge, not depth of perception.

- **Q4 nothink** (effective 76.4, σ 0.55, 30s/img, 0/90 failures): Sparse MoE runs faster than Q3.6-27B (30 vs 70s) but quality matches Q3VL-8B-Q8 (74.4) and Q3.5-4B-nothink (75.5) — not Q3.6-27B (79.6).
- **Q4 think** (effective 75.0, σ 0.31, ~43s/img, 0/178 failures): Slightly worse than nothink, slightly more stable.

**Recommended**: **Skip.** Q3.6-27B-nothink is 0.22 points better at 5GB smaller. Q3VL-8B-Q8 and Q3.5-4B-nothink tie it at a fraction of the size. If you need speed at 24+ GB, Q3.6-27B-nothink at ~70s is the right answer; if you need real speed, drop to a smaller tier.

---

### 3. Qwen3.5-4B (Q4: 3.2GB / Q8: 4.2GB) — hybrid thinker

**Verdict**: Best quality-per-GB at Q4 with thinking disabled. Do not enable thinking. Do not use Q8.

- **Q4 nothink** (score 75.5, σ 0.48, ~20s/img): **23.6 points per GB — the best ratio in the entire study.** Beats G4-31B (74.6) at 1/6 the size and 4× the speed. Notable wins: image 11 collage (8.19, beat everything else), Hundertwasser (tied for top at 6.81).
- **Q4 think** (score 70.6, σ 0.77, ~29s/img): Worse than nothink (−4.9 points) and far less stable. On dense images (Waldo, Hundertwasser) thinking runaways exhaust the 16384-token budget and return empty.
- **Q8 nothink** (score 65.7, σ 0.51, partial sample): −9.8 from Q4 nothink. Partly sample skew from dense-image timeouts, but completed cells also score slightly lower.
- **Q8 think** (score 66.1, σ 0.51, partial sample): Worse than Q4 think.

**Recommended**: **Q4_K_M with `enable_thinking=false`.** This is the right model for any 4-8GB-VRAM Mac.

---

### 4. GLM-4.6V-Flash-9B (Q4: 7.4GB / Q8: 10GB)

**Verdict**: Not actually a general OCR specialist — its only genuine edge is **Chinese signage OCR**. Recommended models handle everything else as well or better.

**OCR/multilingual reality check (per-image judge scores)**:

| Image set | GLM-9B Q4 | Q3.6-27B-nothink | Q3VL-8B-Q8 | Q3.5-4B-nothink |
|---|---|---|---|---|
| OCR-heavy (10 images) | 7.90 | **8.50** | 8.25 | 7.70 |
| Multilingual (5 images) | **7.80** | 7.40 | 6.30 | 7.60 |

GLM-9B ranks **3rd of 4** on general OCR — Q3.6-27B-nothink beats it by 0.6 points. The "foreign-text OCR specialist" reputation is too broad.

**Where GLM-9B genuinely wins** (beats all 3 recommended models): only **3/30 images**:
- Image 20 banner (Chinese 少林寺 + German ÖSTERREICH): GLM=9, next best=7 (+2)
- Image 21 motion blur (Chinese 大新銀行): GLM=9, next best=7 (+2)
- Image 10 rice porridge: GLM=9, next best=8 (+1) — but this is a photo, not OCR

The narrow specialty is **Chinese signage OCR** — when an image contains Chinese characters in a sign/storefront context. For French, German-only, Japanese, or general OCR workloads, Q3.6-27B-nothink handles them as well or better.

- **Q4 think** (score 75.1, σ 0.53, 37s/img): The only model to correctly read 大新銀行 (Dah Sing Bank) on the motion-blur image. Otherwise mid-pack on OCR. 0/90 failure record.
- **Q8 think** (score 73.4 effective 68.5, σ 0.51, 43s/img): Slightly worse than Q4 (−1.7). Not a quantization win. Stick with Q4.

**Recommended**: **Q4_K_M, but only as a specialty swap-in for workflows dominated by Chinese signage.** Most multilingual and OCR workloads are handled better by Q3.6-27B-nothink (the default at 24+GB).

---

### 5. Gemma 4 31B (Q4: 18.1GB / Q8: not tested)

**Verdict**: Slow, and **not actually better than Q3.6-27B-nothink on photos** despite the reputation. Skip for MCP.

**Photo/perception reality check (per-image judge scores)**:

| Image category | Q3.6-27B-nothink mean | G4-31B mean | Δ |
|---|---|---|---|
| **Photo/Art** (7 images: massage, rice porridge, collage, manga, watercolor, motion blur, Hundertwasser) | **7.43** | 6.71 | **+0.71 Q3.6-27B wins** |
| Non-photo (23 images) | 7.67 | 7.70 | −0.02 (tied) |
| All 30 | 7.62 | 7.47 | +0.15 Q3.6-27B |

The "best on photos and manga" reputation doesn't hold up. On the 7 photo/art images, Q3.6-27B-nothink beats G4-31B by an average of 0.71 points — Q3.6-27B won 5/7 photo images outright (massage therapists, collage, watercolor, motion blur +3, Hundertwasser). G4-31B's only photo win was rice porridge (+2).

Where G4-31B does win: **OCR-heavy dense compositions** (spritesheet +3, banner +3, QR code +3, Nausicaa color swatch +3, class schedule +2, album cover +2). These are the cases where its careful, slow perception pays off. But Q3.6-27B-nothink handles them fine in practice and is 1.4× faster.

- **Q4 think** (score 74.6, σ 0.45, 93s/img): Slowest non-hybrid model. σ is low because it's methodical, not because it's accurate.

**Recommended**: **Skip.** Same 18GB footprint as Q3.6-27B-nothink (16.9GB) but worse on photos, tied on non-photos, and 1.4× slower. There is no scenario where G4-31B is the right pick.

---

### 6. Qwen3-VL-8B-Instruct (Q4: 5.8GB / Q8: 8.1GB)

**Verdict**: Quietly excellent mid-tier. The only model with 100% reliability at Q8. Underappreciated speed/quality point.

- **Q4 think** (score 73.1, σ 0.52, 19s/img): Standout on animation (image 23, 8.83 — only model to recognize the 6 frames as a single RUN CYCLE rather than 6 distinct actions). Strong on watercolour (19) and banner (20). Drops on dense scenes — invented a wall clock on Waldo.
- **Q8 think** (score 74.4, σ 0.33, 26s/img, **0/90 failures**): The only Q8 variant with zero timeouts across all 90 cells. +1.3 points over Q4. Stability improves notably (σ 0.52 → 0.33). Size cost is 2.3GB.

**Recommended**: **Q8_0 if you have 12GB+ VRAM; Q4_K_M on tighter budgets.** The Q8 version is the model to point at when arguing "Q8 can be a strict win."

---

### 7. Qwen3.5-9B (Q4: 6.2GB / Q8: 8.9GB) — hybrid thinker

**Verdict**: Solid mid-tier Q4, broken at Q8. Skip Q8 entirely.

- **Q4 nothink** (score 73.1, σ 0.58, ~27s/img): Tied with Q3VL-8B on score, slightly slower, slightly less stable. Beat both Q3VL-8B and GLM-9B on the ONErpm catalog (10.0 deterministic). A reasonable choice.
- **Q4 think** (score 72.7, σ 0.52, ~60s/img): Essentially tied with nothink but 2× slower. On image 03 (error trace) it scored 2.00/10.0/10.0 across three runs — a thinking-induced flip. No advantage over nothink.
- **Q8 think + nothink**: ~28-40% timeout rates in both modes. Unusable.

**Recommended**: **Q4_K_M with `enable_thinking=false`.** Same conclusion as Q3.5-4B.

---

### 8. Gemma 4 26B-A4B (Q4: 17.1GB / Q8: not tested)

**Verdict**: Competent generalist that doesn't justify its size. Outclassed by Qwen3.6-27B-nothink on the same footprint.

- **Q4 think** (score 72.7, σ 0.51, 30s/img): Sparse MoE so it runs much faster than its parameter count suggests. Solid across photos, art, and OCR. One notable failure: hallucinated a pier on the Where's Waldo image (30) which the judge caught.

**Recommended**: **Skip.** Q3.6-27B-nothink is 7 points better at the same size and more stable. No reason to pick this model.

---

### 9. Qwen3-VL-4B-Instruct (Q4: 3.1GB / Q8: 4.0GB)

**Verdict**: Fastest model in the lineup, but degeneration-prone. OK for trivial OCR, dangerous for anything else.

- **Q4 think** (score 65.9, σ 0.76, 14s/img): Fast and unstable. σ 0.76 is the second-worst in the study. On Waldo (30) it falls into a literal repetition loop — *"a man with a hat, a man with a backpack, a man with a camera"* — for thousands of tokens until budget exhausts. On spritesheet (22) it scored 2.00. When it works it's fine; when it fails it fails catastrophically.
- **Q8 think** (score 65.3, σ 1.03): No improvement. σ actually worsens to 1.03 — worst stability in the study. Q8 doesn't fix architectural degeneration.

**Recommended**: **Exclude.** Q3.5-4B-nothink (3.2GB, 75.5) is the same size and 10 points better.

---

### 10. Gemma 4 12B (Q4: 6.8GB / Q8: 13GB)

**Verdict**: **Exclude from production.** Q4 has hallucination flips (showstopper); Q8 looks improved on paper but has a 22% timeout rate (also a showstopper for an MCP).

- **Q4 think** (raw 64.1, σ 0.66): The poster child for "hallucination flips." On image 14 (ONErpm catalog) it scored 8.4 / 0.0 / 0.0 across three runs of the *same image*. In failed runs it invented an "Erno" platform and a "D2Sereno" credit. On image 27 it hallucinated "Atomic acid" for "Domoic acid." **Unusable at Q4 — showstopper.**
- **Q8 think** (raw 76.6, σ 0.28, **effective 59.7**): The raw score looks like a +12.5 point improvement, but 21/95 cells (22%) timed out at the 300s cap. The σ improvement is real (less variance among completed cells), but for an MCP the failure rate is a dealbreaker — 1 in 5 images returns nothing. Effective score 59.7 is *worse than Q4* (which has 0 failures).

**Recommended**: **Exclude from deployment at either quant.** If you have 13GB of VRAM, pick Q3VL-8B-Q8 (74.4, 0 failures) instead.

---

### 11. Gemma 4 E4B (Q4: 5.6GB / Q8: 7.6GB)

**Verdict**: The "Effective 4B" doesn't have enough capacity for vision. Bottom of the pile at either quant.

- **Q4 think** (score 58.8, σ 0.60): Worst-in-class perception failures. Claimed 6 massage therapists instead of 5 (image 07). Hallucinated a second face on Hundertwasser (29). Missed the heart+hands+laurel combination on the Vic Health Club logo (17). Scored 3.80 on Waldo — below Q3VL-4B.
- **Q8 think** (score 63.9 effective 55.4, σ 0.46): +5.1 raw points from Q8 — the perception failures recede but don't vanish. Still bottom tier. At 7.6GB it's the same footprint bracket as Q3VL-8B and Q3.5-9B, both of which crush it.

**Recommended**: **Exclude from deployment.** Pick Q3.5-4B-nothink (3.2GB, 75.5) or Q3VL-8B (5.8GB, 73.1) instead.

---

## Hardware recommendations

**Apple Silicon note**: unified memory means GPU gets ~70-80% of total RAM after OS overhead. "VRAM tier" below means *usable* GPU memory, not total system RAM. A "32GB Mac" typically gives the GPU ~22-25GB.

| VRAM tier | Recommendation | Why |
|---|---|---|
| **4-8 GB** | **Q3.5-4B-nothink @ Q4_K_M** (75.5/100, 3.2GB, ~20s/img, σ 0.48, 0 failures) | Best quality-per-GB by a wide margin (23.6 pts/GB). Configure `enable_thinking=false`. Interactive-tier latency. At 8GB you have headroom for context but no model upgrade is justified — Q3VL-8B (5.8GB) eats most of the budget and leaves no room. |
| **12-16 GB** | **Qwen3-VL-8B @ Q8_0** (74.4/100, 8.1GB, 26s/img, σ 0.33, **0 timeouts across 90 cells**) | The only 100%-reliable Q8 model in the study. Strict win over its Q4 version. Fits comfortably in 12GB and leaves room in 16GB for context. |
| **24+ GB** | **Q3.6-27B-nothink @ Q4_K_M** (79.6/100, 16.9GB, ~70s/img, σ 0.24, 0 failures) | Champion. Comfortable fit at 24GB with headroom for KV cache. At 32GB+ you have room for concurrent sessions or larger context. **No model in the study justifies the larger footprint** (see MoE reality check below). |

**Specialty swap (any tier that fits it)**: **GLM-9B @ Q4_K_M** (75.1/100, 7.4GB) is a niche alternative **only for workloads dominated by Chinese signage OCR** (storefronts, banners with CJK characters). Head-to-head vs Q3VL-8B-Q8 they are statistically tied on quality (per-image means within 0.03), but Q3VL-8B-Q8 is 30% faster, more stable (σ 0.33 vs 0.53), and wins on more diverse image categories (album art, QR codes, animation, watercolor, Hundertwasser). GLM-9B wins by +3 to +4 only on the 3 images containing Chinese signage (banner, motion-blur bank sign, Hawaii station names). For the ~5% of MCP workloads where Chinese signage dominates, GLM-9B is the better pick; for everything else, Q3VL-8B-Q8 is the right answer at 12-16GB.

**Why no speed-alternative at 24+ GB**: Q3.6-35B-A3B looks attractive on paper (sparse MoE, 30s/img vs Q3.6-27B's ~70s/img) but the data shows it ties Q3VL-8B-Q8 (26s/img, 8.1GB) and Q3.5-4B-nothink (20s/img, 3.2GB) on quality. The "35B" parameter count is misleading — only 3B are active per token (A3B = "3B Active"), so on vision tasks it performs like a 3B model despite the 21.9GB footprint. If speed matters at the 24+ GB tier, the right answer is to step down to a smaller model that genuinely is faster (Q3VL-8B-Q8 at 26s, or Q3.5-4B at 20s), not to "compromise" with Q3.6-35B-A3B which is bigger for no perceptual benefit.

**Excluded from every tier**:
- **G4-12B @ both quants**: Q4 has hallucination flips (showstopper); Q8 has 22% failure rate (also showstopper for MCP use).
- **G4-E4B** (any quant): wrong counts, perception failures.
- **Q3VL-4B** (any quant): degeneration loops on dense scenes.
- **Q3.5-9B @ Q8** (either mode): 28-40% timeout rate, unusable.
- **G4-26B**: outclassed by Q3.6-27B-nothink at the same footprint (7 points worse, no reason to pick).
- **Q3.6-35B-A3B**: bigger, slower-to-load, and worse than Q3.6-27B-nothink at no advantage. Skip.
- **G4-31B**: too slow for interactive use (93s/img); no quality advantage over Q3.6-27B-nothink. Skip for MCP, keep in mind for batch photo processing only.

## Use-case-specific recommendations

For a vision-MCP, the most common workloads are screenshots (code, UI, errors) and photos. The recommended hardware-tier model handles every workload competently — only **one specialty swap** is justified across the entire workload spectrum:

| Use case | Best pick | Why | Avoid |
|---|---|---|---|
| **Code screenshots** (most common MCP use) | Q3.6-27B-nothink @ Q4 | Best OCR (8.5 mean on OCR-heavy images) + 0 failures | G4-12B-Q4 (would *invent code* via hallucination flips) |
| **Error messages / stack traces** | Q3.6-27B-nothink @ Q4 | Strongest precise technical OCR — beat GLM-9B on image 03 (10.0 vs 10.0 tie) and on French cassette 13b (10.0 vs 7.0 GLM) | G4-12B-Q4 (hallucinations on technical content) |
| **UI screenshots** | Q3.6-27B-nothink @ Q4 | Component ID + layout + always returns | G4-E4B (perception failures on simple UIs) |
| **Chinese signage OCR** (storefronts, banners with CJK) | **GLM-9B @ Q4** (specialty swap) | Only model that read 大新銀行 (motion blur) and 少林寺+ÖSTERREICH (banner) cleanly — wins +2 over best alternative on each | Qwen3.5/3.6 variants (hallucinated HSBC, DBS, CITIC for Chinese bank names) |
| **Other multilingual** (French, German, Japanese, Hawaiian) | Q3.6-27B-nothink @ Q4 | Beat GLM-9B on French cassette (10 vs 7), tied on Hawaii station names (9 vs 9) | — |
| **Photos and general scenes** | Q3.6-27B-nothink @ Q4 | Best perception overall — wins 5/7 photo/art images vs G4-31B by +0.71 mean | G4-31B (slower, worse on photos); G4-E4B (wrong counts) |
| **Architecture diagrams** | Q3.6-27B-nothink @ Q4 | Component + flow identification | — |
| **Scientific graphs / charts** | Q3.6-27B-nothink @ Q4 | Correct on dual-axis charts and rare station names (Mauna Loa, ALOHA) | G4-12B (hallucinated "Atomic acid" for "Domoic acid") |
| **Dense OCR scenes** (catalogs, spritesheets, schedules) | Q3.6-27B-nothink @ Q4 | Captures rare credits (125scratch), all thumbnail-only titles | G4-12B-Q4 (hallucination flips on titles); Q3VL-4B (degeneration loops) |
| **Art / illustrations** | Q3.6-27B-nothink @ Q4 | Best perception of style + composition (Hundertwasser, watercolor) | Q3.5-4B-think (runaway thinking on art → empty) |
| **Medical images** | **None of these models.** | All 24 variants missed the rib fracture on image 28 | All — do not deploy for clinical use |

**Bottom line**: For a general-purpose vision MCP serving a coding LLM, **Q3.6-27B-nothink @ Q4** is the right answer at 24GB+, **Q3VL-8B-Q8** at 12-16GB, and **Q3.5-4B-nothink @ Q4** at 4-8GB. **GLM-9B @ Q4** is the only specialty swap worth considering — and only when your workload is dominated by Chinese signage OCR (it loses to Q3.6-27B on French, German, and general OCR).

---

## Strategic Q4 vs Q8 guidance

**Q8 is not a uniform upgrade — and even when raw scores improve, reliability may not.** The data shows Q8 reduces variance (lower σ) for non-thinking architectures but does not necessarily improve the *production experience*.

**The G4-12B cautionary tale.** Raw scores say G4-12B jumps +12.5 points at Q8 (64.1 → 76.6) — the largest single-quantization effect in the study. But this is misleading: the σ improvement is real (0.66 → 0.28, less variance among completed cells), yet the model still fails on 22% of cells at the 300s timeout. Effective score (accounting for failures) is 59.7 — *worse than Q4*. For an MCP, this means 1 in 5 images returns nothing, which is unacceptable. The lesson: **always check the reliability column, not just the score column.**

**Qwen hybrid thinkers move the opposite direction.** Q3.5-4B-think drops 4.5 raw points at Q8, Q3.5-4B-nothink drops 9.8 (partly sample skew from timeouts), and Q3.5-9B-Q8 becomes unusable with ~28-40% timeout rates. Hypothesis: Q8 gives the thinking phase more "room to reason" about dense images → longer thinking traces → more budget exhaustion on the 300s timeout. Even when cells complete, Q8 doesn't help — the architectural bottleneck is reasoning, not precision. For these models, stay at Q4 and disable thinking.

**The one clean Q8 win**: **Qwen3-VL-8B @ Q8_0** — 74.4/100, σ 0.33, **0 timeouts across 90 cells**, +1.3 pts over Q4 and lower σ. The model to point at when arguing "Q8 can be a strict win." This is the only Q8 variant that's a strict improvement on every dimension.

### Test standard going forward

- **Default quant for new models: Q4_K_M.** Every model gets a Q4_K_M datapoint first; Q4 results are the canonical scorecard.
- **Supplement with Q8 only when ALL of**:
  - The architecture is non-thinking (Gemma, GLM, Qwen3-VL — not Qwen3.5/3.6 hybrid thinkers)
  - The parameter count is under ~13B (size cost stays acceptable)
  - The Q4 score is below ~75 (marginal gain justifies the 1.5-1.7× size)
- **Always disclose the timeout budget and report effective score (raw × completion rate).** A 300s cap materially shifts which cells complete and can make a model look better than it is.
- **Skip Q8 for**: any hybrid thinker, any model already above 75 at Q4, any model over 13B where size cost is punishing.

---

## Methodology

### Scoring (v5)
Each (variant, image) cell gets:
- **Failure mode detection**: empty / truncated / repetition_loop / normal (caps the score)
- **Deterministic probes**: image-specific OCR, counts, hallucination checks (see `score_v5.py`)
- **LLM-judge on median run**: holistic 0-10 with key_hits / key_misses / hallucinations
- **Final cell**: `0.4 × det_mean + 0.6 × judge_score`, failure-mode capped
- **Image weights**: based on judge-score spread (signal-based)
- **Per-variant aggregate**: weighted sum / max weight × 100

### v5 fixes from v4
- Think and nothink variants properly separated (v4 conflated them as a single cell)
- WebP images converted to PNG via `sips` before encoding (llama-server silently dropped webp)
- 300s call timeout + 360s watchdog (down from 1200s — fail fast on thinking runaways)
- Dense-scene GT exhaustively enumerated (Waldo horses, spritesheet items, etc.) — prevents LLM-judge false positives

### Configuration
- **Gemma 4 vision budget**: `--image-min-tokens 560 --image-max-tokens 2240` (default 280 is "essentially blind")
- **Qwen hybrid thinkers**: `chat_template_kwargs.enable_thinking=false` for vision tasks
- **max_tokens=16384** (handles thinking + visible output combined budget)
- **Batch sizes `-b 4096 -ub 4096`** (image tokens fit in a single physical batch)

### Files
| file | purpose |
|---|---|
| `BENCHMARK-REPORT-v5.md` | This report — the single authoritative consolidated benchmark |
| `SUMMARY.md` | Quick-reference card pointing here |
| `FINAL-ANALYSIS.md` | Qualitative per-model analysis (input to this report) |
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

**Note for future sessions**: there are no split reports. Every variant (Q4, Q8, think, nothink) lives in the master table above. When new models or quants are added, extend this single report — don't create per-quant or per-mode sub-reports.
