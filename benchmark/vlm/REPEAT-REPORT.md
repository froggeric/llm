# Multi-sampling report — re-querying a warm model: latency & correlation

**Date:** 2026-06-23 · 2 models (small + medium) × 3 images × 5 back-to-back identical calls = 30 calls · `run_id=repeat-*`.

## The premise

Instead of querying a model once per image, take advantage that it's already loaded and warmed to fire the **same query N times back-to-back in one session**, then correlate. Two questions:

1. **Latency** — are calls 2..N cheaper than call 1 once the model is loaded + warm?
2. **Quality** — does correlating N identical runs (majority / union) beat a single run?

**Scope (small + medium recommendations):** `Q3.5-4B-nothink` and `Q3VL-8B-Q8`.

**Image selection was driven by run-variance, not just struggle.** At our production temp (0.1) these models are near-deterministic on *extraction* (OCR/code: divergence ~0.00–0.01) but highly variable on *open-ended* UI description. So multi-sampling only has room to lift quality where outputs vary — UI screenshots. `extract-code-test-1` is the low-variance **contrast**.

| Image | Why chosen | Q3.5-4B div | Q3VL-8B-Q8 div |
|---|---|---:|---:|
| `ui-test-1.png` | high-variance struggle | 0.32 | 0.02 |
| `ui-test-2.png` | highest-variance struggle | 0.45 | 0.13 |
| `extract-code-test-1.png` | low-variance contrast | 0.01 | 0.01 |

> 3×/4×/5× in the quality tables are derived from **nested prefixes** of the 5-call session (first 3, first 4, all 5) — same warm state, so the comparison is clean.

## Headline findings

1. **Warm calls are ~1.1–1.6× faster than call 1** — a one-time ~3–8s warmup per image. tok/s is flat across calls, so the saving is entirely in **prompt/image prefill** (the server reuses the warmed slot for an identical re-query). Net: sampling the same image 5× costs only **~70–75% of 5× the single-call time** — repeats are cheap.
2. **Correlation helps coverage tasks, not extraction.** On UI screenshots, taking the **union** of N descriptions recovers +1–4pts of label recall for free; on code, systematic errors are identical every run and correlation changes nothing.
3. **Union is the right aggregator for coverage — majority vote is wrong.** Majority (≥3/5) *drops* the real labels that only 1–2 runs mention; union keeps them.

## 1. Latency — the "already loaded" speedup

Per (model × image), wall-time of call 1 vs the median of warm calls 2–5:

| Model | Image | call 1 | warm med | speedup | tok/s call1 | tok/s warm |
|---|---|---:|---:|---:|---:|---:|
| Q3VL-8B-Q8 | extract-code-test-1 | 21.7s | 13.9s | **1.56×** | 38.2 | 38.4 |
| Q3VL-8B-Q8 | ui-test-1 | 18.0s | 14.2s | 1.27× | 39.9 | 39.3 |
| Q3VL-8B-Q8 | ui-test-2 | 24.2s | 20.8s | 1.17× | 39.3 | 39.4 |
| Q3.5-4B | extract-code-test-1 | 13.2s | 9.0s | **1.47×** | 65.9 | 65.1 |
| Q3.5-4B | ui-test-1 | 13.5s | 12.0s | 1.13× | 65.0 | 64.6 |
| Q3.5-4B | ui-test-2 | 18.1s | 19.2s | 0.94× | 64.3 | 65.0 |

**Reading:**
- The **first image of the session** shows the biggest warmup (1.56× / 1.47×): it pays both the per-image encode **and** the one-time Metal kernel compile. The 2nd/3rd images show smaller warmup (1.1–1.3×) — kernels already compiled, only the per-image prefill is re-paid on its first call.
- **tok/s is flat call-to-call** (38 vs 38, 65 vs 65) → generation speed is unchanged; the warm-call saving is in prefill, consistent with the server retaining the identical prompt+image KV across re-queries in the same slot.
- `Q3.5-4B × ui-test-2` (0.94×) is within noise: that cell's output length swings 1k→5.6k chars run to run, so output-length variance dominates the ~3s warmup signal.

**Cost of sampling 5× vs 1× (caching included):** for Q3VL-8B-Q8 on code, 5 calls = 21.7 + 4×13.9 = **77s**, vs 5×21.7 = 108s naïvely — **~29% saved**. For Q3.5-4B: 49s vs 66s, ~25% saved.

