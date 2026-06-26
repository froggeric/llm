#!/usr/bin/env python3
"""Dispatch LLM-judge subagents for the multi-run benchmark.

This is a CLI helper that prints judge prompts. The actual dispatch happens via the
Agent tool from the main session — one subagent per input file.

Usage:
  # v5 (legacy):
  python3 dispatch_multirun_judges.py list                  # list v5 images needing judges
  python3 dispatch_multirun_judges.py show <image>          # print the v5 judge prompt for one image
  # generic (repeat/cat — works for ANY input_*.json):
  python3 dispatch_multirun_judges.py prompt <input_file>   # print the judge prompt for that input
"""
import argparse
import json
import sys
from pathlib import Path

JUDGE_DIR = Path("benchmark-results/judgments_v5")

RUBRIC = """You are a vision-LLM judge evaluating VLM responses on a multi-sampling benchmark.

INPUT FILE: {input_file}
IMAGE FILE: {image_file}

CONTEXT: The input file contains:
  - ground_truth: the authoritative owner-verified description
  - responses: a list of model responses, EACH with a `key` field (a unique id like
    "<MODEL>|rep<N>" or "<MODEL>|t<temp>|rep<N>"). Multiple responses may share a model
    (different reps / temperatures) — so you MUST key each judgment by the response's
    exact `key` string, NOT by model name.

STEP 1: Read the input JSON.
STEP 2: View the actual image at the IMAGE FILE path.
STEP 3: For EACH response, evaluate against the image (the image is the ground-truth
        arbiter; the ground_truth text is a reference):
  - **holistic_score (0-10)**: overall quality.
    * 10 = flawless, captures all key details, zero hallucination
    * 8 = solid, captures most key details, minor omissions
    * 5 = partial, missed major elements OR minor hallucination
    * 2 = mostly wrong, repetitive, or severe hallucination
    * 0 = empty, refused, or pure fabrication
  - **failure_mode**: normal / truncated / empty / repetition_loop

CRITICAL JUDGING RULES:
  - Hallucinated content is the most serious failure (-3+ per major hallucination).
  - **What IS a hallucination** (list under `hallucinations`): something the model asserts that is genuinely NOT in the image — an invented object, person, label, source citation, or structure.
  - **What is NOT a hallucination** (list under `key_misses`, NOT `hallucinations`): a *misread* of text that IS visible (wrong digits, misspelled name, wrong domain spelling) — that is an OCR/reading error, not a fabrication.
  - **Confidence threshold — only flag what you can verify is absent.** If you are not sure an element is missing (you can't fully read the image, or the scene is dense), do NOT flag it. "I didn't see it" is NOT "it's not there."
  - **Dense/complex scenes** (crowded illustrations like Where's Waldo, busy screenshots): these contain hundreds of small, varied details. Do NOT flag specific minor characters/objects (e.g. "a person in a crown", "a unicycle", "a red sail") as hallucinations unless they are clearly impossible/absurd — you very likely just can't see them. Restrict hallucination flags to clearly-impossible inventions (text in a text-free image; a wrong artist attribution; an ocean liner where there is open water; a source citation that isn't printed).
  - Legitimate discussion of visible text is NOT hallucination.
  - Wrong counts are real errors (key_misses).
  - Do NOT reward length or formatting alone. A short correct answer beats a long one with errors.
  - Score EACH response independently on its own merits (these are repeat/temperature variants).

STEP 4: Write your output as JSON to:
  {output_file}

OUTPUT FORMAT:
{{
  "image": "{image}",
  "judgments": {{
    "<exact key from the response>": {{
      "holistic_score": <0-10>,
      "failure_mode": "normal|truncated|empty|repetition_loop",
      "key_hits": ["1-3 things right"],
      "key_misses": ["1-3 things wrong/missed"],
      "hallucinations": ["specific fabrications, empty list if none"],
      "justification": "<one sentence>"
    }}
  }},
  "ranking": ["best_key", "second_best_key", "..."],
  "notes": "optional cross-response observations"
}}

IMPORTANT: every response `key` in the input MUST appear in `judgments`. Return a
2-sentence summary. The JSON file is the authoritative output."""


def gen_prompt(input_file):
    """Build the judge prompt for ANY input_*.json (v5, repeat, or cat)."""
    input_path = Path(input_file)
    data = json.loads(input_path.read_text())
    output_file = input_path.parent / input_path.name.replace("input_", "judgment_", 1)
    return RUBRIC.format(
        input_file=input_path,
        image_file=data.get("image_path", ""),
        output_file=output_file,
        image=data.get("image", ""),
    )


def make_prompt(image_name, gt_chars, n_models):
    """Legacy v5 prompt (keyed by model name). Kept for backward compatibility."""
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
  "ranking": ["best_model", "second_best", "..."],
  "notes": "optional cross-model observations"
}}

Return a 2-sentence summary. The JSON file is the authoritative output."""


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd")
    sub.add_parser("list")
    show = sub.add_parser("show")
    show.add_argument("image")
    prompt = sub.add_parser("prompt")
    prompt.add_argument("input_file")
    args = parser.parse_args()

    if args.cmd == "prompt":
        p = Path(args.input_file)
        if not p.exists():
            print(f"Input file not found: {p}", file=sys.stderr)
            sys.exit(1)
        print(gen_prompt(p))
    elif args.cmd == "list":
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
        candidate = JUDGE_DIR / f"input_{args.image.replace('/', '_')}.json"
        if not candidate.exists():
            print(f"Input file not found: {candidate}", file=sys.stderr)
            sys.exit(1)
        data = json.loads(candidate.read_text())
        print(make_prompt(args.image, len(data.get("ground_truth", "")), len(data.get("responses", []))))


if __name__ == "__main__":
    main()
