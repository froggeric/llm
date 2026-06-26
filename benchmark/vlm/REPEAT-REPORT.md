# Multi-sampling report — re-querying a warm model: latency, correlation & temperature

**Date:** 2026-06-23 (latency + temp-0.1 correlation) · **revised 2026-06-24** (temperature sweep folded in from [`CATEGORY-REPORT`](./CATEGORY-REPORT.md)) · 2 models (small + medium) × 3 images × 5 back-to-back identical calls = 30 calls · `run_id=repeat-*`.

> **Two halves.** §1–§2 are the original latency + correlation study at the production temp (0.1). §3 folds in the **temperature sweep** this report originally recommended as "next step" — it was run the next day across 7 models × 3 temps × 8 categories ([`CATEGORY-REPORT`](./CATEGORY-REPORT.md)). The headline: at temp 0.1 correlation barely helps; **temperature is the gate** that decides whether multi-sampling pays at all.

## The premise

Instead of querying a model once per image, take advantage that it's already loaded and warmed to fire the **same query N times back-to-back in one session**, then correlate. Three questions:

1. **Latency** — are calls 2..N cheaper than call 1 once the model is loaded + warm?
2. **Quality** — does correlating N identical runs (majority / union) beat a single run?
3. **Temperature** — correlation only helps where outputs *vary*; at temp 0.1 they barely do. Does multi-sampling only pay once temperature is raised, and for which tools?

**Scope (small + medium recommendations):** `Q3.5-4B-nothink` and `Q3VL-8B-Q8`.

**Image selection was driven by run-variance, not just struggle.** At our production temp (0.1) these models are near-deterministic on *extraction* (OCR/code: divergence ~0.00–0.01) but highly variable on *open-ended* UI description. So multi-sampling only has room to lift quality where outputs vary — UI screenshots. `extract-code-test-1` is the low-variance **contrast**.

| Image | Why chosen | Q3.5-4B div | Q3VL-8B-Q8 div |
|---|---|---:|---:|
| `ui-test-1.png` | high-variance struggle | 0.32 | 0.02 |
| `ui-test-2.png` | highest-variance struggle | 0.45 | 0.13 |
| `extract-code-test-1.png` | low-variance contrast | 0.01 | 0.01 |

> 3×/4×/5× in the quality tables are derived from **nested prefixes** of the 5-call session (first 3, first 4, all 5) — same warm state, so the comparison is clean.

## Headline findings

1. **Warm calls are ~1.1–1.6× faster than call 1** — a one-time ~3–8s warmup per image. The saving is entirely in **prompt/image-eval wall-time** (the warmed slot reuses the KV cache, so prefill executes at ~0 wall-time even though the token count is logged in full) — *not* model load and *not* generation. Net: sampling the same image 5× costs only **~70–75% of 5× the single-call time** — repeats are cheap. *(See §1 for the per-call breakdown.)*
2. **At temp 0.1, correlation helps coverage tasks, not extraction.** On UI screenshots, taking the **union** of N descriptions recovers +1–4pts of label recall for free; on code, systematic errors are identical every run and correlation changes nothing.
3. **Union is the right aggregator for coverage — majority vote is wrong (there).** Majority (≥3/5) *drops* the real labels that only 1–2 runs mention; union keeps them.
4. **Temperature is the gate.** At temp 0.1 the runs come out ~identical, so correlation adds ≈0 for *every* category (`gap@0.1 ≈ 0`). The benefit **unlocks at 0.4–0.7**, where runs diverge and union can merge what different runs noticed. Without raising temperature, multi-sampling is pure latency cost. *(§3)*
5. **The aggregator must match the task.** Union for **coverage** (UI/chart/scene); **majority** for precision-sensitive extraction (tables — union accumulates high-temp noise); **single** for deterministic tasks (code IDs, error file:line). *(§3)*
6. **Small models are fragile at high temp.** At temp 0.7 the 4B's code runs go noisy and **union collapses to 38% F1** — but **majority recovers 97% F1**. The 8B tolerates high-temp extraction fine. Corollary: prefer 4B-Q8 over the flaky 4B-Q4. *(§3)*

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

