#!/usr/bin/env python3
"""Score the multi-sampling experiment (run_id=repeat-*) — code/benchmark_llamaserver.py --repeat N.

Two questions:
  (1) LATENCY: are calls 2..N cheaper than call 1 once the model is loaded + warm?
      (per image: call-1 elapsed vs median of warm calls 2..N, same for tok/s)
  (2) QUALITY: does correlating N identical runs beat a single run?
      single = mean per-run recall (what one random run gets)
      maj@k  = recall of the per-label/token MAJORITY vote over the first k runs
      union@k (a.k.a. pass@k) = recall of the UNION over the first k runs (≥1 run has it)
      We derive 3x/4x/5x from prefixes of the 5 repeats.

Quality targets:
  - ui-test-1 / ui-test-2 : UI LABEL recall vs a curated verbatim label set (describe_ui).
  - extract-code-test-1   : code discriminator hits + token recall vs owner-verified GT.

Run from benchmark/vlm/ :  python3 code/score_repeat.py
"""
import json
import re
import statistics
from collections import defaultdict
from pathlib import Path

RAW = Path("benchmark-results/raw.jsonl")
GT = Path("test-images/REFINE-GROUND-TRUTH.md")
WORD = re.compile(r"[A-Za-z0-9_'.À-ɏ]+")

SHORT = {"qwen3.5-4b": "Q3.5-4B-nt", "qwen3-vl-8b": "Q3VL-8B-Q8",
         "qwen3.6-27b": "Q3.6-27B-nt", "qwen3.5-9b": "Q3.5-9B-nt"}

# Curated verbatim UI labels (from REFINE-GROUND-TRUTH.md). Matched case-insensitively
# as substrings. These are the coverage target — "did the model surface this element?"
UI1 = ["New Chat", "New Projects", "Search", "Hub", "Settings", "Chats", "INTEGRATIONS",
       "Experimental", "MCP Servers", "Claude Code", "MODEL PROVIDERS", "Llama.cpp", "MLX",
       "OpenAI", "Azure", "Anthropic", "OpenRouter", "Mistral", "Groq", "vAI", "General",
       "App Version", "v0.8.2", "Automatic Update Check", "Check for Updates", "Language",
       "English", "Data Folder", "App Data", "Change Location", "App Logs", "Show in Finder",
       "Open Logs", "Advanced", "Jan CLI", "Uninstall", "Reset To Factory Settings"]
UI2 = ["Default Project", "Projects", "Active", "Estimated", "Settings", "Staging", "Results",
       "History", "20 edits found", "4K", "1:1", "FAILED", "Queue", "Show Logs", "Issues",
       "Clear All", "Unknown error", "Queue finished with issues", "Hide Queue", "Image",
       "Text", "Generate Image", "VARIATIONS", "OUTPUT LOCATION", "Downloads", "PROMPT",
       "ASPECT RATIO", "Auto Mode", "SIZE", "BATCH TIER", "50% cost savings", "MULTI-INPUT MODE",
       "Merge all to 1 output", "PROJECTED COST", "Standard tier", "gemini-3-pro-image-preview"]

# Code hard discriminators (owner-verified).
CODE_DISC = [
    ("createAujourdhui (no apos)", lambda c: "createAujourdhui" in c and "createAujourd'hui" not in c),
    ("validatePlaylistKeys_", lambda c: "validatePlaylistKeys_" in c),
    ("Combiner.push", lambda c: "Combiner.push" in c),
    ("d\\'aujourd\\'hui (escaped)", lambda c: "d\\'aujourd\\'hui" in c),
]
UI_LABELS = {"ui-test-1.png": UI1, "ui-test-2.png": UI2}


def tokens(s):
    return set(t.lower() for t in WORD.findall(s))


def code_gt():
    """Owner-verified code GT from REFINE-GROUND-TRUTH.md fenced block."""
    txt = GT.read_text()
    m = re.search(r"^## extract-code-test-1\.png.*?```\w*\n(.*?)```", txt, re.S | re.M)
    return m.group(1) if m else ""


def load_repeat():
    """(model, image, temp) -> list[(repeat_idx, content, elapsed_s, tps)] sorted by idx."""
    cells = defaultdict(list)
    for line in open(RAW):
        try:
            r = json.loads(line)
        except json.JSONDecodeError:
            continue
        if r.get("type") != "result" or not str(r.get("run_id", "")).startswith("repeat"):
            continue
        if "repeat_idx" not in r:  # only --repeat rows
            continue
        key = (r["model"], r["image"], r.get("temperature", 0.1))
        cells[key].append((r["repeat_idx"], r.get("content", "") if r.get("ok") else "",
                           r.get("elapsed_s", 0), r.get("tokens_per_s", 0)))
    for k in cells:
        cells[k].sort()
    return cells


def med(xs):
    xs = [x for x in xs if x]
    return statistics.median(xs) if xs else 0.0


def label_hits(label, contents):
    """list of bool: is `label` present in each content?"""
    return [(label.lower() in (c or "").lower()) for c in contents]


def recall_over(contents, labels):
    """recall = fraction of labels present in the UNION of given contents."""
    joined = " ".join(contents).lower()
    return sum(1 for lab in labels if lab.lower() in joined) / len(labels) if labels else 0.0


