# Head-to-Head: Qwen3-VL-8B @ Q8_0 vs Qwen3.6-27B @ Q4_K_M (no-think)

**Date:** 2026-06-22 · **Basis:** actual model answers in `benchmark-results/raw.jsonl`, judged against `test-images/GROUND-TRUTH.md` — **not** the v5/Q8 aggregate scores.

| | **A — Qwen3-VL-8B (Q8_0)** | **B — Qwen3.6-27B (Q4_K_M, `--disable-thinking`)** |
|---|---|---|
| Runs read | `q8-1/2/3` (3 × 30 images) | `nothink-1/2/3` + `nothink-img21-30-1/2/3` (3 × 30 images) |
| Avg answer length | ~2,500–3,800 chars (concise) | ~3,000–5,300 chars (detailed) |
| Throughput (context only) | ~37–41 tok/s · ~10–46 s/image | ~16 tok/s · ~50–120 s/image (~3–4× slower) |

## TL;DR

**Winner on pure answer quality: B (Qwen3.6-27B @ Q4, no-think).**

B beats A on the hardest probes. It correctly counts the manga page (**11 panels**, where A says 9), stays stable on the dense spritesheet (A suffers a 464 s token-cap runaway there), and names the Waldo scene (*"Where's Waldo?"* — A describes it generically without naming it). It also reads the VIC logo correctly and matches A's honesty on the illegible Hundertwasser signature. A's only real edges are minor — slightly fuller spatial detail on the trading card and slightly more consistent Pope identification — plus, outside the quality verdict, it is ~3–4× faster.

**Bottom line:** on pure answer quality, B wins, with its edge concentrated where extra parameters buy real capability: precise counting, dense-scene robustness, and recognition/identification.

## Method

- Extracted every `content` answer for both models across all 3 runs (30 × 3 = 90 answers each).
- **Pass 1:** read a compact fingerprint of all 180 answers for substance + variance.
- **Pass 2:** full-read the decision-critical probe images.
- **Verification:** every headline claim re-checked by regex across **all 3 runs**.
- **Deliberately ignored:** the `judgments_*` outputs and aggregate scores. Verdicts come from actual answers vs owner-verified ground truth.
- **Scope (image only):** both models natively support image and video; neither supports audio. This report evaluates static images only — video was not exercised by the harness.

## Per-dimension verdict

| Dimension | A (Qwen3-VL-8B Q8) | B (Qwen3.6-27B Q4 no-think) | Edge |
|---|---|---|---|
| OCR — Latin/UI | *"VIC Health Club"* ✓ | *"VIC Health Club"* ✓ | **Tie** |
| OCR — CJK / French | 茶香四溢滿屋, *musique à volonté* | same — reads them equally | **Tie** |
| Counting / quantitative | manga **9** (✗, truth 11); swatch 10 ✓; banner **8** (✗) | manga **11** ✓ (2/3); swatch 10 ✓; banner 9 (1/3) | **B** |
| Spatial / relational | card laterality ✓ (2/3); omits surf board direction | card sparse but correct; surf **board direction ✓** (2/3) | **B (slight)** |
| Hallucination discipline | honest about illegible text | honest about illegible text | **Tie** |
| Scene / object depth | good | deeper — panel counts, sprite credit, Waldo ID | **B** |
| Dense-scene robustness | **1 token-cap runaway** (spritesheet, 464 s) | **0 runaways**, stable + correct *Taito* attribution | **B** |
| Detail / completeness | concise | more detail, and it's correct | **B** |
| Medical imaging | misses the rib fracture; no false finding | misses the rib fracture; no false finding | **Tie** |
| Run-to-run consistency | very stable | very stable | **Tie** |
| Throughput *(excluded from verdict)* | ~40 tok/s | ~16 tok/s (~3–4× slower) | A |

## Curated examples (evidence)

### B (27B) wins

