# Head-to-Head: Qwen3-VL-8B @ Q8_0 vs Qwen3.5-9B @ Q4_K_M (no-think)

**Date:** 2026-06-22 · **Basis:** actual model answers in `benchmark-results/raw.jsonl`, judged against `test-images/GROUND-TRUTH.md` — **not** the v5/Q8 aggregate scores.

| | **A — Qwen3-VL-8B (Q8_0)** | **B — Qwen3.5-9B (Q4_K_M, `--disable-thinking`)** |
|---|---|---|
| Runs read | `q8-1/2/3` (3 × 30 images) | `nothink-1/2/3` + `nothink-img21-30-1/2/3` (3 × 30 images) |
| Avg answer length | ~2,500–3,800 chars (concise) | ~3,000–6,000 chars (verbose, more sections) |

## TL;DR

**Winner on pure answer quality: A (Qwen3-VL-8B @ Q8_0)** — but by a real, not overwhelming, margin.

The deciding axis is **hallucination discipline**, not raw OCR power. Both models read Latin, French and Chinese text well; the gap is that **B confidently fabricates text A doesn't** — it misreads the VIC Health Club logo as *"vivo health hub"* (3/3 runs), invents street signs (*"Carmen"*, *"HSBC"*) on the Hong Kong photo, and fabricates the Hundertwasser signature year differently every run (*"© 2016"*, then *"D. K. 2005"*, then *"illegible"*). A either reads it correctly or honestly says it can't tell. A is also sharper on spatial reasoning and more measured about uncertainty. B fights back on **detail/completeness** and **people-counting** (it alone counted 9 people on the kung-fu banner), so it's competitive — not dominated.

---

## Method

- Extracted every `content` answer for both models across all 3 runs (30 images × 3 = 90 answers each, ~189k tokens total).
- **Pass 1:** read a compact fingerprint (ok/length/first 360 chars) of all 180 answers to see substance + run-to-run variance everywhere.
- **Pass 2:** full-read 11 decision-critical images (spatial test, counting tests, OCR wins/losses, dense-scene runaways, medical).
- **Verification:** every headline claim below was re-checked by regex across all 3 runs — the run that's quoted is representative, not cherry-picked.
- **Deliberately ignored:** the `judgments_v5/`, `judgments_q8/` LLM-judge outputs and the aggregate scores. Verdicts come from comparing actual answers to owner-verified ground truth.

---

## Per-dimension verdict

| Dimension | A (Qwen3-VL-8B Q8) | B (Qwen3.5-9B Q4 no-think) | Edge |
|---|---|---|---|
| OCR — Latin/UI text | Strong; nails *"VIC Health Club"* | Strong reader but **fabricates** *"vivo health hub"* (3/3) | **A** |
| OCR — CJK / French | Reads 茶香四溢滿屋, 少林寺, 大新銀行, *musique à volonté* | Same — reads them just as well | **Tie** |
| Hallucination discipline | Honest about illegible text; few fabrications | Frequent confident fabrications (logo, signs, signature) | **A** |
| Counting / quantitative | Correct 10 swatches; says 8 on banner (✗) | Correct 9 on banner; says 9 swatches (✗) | **Tie*** |
| Spatial / relational | Correct laterality + "concentric, not spiral" on trading card | Slides into "spiral" (2/3 runs); vague on left/right | **A** |
| Detail / completeness | Concise, clean | More verbose, more structure, captures more surface detail | **B** |
| Honesty about uncertainty | "not clearly legible", "partially obscured" | Confidently invents where it can't read | **A** |
| Medical imaging | Misses the rib fracture; no false finding | Misses the rib fracture; no false finding | **Tie** |
| Failure-mode robustness | 1 token-cap runaway (spritesheet) | 1 token-cap runaway (Waldo) | **Tie** |
| Run-to-run consistency | Very stable | Stable, except high variance in *fabricated* details | **A (slight)** |

\* Counting is 1–1 on decisive tasks (A: swatch ✓/banner ✗; B: banner ✓/swatch ✗; both ✗ on manga panels), so functionally a tie — but A's misses are "wrong number," B's include self-contradiction.

---

## Curated examples (evidence)

### A wins

**1. `17-logo-vic-health-club.png` — OCR accuracy (decisive).** Text is *"VIC Health Club"*.
- **A (all 3 runs):** *"Vic Health Club"* — **correct.**
- **B (all 3 runs):** *"vivo health hub"* — **confidently, consistently wrong** (it even "re-examines" and doubles down on lowercase *"vivo health hub"*).

