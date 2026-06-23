# Head-to-Head: Qwen3-VL-8B @ Q8_0 vs Qwen3.6-27B @ Q4_K_M (no-think)

**Date:** 2026-06-22 · **Basis:** actual model answers in `benchmark-results/raw.jsonl`, judged against `test-images/GROUND-TRUTH.md` — **not** the v5/Q8 aggregate scores.
Companion reports: [vs Qwen3.5-9B](./HEAD-TO-HEAD-qwen3vl-8b-Q8_vs_qwen3.5-9b-Q4-nothink.md) · [vs Qwen3.5-4B](./HEAD-TO-HEAD-qwen3vl-8b-Q8_vs_qwen3.5-4b-Q4-nothink.md) (same method).

| | **A — Qwen3-VL-8B (Q8_0)** | **B — Qwen3.6-27B (Q4_K_M, `--disable-thinking`)** |
|---|---|---|
| Runs read | `q8-1/2/3` (3 × 30 images) | `nothink-1/2/3` + `nothink-img21-30-1/2/3` (3 × 30 images) |
| Avg answer length | ~2,500–3,800 chars (concise) | ~3,000–5,300 chars (detailed) |
| Throughput (context only) | ~37–41 tok/s · ~10–46 s/image | ~16 tok/s · ~50–120 s/image (~3–4× slower) |

## TL;DR

**Winner on pure answer quality: B (Qwen3.6-27B @ Q4, no-think)** — and this **vindicates the benchmark metrics**, which rank `Q3.6-27B-nothink` #1 (~79.6/100).

Unlike the 4B and 9B — both of which *lost* to the 8B-VL — the 27B genuinely beats it on the hardest probes. It is the **only model that correctly counts the manga page (11 panels)**, the **only one that stays stable on the dense spritesheet** (the 8B-VL has a 464 s token-cap runaway there), and the **only one that names the Waldo scene** ("Where's Waldo?"). It also reads the VIC logo correctly (the 4B/9B both mangled it) and matches the 8B-VL's honesty on the illegible Hundertwasser signature. The 8B-VL's only real edges are minor — slightly fuller spatial detail on the trading card and slightly more consistent Pope identification — plus, outside the quality verdict, it is ~3–4× faster.

**Bottom line:** on the criterion you chose (pure answer quality), the 27B wins. The quality ranking by inspection is **27B > 8B-VL > {9B, 4B}**, exactly matching the metrics. The 27B's edge concentrates on the tasks where parameters buy real capability: precise counting, dense-scene robustness, and recognition/identification.

## Method

- Extracted every `content` answer for both models across all 3 runs (30 × 3 = 90 answers each).
- **Pass 1:** read a compact fingerprint of all 180 answers for substance + variance.
- **Pass 2:** full-read the decision-critical images (same probe set used in the 4B/9B reports).
- **Verification:** every headline claim re-checked by regex across **all 3 runs**.
- **Deliberately ignored:** the `judgments_*` outputs and aggregate scores. Verdicts come from actual answers vs owner-verified ground truth.
- **Scope (image only):** these verdicts cover static images only. Both models also support **video** natively (neither supports audio); the harness did not exercise video. See `BENCHMARK-REPORT-v5.md` § *Model specs & media support* for the full media matrix and mmproj sizes.

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
- **B:** *"11 panels"* (runs 1 & 3) / *"10 panels"* (run 2) — **correct in 2/3.** It's the only model in the whole series to count 11. It also reads the Japanese SFX (*"ボム — Explosion sound effect, fourth row left panel"*) and places the explosion — without the 9B's *"Link from Zelda"* hallucination.

**2. `22-spritesheet-bubble-bobble.png` — dense-scene robustness.**
- **A `q8-1`:** hits the 16,384-token cap — **74,757 chars, 464 s**, a runaway (its other two runs are normal).
- **B:** **zero runaways** across all 3 runs, and correctly transcribes the credit: *"Sprites made by 125scratch… Bubble Bobble belongs to Taito."* Stable + accurate where A fails to terminate.

