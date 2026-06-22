#!/usr/bin/env python3
"""Multi-run VLM scorecard v5.

Same aggregation as v4 but uses score_v5 probes (adds images 21-30) and
reads/writes judgments from benchmark-results/judgments_v5/.

Usage:
  python3 score_v5_multirun.py                    # full multi-run report
  python3 score_v5_multirun.py --prepare-median   # write median-run judge inputs
"""
import argparse
import json
import statistics
from collections import defaultdict
from pathlib import Path

import score_v5 as v4

# Short display names matching BENCHMARK-REPORT-v5.md conventions.
SHORT = {
    "gemma4-12b":          "G4-12B",
    "gemma4-12b-Q8":       "G4-12B-Q8",
    "gemma4-26b-a4b":      "G4-26B-A4B",
    "gemma4-31b":          "G4-31B",
    "gemma4-e4b":          "G4-E4B",
    "gemma4-e4b-Q8":       "G4-E4B-Q8",
    "glm-4.6v-flash-9b":   "GLM-9B",
    "glm-4.6v-flash-9b-Q8":"GLM-9B-Q8",
    "qwen3-vl-4b":         "Q3VL-4B",
    "qwen3-vl-4b-Q8":      "Q3VL-4B-Q8",
    "qwen3-vl-8b":         "Q3VL-8B",
    "qwen3-vl-8b-Q8":      "Q3VL-8B-Q8",
    "qwen3.5-4b":          "Q3.5-4B",
    "qwen3.5-4b-Q8":       "Q3.5-4B-Q8",
    "qwen3.5-9b":          "Q3.5-9B",
    "qwen3.5-9b-Q8":       "Q3.5-9B-Q8",
    "qwen3.6-27b":         "Q3.6-27B",
    "qwen3.6-35b-a3b":     "Q3.6-35B-A3B",
}

RAW = Path("benchmark-results/raw.jsonl")
JUDGE_DIR = Path("benchmark-results/judgments_v5")
JUDGE_DIR.mkdir(parents=True, exist_ok=True)


def short_name(variant_key):
    """Convert 'qwen3.5-4b|think' → 'Q3.5-4B', 'qwen3.5-4b|nothink' → 'Q3.5-4B-nothink'.

    For non-hybrid models (Gemma, GLM, Q3VL), the |think suffix is dropped
    from the display since these models don't have a thinking phase.
    """
    if "|" not in variant_key:
        return SHORT.get(variant_key, variant_key[:12])
    model, variant = variant_key.rsplit("|", 1)
    base = SHORT.get(model, model[:12])
    # Hybrid thinkers (Qwen3.5/3.6) get explicit -nothink suffix when applicable
    if variant == "nothink":
        # Only show -nothink for models that actually have a thinking mode
        if model.startswith("qwen3.5") or model.startswith("qwen3.6"):
            return f"{base}-nothink"
    return base


def load_results_by_run():
    """Returns {(model_variant, image): {run_id: record}}.

    Model variant distinguishes thinking vs no-think configurations:
      - "qwen3.5-4b" (base) → "qwen3.5-4b|think" or "qwen3.5-4b|nothink"
    This fixes a v4 bug where think and nothink runs of the same model
    were aggregated together, conflating two distinct configurations.
    Models without thinking (Gemma, GLM, Q3VL) all map to "|think" since
    the distinction is meaningless for them (only one config exists).

    Filters out Q8 records (run_id starts with 'q8-') so they don't pollute
    the Q4 scorecard. Q8 results are analyzed separately.
    """
    by_cell = defaultdict(dict)
    with open(RAW) as f:
        for line in f:
            r = json.loads(line)
            if r.get("type") == "result" and r.get("ok"):
                run_id = str(r.get("run_id", "1"))
                # Skip Q8 runs (analyzed separately)
                if run_id.startswith("q8-"):
                    continue
                # Determine variant from thinking_disabled field.
                model = r["model"]
                if r.get("thinking_disabled"):
                    variant = f"{model}|nothink"
                else:
                    variant = f"{model}|think"
                by_cell[(variant, r["image"])][run_id] = r
    return by_cell


