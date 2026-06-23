# Head-to-Head: Qwen3-VL-8B @ Q8_0 vs Qwen3.5-4B @ Q4_K_M (no-think)

**Date:** 2026-06-22 · **Basis:** actual model answers in `benchmark-results/raw.jsonl`, judged against `test-images/GROUND-TRUTH.md` — **not** the v5/Q8 aggregate scores.

| | **A — Qwen3-VL-8B (Q8_0)** | **B — Qwen3.5-4B (Q4_K_M, `--disable-thinking`)** |
|---|---|---|
| Runs read | `q8-1/2/3` (3 × 30 images) | `nothink-1/2/3` + `nothink-img21-30-1/2/3` (3 × 30 images) |
| Avg answer length | ~2,500–3,800 chars (concise) | ~3,100–6,500 chars (verbose, emoji-sectioned) |
| Throughput (context only) | ~37–41 tok/s | ~65–72 tok/s (~1.7× faster) |

## TL;DR

**Winner on pure answer quality: A (Qwen3-VL-8B @ Q8_0)** — by a clear margin.

A is more accurate and more disciplined. It reads the VIC logo correctly, identifies the Pope and the embedded Hundertwasser face, and stays honest about the illegible signature. B reads the Hong Kong signage cleanly (no invented signs) and hedges *"no legible text"* on the signature — yet still makes three confident errors A avoids: it reads the logo as **"WIC Health Club"**, misattributes the Waldo scene to **"Dutch artist Dick Bruna"** (ground truth: Martin Handford), and in one run fabricates the painting signature as **"© 2015"**. B fights back on **people-counting** (it alone counts 9 on the kung-fu banner) and is more verbose, but loses on precision and scene-depth.

## Method

- Extracted every `content` answer for both models across all 3 runs (30 × 3 = 90 answers each).
- **Pass 1:** read a compact fingerprint of all 180 answers for substance + run-to-run variance.
- **Pass 2:** full-read the decision-critical images.
- **Verification:** every headline claim re-checked by regex across **all 3 runs** — quoted runs are representative.
- **Deliberately ignored:** the `judgments_*` LLM-judge outputs and aggregate scores. Verdicts come from actual answers vs owner-verified ground truth.
- **Scope (image only):** both models natively support image and video; neither supports audio. This report evaluates static images only — video was not exercised by the harness.

## Per-dimension verdict

| Dimension | A (Qwen3-VL-8B Q8) | B (Qwen3.5-4B Q4 no-think) | Edge |
|---|---|---|---|
| OCR — Latin/UI | Strong; reads *"VIC Health Club"* | Reads it as **"WIC Health Club"** (3/3) | **A** |
| OCR — CJK / French | Reads 茶香四溢滿屋, 大新銀行, *musique à volonté* | Same — reads them just as well | **Tie** |
| Hallucination discipline | Honest about illegible text | Confident wrongs: "Dick Bruna", "© 2015" (1/3), "WIC" | **A** |
| Counting / quantitative | 8 on banner (✗); 10 swatches (✓) | 9 on banner (✓); 10 swatches (✓, as "10") | **Tie*** |
| Spatial / relational | Consistent laterality + "concentric" on card | Mixed; slips *"spiral"* in 1/3, weak on left/right | **A (slight)** |
| Scene / object depth | Catches the Pope, the embedded face | Misses both ("possibly monks"; no face) | **A** |
| Detail / completeness | Concise | More verbose, structured, more surface detail | **B** |
| Honesty about uncertainty | "not clearly legible" | Hedges *"no legible text"* — but still fabricates 1/3 of the time | **A (slight)** |
| Medical imaging | Misses the rib fracture; no false finding | Misses the rib fracture; no false finding | **Tie** |
| Failure-mode robustness | 1 token-cap runaway (spritesheet) | 1 token-cap runaway (same spritesheet) | **Tie** |
| Run-to-run consistency | Very stable | More variable (signature 1/3 fabricated; digit-vs-word counts) | **A (slight)** |

\* Counting is 1–1 on decisive tasks (A: banner ✗ / swatch ✓; B: banner ✓ / swatch ✓), and both miss the manga panel count, so functionally a tie — B edges the headline count, A is more consistent.

## Curated examples (evidence)

### A wins

