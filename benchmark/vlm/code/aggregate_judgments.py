#!/usr/bin/env python3
"""Aggregate LLM-judge outputs for the repeat-* and cat-* experiments.

Reads judgment_*.json from benchmark-results/judgments_repeat/ and judgments_cat/,
normalizes (holistic_score -> 0-10; missing hallucinations -> []), and prints:
  repeat: per (model x image) holistic mean +/- std across reps + hallucination count
  cat:    per (model x category x temp) holistic mean + hallucination count

Keys are encoded by prepare_judge.py as:
  repeat:  "<MODEL>|rep<N>"            (file = judgment_<image>.json)
  cat:     "<MODEL>|t<temp>|rep<N>"    (file = judgment_<image>__<model>.json)

Run from benchmark/vlm/:  python3 code/aggregate_judgments.py [repeat|cat|both]
"""
import json
import re
import statistics
import sys
from collections import defaultdict
from pathlib import Path

REP_DIR = Path("benchmark-results/judgments_repeat")
CAT_DIR = Path("benchmark-results/judgments_cat")

# image -> localvision category (from score_category.py)
IMG_CAT = {
    "30-where-is-waldo.webp": "read_image", "ocr-test-4.png": "extract_text",
    "extract-code-test-1.png": "extract_code", "08-poster-class-schedule.jpg": "extract_table",
    "ui-test-1.png": "describe_ui", "04_architecture.png": "describe_diagram",
    "26-graph-ocean-acidification-hawaii.jpg": "describe_chart", "03_error_trace.png": "diagnose_error",
}
CAT_ORDER = ["read_image", "extract_text", "extract_code", "extract_table",
             "describe_ui", "describe_diagram", "describe_chart", "diagnose_error"]


def norm_judgment(j):
    """Normalize one entry (mirror score_v5_multirun.py:283-296)."""
    if "holistic_score" not in j:
        j["holistic_score"] = j.get("score") or j.get("judge_score")
    hs = j["holistic_score"]
    if isinstance(hs, str):
        m = re.search(r"\d+(\.\d+)?", hs)
        hs = float(m.group()) if m else None
    if hs is not None and hs > 10:
        hs = hs / 10.0
    j["holistic_score"] = hs
    if "hallucinations" not in j:
        j["hallucinations"] = j.get("hallucination") or []
    return j


def load_dir(d):
    """Return list of (image, {key: judgment}) per judgment file in dir d."""
    out = []
    for f in sorted(d.glob("judgment_*.json")):
        stem = f.stem[len("judgment_"):]
        try:
            data = json.loads(f.read_text())
        except Exception:
            continue
        raw = data.get("judgments") or data.get("evaluations") or {}
        if isinstance(raw, list):
            raw = {(e.get("key") or e.get("model_variant") or e.get("model")): e
                   for e in raw if (e.get("key") or e.get("model"))}
        judgments = {k: norm_judgment(dict(v)) for k, v in raw.items() if k}
        # image is everything before "__" (cat) or the whole stem (repeat)
        image = stem.split("__")[0]
        out.append((image, judgments))
    return out


def parse_repkey(k):
    """'Q3VL-8B-Q8|rep0' -> ('Q3VL-8B-Q8', 0)."""
    m = re.match(r"^(.+)\|rep(\d+)$", k)
    return (m.group(1), int(m.group(2))) if m else (k, None)


def parse_catkey(k):
    """'Q3VL-8B-Q8|t0.7|rep2' -> ('Q3VL-8B-Q8', 0.7, 2)."""
    m = re.match(r"^(.+)\|t([0-9.]+)\|rep(\d+)$", k)
    return (m.group(1), float(m.group(2)), int(m.group(3))) if m else None


def agg_repeat():
    files = load_dir(REP_DIR)
    if not files:
        print("[repeat] no judgments found in", REP_DIR); return
    # (model, image) -> list[historic holistic]; (model, image) -> halluc total
    by_cell = defaultdict(list)
    halluc = defaultdict(int)
    n_keys = 0
    for image, judgments in files:
        for k, j in judgments.items():
            model, rep = parse_repkey(k)
            if j["holistic_score"] is not None:
                by_cell[(model, image)].append(j["holistic_score"])
            halluc[(model, image)] += len(j.get("hallucinations") or [])
            n_keys += 1
    print("# repeat — LLM-judge holistic_score (0-10) + hallucinations\n")
    print(f"> {n_keys} responses judged across {len(files)} images.\n")
    print("| Model | Image | holistic mean | ±std | halluc (total) |")
    print("|---|---|---:|---:|---:|")
    for (model, image) in sorted(by_cell):
        xs = by_cell[(model, image)]
        mean = statistics.mean(xs) if xs else 0
        sd = statistics.pstdev(xs) if len(xs) > 1 else 0
        print(f"| {model} | {image} | {mean:.1f} | ±{sd:.2f} | {halluc[(model,image)]} |")


def agg_cat():
    files = load_dir(CAT_DIR)
    if not files:
        print("[cat] no judgments found in", CAT_DIR); return
    # (category, model, temp) -> list[historic holistic]; halluc per (cat, model, temp)
    by_cell = defaultdict(list)
    halluc = defaultdict(int)
    n_keys = 0
    for image, judgments in files:
        cat = IMG_CAT.get(image, image)
        for k, j in judgments.items():
            parsed = parse_catkey(k)
            if not parsed:
                continue
            model, temp, rep = parsed
            if j["holistic_score"] is not None:
                by_cell[(cat, model, temp)].append(j["holistic_score"])
            halluc[(cat, model, temp)] += len(j.get("hallucinations") or [])
            n_keys += 1
    temps = sorted({t for (_, _, t) in by_cell})
    print("# cat — LLM-judge holistic_score (0-10) by model × category × temp\n")
    print(f"> {n_keys} responses judged. Values are mean holistic across reps.\n")
    thead = "| Category | Model | " + " | ".join(f"t{t:g}" for t in temps) + " |"
    sep = "|---|---|" + "---:|" * len(temps)
    print(thead); print(sep)
    for cat in CAT_ORDER:
        models = sorted({m for (c, m, t) in by_cell if c == cat})
        for model in models:
            cells = []
            for t in temps:
                xs = by_cell.get((cat, model, t), [])
                cells.append(f"{statistics.mean(xs):.1f}" if xs else "—")
            print(f"| {cat} | {model} | " + " | ".join(cells) + " |")
    # hallucination summary
    print("\n## Hallucinations flagged (total, any rep)\n")
    print("| Category | Model | " + " | ".join(f"t{t:g}" for t in temps) + " |")
    print("|---|---|" + "---:|" * len(temps))
    for cat in CAT_ORDER:
        models = sorted({m for (c, m, t) in halluc if c == cat})
        for model in models:
            cells = [str(halluc.get((cat, model, t), 0)) for t in temps]
            print(f"| {cat} | {model} | " + " | ".join(cells) + " |")


def main():
    which = sys.argv[1] if len(sys.argv) > 1 else "both"
    if which in ("repeat", "both"):
        agg_repeat(); print()
    if which in ("cat", "both"):
        agg_cat()


if __name__ == "__main__":
    main()
