#!/usr/bin/env python3
"""Score the refinement benchmark (run_id=refine-*) against REFINE-GROUND-TRUTH.md.

- Verified images (extract-code-test-1, ocr-test-1, ocr-test-2): token-recall vs the
  owner-verified GT, plus the code "hard discriminator" checks.
- Differential images (ocr-test-3/4, ui-test-1/2): no scored accuracy yet — instead
  extract the DISAGREEMENT tokens (present in some variants but not all) for owner
  adjudication.
- Reliability: per-variant success rate (timeouts penalized).

Run from benchmark/vlm/ :  python3 code/score_refine.py
"""
import json
import re
from collections import defaultdict
from pathlib import Path

RAW = Path("benchmark-results/raw.jsonl")
GT = Path("test-images/REFINE-GROUND-TRUTH.md")

VERIFIED = ["extract-code-test-1.png", "ocr-test-1.jpg", "ocr-test-2.jpg"]
DIFFERENTIAL = ["ocr-test-3.jpg", "ocr-test-4.png", "ui-test-1.png", "ui-test-2.png"]
SHORT = {"qwen3-vl-8b-Q8": "Q3VL-8B-Q8", "qwen3.6-27b": "Q3.6-27B", "qwen3.5-4b": "Q3.5-4B",
         "qwen3.5-9b": "Q3.5-9B", "glm-4.6v-flash-9b": "GLM-9B", "glm-4.6v-flash-9b-Q8": "GLM-9B-Q8"}
WORD = re.compile(r"[A-Za-z0-9_'.À-ɏ]+")  # incl. accented + _ ' . for identifiers


def variant_key(r):
    return r["model"] + ("|nothink" if r.get("thinking_disabled") else "|think")


def short(v):
    m, mode = v.rsplit("|", 1)
    base = SHORT.get(m, m)
    return f"{base}-nothink" if mode == "nothink" and ("qwen3.5" in m or "qwen3.6" in m) else base


def parse_gt():
    """Return {image_filename: gt_text} for fenced code blocks under each ## header."""
    txt = GT.read_text()
    gt = {}
    for m in re.finditer(r"^## (\S+[.](?:png|jpg|jpeg)).*?\n.*?```\w*\n(.*?)```", txt, re.S | re.M):
        gt[m.group(1)] = m.group(2)
    return gt


def tokens(s):
    return set(t.lower() for t in WORD.findall(s))


def load_refine():
    cells = defaultdict(lambda: defaultdict(list))  # variant -> image -> [contents]
    rel = defaultdict(lambda: {"ok": 0, "fail": 0})
    for line in open(RAW):
        r = json.loads(line)
        if not str(r.get("run_id", "")).startswith("refine"):
            continue
        v = variant_key(r)
        rel[v]["ok" if r.get("ok") else "fail"] += 1
        if r.get("ok"):
            cells[v][r["image"]].append(r.get("content", ""))
    return cells, rel


def recall(output, gt_text):
    gt_tok = tokens(gt_text)
    out_tok = tokens(output)
    if not gt_tok:
        return 0.0
    return len(gt_tok & out_tok) / len(gt_tok)


def main():
    cells, rel = load_refine()
    gt = parse_gt()
    variants = sorted(cells)

    print("<!-- Refinement scoring: verified images = token recall vs owner-verified GT; "
          "differential images = disagreements for adjudication; reliability = run success rate. -->\n")

    # ---- Reliability ----
    print("## Reliability (refine runs)\n")
    print("| Variant | ok | fail | rate |")
    print("|---|---|---|---|")
    for v in variants:
        o, f = rel[v]["ok"], rel[v]["fail"]
        tot = o + f
        print(f"| {short(v)} | {o} | {f} | {o/tot*100:.0f}% |" if tot else f"| {short(v)} | 0 | 0 | — |")
    print()

    # ---- Verified images: token recall ----
    print("## Verified images — token recall vs GT (mean over runs)\n")
    print("| Variant | " + " | ".join(VERIFIED) + " |")
    print("|---|" + "---|" * len(VERIFIED))
    for v in variants:
        row = [short(v)]
        for img in VERIFIED:
            outs = cells[v].get(img, [])
            row.append(f"{(sum(recall(o, gt.get(img,'')) for o in outs)/len(outs)*100):.0f}" if outs else "—")
        print("| " + " | ".join(row) + " |")
    print()

    # ---- Code hard discriminators (extract-code-test-1) ----
    print("## extract-code-test-1 — hard discriminators (run 1)\n")
    print("Correct: `createAujourdhui` (no apostrophe), `validatePlaylistKeys_`, `Combiner.push`, escaped `d\\'aujourd\\'hui`.\n")
    print("| Variant | fn | validate_ | Combiner | escaped | lang |")
    print("|---|---|---|---|---|---|")
    for v in variants:
        outs = cells[v].get("extract-code-test-1.png", [])
        c = outs[0] if outs else ""
        fn = ("createAujourdhui" in c) and ("createAujourd'hui" not in c)
        vk = "validatePlaylistKeys_" in c
        comb = "Combiner.push" in c
        esc = "d\\'aujourd\\'hui" in c
        lang = ("javascript" in c.lower()) or ("apps script" in c.lower())
        print(f"| {short(v)} | {'✓' if fn else '✗'} | {'✓' if vk else '✗'} | {'✓' if comb else '✗'} | {'✓' if esc else '✗'} | {'✓' if lang else '✗'} |")
    print()

    # ---- Differential images: disagreement tokens ----
    print("## Differential images — tokens where variants disagree (for owner adjudication)\n")
    print("(For each image: tokens present in some but not all variants' run-1 outputs. "
          "Owner adjudicates the disputed ones.)\n")
    for img in DIFFERENTIAL:
        per_var = {}
        for v in variants:
            outs = cells[v].get(img, [])
            if outs:
                per_var[v] = tokens(outs[0])
        if not per_var:
            print(f"### {img}\n_(no outputs yet)_\n")
            continue
        alltok = set().union(*per_var.values())
        # tokens not unanimous (present in some, absent in others)
        disputed = sorted(t for t in alltok if any(t in s for s in per_var.values()) and not all(t in s for s in per_var.values()))
        print(f"### {img} — {len(disputed)} disputed tokens (of {len(alltok)} unique)")
        # group: for each disputed token, which variants have it
        rows = []
        for t in disputed:
            have = [short(v) for v, s in per_var.items() if t in s]
            miss = [short(v) for v, s in per_var.items() if t not in s]
            rows.append((t, have, miss))
        # show up to 40 most "split" (closest to 50/50)
        rows.sort(key=lambda r: abs(len(r[1]) - len(r[2])))
        if rows:
            print("| token | have | missing |")
            print("|---|---|---|")
            for t, have, miss in rows[:40]:
                print(f"| `{t}` | {', '.join(have) or '—'} | {', '.join(miss) or '—'} |")
        print()


if __name__ == "__main__":
    main()