def compute_per_run_scores(by_cell):
    """For each (model, image), compute scores for each run.
    Returns {(model, image): {run_id: {det, fm, content}}}.
    """
    per_run = defaultdict(dict)
    for (model, image), runs in by_cell.items():
        for run_id, r in runs.items():
            fm = v4.detect_failure_mode(r)
            probes = v4.probes_for_image(image, r.get("content", ""))
            det, _ = v4.deterministic_score(image, probes, fm)
            per_run[(model, image)][run_id] = {
                "det": det,
                "fm": fm,
                "content": r.get("content", ""),
                "eval_count": r.get("eval_count", 0),
                "reasoning_len": len(r.get("reasoning_content", "")),
            }
    return per_run


def select_median_run(per_run_cell, judge_scores=None):
    """Pick the median run for a cell.
    - 0 runs: None
    - 1 run: that run
    - 2 runs: lower-scoring (conservative — represents worst-case of typical performance)
    - 3+ runs: middle run after sorting
    """
    runs = sorted(per_run_cell.items(), key=lambda x: (x[1]["det"], x[0]))
    n = len(runs)
    if n == 0:
        return None
    if n == 1:
        return runs[0][0]
    if n == 2:
        # Conservative: pick lower-scoring run
        return runs[0][0]
    # 3+ runs: middle
    return runs[n // 2][0]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--prepare-median", action="store_true",
                        help="Write per-image JSON files with median-run responses for judge input")
    args = parser.parse_args()

    by_cell = load_results_by_run()
    per_run = compute_per_run_scores(by_cell)

    models = sorted(set(m for (m, i) in by_cell.keys()))
    images = sorted(set(i for (m, i) in by_cell.keys()))

    # === Run-count summary ===
    print("=" * 100)
    print("Multi-run status")
    print("=" * 100)
    run_counts = defaultdict(lambda: defaultdict(int))
    for (m, i), runs in by_cell.items():
        for rid in runs:
            run_counts[rid][m] += 1
    all_runs = sorted(run_counts.keys())
    print(f"\nRuns present: {all_runs}")
    header = f"{'model':<22}" + "".join(f" run_{r:>6}" for r in all_runs)
    print(header)
    for m in models:
        row = f"{m:<22}"
        for r in all_runs:
            row += f" {run_counts[r].get(m, 0):>7}"
        print(row)

    # === Per-cell stats: mean ± std of det scores ===
    print("\n" + "=" * 100)
    print("Per-cell deterministic score (mean across runs)")
    print("=" * 100)
    cell_stats = {}  # (model, image) -> {mean, std, median_run, runs: {rid: det}}
    header = f"{'image':<48}"
    for m in models:
        header += f" {short_name(m)[:9]:>7}"
    print(header)
    print("-" * (48 + 8 * len(models)))

    for img in images:
        row = f"  {img[:46]:<48}"
        for m in models:
            cell_data = per_run.get((m, img), {})
            if not cell_data:
                row += f" {'—':>7}"
                continue
            dets = [r["det"] for r in cell_data.values()]
            mean = statistics.mean(dets)
            row += f" {mean:>7.2f}"
            cell_stats[(m, img)] = {
                "mean": mean,
                "std": statistics.stdev(dets) if len(dets) > 1 else 0.0,
                "min": min(dets),
                "max": max(dets),
                "median_run": select_median_run(cell_data),
                "runs": {rid: r["det"] for rid, r in cell_data.items()},
                "fms": {rid: r["fm"] for rid, r in cell_data.items()},
            }
        print(row)

    # === Stability report ===
    print("\n" + "=" * 100)
    print("Stability: cells with high variance across runs (std > 1.0)")
    print("=" * 100)
    unstable = []
    for (m, img), s in cell_stats.items():
        if s["std"] > 1.0:
            unstable.append((m, img, s))
    unstable.sort(key=lambda x: -x[2]["std"])
    print(f"\n{'model':<22} {'image':<50} {'min':>5} {'max':>5} {'std':>5} {'runs':>20}")
    for m, img, s in unstable[:30]:
        runs_str = ",".join(f"{r}:{v:.0f}" for r, v in sorted(s["runs"].items()))
        print(f"  {m:<22} {img[:48]:<50} {s['min']:>5.1f} {s['max']:>5.1f} {s['std']:>5.2f} {runs_str:>20}")
    print(f"\nTotal unstable cells (std > 1.0): {len(unstable)} / {len(cell_stats)}")

    if args.prepare_median:
        # === Write median-run judge inputs ===
        print("\n" + "=" * 100)
        print(f"Writing median-run judge inputs to {JUDGE_DIR}")
        print("=" * 100)
        # Load ground truth per image
        gt_path = Path("test-images/GROUND-TRUTH.md")
        gt_text = gt_path.read_text()
        import re
        gt_per_image = {}
        current = None
        buf = []
        for line in gt_text.splitlines():
            mt = re.match(r'^## (\S+\.png|\S+\.jpg|\S+\.jpeg)', line)
            if mt:
                if current:
                    gt_per_image[current] = "\n".join(buf).strip()
                current = mt.group(1)
                buf = []
            elif current:
                buf.append(line)
        if current:
            gt_per_image[current] = "\n".join(buf).strip()

        for img in images:
            rs = []
            for m in models:
                cell_data = per_run.get((m, img), {})
                if not cell_data:
                    continue
                median_run = cell_stats[(m, img)]["median_run"]
                median_record = cell_data[median_run]
                rs.append({
                    "model": m,
                    "run_id": median_run,
                    "content": median_record["content"][:4000],
                    "failure_mode": median_record["fm"],
                    "deterministic_score": median_record["det"],
                    "det_mean": cell_stats[(m, img)]["mean"],
                    "det_std": cell_stats[(m, img)]["std"],
                })
            gt = gt_per_image.get(img, "")
            out = JUDGE_DIR / f"input_{img.replace('/', '_')}.json"
            payload = {
                "image": img,
                "image_path": str(Path("test-images") / img),
                "ground_truth": gt,
                "responses": rs,
            }
            out.write_text(json.dumps(payload, indent=2, ensure_ascii=False))
            print(f"  {img}: {len(rs)} responses → {out.name}")

    # === If judgments exist, compute multi-run final ===
    multirun_judgments = defaultdict(dict)
    for f in JUDGE_DIR.glob("judgment_*.json"):
        image = f.stem[len("judgment_"):]
        try:
            data = json.loads(f.read_text())
        except Exception:
            continue
        # Judges may use "judgments" or "evaluations" as the key
        raw_judgments = data.get("judgments") or data.get("evaluations") or {}
        if isinstance(raw_judgments, list):
            judgments_dict = {}
            for entry in raw_judgments:
                key = entry.get("model_variant") or entry.get("model")
                if key:
                    judgments_dict[key] = entry
            raw_judgments = judgments_dict
        for model, j in raw_judgments.items():
            # Score field may be "holistic_score", "score", or "judge_score"
            if "holistic_score" not in j:
                j["holistic_score"] = j.get("score") or j.get("judge_score")
            # Normalize: some judges used 0-100 scale instead of 0-10
            if j["holistic_score"] is not None and j["holistic_score"] > 10:
                j["holistic_score"] = j["holistic_score"] / 10.0
            if "key_misses" not in j and "weaknesses" in j:
                j["key_misses"] = j["weaknesses"]
            if "key_hits" not in j and "strengths" in j:
                j["key_hits"] = j["strengths"]
            if "justification" not in j:
                j["justification"] = j.get("notes") or j.get("reasoning") or j.get("rationale") or ""
            if "hallucinations" not in j:
                j["hallucinations"] = []
            multirun_judgments[image][model] = j

    if multirun_judgments:
        compute_final(cell_stats, per_run, multirun_judgments, models, images)
    else:
        print("\n[No multi-run judgments found yet. Run with --prepare-median, dispatch judges, then re-run.]")