**2. `06-trading-card.jpg` — spatial reasoning.** Truth: left half yellow + red ghost, right half red + yellow Pac-Man, **cyan concentric circles** (not spiral).
- **A (all 3 runs):** *"left side of the yin-yang is yellow, and contains a red ghost… right side is red, and contains a yellow Pacman"* + *"concentric circles"* — **fully correct.**
- **B (2/3 runs):** *"hypnotic spiral background"* — the flagged error; also vague on which color is left/right.

**3. `29-painting-hundertwasser.jpeg` — honesty vs fabrication.** Signature is a red/pink box, illegible except **1965**.
- **A:** *"a small signature… not clearly legible in the image"* — honest; otherwise captures the face, teardrop windows, onion domes accurately.
- **B (2/3 runs):** invents *"© 2016"* then *"D. K. 2005"* — **wrong year, different every run.**

**4. `21-motion-blur.jpg` (Hong Kong) — discipline under blur.** Real signs: 大新銀行, DVFX.
- **A:** reads 大新銀行; on the blurred sign writes *"DWF"* and flags the rest as *"partially obscured or blurred"* — measured.
- **B:** reads 大新銀行; misreads DVFX as *"DVPK"*, and in one run **invents** *"Carmen"* and *"HSBC"*.

### B wins

**5. `20-ultrawide-kung-fu-banner-with-logo.png` — people counting.** 9 performers (2 unarmed monks + 7 disciples).
- **A (all 3 runs):** *"a group of eight martial artists"* — **undercounts (8).**
- **B (all 3 runs):** *"nine martial artists"* — **correct**, and correctly distinguishes the unarmed center monks. (Both still misdescribe the badge as red/gold rather than the actual yellow.)

### Ties / both fail

**6. `28-xray-ribfracture-lowerright.jpg` — medical.** There is a left 10th posterior rib fracture.
- **A:** *"ribs and spine appear to be intact and without obvious fractures."* **B:** *"no fractures or lytic lesions."* **Both miss it** (and, to their credit, neither invents a false finding). Not a differentiator.

**7. `05-cropped-youtube-capture.png` — shared blind spot.** Flag is **Austrian**; both models say **"Canadian flag"** (a known trap). Both otherwise read the Chinese wall text 茶香四溢滿屋 and correctly identify the framed Pope photo — strong and equal.

**8. `12-manga-nausicaa-colour.png` — both miscount panels** (A: 9, B: 10, truth: 11). The differentiator: **B also hallucinates** *"likely Link from The Legend of Zelda"* (it's Nausicaä); A stays grounded.

---

## Variance & failure modes

- **Stability:** both are very consistent run-to-run on content; differences are cosmetic. The exception is that **B's fabricated details vary across runs** (signature 2016 vs 2005; Carmen/HSBC appears in one run only) — a symptom of fabrication rather than perception.
- **Dense-scene runaways:** each model fails to terminate once. **A** `q8-1` on the Bubble-Bobble spritesheet hit the 16,384-token cap (74k chars, **464 s**). **B** `nothink-img21-30-3` on the Where's-Waldo scene did the same (68k chars, **399 s**). Symmetric; both need a hard `max_tokens`/timeout guard on dense images.
- **No crashes / `ok=False`** for either model across all 180 answers.

---

## Recommendation (pure answer quality)

**Pick A — Qwen3-VL-8B @ Q8_0.**

The single most important property for a vision tool is **not fabricating confident falsehoods**, and that is where A is clearly stronger: it read the logo, the trading-card layout, and the Hundertwasser signature honestly, while B invented *"vivo health hub"*, street signs, and a signature year. A is also more precise spatially and more disciplined about saying "illegible" when it can't read something. Its answers are a little shorter, but they are more trustworthy.

**Where B is the better pick:** if you specifically value **maximum descriptive detail and structure** (longer, more sectioned answers, occasionally deeper scene coverage — e.g. it alone got the 9-person count), and you have a downstream step that can verify or tolerate fabrications. B is a legitimate, competitive alternative, not a clearly inferior model.

**Caveats:**
- The margin is meaningful but not large — roughly 5 decisive A-wins to 1 B-win among contested items, with OCR power otherwise equal.
- *Pure-quality verdict only, as requested.* Operationally, A is the lighter/faster config (8B @ Q8, ~40 tok/s, ~12–40 s/image) versus B (9B @ Q4, ~47 tok/s, ~15–56 s/image) — a secondary consideration here, not a driver of the recommendation.
- This head-to-head covers only these two configs. The project's overall top pick remains a different model (Q3.6-27B, no-think); see `BENCHMARK-REPORT-v5.md`.
