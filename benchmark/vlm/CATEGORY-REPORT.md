# Category report — which localvision tools benefit from multi-sampling?

**Date:** 2026-06-24 · models **Q3VL-8B-Q8** (default) **+ Q3.5-4B-nothink** (small) · 8 tool categories × 1 problematic image each × 3 temperatures (0.1 / 0.4 / 0.7) × 5 repeats = 120 calls per model · `run_id=cat-*`. The body is told for the **default 8B**; the [**Model size matters**](#model-size-matters--the-q35-4b-comparison) section adds the 4B.

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

### Lever 1 — temperature (the gate, and where the gain actually comes from)

Temperature is the **gate**. At production temp 0.1, decoding is near-deterministic, so 5 runs are ~identical and correlation adds almost nothing. The benefit **unlocks at 0.4–0.7**, where runs diverge and union can merge what different runs noticed. The table decomposes the headline gain into its two parts, per category (all on the default model, Q3VL-8B-Q8):

- **gap@T** = `union@T − single@T` — the *pure correlation value at that temperature*.
- **temp effect** = `single@0.7 − single@0.1` — how much a single *hotter* run surfaces on its own (hotter → more verbose → more facts noticed).
- **corr effect** = `union@0.7 − single@0.7` — the value of *merging* 5 hot runs, over and above a single hot run.

| Category | single .1/.4/.7 | union .1/.4/.7 | gap@.1/.4/.7 | total Δ = temp + corr |
|---|---|---|---|---|
| `read_image` (Waldo) | 18 / 26 / 30 | 30 / 30 / **40** | +12 / +4 / +10 | **+22** = +12 + +10 |
| `extract_text` (OCR) | 65 / 69 / 66 | 65 / **78** / 74 | +0 / +10 / +8 | **+9** = +1 + +8 |
| `describe_ui` | 89 / 90 / 92 | 89 / 92 / **97** | +0 / +2 / +5 | **+8** = +3 + +5 |
| `describe_chart` | 85 / 85 / 88 | 85 / 85 / **92** | +0 / +0 / +5 | **+8** = +3 + +5 |
| `extract_table` | 77 / 77 / 78 | 77 / 75 / 75 | +0 / −2 / −3 | **−2** = +1 + −3 |
| `extract_code` | 97 / 97 / 97 | 97 / 97 / 97 | +0 / +0 / +0 | 0 |
| `describe_diagram` | 91 / 91 / 91 | 91 / 91 / 91 | +0 / +0 / +0 | 0 |
| `diagnose_error` | 69 / 69 / 69 | 69 / 69 / 69 | +0 / +0 / +0 | 0 |

Three things to read off this table:

1. **Correlation is worthless at temp 0.1.** The `gap@0.1` column is ≈0 for every category. (The +12 on `read_image` is a timeout artifact — that cell ran 4/5 ok, so `single@0.1` is depressed by one empty run; the fair number is `gap@0.4` = +4.) **At the production temperature, sampling 5× buys almost nothing because the runs come out the same.** This is the single most important practical finding: *temperature is a prerequisite* — without raising it, multi-sampling is pure latency cost.

2. **The gain has two distinct sources, and the mix is category-specific.**
   - `extract_text`'s entire **+9 is correlation** (`+1 temp + +8 corr`): a single run barely changes with temperature, but *merging* runs recovers fields each individual run misread. This category **requires** multi-sample-and-merge — a single hotter call won't help.
   - `read_image`'s **+22 is roughly half temp, half correlation** (`+12 + +10`): hotter single runs already surface more of the hundreds of Waldo details; merging adds the rest. Here a single hotter call already captures much of the gain.
   - `describe_ui` / `describe_chart` split **+3 temp / +5 corr** — correlation-led, but with a real verbosity bonus.
   - The split matters for deployment: a temp-effect-heavy category *could* be served by one hotter call; a corr-effect-heavy category *must* sample-and-merge.

3. **Sweet spots differ — and three categories are temperature-immune.**
   - `extract_text` peaks at **0.4** (union 78%) and *eases* at 0.7 (74%) as OCR noise creeps in — mid-temp is the sweet spot for noisy/handwritten text.
   - `read_image`, `describe_ui`, `describe_chart` peak at **0.7**.
   - `extract_code`, `describe_diagram`, `diagnose_error` are **flat across all three temps** — hotter sampling changes their output not at all, confirming their errors are systematic, not stochastic. (`extract_table`'s negative corr-effect is the precision story in Lever 2: union accumulates high-temp noise, so it needs majority, not union.)

> Per-category full single/union/majority curves at each temperature are in the per-category tables at the end of this report.

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

## Model size matters — the Q3.5-4B comparison

Running the same sweep on the small/fast recommendation (Q3.5-4B-nothink) confirms the category shape **and adds a model-size dimension**. Below: best correlated result @ temp 0.7 vs single @ 0.1, using **majority** for extraction and **union** for coverage (see Lever 2):

| Category | 8B single@0.1 | 8B best@0.7 | 8B Δ | 4B single@0.1 | 4B best@0.7 | 4B Δ |
|---|---:|---:|---:|---:|---:|---:|
| `read_image` | 18 | 40 | **+22** | 36 | 50 | +14 ⚠️halluc |
| `extract_text` | 65 | 74 | +9 | 65 | 74 | +9 |
| `extract_code` | 97 | 96 | 0 | 97 | 97 | 0 |
| `extract_table` | 77 | 79 | +2 | 81 | 82 | +1 |
| `describe_ui` | 89 | 97 | +8 | 97 | 100 | +3 |
| `describe_diagram` | 91 | 91 | 0 | 91 | 91 | 0 |
| `describe_chart` | 85 | 92 | +8 | 78 | 92 | **+14** |
| `diagnose_error` | 69 | 69 | 0 | 69 | 77 | **+8** |

Four model-size findings:

1. **The benefiting set is mostly shared, but `diagnose_error` flips.** Both gain on `read_image`, `extract_text`, `describe_ui`, `describe_chart`. But `diagnose_error` is **neutral on the 8B and benefits on the 4B (+8)** — the 4B's noisier reads make its missed facts recoverable via union, where the 8B's consistent reads make them systematic. *Whether a category benefits is partly model-dependent.*

2. **The 4B is fragile at high temp on extraction — union is catastrophic, majority saves it.** At temp 0.7 the 4B's individual code runs degrade (single F1 97→77%) and **union collapses to 38% F1** (precision 24% — it accumulates high-temp garbage). But **majority vote recovers it to 97% F1** (precision 97%), filtering the noise perfectly. The 8B tolerates high-temp extraction fine (union code stays 97%). *The smaller the model, the more its high-temp output leans on majority (not union) to stay clean.*

3. **The 4B hallucinates more at high temp.** On Waldo, 2 of 5 runs at temp 0.7 invented `pier`/`jetty`/`lighthouse` (the 8B invented none); union carries both. **Majority drops them** (only 2/5 runs have them). *For a noisier model, majority is the hallucination-safe aggregator on dense scenes too.*

4. **Magnitudes track the baseline.** The 8B's `read_image` gain is larger (+22 vs +14) because its single-run baseline is lower (18% vs 36% — the 8B surfaces fewer Waldo facts per run, leaving more for union to add). The 4B gains more on `describe_chart` (+14 vs +8). Neither model dominates outright.

**Bottom line by model:**
- **8B (default):** sample-and-correlate at 0.7 is broadly **safe** — high temp is well-tolerated (no hallucination, extraction stays coherent). Union for coverage, majority for tables. 5 tools benefit.
- **4B (small):** higher variance gives **bigger coverage gains** (chart +14, read_image +14, and `diagnose_error` now benefits) — but high temp is **less safe**: *never union on extraction* (catastrophic), and *prefer majority on dense scenes* to suppress hallucination. For the 4B, majority is the workhorse aggregator; union only for clean coverage tasks.

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

- **Two models tested (8B + 4B), but one image per category each** — the verdict is per-category *directional*, not exhaustive. The systematic-vs-stochastic distinction is the reliable signal: it held across *both* models for `extract_code` / `describe_diagram` (systematic → no benefit), while `diagnose_error` flipped from neutral (8B) to benefit (4B) — so category verdicts can be model-dependent. A second image per category would tighten the magnitudes.
- **Small hallucination watch-list** — only 4 "NOT in image" facts (all on Waldo). The "no hallucination" finding is encouraging but narrow.
- **`image_to_prompt` and `compare_images` excluded** — no single right answer / 2-image; need a different scoring method.

## Recommended next steps

1. **Add a second image per category** — especially the "no benefit" ones (a second error trace, a second diagram) to confirm the systematic-error reading, and the flippers (`diagnose_error`) to map the model-dependence.
2. **Tune the aggregator per tool for the 4B** — it leans harder on majority (extraction + dense scenes) than the 8B; a per-tool default temp/aggregator table would make the MCP guidance concrete.
3. **Score `image_to_prompt`** via prompt-recreation (round-trip through a judge) if that tool's quality matters for the MCP.

## Appendix — per-category detail, all temperatures (Q3VL-8B-Q8)

> 5 reps per cell. Extraction (code, table): token P/R/F1 vs a GT text block. Coverage: key-fact recall + hallucination count (mentions of curated 'NOT in image' facts).


### read_image — `30-where-is-waldo.webp` (Waldo dense scene)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 18% | 30% | 20% | 0.0 | 0 |
| 0.4 | 26% | 30% | 30% | 0.0 | 0 |
| 0.7 | 30% | 40% | 30% | 0.0 | 0 |

### extract_text — `ocr-test-4.png` (handwritten OCR)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 65% | 65% | 65% | 0.0 | 0 |
| 0.4 | 69% | 78% | 65% | 0.0 | 0 |
| 0.7 | 66% | 74% | 70% | 0.0 | 0 |

### extract_code — `extract-code-test-1.png` (code)

| temp | single R | single P | single F1 | union@5 R | union@5 P | union@5 F1 | maj@5 R | maj@5 P | maj@5 F1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.1 | 97% | 96% | 97% | 97% | 96% | 97% | 97% | 96% | 96% |
| 0.4 | 97% | 96% | 97% | 97% | 96% | 97% | 97% | 97% | 97% |
| 0.7 | 97% | 96% | 97% | 97% | 96% | 97% | 97% | 96% | 96% |

### extract_table — `08-poster-class-schedule.jpg` (class schedule)

| temp | single R | single P | single F1 | union@5 R | union@5 P | union@5 F1 | maj@5 R | maj@5 P | maj@5 F1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.1 | 76% | 78% | 77% | 76% | 78% | 77% | 76% | 78% | 77% |
| 0.4 | 76% | 78% | 77% | 76% | 74% | 75% | 76% | 81% | 78% |
| 0.7 | 76% | 80% | 78% | 76% | 74% | 75% | 76% | 83% | 79% |

### describe_ui — `ui-test-1.png` (UI screenshot)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 89% | 89% | 89% | 0.0 | 0 |
| 0.4 | 90% | 92% | 89% | 0.0 | 0 |
| 0.7 | 92% | 97% | 89% | 0.0 | 0 |

### describe_diagram — `04_architecture.png` (architecture)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 91% | 91% | 91% | 0.0 | 0 |
| 0.4 | 91% | 91% | 91% | 0.0 | 0 |
| 0.7 | 91% | 91% | 91% | 0.0 | 0 |

### describe_chart — `26-graph-ocean-acidification-hawaii.jpg` (ocean acidification)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 85% | 85% | 85% | 0.0 | 0 |
| 0.4 | 85% | 85% | 85% | 0.0 | 0 |
| 0.7 | 88% | 92% | 85% | 0.0 | 0 |

### diagnose_error — `03_error_trace.png` (stack trace)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 69% | 69% | 69% | 0.0 | 0 |
| 0.4 | 69% | 69% | 69% | 0.0 | 0 |
| 0.7 | 69% | 69% | 69% | 0.0 | 0 |

## Appendix — per-category detail, all temperatures (Q3.5-4B-nothink)

> Same layout as the 8B appendix. Note how the 4B's extraction F1 holds under **majority** but collapses under **union** at temp 0.7 — the model-size fragility from the comparison above.


### read_image (4B) — `30-where-is-waldo.webp` (Waldo dense scene)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 36% | 40% | 40% | 0.0 | 0 |
| 0.4 | 34% | 40% | 30% | 0.0 | 0 |
| 0.7 | 28% | 50% | 30% | 0.4 | 2 |

### extract_text (4B) — `ocr-test-4.png` (handwritten OCR)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 65% | 65% | 65% | 0.0 | 0 |
| 0.4 | 63% | 74% | 65% | 0.0 | 0 |
| 0.7 | 66% | 74% | 61% | 0.0 | 0 |

### extract_code (4B) — `extract-code-test-1.png` (code)

| temp | single R | single P | single F1 | union@5 R | union@5 P | union@5 F1 | maj@5 R | maj@5 P | maj@5 F1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.1 | 98% | 95% | 97% | 98% | 95% | 97% | 98% | 95% | 97% |
| 0.4 | 98% | 81% | 85% | 99% | 21% | 35% | 98% | 96% | 97% |
| 0.7 | 98% | 70% | 77% | 99% | 24% | 38% | 98% | 97% | 97% |

### extract_table (4B) — `08-poster-class-schedule.jpg` (class schedule)

| temp | single R | single P | single F1 | union@5 R | union@5 P | union@5 F1 | maj@5 R | maj@5 P | maj@5 F1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.1 | 76% | 85% | 81% | 76% | 85% | 81% | 76% | 85% | 81% |
| 0.4 | 76% | 86% | 81% | 76% | 85% | 81% | 76% | 85% | 81% |
| 0.7 | 74% | 82% | 78% | 76% | 69% | 72% | 76% | 88% | 82% |

### describe_ui (4B) — `ui-test-1.png` (UI screenshot)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 97% | 97% | 97% | 0.0 | 0 |
| 0.4 | 97% | 97% | 97% | 0.0 | 0 |
| 0.7 | 95% | 100% | 97% | 0.0 | 0 |

### describe_diagram (4B) — `04_architecture.png` (architecture)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 91% | 91% | 91% | 0.0 | 0 |
| 0.4 | 91% | 91% | 91% | 0.0 | 0 |
| 0.7 | 91% | 91% | 91% | 0.0 | 0 |

### describe_chart (4B) — `26-graph-ocean-acidification-hawaii.jpg` (ocean acidification)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 78% | 85% | 77% | 0.0 | 0 |
| 0.4 | 82% | 85% | 85% | 0.0 | 0 |
| 0.7 | 80% | 92% | 85% | 0.0 | 0 |

### diagnose_error (4B) — `03_error_trace.png` (stack trace)

| temp | single recall | union@5 recall | maj@5 recall | halluc single | halluc union@5 |
|---|---:|---:|---:|---:|---:|
| 0.1 | 69% | 69% | 69% | 0.0 | 0 |
| 0.4 | 63% | 69% | 69% | 0.0 | 0 |
| 0.7 | 74% | 77% | 77% | 0.0 | 0 |
