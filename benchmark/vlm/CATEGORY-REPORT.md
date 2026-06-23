# Category report — which localvision tools benefit from multi-sampling?

**Date:** 2026-06-24 · default model **Q3VL-8B-Q8** · 8 tool categories × 1 problematic image each × 3 temperatures (0.1 / 0.4 / 0.7) × 5 repeats = 120 calls · `run_id=cat-*`.

This follows [`REPEAT-REPORT.md`](./REPEAT-REPORT.md) (which showed, at temp 0.1 only, that correlation helps UI but not code). Here we add the two missing levers — **temperature** and **aggregator choice** — across **every category**, to answer: *which tools can benefit, and how?*

## Headline — three regimes

Multi-sampling + correlation is **not** uniform across categories. Splitting by task shape:

| Regime | Categories | What works | Gain |
|---|---|---|---|
| 🟢 **Benefits — union @ higher temp** | `read_image`, `describe_ui`, `describe_chart`, `extract_text` | sample 5× at temp 0.7, take the **union** | **+8 to +22 pts** |
| 🟡 **Benefits — majority only** | `extract_table` | sample 5× at temp 0.7, take the **majority** (union *hurts*) | +2 pts |
| ⚪ **No benefit** | `extract_code`, `describe_diagram`, `diagnose_error` | single call is as good; errors are systematic | 0 |

So **5 of 8 tested tools benefit, 3 don't** — and the *aggregator* matters as much as the *sampling*.

## Verdict matrix — union@5 @ temp 0.7 vs single @ temp 0.1 (production)

> Primary metric: F1 for extraction categories (code, table), key-fact recall for coverage. Hallucination = mentions of curated "NOT in image" facts.

| Category | baseline single@0.1 | best correlated@0.7 | Δ | aggregator | verdict |
|---|---:|---:|---:|---|---|
| `read_image` (Waldo) | 18% | 40% | **+22** | union | 🟢 BENEFIT |
| `extract_text` (OCR) | 65% | 74% | **+9** | union | 🟢 BENEFIT |
| `describe_ui` | 89% | 97% | **+8** | union | 🟢 BENEFIT |
| `describe_chart` | 85% | 92% | **+8** | union | 🟢 BENEFIT |
| `extract_table` | 77% F1 | 79% F1 | +2 | **majority** | 🟡 BENEFIT (majority only) |
| `extract_code` | 97% F1 | 97% F1 | 0 | — | ⚪ none |
| `describe_diagram` | 91% | 91% | 0 | — | ⚪ none |
| `diagnose_error` | 69% | 69% | 0 | — | ⚪ none |

> The automated scorer flagged `extract_table` as ❌ HURTS because it compares *union* uniformly — and union does hurt tables (−2 F1). The manual read shows **majority** is the correct aggregator there and gives +2. This is the report's key nuance: **the aggregator must match the task.**

## The two levers

### Lever 1 — temperature (diversity unlocks the benefit)

At the production temp **0.1**, almost nothing benefits — decoding is near-deterministic, so 5 runs are ~identical and correlation ≈ single. The gains **unlock at 0.4–0.7**, where runs diverge and union can merge what different runs noticed. This is most visible on coverage tasks:

| Category | union@5 @0.1 | union@5 @0.4 | union@5 @0.7 |
|---|---:|---:|---:|
| `describe_ui` | 89% | 92% | **97%** |
| `describe_chart` | 85% | 85% | **92%** |
| `extract_text` | 65% | **78%** | 74% |

> Note `extract_text` peaks at **0.4** (78%) then eases at 0.7 (74%) — for noisy/handwritten OCR, mid-temp is the sweet spot; too-hot starts adding noise.

### Lever 2 — aggregator (union vs majority)

| Task shape | Right aggregator | Why |
|---|---|---|
| **Coverage** (list everything you see: UI, chart, scene) | **union** | Real elements often appear in only 1–2 of 5 runs; union keeps them, majority drops them. |
| **Precision-sensitive extraction** (tables) | **majority** | At higher temp the model emits spurious cells/scaffolding; union accumulates that noise (precision 78→74%), while majority (≥3/5) filters it (precision →83%). |
| **Single-answer / deterministic** (code IDs, error file:line) | single | No diversity to exploit; the model's errors are consistent. |