def compute_final(cell_stats, per_run, judgments, models, images):
    """Compute final multi-run scorecard using median-run judge scores."""
    print("\n" + "=" * 100)
    print("FINAL MULTI-RUN SCORECARD")
    print("=" * 100)
    print()
    print("Methodology:")
    print("  - For each (model, image):")
    print("    * Deterministic score: mean across all runs")
    print("    * Judge score: LLM-judge on the median run (most representative)")
    print("    * Final cell: 0.4 * det_mean + 0.6 * judge_score")
    print("    * Failure-mode: most severe across runs (worst case)")
    print("  - Image weights: based on judge-score spread (signal-based)")
    print("  - Per-model aggregate: weighted sum / max weight × 100")
    print()

    per_cell = defaultdict(dict)
    for img in images:
        for m in models:
            s = cell_stats.get((m, img))
            if not s:
                continue
            judge = judgments.get(img, {}).get(m, {})
            judge_score = judge.get("holistic_score")
            if judge_score is None:
                continue

            # Worst-case failure mode across runs
            fms = list(s["fms"].values())
            if "empty" in fms:
                fm_worst = "empty"
            elif "repetition_loop" in fms:
                fm_worst = "repetition_loop"
            elif "truncated" in fms:
                fm_worst = "truncated"
            else:
                fm_worst = "normal"

            det_mean = s["mean"]
            if fm_worst == "empty":
                final = 0.0
            elif fm_worst == "repetition_loop":
                final = 0.5
            elif fm_worst == "truncated":
                combined = 0.4 * det_mean + 0.6 * judge_score
                final = min(combined, 2.0)
            else:
                final = 0.4 * det_mean + 0.6 * judge_score

            per_cell[img][m] = {
                "final": final,
                "det_mean": det_mean,
                "det_std": s["std"],
                "judge": judge_score,
                "fm_worst": fm_worst,
                "runs": s["runs"],
                "key_misses": judge.get("key_misses", []),
                "hallucinations": judge.get("hallucinations", []),
            }

    # Compute weights from judge spread
    weights = {}
    for img in images:
        scores = [per_cell[img][m]["judge"] for m in models
                  if m in per_cell[img] and per_cell[img][m]["judge"] is not None]
        spread = (max(scores) - min(scores)) if len(scores) >= 5 else 0
        if spread == 0:
            w = 0
        elif spread >= 6.0:
            w = 8
        elif spread >= 4.0:
            w = 7
        elif spread >= 3.0:
            w = 6
        elif spread >= 2.0:
            w = 5
        elif spread >= 1.0:
            w = 3
        else:
            w = 2
        weights[img] = w

    # Print per-image table
    header = f"{'image':<48}"
    for m in models:
        header += f" {short_name(m)[:9]:>7}"
    header += f" {'wt':>4}"
    print(header)
    print("-" * (48 + 8 * len(models) + 5))

    totals = defaultdict(float)
    weight_completed = defaultdict(float)
    for img in images:
        w = weights[img]
        row = f"  {img[:46]:<48}"
        for m in models:
            cell = per_cell[img].get(m)
            if not cell:
                row += f" {'—':>7}"
                continue
            row += f" {cell['final']:>7.2f}"
            totals[m] += cell["final"] * w / 10.0
            weight_completed[m] += w
        row += f" {w:>4}"
        print(row)

    max_weight = sum(weights.values())
    print("-" * (48 + 8 * len(models) + 5))
    print(f"\nMax possible weight: {max_weight}")

    # Final /100 with stability info
    print()
    print(f"{'METRIC':<48}" + "".join(f" {short_name(m)[:9]:>7}" for m in models))
    rankings = []
    row_final = f"  {'FINAL (/100)':<48}"
    row_std = f"  {'instability (mean std)':<48}"
    for m in models:
        if weight_completed[m] >= max_weight * 0.85:
            score_100 = totals[m] / max_weight * 100
            rankings.append((m, score_100, weight_completed[m]))
            row_final += f" {score_100:>7.1f}"
            # Mean per-cell std (instability indicator)
            cell_stds = [per_cell[img][m]["det_std"] for img in images if img in per_cell and m in per_cell[img]]
            mean_std = statistics.mean(cell_stds) if cell_stds else 0
            row_std += f" {mean_std:>7.2f}"
        else:
            row_final += f" {'partial':>7}"
            row_std += f" {'—':>7}"
    print(row_final)
    print(row_std)

    # Ranking
    rankings.sort(key=lambda x: -x[1])
    print("\n" + "=" * 80)
    print("FINAL RANKING (3-run aggregated)")
    print("=" * 80)
    medals = ['🥇', '🥈', '🥉']
    for i, (m, score, w) in enumerate(rankings):
        medal = medals[i] if i < 3 else f"  {i+1}."
        print(f"  {medal} {short_name(m):<12} {score:>5.1f}/100")
    if len(rankings) >= 2:
        spread = rankings[0][1] - rankings[-1][1]
        print(f"\n  Spread: {rankings[0][1]:.1f} (top) − {rankings[-1][1]:.1f} (bottom) = {spread:.1f} points")


if __name__ == "__main__":
    main()
