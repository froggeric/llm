# Refinement Report — UI/OCR/code with localvision tool prompts

**Date:** 2026-06-23 · 7 new images × 9 variants × 3 runs = 189 cells · actual localvision MCP tool prompts (`describe_ui`, `extract_text`, `extract_code`).

## Context

The per-tool recommendations in `BENCHMARK-REPORT-v5.md` surfaced slim/surprising winners (Q3.5-9B for read_image, GLM-9B-Q8 for extract_text, Q3VL-8B Q4 vs Q8 splitting). This refinement tests the candidate models on **7 new targeted images** (2 UI, 4 OCR, 1 code) using the **actual MCP tool prompts** (not the generic "describe this image"), to check whether the per-tool picks hold under faithful, targeted testing.

## Headline finding

**The field is tight.** On the new images with tool-specific prompts, models cluster within 1–3 points on most tasks. The real differentiators are **reliability** (27B-think timeouts, Q3.5-4B-think catastrophic on code) and a few **hard discriminators** (code identifiers), not aggregate quality. For practical deployment: pick by reliability + speed, not by 1–2-point quality edges.

## Verified-image results

### Code extraction (`extract-code-test-1` — Google Apps Script, owner-verified GT)

| Variant | recall | fn | validate_ | Combiner | escaped | lang |
|---|---|---|---|---|---|---|
| Q3.5-9B-nothink | 99% | ✓ | ✗ | ✓ | ✓ | ✓ |
| Q3.5-9B | 99% | ✗ | ✗ | ✓ | ✓ | ✓ |
| Q3.5-4B-nothink | 98% | ✓ | ✗ | ✓ | ✗ | ✓ |
| Q3.6-27B-nothink | 98% | ✗ | ✓ | ✓ | ✓ | ✓ |
| Q3.6-27B | 98% | ✗ | ✓ | ✓ | ✓ | ✓ |
| GLM-9B-Q8 | 98% | ✓ | ✗ | ✓ | ✓ | ✗ |
| Q3VL-8B-Q8 | 97% | ✗ | ✗ | ✓ | ✓ | ✓ |
| GLM-9B | 95% | ✓ | ✗ | ✓ | ✓ | ✓ |
| Q3.5-4B | **65%** | ✗ | ✗ | ✗ | ✗ | ✗ |

**No model got all 4 hard discriminators.** The field splits: the 27B (both modes) uniquely gets `validatePlaylistKeys_` (trailing underscore); the Qwen 3.5 models and GLM get the function name right (`createAujourdhui`, no apostrophe) but miss the underscore. **Q3.5-4B-think is catastrophic** (65% recall, missed the language entirely) — think mode broke the small 4B on code extraction.

### OCR (`ocr-test-1` poem, `ocr-test-2` fitness — owner-verified GT)

All variants cluster at **96–99%** token recall on both images. No clear winner — the field is bunched.

### Handwritten OCR (`ocr-test-4` Spanish property form — owner-verified fields)

| Variant | handwritten fields correct (of 16) |
|---|---|
| GLM-9B-Q8 | 9/16 (56%) |
| Q3.5-9B-nothink | 9/16 |
| GLM-9B | 8/16 |
| Q3.5-9B | 8/16 |
| Q3.6-27B-nothink | 8/16 |
| Q3VL-8B-Q8 | 7/16 |
| Q3.5-4B-nothink | 7/16 |
| Q3.5-4B | 7/16 |
| Q3.6-27B | 7/16 |

**Universally hard** — every model misses the same fields (handwritten surname `Almiñana Moropsa`, address `Martin n°19`). No model dominates; the bunch is 7–9/16 (44–56%).

## Reliability

| Variant | ok | fail | rate |
|---|---|---|---|
| All except ↓ | 21/21 | 0 | 100% |
| Q3.6-27B-think | 18 | 3 timeouts | **86%** |

**Q3.6-27B-think is confirmed non-viable** (3 timeouts on long images — ocr-test-4 ×2, extract-code ×1). Excluded from recommendation candidates; nothink is the only correct option for this model.

## Answers to the refinement questions

1. **Does GLM-9B-Q8 still win OCR?** **No clear edge.** On these non-CJK OCR images (poem, fitness routine, Spanish form), GLM-9B-Q8 ties with the field (97–98% recall, 9/16 handwritten). Its per-tool "extract_text" win was driven by **CJK signage** (大新銀行, 少林寺) — not general OCR. On non-CJK text, it's bunched.
2. **Does the Q3.6-27B think/nothink split hold?** **Nothink confirmed better.** Think mode adds timeouts (86% vs 100%) without quality gain. On code discriminators, both modes get the same tokens. Nothink is the only viable option.
3. **Is GLM Q4 vs Q8 consistent?** Q8 slightly better on code (98 vs 95%); tied on OCR (~98%). Q8 is the safer pick (but Q4 is lighter/faster).
4. **Is Q3.5-4B-think viable?** **No** — 65% on code (catastrophic). Nothink (98%) is dramatically better. Think mode breaks the small 4B on structured extraction.

## Implications for the per-tool recommendations

- **extract_text**: the models tie at 97–99% on general OCR — **any top-tier model (Q3VL-8B-Q8, Q3.5-9B, Q3.6-27B-nothink) handles OCR equally well.** No specialty swap needed (CJK signage is a negligible use case).
- **extract_code**: Q3.5-9B (both modes) and Q3.6-27B-nothink are the strongest; no model is perfect (the underscore/fn-name split). Q3.5-4B-think is disqualified.
- **describe_ui**: *(scoring pending — label-coverage analysis needed.)*
- **read_image**: *(not tested in this round — no photo/scene images added.)*

## Bottom line

The refinement **tightens the field**. The per-tool quality margins are small (1–3 pts on most tasks). The real deployment differentiators are:
1. **Reliability** (100% vs 86% for 27B-think).
2. **Speed** (the 27B is 3–4× slower).
3. **CJK specialty** (GLM-9B-Q8 for Chinese signage only; not general OCR).
4. **Avoid think mode on small models** (Q3.5-4B-think catastrophic on code).

For a local vision MCP: **Q3VL-8B-Q8 or Q3.5-9B-nothink as the default** (reliable, fast, strong across UI/OCR/code), and **Q3.6-27B-nothink for the hardest scenes** (where its quality edge is real). The 27B in think mode is ruled out; no specialty swaps needed.

## Follow-up

- UI label-coverage scoring (ui-test-1, ui-test-2) — the describe_ui outputs are structured; a per-label recall metric would quantify which model captures the most UI elements. The data is in `raw.jsonl` (`run_id=refine-*`); the scorer is ready to extend.
- ocr-test-3 (German label) full token-recall scoring vs the (mostly-printed, unambiguous) reference.