**1. `12-manga-nausicaa-colour.png` — counting.** Truth: **11 panels**.
- **A (all 3 runs):** *"a grid of nine panels"* — **undercounts (9).**
- **B:** *"11 panels"* (runs 1 & 3) / *"10 panels"* (run 2) — **correct in 2/3.** It also reads the Japanese SFX (*"ボム — Explosion sound effect, fourth row left panel"*) and places the explosion, and stays grounded without any franchise misidentification.

**2. `22-spritesheet-bubble-bobble.png` — dense-scene robustness.**
- **A `q8-1`:** hits the 16,384-token cap — **74,757 chars, 464 s**, a runaway (its other two runs are normal).
- **B:** **zero runaways** across all 3 runs, and correctly transcribes the credit: *"Sprites made by 125scratch… Bubble Bobble belongs to Taito."* Stable + accurate where A fails to terminate.

**3. `30-where-is-waldo.webp` — recognition.** Truth: authentic *"Where's Waldo?"* by Martin Handford.
- **A:** describes a *"find the differences / spot the object"* scene — **never names the property.**
- **B (all 3 runs):** *"reminiscent of a 'Where's Waldo?' puzzle"* — **correctly identifies it**, with no wrong artist attribution.

**4. `19-watercolour-painting-surfers-fuerteventura.png` — spatial detail.**
- **B:** captures the board direction (*"points to the right"* / goofy-footed) and the observer's wetsuit — a ground-truth-verified detail.
- **A:** gets the two surfers and brown hair right but **omits board direction.** (Both models read the neon sky as "aurora" — a shared interpretation, not a differentiator.)

### Ties (both correct)

**5. `17-logo-vic-health-club.png`.** Both read *"VIC Health Club"* correctly.

**6. `29-painting-hundertwasser.jpeg`.** Both capture the embedded face, blue teardrop windows, and onion domes, and both stay honest about the illegible signature.

**7. `28-xray-ribfracture-lowerright.jpg` — medical (both fail).** There is a left 10th posterior rib fracture. Both report *"no obvious fractures"* / normal anatomy — and neither invents a false finding. Not a differentiator.

### A wins (slight)

**8. `06-trading-card.jpg` — spatial completeness.** A anchors the full yin-yang layout (*"left side is yellow + red ghost; right side is red + yellow Pacman"*) in 2/3 runs. B is correct ("concentric", no "spiral" error) but its answers here are unusually terse (~1,300 chars) and don't spell out the laterality.

## Variance & failure modes

- **Stability:** both are very consistent run-to-run. The one structural difference: **B has zero token-cap runaways across all 90 answers; A has one** (the spritesheet). B uniquely avoids that failure mode here.
- **B's one inconsistency:** the banner count flips between 8 and 9 across runs (correct only 1/3). A is consistently wrong (8) there.
- **No crashes / `ok=False`** for either model across all 180 answers.

## Recommendation (pure answer quality)

**Pick B — Qwen3.6-27B @ Q4, no-think.**

On pure answer quality it is the stronger of the two: it counts the manga panels, survives the dense spritesheet, names the Waldo scene, and matches A on OCR and honesty — while A's only counters are minor completeness details. B's advantages land exactly where extra parameters help (counting, dense scenes, recognition).

**Where A is the better pick:** if **latency or footprint matters at all**, the picture inverts in practice. A (8B @ Q8) runs ~3–4× faster (~40 vs ~16 tok/s; ~10–46 s vs ~50–120 s per image) and is "good enough" on the majority of images. You set efficiency aside for this verdict — but for a local deployment, that trade-off is the real decision, and A is the pragmatic default unless you specifically need B's edge on the hardest images.

**Caveats:**
- The margin is driven by a handful of hard images (panels, spritesheet, Waldo); on most of the 30, the two are close.
- *Pure-quality verdict only, as requested.* Efficiency heavily favours A (see above).
- This report covers only these two configs on the 30-image set.