**Per-call breakdown — where exactly is the warm-up time spent?** call 1 vs the mean of warm calls 2–5, with the per-call fields the harness records (medians in the speedup table above are within ±0.2s of these means):

| Model | Image | call | elapsed_s | prefill tok (`prompt_eval_count`) | gen tok (`eval_count`) | `load_duration_s` | tok/s |
|---|---|---|---:|---:|---:|---:|---:|
| Q3VL-8B-Q8 | extract-code-test-1 | 1 | 21.7 | 2348 | 528 | 0.0 | 38.2 |
| Q3VL-8B-Q8 | extract-code-test-1 | 2–5 | 13.9 | 2348 | 528 | 0.0 | 38.4 |
| Q3VL-8B-Q8 | ui-test-1 | 1 | 18.0 | 1274 | 577 | 0.0 | 39.9 |
| Q3VL-8B-Q8 | ui-test-1 | 2–5 | 14.1 | 1274 | 552 | 0.0 | 39.3 |
| Q3VL-8B-Q8 | ui-test-2 | 1 | 24.2 | 1205 | 826 | 0.0 | 39.3 |
| Q3VL-8B-Q8 | ui-test-2 | 2–5 | 21.0 | 1205 | 823 | 0.0 | 39.4 |
| Q3.5-4B | extract-code-test-1 | 1 | 13.2 | 2352 | 572 | 0.0 | 65.9 |
| Q3.5-4B | extract-code-test-1 | 2–5 | 9.0 | 2352 | 576 | 0.0 | 65.1 |
| Q3.5-4B | ui-test-1 | 1 | 13.5 | 1283 | 743 | 0.0 | 65.0 |
| Q3.5-4B | ui-test-1 | 2–5 | 11.8 | 1283 | 761 | 0.0 | 64.8 |
| Q3.5-4B | ui-test-2 | 1 | 18.1 | 1214 | 1045 | 0.0 | 64.3 |
| Q3.5-4B | ui-test-2 | 2–5 | 20.1 | 1214 | 1306 | 0.0 | 65.2 |

**Reading:**
- **The saving is in prompt-eval wall-time — not token count, not model load.** `prompt_eval_count` is *identical* cold vs warm in every cell (2348/2348, 1274/1274…) → warm calls do not log fewer prefill tokens; `load_duration_s` is **0.0 on every call, including the cold one** → it is *not* model load; `tok/s` is flat call-to-call (38 vs 38, 65 vs 65) and `eval_count` is ~constant → generation speed and length are unchanged.
- **Decompose `extract-code-test-1` (8B):** cold 21.7s ≈ 13.8s generation (528 tok @ 38.2) + **~7.9s non-generation**; warm 13.9s ≈ 13.8s generation + **~0.15s non-generation**. The entire warm-call saving is that ~7.9s of prompt/image-eval wall-time, which collapses to ~0 because the warmed slot reuses the identical prompt+image KV across re-queries. Generation runs at full speed either way.
- The **first image of the session** shows the biggest warmup (1.56× / 1.47×): it pays the most prefill wall-time (image encode + likely a one-time Metal kernel compile). The 2nd/3rd images show smaller warmup (1.1–1.3×). ⚠️ `load_duration_s=0` does *not* capture kernel-compile time, so splitting the first-image cost between "image encode" and "kernel compile" remains an inference — the data surfaces it only as prefill wall-time.
- `Q3.5-4B × ui-test-2` (0.94×) is within noise: that cell's output length swings run to run (gen 1045 → 1306 tok), so output-length variance dominates the ~3s warmup signal.

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