## Why the "no benefit" categories don't benefit

These aren't suffering from sampling noise — their errors are **systematic**, identical across all 5 runs **and** all 3 temps:

- **`extract_code`**: stuck at 97% / 2-of-4 discriminators at every temp. `validatePlaylistKeys_` (trailing underscore) and the apostrophe in `createAujourdhui` are wrong in *every* run. No temperature or aggregator recovers a token the model never emits.
- **`describe_diagram`**: 91% flat. The architecture's components are named consistently; the deliberate discriminator (the dangling "phantom" gRPC line) is missed at every temp.
- **`diagnose_error`**: 69% flat. The facts the model gets (exception class, root-cause message) it gets every time; the ones it misses (exact `db.internal`, IP, line numbers) it misses every time.

**Takeaway:** multi-sampling fixes *stochastic* errors (run-to-run variance), not *systematic* ones. Before deploying sample-and-correlate on a tool, check that the tool's failures are variable, not consistent.

## Precision & hallucination

- **Known-wrong facts were never hallucinated**, even at temp 0.7 union (0/4 `pier`/`jetty`/`lighthouse`/`Dick Bruna` on Waldo across all temps). Higher temp does **not** invent the specific "NOT in image" objects in this set.
- **But high-temp extraction adds spurious tokens.** On `extract_table`, union@0.7 carries 10 tokens not in the GT (e.g. an invented `fitness`/`type` row label plus Markdown scaffolding) vs 8 at temp 0.1. This is the real precision risk — and it's why tables need majority, not union.

## Implications for the localvision MCP

| Tool | Sample-and-correlate? | Recipe |
|---|---|---|
| `read_image` | **Yes — high value.** | 5× @ temp 0.7, **union**. Biggest gain (+22 on dense scenes); the one run that finds Waldo is worth merging. |
| `describe_ui` | **Yes.** | 5× @ temp 0.7, **union** (+8). |
| `describe_chart` | **Yes.** | 5× @ temp 0.7, **union** (+8). |
| `extract_text` | **Yes (noisy/handwritten OCR).** | 5× @ **temp 0.4**, union (+9; 0.4 beats 0.7 here). |
| `extract_table` | **Yes — but majority.** | 5× @ temp 0.7, **majority** (+2; union hurts). |
| `extract_code` | **No.** | Single call. Errors are systematic. |
| `describe_diagram` | **No.** | Single call. |
| `diagnose_error` | **No.** | Single call. |
| `image_to_prompt` | *(untested — generative, no recall GT)* | — |
| `compare_images` | *(untested — 2-image tool)* | — |

**Operational cost** (from `REPEAT-REPORT.md`): repeats are cheap — warm calls are 1.1–1.6× faster than the first (the server reuses the warmed slot), so sampling 5× costs ~70–75% of 5× the single-call time. For the 5 benefiting tools that's a good trade; for the 3 non-benefiting tools it's pure waste.

## Limitations

- **One image per category** — the verdict is per-category *directional*, not exhaustive. A category flagged "no benefit" on one image might benefit on another (e.g. `diagnose_error` on a noisier trace). The systematic-vs-stochastic distinction is the reliable signal, not the absolute per-category number.
- **One model (the default Q3VL-8B-Q8).** The smaller Q3.5-4B has higher run-variance and would likely show *larger* effects; not yet run on the category sweep.
- **Small hallucination watch-list** — only 4 "NOT in image" facts (all on Waldo). The "no hallucination" finding is encouraging but narrow.
- **`image_to_prompt` and `compare_images` excluded** — no single right answer / 2-image; need a different scoring method.

## Recommended next steps

1. **Confirm on Q3.5-4B** for the 5 benefiting categories (expect larger gains; cheap — the 4B is fast).
2. **Add a second image per "no-benefit" category** to confirm the systematic-error reading (e.g. a second error trace, a second diagram).
3. **Score `image_to_prompt`** via prompt-recreation (round-trip through a judge) if that tool's quality matters for the MCP.