## Total time per image — 3 vs 5 repeats

How much wall time does sampling actually cost, and is 5 worth it over 3? Totals per image (repeat experiment, temp 0.1, 5 back-to-back calls in one warmed session):

| Model | Image | call 1 | mean warm | total 3 reps | total 5 reps | +3→5 | 5 over 3 |
|---|---|---:|---:|---:|---:|---:|---:|
| Q3VL-8B-Q8 | extract-code-test-1 | 21.7 | 13.9 | 49.3s | 77.1s | +27.8s | +56% |
| Q3VL-8B-Q8 | ui-test-1 | 18.0 | 14.1 | 46.2s | 74.5s | +28.3s | +61% |
| Q3VL-8B-Q8 | ui-test-2 | 24.2 | 21.0 | 65.7s | 108.1s | +42.4s | +65% |
| Q3.5-4B | extract-code-test-1 | 13.2 | 9.0 | 30.9s | 49.1s | +18.1s | +59% |
| Q3.5-4B | ui-test-1 | 13.5 | 11.8 | 38.0s | 60.9s | +22.9s | +60% |
| Q3.5-4B | ui-test-2 | 18.1 | 20.1 | 55.4s | 98.6s | +43.2s | +78% |

Per model (mean over images): **Q3VL-8B-Q8 53.7s → 86.6s (+61%)**, **Q3.5-4B 41.4s → 69.5s (+68%)**.

**Reading:**
- **5 repeats costs ~60% more wall time than 3.** Each extra repeat (4th, 5th) is roughly one warm call (~9–21s): warm calls are cheaper than the cold call 1 (see §1), but they are *not* free — you still pay a full inference per repeat.
- **But the quality has already plateaued at 3.** On the category sweep (temp 0.7, where multi-sampling matters), `union@3 ≈ union@5` almost everywhere — the extra two reps recover ~0 additional facts:

| category | 8B union@3→@5 | 4B union@3→@5 |
|---|---|---|
| read_image | 40→40 (+0) | 50→50 (+0) |
| extract_text | 74→74 (+0) | 74→74 (+0) |
| describe_ui | 97→97 (+0) | 97→100 (+3) |
| describe_chart | 92→92 (+0) | 92→92 (+0) |
| diagnose_error | 69→69 (+0) | 77→77 (+0) |