- **3 repeats is the efficient knee** — for *coverage* (union). It captures essentially all the union benefit for ~60% less wall time than 5; pushing to 5 only makes sense as maximum-coverage insurance on the hardest scenes, and even there the marginal gain is ~0. **One exception surfaced by the 7-model category sweep ([CATEGORY-REPORT](./CATEGORY-REPORT.md)):** *majority-filtered extraction on the noisiest models* needs ≥4 reps — e.g. Q3.5-4B-Q4 code @0.7 is 62% F1 at 3-rep majority (≥2/3) but 97% at 4-rep (≥3/5). So: **3 reps for coverage; ≥4 reps (or low temp) for noisy-model extraction.** (The §1 "5× ≈ 70–75% of naïve 5×" saving is a *different* comparison — caching vs cold restarts — and doesn't change that reps 4–5 each still cost a full warm call.)

## 2. Quality at temp 0.1 — single vs majority vs union

> This is the **temp-0.1** correlation result — the foundation that motivates §3. At the production temp the gains are small *because decoding barely varies*; §3 shows what happens once temperature is raised.

`single` = mean per-run label recall (one random run). `maj@k` = per-label majority vote over first k runs (≥⌈k/2⌉). `union@k (pass@k)` = union over first k runs (≥1 run has it).

### UI screenshots — high variance → union helps

| Model | Image | single | maj@5 | union@5 | agree% |
|---|---|---:|---:|---:|---:|
| Q3VL-8B-Q8 | ui-test-1 | 90% | 89% | **95%** | 95% |
| Q3.5-4B | ui-test-1 | 100% | 100% | **100%** | 100% |
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

### LLM-judge holistic (0–10) + hallucinations

An LLM judge (Claude subagent viewing each image + scoring all 5 reps; outputs in `judgments_repeat/`, aggregated by `code/aggregate_judgments.py`) supplies the holistic + hallucination signal `score_repeat.py` can't compute:

| Model | Image | holistic mean | ±std | halluc (any rep) |
|---|---|---:|---:|---:|
| Q3VL-8B-Q8 | ui-test-1 | 7.2 | ±0.40 | 0 |
| Q3VL-8B-Q8 | ui-test-2 | 6.0 | ±0.00 | 0 |
| Q3VL-8B-Q8 | extract-code-test-1 | 4.2 | ±0.40 | 5 |
| Q3.5-4B | ui-test-1 | 5.2 | ±0.40 | 0 |
| Q3.5-4B | ui-test-2 | 5.2 | ±0.98 | 0 |
| Q3.5-4B | extract-code-test-1 | 6.8 | ±0.40 | 5 |

The judge **reverses the deterministic ranking on code**: on `extract-code-test-1` the 4B beats the 8B holistically (6.8 vs 4.2) even though token-recall had both ~97–98% — it caught the 8B's syntax-breaking errors (unclosed `rateLimitedForEach(` parens) that recall can't penalize. On UI it agrees with recall (8B > 4B). The only genuine hallucinations in the repeat set are on `extract-code-test-1` (both models invent a `catch` block absent from the bottom-truncated screenshot); the UI images show **0 hallucinations**.

> ⚠️ **Judge-reliability caveat (owner-verified 2026-06-26).** A first pass over-flagged "hallucinations" on the densest images — punishing correct details it couldn't see (`⌘N/⌘P/⌘K` shortcuts, extra top-bar icons, the 3-column layout) and trusting an incomplete ground truth (the last provider is **x.AI**, not "vAI" as the GT had it). After owner-verification of every flagged claim, correcting the GT, and tightening the rubric ("only flag if you can verify it's absent; dense scenes: don't flag minor details; misreads ≠ hallucinations"), the UI hallucination counts dropped 4–7 → 0. **Treat LLM-judge hallucination counts as soft (upper bounds), especially on dense scenes; deterministic scorers remain authoritative. Holistic scores are more robust — they survived the re-judge largely unchanged.**

## 3. Temperature — the gate (the sweep the original "next step" recommended, now done)

> This section is the answer to the report's original "recommended next step" — a temp sweep (0.1 / 0.4 / 0.7) scored with recall **and** precision/hallucination. It was run across **7 models × 8 categories × 3 temps** ([`CATEGORY-REPORT`](./CATEGORY-REPORT.md)); below is the consolidated picture for the two recommendation models. §2 found small gains at temp 0.1 — this explains *why*, and shows how much bigger the gains get once temperature is raised.

### The gate — correlation is worthless at temp 0.1

At production temp 0.1, decoding is near-deterministic, so 5 runs come out ~identical and correlation adds almost nothing. Measured as `gap@T = union@T − single@T` (the pure correlation value at that temperature), **`gap@0.1 ≈ 0` for every category.** The benefit **unlocks at 0.4–0.7**, where runs diverge and union can merge what different runs noticed.

This is the single most important practical finding, and it directly answers §2's "why are the gains so small": at temp 0.1 there is almost no run-to-run variance for union to exploit. **Temperature is a prerequisite** — without raising it, multi-sampling is pure latency cost.

### Two distinct gain sources — and the mix is category-specific

The headline Δ (`best@0.7 − single@0.1`) splits into two parts, and the split matters for deployment:

- **temp effect** (`single@0.7 − single@0.1`): a single *hotter* run is more verbose → notices more facts on its own.
- **corr effect** (`union@0.7 − single@0.7`): the value of *merging* 5 hot runs, over and above one hot run.

| Category (8B) | total Δ | = temp + corr | reading |
|---|---:|---|---|
| `read_image` (Waldo) | **+22** | +12 + +10 | half/half — a single hotter call already captures much of it |
| `extract_text` (OCR) | **+9** | +1 + **+8** | **correlation-led** — a single hotter call barely helps; you *must* sample-and-merge |
| `describe_ui` | **+8** | +3 + +5 | correlation-led, with a verbosity bonus |
| `describe_chart` | **+8** | +3 + +5 | correlation-led, with a verbosity bonus |
| `extract_table` | −2 | +1 + −3 | union *hurts* at high temp → needs majority (below) |
| `extract_code` / `describe_diagram` / `diagnose_error` | 0 | 0 + 0 | flat — errors are systematic |

Deployment take: a **temp-effect-heavy** category could be served by *one hotter call*; a **corr-effect-heavy** category (OCR) *must* sample-and-merge.

### Sweet spots differ — three categories are temperature-immune

- `extract_text` (handwritten OCR) peaks at **0.4** (union 78%) and *eases* at 0.7 (74%) as OCR noise creeps in — mid-temp is the sweet spot for noisy text.
- `read_image`, `describe_ui`, `describe_chart` peak at **0.7**.
- `extract_code`, `describe_diagram`, `diagnose_error` are **flat across all three temps** — hotter sampling changes their output not at all, confirming their errors are systematic, not stochastic.

### The aggregator must match the task (Lever 2)

| Task shape | Right aggregator | Why |
|---|---|---|
| **Coverage** (list everything: UI, chart, scene) | **union** | Real elements often appear in only 1–2 of 5 runs; union keeps them, majority drops them (this is §2's finding). |
| **Precision-sensitive extraction** (tables) | **majority** | At higher temp the model emits spurious cells / Markdown scaffolding; union accumulates that noise (precision 78→74%), while majority (≥3/5) filters it (precision →83%). |
| **Single-answer / deterministic** (code IDs, error file:line) | **single** | No diversity to exploit; the model's errors are consistent every run. |

So §2's "union, not majority" rule holds **for coverage** — but tables are the exception: there, majority is right and union hurts.

### Precision & hallucination — the open question, now answered

The original report flagged that "union can't tell a correct rare label from a hallucinated one" and recommended a hallucination metric. That metric now exists (`HALLUC`/`halluc_count` in `score_category.py`, plus 28+ owner-verified probes in `score_v5.py`), and the sweep answered it:

- **Known-wrong facts were never hallucinated**, even at temp 0.7 union — 0/4 (`pier`/`jetty`/`lighthouse`/`Dick Bruna`) on Waldo across all temps. Higher temp does *not* invent the specific "NOT in image" objects in this set.
- **The real precision risk is spurious tokens on extraction at high temp**, not invented objects. On `extract_table`, union@0.7 carries ~10 non-GT tokens (an invented row label + Markdown scaffolding) vs ~8 at 0.1 — which is exactly why tables need majority, not union.

### Small models are fragile at high temp — union collapses, majority recovers

At temp 0.7 the 4B's individual code runs degrade (single F1 97→77%) and **union collapses to 38% F1** (precision 24% — it accumulates high-temp garbage). But **majority vote recovers it to 97% F1** (precision 97%), filtering the noise perfectly. The 8B tolerates high-temp extraction fine (union code stays 97%). The 4B also hallucinates more on dense scenes (2/5 Waldo runs invent `pier`/`jetty`/`lighthouse` @0.7; majority drops them).

→ *The smaller / noisier the model, the more its high-temp output leans on **majority** (not union) to stay clean.* Practical corollary (from [`BENCHMARK-REPORT-v5`](./BENCHMARK-REPORT-v5.md)): prefer **4B-Q8** over the flaky 4B-Q4, which emits HTML garbage / malformed tables on some runs.

> Full per-category tables (all 3 temps, both models) and the **7-model master picture** (which categories benefit for which models — `describe_diagram` benefits for 0/7; multi-sampling never lifts a weak model past a strong one) are in [`CATEGORY-REPORT`](./CATEGORY-REPORT.md).

## Implications for the localvision MCP

Recipes use **3 reps** (the operating point — `union@3 ≈ union@5` per "Total time per image", so 5 reps buys ~0 extra coverage for ~60% more time). Temp/aggregator per tool (from §3 / [`CATEGORY-REPORT`](./CATEGORY-REPORT.md)):

| Tool | Sample-and-correlate? | Temp | Aggregator | Why |
|---|---|---|---|---|
| `read_image` (dense scenes) | **Yes — highest value.** | 0.7 | **union** | Biggest Δ on the 8B (+22/+23) because its per-run baseline is lowest. |
| `describe_ui` | **Yes — cheap win.** | 0.7 | **union** | Outputs vary run-to-run; union recovers the long tail of occasionally-mentioned labels (+8 pts on the 8B). Marginal calls are cheap (prefill wall-time cached). |
| `describe_chart` | **Yes.** | 0.7 | union | Tested now (was "probably yes"): benefits 6/7 models (+8 on the 8B). |
| `extract_text` (noisy/handwritten OCR) | **Yes — but must merge.** | **0.4** | union | Entirely correlation-led (+8 corr); a single hotter call won't help. Peaks at 0.4 (OCR noise creeps in at 0.7). |
| `extract_table` | **Yes — but majority, not union.** | 0.7 | **majority** | Union accumulates high-temp spurious cells; majority filters them. |
| `extract_code` | **No** (single, low temp). | low | single | Systematic errors identical every run (the hard discriminators are a ceiling). Exception: noisy small models need ≥4-rep majority, or just low temp. |
| `describe_diagram` | **No (0/7 models).** | — | single | The phantom gRPC line is a systematic error every model misses at every temp. |

**Operational:**
- **Keep the model server warm** (don't restart per call). The first call per image pays ~3–8s of one-time prompt/image-eval wall-time (+ likely a kernel compile); steady state is 1.1–1.6× faster because prefill reuses the warmed KV.
- **Match the aggregator to the task:** union for coverage, majority for tables, single for deterministic. Majority vote is *not* a universal default — on coverage it drops the real-but-rare labels.
- **Raise temperature where you multi-sample.** At the production temp 0.1, correlation adds ≈0; the lever only pays at 0.4–0.7. (On the 8B this is broadly safe; on the 4B, never union on extraction.)
- The temp-0.1 gains are small (the models are already 93–99% on UI); the bigger wins (+8 to +23 pts) only appear once temperature is raised. Treat temp-0.1 union-sampling as cheap long-tail polish; treat temp-raised sampling as a real quality lever.

## Bottom line

- **Latency:** warm calls are 1.1–1.6× faster; keep the server warm and expect the first call per image to be the slow one. The saving is in prompt/image-eval **wall-time** (warmed-KV reuse), *not* token count or model load — re-querying is cheap (~25–29% cheaper than naïve N×).
- **Quality at temp 0.1:** correlation helps **only where outputs vary** — open-ended UI, via **union** (+1–4 pts label recall, for free). It does nothing for deterministic extraction (OCR/code), and **majority is the wrong aggregator** for coverage.
- **Temperature is the gate.** The temp-0.1 gains are small *because decoding barely varies*; raise to 0.4–0.7 and union lifts coverage by +8 to +23 pts — but only for categories whose errors are stochastic, not systematic. **Match the aggregator to the task** (union / majority / single), and on small/noisy models prefer majority and low temp on extraction.

## What's still open

- **`ui-test-2` was never temperature-swept.** The category sweep covered `ui-test-1` but not this report's highest-variance image, so ui-test-2's temperature curve is still unmeasured.
- **One image per category** in the sweep — so per-cell verdicts are directional, not exhaustive (a second image would tighten the magnitudes). See [`CATEGORY-REPORT`](./CATEGORY-REPORT.md) §Limitations.