**1. `17-logo-vic-health-club.png` — OCR accuracy.** Text is *"VIC Health Club"*.
- **A (all 3 runs):** *"Vic Health Club"* — **correct.**
- **B (all 3 runs):** *"WIC Health Club"* — **wrong** (a one-letter misread, but still confidently incorrect).

**2. `30-where-is-waldo.webp` — artist attribution.** It's authentic Martin Handford; calling it **Dick Bruna is explicitly flagged as wrong** in ground truth.
- **A:** recognizes the *"find the differences / spot the object"* genre **without naming a wrong artist.**
- **B (all 3 runs):** *"likely inspired by the style of Dutch artist Dick Bruna"* — **confident, wrong attribution.**

**3. `05-cropped-youtube-capture.png` — scene depth.** A framed **Pope photo** is GT-verified as present (identifying it is correct).
- **A:** *"a framed photograph of Pope Francis."* Also reads 茶香四溢滿屋 and (incorrectly) calls the flag Canadian.
- **B:** reads the Chinese scroll too, but on the photo only manages *"two individuals, possibly monks"* — **misses the Pope entirely**, and doesn't engage the flag.

**4. `29-painting-hundertwasser.jpeg` — motifs + signature honesty.** Hallmarks: embedded human face (lower-left), blue teardrop windows, onion domes; signature is a red/pink box showing **1965**.
- **A:** captures the face (*"a woman's face emerging from a window"*), teardrops, onion domes, and honestly calls the signature *"not clearly legible."*
- **B:** captures teardrops + onion domes, but **misses the embedded face**, and in one run writes *"© 2015 [illegible name]"* — **a fabricated © and wrong year** (other runs just say "illegible", hence the 1/3 inconsistency).

### B wins

**5. `20-ultrawide-kung-fu-banner-with-logo.png` — people counting.** 9 performers (2 unarmed monks + 7 disciples).
- **A (all 3 runs):** *"a group of eight martial artists"* — **undercounts.**
- **B (all 3 runs):** *"nine performers"* — **correct.**

### Ties

**6. `13-colour-swatch-nausicaa.jpg` — counting (both right).** 10 colours.
- **A:** *"ten horizontal color bars."* **B:** *"10 horizontal rectangular bars."* **Both correct** (B uses the digit; a word-only search originally hid this).

**7. `21-motion-blur.jpg` (Hong Kong) — OCR under blur.** Both read 大新銀行 and don't fabricate extra signs. Both approximate DVFX (A: "DWF", B: "DWF").

**8. `28-xray-ribfracture-lowerright.jpg` — medical (both fail).** There is a left 10th posterior rib fracture. Both report normal anatomy with no fracture — and, to their credit, neither invents a false finding. Not a differentiator.

## Variance & failure modes

- **Stability:** A is very consistent run-to-run. B is mostly stable but **more variable in fabricated details** (the Hundertwasser signature is fabricated in 1/3 runs; count is given as a digit in some runs, a word in others).
- **Dense-scene runaways:** each model fails to terminate **once, on the same image** — A `q8-1` and B `nothink-img21-30-1` both hit the 16,384-token cap on the Bubble-Bobble spritesheet (A: 464 s; B: 270 s). Both correctly identify it as Bubble Bobble in their normal runs.
- **No crashes / `ok=False`** for either model across all 180 answers.

## Recommendation (pure answer quality)

**Pick A — Qwen3-VL-8B @ Q8_0.**

A is more accurate and more trustworthy: it read the logo, the Pope, and the embedded face, and it doesn't fabricate artist attributions or signature years. B's answers are longer and it alone got the 9-person count, but it pairs that detail with confident errors (*"WIC Health Club"*, *"Dick Bruna"*, *"© 2015"*) and shallower scene reading. For a vision tool, fewer confident falsehoods is the property that matters most.

**Where B is the better pick:** if you specifically value **verbose, highly-structured descriptions** and will verify outputs downstream, B is a legitimate, competitive alternative.

**Caveats:**
- The margin is real but moderate — ~5 decisive A-wins to 1 B-win among contested items, with OCR power (Latin, CJK, French) otherwise equal.
- *Pure-quality verdict only, as requested.* Operationally this is a size mismatch: B (4B @ Q4) is ~1.7× faster (~65–72 tok/s) and far lighter than A (8B @ Q8, ~37–41 tok/s) — a secondary consideration here, not a driver of the recommendation.
- This report covers only these two configs on the 30-image set.
