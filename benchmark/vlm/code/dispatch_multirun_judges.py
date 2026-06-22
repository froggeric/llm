#!/usr/bin/env python3
"""Dispatch LLM-judge subagents for the multi-run benchmark.

This is a CLI helper that prints out the 20 agent prompts to dispatch.
The actual dispatch happens via the Agent tool from the main session.

Usage:
  python3 dispatch_multirun_judges.py list         # list images needing judges
  python3 dispatch_multirun_judges.py show <image>  # print the judge prompt for one image
"""
import argparse
import json
import sys
from pathlib import Path

JUDGE_DIR = Path("benchmark-results/judgments_v5")


def make_prompt(image_name, gt_chars, n_models):
    return f"""You are a vision-LLM judge evaluating VLM responses on a multi-run benchmark.

INPUT FILE: {JUDGE_DIR}/input_{image_name.replace('/', '_')}.json
IMAGE FILE: test-images/{image_name}

CONTEXT: This is the **median run** (most representative of 3 runs) for each of {n_models} models.
The input file contains:
  - ground_truth: the authoritative verified description
  - responses: list of {n_models} median-run responses

STEP 1: Read the input JSON.
STEP 2: View the actual image at the IMAGE FILE path.
STEP 3: For EACH model response, evaluate:
  - **holistic_score (0-10)**: overall quality.
    * 10 = flawless, captures all key details, zero hallucination
    * 8 = solid, captures most key details, minor omissions
    * 5 = partial, missed major elements OR minor hallucination
    * 2 = mostly wrong, repetitive, or severe hallucination
    * 0 = empty, refused, or pure fabrication
  - **failure_mode**: normal / truncated / empty / repetition_loop

CRITICAL JUDGING RULES:
  - Hallucinated content is the most serious failure (-3+ per major hallucination).
  - Legitimate discussion of visible text is NOT hallucination.
  - Wrong counts are real errors.
  - Do NOT reward length or formatting alone. A short correct answer beats a long one with errors.
  - Use the actual image as ground truth arbiter when text is ambiguous.

STEP 4: Write your output as JSON to:
  {JUDGE_DIR}/judgment_{image_name.replace('/', '_')}.json

OUTPUT FORMAT:
{{
  "image": "{image_name}",
  "judgments": {{
    "<model_name>": {{
      "holistic_score": <0-10>,
      "failure_mode": "normal|truncated|empty|repetition_loop",
      "key_hits": ["1-3 things right"],
      "key_misses": ["1-3 things wrong/missed"],
      "hallucinations": ["specific fabrications, empty list if none"],
      "justification": "<one sentence>"
    }}
  }},
  "ranking": ["best_model", "second_best", ...],
  "notes": "optional cross-model observations"
}}

Return a 2-sentence summary. The JSON file is the authoritative output."""


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd")
    sub.add_parser("list")
    show = sub.add_parser("show")
    show.add_argument("image")
    args = parser.parse_args()

    if args.cmd == "list":
        inputs = sorted(JUDGE_DIR.glob("input_*.json"))
        judged = {f.stem[len("judgment_"):] for f in JUDGE_DIR.glob("judgment_*.json")}
        print(f"{'image':<55} {'models':>6} {'status':>10}")
        print("-" * 75)
        for f in inputs:
            image = f.stem[len("input_"):]
            data = json.loads(f.read_text())
            n = len(data.get("responses", []))
            status = "JUDGED" if image in judged else "PENDING"
            print(f"  {image[:53]:<55} {n:>6} {status:>10}")
    elif args.cmd == "show":
        # Find input file
        candidate = JUDGE_DIR / f"input_{args.image.replace('/', '_')}.json"
        if not candidate.exists():
            print(f"Input file not found: {candidate}", file=sys.stderr)
            sys.exit(1)
        data = json.loads(candidate.read_text())
        print(make_prompt(args.image, len(data.get("ground_truth", "")), len(data.get("responses", []))))


if __name__ == "__main__":
    main()