- **3 repeats is the efficient knee.** It captures essentially all the union benefit for ~60% less wall time than 5. Pushing to 5 only makes sense as maximum-coverage insurance on the hardest scenes — and even there, the marginal gain in this data is ~0. (The §1 "5× ≈ 70–75% of naïve 5×" saving is a *different* comparison — caching vs cold restarts — and doesn't change that reps 4–5 each still cost a full warm call.)

## 2. Quality — single vs majority vs union

`single` = mean per-run label recall (one random run). `maj@k` = per-label majority vote over first k runs (≥⌈k/2⌉). `union@k (pass@k)` = union over first k runs (≥1 run has it).

### UI screenshots — high variance → union helps

| Model | Image | single | maj@5 | union@5 | agree% |
|---|---|---:|---:|---:|---:|
| Q3VL-8B-Q8 | ui-test-1 | 93% | 92% | **97%** | 95% |
| Q3.5-4B | ui-test-1 | 97% | 97% | 97% | 100% |
| Q3VL-8B-Q8 | ui-test-2 | 99% | 100% | **100%** | 97% |
| Q3.5-4B | ui-test-2 | 96% | 94% | **100%** | 92% |

**Union beats single by +1 to +4 pts; majority does not** (for Q3.5-4B/ui-test-2, maj@5 is *below* single). The labels union recovers are exactly those a 3/5 majority discards — the ones only 1–2 runs mention:

| Model / image | label | hits per rep (1–5) | single? | maj@5? | union@5? |
|---|---|---|:-:|:-:|:-:|
| Q3VL-8B-Q8 / ui-test-1 | `INTEGRATIONS` | Y · · · · (1/5) | ✗ | ✗ | ✓ |
| Q3VL-8B-Q8 / ui-test-1 | `MODEL PROVIDERS` | Y · · · · (1/5) | ✗ | ✗ | ✓ |
| Q3.5-4B / ui-test-2 | `50% cost savings` | · Y · Y · (2/5) | ✗ | ✗ | ✓ |
| Q3.5-4B / ui-test-2 | `Merge all to 1 output` | · Y · Y · (2/5) | ✗ | ✗ | ✓ |
| Q3.5-4B / ui-test-2 | `gemini-3-pro-image-preview` | Y Y Y Y · (4/5) | ✗ | ✓ | ✓ |

→ For coverage tasks, **majority is the wrong aggregator**: it requires a label to appear in ≥3/5 runs, so it drops real-but-rare elements. Union keeps everything any run caught.

### extract-code-test-1 — low variance → correlation can't help

| Model | single recall | maj@5 recall | union@5 recall | discrim single | discrim maj@5 | discrim union@5 |
|---|---:|---:|---:|---:|---:|---:|
| Q3VL-8B-Q8 | 97% | 97% | 97% | 50% | 50% | 50% |
| Q3.5-4B | 98% | 98% | 98% | 50% | 50% | 50% |

Token recall is pinned; the **4 hard discriminators are stuck at 2/4 for both models, identical across all 5 runs.** The misses are *systematic*, not stochastic — e.g. Q3VL-8B-Q8 writes `createAujourd'hui` (with apostrophe) in every run; nobody but the 27B ever gets `validatePlaylistKeys_`. Sampling 5× returns the same wrong answer. **Correlation cannot fix a deterministic error.**

## Implications for the localvision MCP

| Tool | Multi-sampling worth it? | Why |
|---|---|---|
| `describe_ui` (and other open-ended/coverage) | **Yes — cheap win.** Sample 2–3×, take the **union** of mentioned elements. | Outputs vary run-to-run; union recovers the long tail of occasionally-mentioned labels (+1–4 pts). Marginal calls are cheap (prefill is cached). |
| `extract_text`, `extract_code` | **No.** Single call is as good. | Deterministic at temp 0.1; systematic OCR/code errors are identical across runs. |
| `describe_chart`, `describe_diagram` | **Probably yes** (open-ended) — untested, but structurally like `describe_ui`. | Likely high run-variance → union should help; needs a test to confirm. |

**Operational:**
- **Keep the model server warm** (don't restart per call). The first call on each image pays ~3–8s of one-time prefill + kernel warmup; steady state is 1.1–1.6× faster.
- For coverage tools, **union, not majority.** Majority vote is for tasks with one right answer; coverage has many.
- The absolute gains are small (the models are already 93–99% on UI) — treat union-sampling as a cheap long-tail polish, not a quality revolution.

## Why the gains are small — and the obvious next lever

At **temp 0.1**, decoding diversity is low, so even on UI only a handful of labels vary run-to-run (the union recovers ~2 of ~37). The two knobs that decide whether correlation helps at all:

1. **Aggregator** — union (coverage) vs majority (single-answer). We showed majority is wrong for UI.
2. **Diversity (temperature)** — *not yet tested.* At temp 0.1 the code discriminators are a hard ceiling (0/5 every run). Raising temp (0.6–0.8) would make runs diverge more, giving union more to recover — and *might* occasionally land a discriminator like `validatePlaylistKeys_`, letting union break the systematic-error ceiling. The trade-off: higher temp also injects wrong tokens, so a fair test needs a **precision/hallucination** metric alongside recall (union currently can't tell a correct rare label from a hallucinated one).

**Recommended next step:** a temp sweep (0.1 / 0.4 / 0.7) on `ui-test-2` + `extract-code-test-1`, 5 reps each, scored with recall **and** hallucination rate, to see whether self-consistency unlocks larger gains than the faithful-identical pass above. The harness already supports `--temp`.

## Bottom line

- **Latency:** yes — warm calls are 1.1–1.6× faster; keep the server warm and expect the first call per image to be the slow one. Re-querying is cheap (~25–29% cheaper than naïve N×).
- **Quality:** correlation helps **only where outputs vary** — open-ended UI description, via **union** (+1–4 pts label recall, for free). It does nothing for deterministic extraction (OCR/code), and **majority vote is the wrong aggregator** for coverage.
- For localvision: sample-and-union on `describe_ui`-type tools; single call on extractors; keep the server warm.