**3. `30-where-is-waldo.webp` — recognition.** Truth: authentic *"Where's Waldo?"* by Martin Handford.
- **A:** describes a *"find the differences / spot the object"* scene — **never names the property.**
- **B (all 3 runs):** *"reminiscent of a 'Where's Waldo?' puzzle"* — **correctly identifies it** (and, unlike the 4B, does **not** misattribute it to Dick Bruna).

**4. `19-watercolour-painting-surfers-fuerteventura.png` — spatial detail.**
- **B:** captures the board direction (*"points to the right"* / goofy-footed) and the observer's wetsuit — a ground-truth-verified detail.
- **A:** gets the two surfers and brown hair right but **omits board direction.** (Both models read the neon sky as "aurora" — a shared interpretation, not a differentiator.)

### Ties (both correct — and notably better than the 4B/9B)

**5. `17-logo-vic-health-club.png`.** Both read *"VIC Health Club"* correctly. (The 4B read "WIC", the 9B "vivo health hub" — the 27B doesn't share that failure.)

**6. `29-painting-hundertwasser.jpeg`.** Both capture the embedded face, blue teardrop windows, and onion domes, and both stay honest about the illegible signature. (The 4B missed the face and fabricated *"© 2015"*.)

**7. `28-xray-ribfracture-lowerright.jpg` — medical (both fail).** There is a left 10th posterior rib fracture. Both report *"no obvious fractures"* / normal anatomy — and neither invents a false finding. Not a differentiator.

### A wins (slight)

**8. `06-trading-card.jpg` — spatial completeness.** A anchors the full yin-yang layout (*"left side is yellow + red ghost; right side is red + yellow Pacman"*) in 2/3 runs. B is correct ("concentric", no "spiral" error) but its answers here are unusually terse (~1,300 chars) and don't spell out the laterality.

## Variance & failure modes

- **Stability:** both are very consistent run-to-run. The one structural difference: **B has zero token-cap runaways across all 90 answers; A has one** (the spritesheet). This is the same failure that also hit the 4B and 9B on dense images — the 27B uniquely avoids it.
- **B's one inconsistency:** the banner count flips between 8 and 9 across runs (correct only 1/3). A is consistently wrong (8) there.
- **No crashes / `ok=False`** for either model across all 180 answers.

## Recommendation (pure answer quality)

**Pick B — Qwen3.6-27B @ Q4, no-think.**

On pure answer quality it is the strongest model in the series: it counts the manga panels, survives the dense spritesheet, names the Waldo scene, and matches the 8B-VL on OCR and honesty — while the 8B-VL's only counters are minor completeness details. This is the one head-to-head where the metrics champion also wins on inspection, and its advantages land exactly where extra parameters help (counting, dense scenes, recognition).

**Where A is the better pick:** if **latency or footprint matters at all**, the picture inverts in practice. A (8B @ Q8) runs ~3–4× faster (~40 vs ~16 tok/s; ~10–46 s vs ~50–120 s per image) and is "good enough" on the majority of images. You explicitly set efficiency aside for this verdict — but for the actual local vision-MCP use case, that trade-off is the real decision, and the 8B-VL is the pragmatic default unless you specifically need the 27B's edge on the hardest images.

**Caveats:**
- The margin is driven by a handful of hard images (panels, spritesheet, Waldo); on most of the 30, the two are close.
- *Pure-quality verdict only, as requested.* Efficiency heavily favours A (see above).
- **Cross-series synthesis:** by actual-answer inspection, **27B > 8B-VL > {9B, 4B}**, and between the two Qwen3.5 nothink models the 4B is the more honest. This ordering matches the benchmark metrics — lending independent support to `Q3.6-27B-nothink` as the project's top pick, while the 8B-VL is the best *efficient* alternative.
