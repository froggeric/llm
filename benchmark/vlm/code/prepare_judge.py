#!/usr/bin/env python3
"""Prepare LLM-judge inputs for the repeat-* and cat-* multi-sampling experiments.

Mirrors the v5 prepare step (score_v5_multirun.py --prepare-median): for each unit
to be judged, write an input_<key>.json with:
    {image, image_path, ground_truth, responses[]}
Each response carries a unique `key` the judge must use as its judgment key, so the
aggregator can map judgments back to (model, rep[, temp]).

  repeat -> benchmark-results/judgments_repeat/input_<image>.json
            (one per image; 5 reps x 2 models = 10 responses, keyed <MODEL>|rep<N>)
  cat    -> benchmark-results/judgments_cat/input_<image>__<model>.json
            (one per image x model; reps x temps, keyed <MODEL>|t<temp>|rep<N>)

Ground truth is loaded from BOTH test-images/GROUND-TRUTH.md and REFINE-GROUND-TRUTH.md
(whichever has the image). webp inputs get a .png copy via sips for the judge to view.

Run from benchmark/vlm/:  python3 code/prepare_judge.py [repeat|cat|both]
"""
import json
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

RAW = Path("benchmark-results/raw.jsonl")
TEST = Path("test-images")
GT_FILES = [TEST / "GROUND-TRUTH.md", TEST / "REFINE-GROUND-TRUTH.md"]
REP_DIR = Path("benchmark-results/judgments_repeat")
CAT_DIR = Path("benchmark-results/judgments_cat")
WEBP_PNG = Path("benchmark-results/_webp_png")
MAXCHAR = 4000

SHORT = {
    "qwen3-vl-8b": "Q3VL-8B-Q8", "qwen3.5-4b": "Q3.5-4B", "qwen3.5-9b": "Q3.5-9B",
    "glm-4.6v-flash-9b": "GLM-9B", "qwen3.5-4b-Q8": "Q3.5-4B-Q8",
    "gemma4-e4b": "G4-E4B", "qwen3.6-35b-a3b": "Q3.6-35B-A3B",
}


def gt_for(image):
    """Owner-verified GT text for `image` from whichever GT file has it (line-by-line,
    mirroring score_v5_multirun.py's parser but matching any extension)."""
    base = image.split("/")[-1]
    head = re.compile(rf"^## +{re.escape(base)}\b")
    for gt_path in GT_FILES:
        if not gt_path.exists():
            continue
        lines = gt_path.read_text().splitlines()
        for i, line in enumerate(lines):
            if head.match(line):
                body = []
                for nxt in lines[i + 1:]:
                    if re.match(r"^## +\S", nxt):
                        break
                    body.append(nxt)
                return "\n".join(body).strip()
    return ""


def image_view_path(image):
    """Path the judge should view; convert webp -> png copy via sips."""
    src = TEST / image
    if src.suffix.lower() == ".webp":
        WEBP_PNG.mkdir(parents=True, exist_ok=True)
        dst = WEBP_PNG / (src.stem + ".png")
        if not dst.exists():
            subprocess.run(
                ["sips", "-s", "format", "png", str(src), "--out", str(dst)],
                check=True, capture_output=True,
            )
        return dst
    return src


def load_rows(prefix):
    """(model, image, temp) -> sorted list[(repeat_idx, content)] for run_id starting with prefix."""
    cells = defaultdict(list)
    with open(RAW) as f:
        for line in f:
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            if r.get("type") != "result" or not str(r.get("run_id", "")).startswith(prefix):
                continue
            if "repeat_idx" not in r:
                continue
            cells[(r["model"], r["image"], r.get("temperature", 0.1))].append(
                (r["repeat_idx"], r.get("content", "") if r.get("ok") else ""))
    for k in cells:
        cells[k].sort()
    return cells


def write_input(path, image, responses):
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "image": image,
        "image_path": str(image_view_path(image)),
        "ground_truth": gt_for(image),
        "responses": responses,
    }
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False))
    print(f"  {image}: {len(responses):>2} responses -> {path.name}")


def prepare_repeat():
    cells = load_rows("repeat")
    if not cells:
        print("[repeat] no repeat-* rows found"); return 0
    n = 0
    for img in sorted({im for (_, im, _) in cells}):
        responses = []
        for (m, im, _t), runs in sorted(cells.items()):
            if im != img:
                continue
            mk = SHORT.get(m, m)
            for rep, content in runs:
                responses.append({"key": f"{mk}|rep{rep}", "model": mk,
                                  "rep": rep, "content": content[:MAXCHAR]})
        write_input(REP_DIR / f"input_{img.replace('/', '_')}.json", img, responses)
        n += 1
    return n


def prepare_cat():
    cells = load_rows("cat")
    if not cells:
        print("[cat] no cat-* rows found"); return 0
    by_im_model = defaultdict(list)  # (image, model) -> list[(temp, rep, content)]
    for (m, im, t), runs in sorted(cells.items()):
        for rep, content in runs:
            by_im_model[(im, m)].append((t, rep, content))
    n = 0
    for (img, m), entries in sorted(by_im_model.items()):
        mk = SHORT.get(m, m)
        responses = []
        for t, rep, content in sorted(entries):
            responses.append({"key": f"{mk}|t{t:g}|rep{rep}", "model": mk,
                              "temp": t, "rep": rep, "content": content[:MAXCHAR]})
        fname = f"input_{img.replace('/', '_')}__{m}.json"
        write_input(CAT_DIR / fname, img, responses)
        n += 1
    return n


def main():
    which = sys.argv[1] if len(sys.argv) > 1 else "both"
    if which in ("repeat", "both"):
        print(f"=== repeat -> {REP_DIR} ===")
        print(f"  wrote {prepare_repeat()} input files")
    if which in ("cat", "both"):
        print(f"=== cat -> {CAT_DIR} ===")
        print(f"  wrote {prepare_cat()} input files")


if __name__ == "__main__":
    main()