def main():
    cells = load_repeat()
    if not cells:
        print("No repeat-* rows found. Run code/run_repeat.sh first.")
        return
    gt_code = code_gt()

    print("# Multi-sampling experiment — scoring (run_id=repeat-*)\n")
    print("> N = back-to-back identical calls per image in ONE warmed llama-server session.\n")

    # ---------------- LATENCY ----------------
    print("## 1. Latency — call 1 vs warm calls 2..N (the \"already loaded\" speedup)\n")
    print("> per (model × image): wall-time and tok/s of the first call vs the median of the rest.\n")
    print("| Model | Image | call1 s | warm med s | speedup | call1 tok/s | warm tok/s |")
    print("|---|---|---:|---:|---:|---:|---:|")
    for (m, img, temp), runs in sorted(cells.items()):
        elapsed = [e for _, _, e, _ in runs]
        tps = [t for _, _, _, t in runs]
        c1 = elapsed[0] if elapsed else 0
        warm = med(elapsed[1:])
        speedup = (c1 / warm) if warm else 0
        print(f"| {SHORT.get(m,m)} | {img} | {c1:.1f} | {warm:.1f} | {speedup:.2f}× | "
              f"{tps[0] if tps else 0:.1f} | {med(tps[1:]):.1f} |")
    print()

    # ---------------- QUALITY: UI labels ----------------
    print("## 2. Quality — does correlating N runs beat a single run?\n")
    print("> **single** = mean per-run label recall (one random run). "
          "**maj@k** = recall of the per-label majority vote over the first k runs. "
          "**union@k (pass@k)** = recall of the union over the first k runs (≥1 run has it).\n")
    print("### UI screenshots (high run-variance → multi-sampling should help)\n")
    print("| Model | Image | labels | single | maj@3 | maj@4 | maj@5 | union@3 | union@4 | union@5 | agree% |")
    print("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
    ui_rows = []
    for img in ("ui-test-1.png", "ui-test-2.png"):
        for (m, im, temp), runs in sorted(cells.items()):
            if im != img:
                continue
            labels = UI_LABELS[img]
            contents = [c for _, c, _, _ in runs]
            n = len(contents)
            single = sum(sum(label_hits(lab, [c])[0] for lab in labels) / len(labels)
                         for c in contents) / n if n else 0.0
            def maj_at(k):
                hit = 0
                for lab in labels:
                    h = label_hits(lab, contents[:k])
                    hit += 1 if sum(h) >= (k // 2 + 1) else 0
                return hit / len(labels)
            union_at = lambda k: recall_over(contents[:k], labels)
            agree = (sum(1 for lab in labels if len(set(label_hits(lab, contents))) == 1)
                     / len(labels))
            print(f"| {SHORT.get(m,m)} | {img} | {len(labels)} | {single*100:.0f}% | "
                  f"{maj_at(3)*100:.0f}% | {maj_at(4)*100:.0f}% | {maj_at(5)*100:.0f}% | "
                  f"{union_at(3)*100:.0f}% | {union_at(4)*100:.0f}% | {union_at(5)*100:.0f}% | "
                  f"{agree*100:.0f}% |")
    print()

    # ---------------- QUALITY: code ----------------
    print("### extract-code-test-1 (low run-variance CONTRAST — systematic errors)\n")
    print("| Model | single recall | maj@5 recall | union@5 recall | discrim single | discrim maj@5 | discrim union@5 |")
    print("|---|---:|---:|---:|---:|---:|---:|")
    gt_tok = tokens(gt_code)
    for (m, im, temp), runs in sorted(cells.items()):
        if im != "extract-code-test-1.png":
            continue
        contents = [c for _, c, _, _ in runs]
        n = len(contents)
        per_recall = [len(gt_tok & tokens(c)) / len(gt_tok) for c in contents if gt_tok]
        single = sum(per_recall) / len(per_recall) if per_recall else 0.0
        maj_recall = len(gt_tok & tokens(" ".join(
            # majority-vote token presence across runs
            _majority_tokens(contents))) ) / len(gt_tok) if gt_tok else 0.0
        union_recall = len(gt_tok & set().union(*(tokens(c) for c in contents))) / len(gt_tok) if gt_tok else 0.0
        # discriminators
        d_single = sum(sum(fn(c) for c in contents) / n for _, fn in CODE_DISC) / len(CODE_DISC)
        d_maj = sum(1 for _, fn in CODE_DISC
                    if sum(fn(c) for c in contents) >= (n // 2 + 1)) / len(CODE_DISC)
        d_union = sum(1 for _, fn in CODE_DISC if any(fn(c) for c in contents)) / len(CODE_DISC)
        print(f"| {SHORT.get(m,m)} | {single*100:.0f}% | {maj_recall*100:.0f}% | {union_recall*100:.0f}% | "
              f"{d_single*100:.0f}% | {d_maj*100:.0f}% | {d_union*100:.0f}% |")
    print()
    print("> `discrim` = the 4 hard code identifiers (createAujourdhui, validatePlaylistKeys_, "
          "Combiner.push, escaped d\\'aujourd\\'hui). Recall is token-recall vs owner-verified GT.")


def _majority_tokens(contents):
    """Token-level majority vote: keep tokens present in ≥ half the runs."""
    toks_per = [tokens(c) for c in contents]
    n = len(toks_per)
    alltok = set().union(*toks_per)
    return {t for t in alltok if sum(t in s for s in toks_per) >= (n // 2 + 1)}


if __name__ == "__main__":
    main()
