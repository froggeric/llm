#!/usr/bin/env python3
"""Quality- and reliability-aware recommendations for the 10 localvision MCP tools.

Two lenses, kept separate:
  - **Quality table** (the recommendation): per tool, the DENSE model with the
    best *effective quality*. Per (variant,image) the quality is the v5 final cell
    (`0.4*det_mean + 0.6*judge`, failure-mode capped) MULTIPLIED BY the run-success
    rate (n_ok / n_attempted) -- i.e. expected per-call quality, so a model that
    succeeds on only 1 of 3 runs scores a third of its good-run quality (failures
    are a big penalty, partial ones included). A total timeout (0 ok runs) scores 0.
    Across-run OUTPUT variability sigma = mean pairwise token-Jaccard divergence of
    the successful runs; a cell with <2 successful runs contributes sigma=1.0
    (unmeasurable consistency => treat as maximally variable), so sigma is never an
    artifact-of-one-run zero. effective = quality * (1 - k*sigma).
    MoE (A3B/A4B) are shown but NEVER recommended; badge goes to best dense model.
    NO latency/size here -- this table is about quality.
  - **Latency menu**: speed lens -- best effective quality within a wall-clock
    budget per tool (dense; --include-moe adds MoE).

Quality cells reuse the v5/q8 pipelines verbatim; Q4 and Q8 are directly
comparable. Caveat: description quality is a strong proxy for
OCR/code/table/chart/diagram/UI/error, weaker for image_to_prompt, absent for
compare_images (no image pairs).

Run from benchmark/vlm/ :  PYTHONPATH=code python3 code/recommend_by_tasktype.py
"""
import argparse
import json
import re
import statistics
from collections import defaultdict
from pathlib import Path

import score_v5_multirun as q4mod
import score_q8_multirun as q8mod

short_name = q8mod.short_name
RAW = Path("benchmark-results/raw.jsonl")
MOE = re.compile(r"-a\d+b", re.I)
_WORD = re.compile(r"[a-z0-9]+")
N_RUNS = 3   # attempted runs per (variant, image)


TASK_TYPES = {
    "read_image": [
        "05-cropped-youtube-capture.png", "07-photo-massage-therapists.jpg",
        "10-photo-rice-porridge-breakfast.png", "11-collage-therapist-photo.jpg",
        "12-manga-nausicaa-colour.png", "13-colour-swatch-nausicaa.jpg",
        "20-ultrawide-kung-fu-banner-with-logo.png", "22-spritesheet-bubble-bobble.png",
        "25-mri-brain-parkinson.jpeg", "28-xray-ribfracture-lowerright.jpg",
        "30-where-is-waldo.webp",
    ],
    "extract_text": [
        "13-marker-drawing-spotify-cassette.jpeg", "16-album-cover-lofi.jpg",
        "17-logo-vic-health-club.png", "18-qrcode-vic-health-club.png", "21-motion-blur.jpg",
    ],
    "extract_code": ["02_code_python.png"],
    "extract_table": ["08-poster-class-schedule.jpg", "14-screenshot-onerpm-catalog.jpg"],
    "describe_ui": ["01_ui_login.png"],
    "describe_diagram": ["04_architecture.png", "23-animation.jpg", "24-lymphatic-system-handdrawn.jpg"],
    "describe_chart": ["26-graph-ocean-acidification-hawaii.jpg",
                       "27-graph-carbonate-based-marine-life-survival-against-ph.jpeg"],
    "diagnose_error": ["03_error_trace.png"],
    "image_to_prompt": [
        "06-trading-card.jpg", "09-poster-artnight.jpg",
        "19-watercolour-painting-surfers-fuerteventura.png", "29-painting-hundertwasser.jpeg",
    ],
    "compare_images": [],
}
_ALL = [i for imgs in TASK_TYPES.values() for i in imgs]
assert len(_ALL) == 30 and len(set(_ALL)) == 30, "bucketing must cover 30 unique images"

WEAK_PROXY = {"image_to_prompt"}
NO_DATA = {"compare_images"}

# Curated per-variant notes (editorial; references the per-tool winners from this run).
NOTES = {
    "qwen3.6-27b|nothink":      "**Champion** — best effective, 100% reliable",
    "qwen3.6-35b-a3b|nothink":  "MoE — fast + top-tier; not in per-tool recs",
    "qwen3-vl-8b-Q8|think":     "**Best mid-tier** — Q8 strict win (0 fails, low σ); won describe_diagram + image_to_prompt",
    "qwen3.6-35b-a3b|think":    "MoE — think twin, slower than nothink",
    "glm-4.6v-flash-9b|think":  "CJK-signage specialist; Q8 twin wins extract_text",
    "gemma4-31b|think":         "Slow (~97s); not better than Q3.6-27B — skip",
    "qwen3.5-4b|nothink":       "**Best small** — fast, reliable; ≤20s menu workhorse",
    "qwen3-vl-8b|think":        "Sweet spot — fast, low σ; won diagnose_error",
    "qwen3.5-9b|nothink":       "Solid mid; think twin wins read_image + extract_table",
    "gemma4-26b-a4b|think":     "MoE — outclassed",
    "glm-4.6v-flash-9b-Q8|think": "Won extract_text (CJK OCR); slight timeout risk (93%)",
    "qwen3.5-9b|think":         "Won read_image + extract_table; nothink twin slightly higher",
    "qwen3.6-27b|think":        "Won describe_ui + describe_chart; think mode slower/less reliable — use nothink",
    "qwen3.5-4b|think":         "Think twin of best-small; nothink preferred",
    "qwen3-vl-4b|think":        "Tiny + fast; ≤20s menu pick; dense-scene runaways",
    "qwen3-vl-4b-Q8|think":     "Unstable at Q8 (94% ok); Q4 preferred",
    "gemma4-12b|think":         "**Avoid** — hallucination flips ('Atomic acid' for 'Domoic acid')",
    "qwen3.5-4b-Q8|nothink":    "Drop vs Q4 — timeout-prone at Q8 (87%)",
    "qwen3.5-9b-Q8|nothink":    "Unusable at Q8 (81% ok)",
    "gemma4-12b-Q8|think":      "⚠ raw 76.6 looks good, fails ~1-in-5 (81% ok)",
    "gemma4-e4b|think":         "**Avoid** — perception fails",
    "gemma4-e4b-Q8|think":      "Q8 helps, still weak (perception)",
    "qwen3.5-4b-Q8|think":      "Unusable at Q8 (76% ok, high σ)",
    "qwen3.5-9b-Q8|think":      "Unusable at Q8 (62% ok); slow",
}


def load_judgments(judge_dir):
    judgments = defaultdict(dict)
    for f in Path(judge_dir).glob("judgment_*.json"):
        image = f.stem[len("judgment_"):]
        try:
            data = json.loads(f.read_text())
        except Exception:
            continue
        raw = data.get("judgments") or data.get("evaluations") or {}
        if isinstance(raw, list):
            d = {}
            for entry in raw:
                key = entry.get("model_variant") or entry.get("model")
                if key:
                    d[key] = entry
            raw = d
        for model, j in raw.items():
            if "holistic_score" not in j:
                j["holistic_score"] = j.get("score") or j.get("judge_score")
            if j["holistic_score"] is not None and j["holistic_score"] > 10:
                j["holistic_score"] = j["holistic_score"] / 10.0
            judgments[image][model] = j
    return judgments


def jaccard_divergence(contents):
    toks = [set(_WORD.findall(c.lower())) for c in contents]
    ds = []
    for i in range(len(toks)):
        for j in range(i + 1, len(toks)):
            u = toks[i] | toks[j]
            ds.append(0.0 if not u else 1 - len(toks[i] & toks[j]) / len(u))
    return statistics.mean(ds) if ds else 0.0


def build_per_cell(mod, judge_dir):
    by_cell = mod.load_results_by_run()
    per_run = mod.compute_per_run_scores(by_cell)
    models = sorted(set(m for (m, i) in by_cell))
    images = sorted(set(i for (m, i) in by_cell))
    cell_stats = {}
    for (m, img), cd in per_run.items():
        dets = [r["det"] for r in cd.values()]
        cell_stats[(m, img)] = {"mean": statistics.mean(dets),
                                "fms": {rid: r["fm"] for rid, r in cd.items()}}
    judgments = load_judgments(judge_dir)
    per_cell = defaultdict(dict)
    run_contents = defaultdict(list)
    for img in images:
        for m in models:
            s = cell_stats.get((m, img))
            if not s:
                continue
            js = judgments.get(img, {}).get(m, {}).get("holistic_score")
            if js is None:
                continue
            fms = list(s["fms"].values())
            fm_worst = ("empty" if "empty" in fms else "repetition_loop" if "repetition_loop" in fms
                        else "truncated" if "truncated" in fms else "normal")
            det = s["mean"]
            if fm_worst == "empty":
                final = 0.0
            elif fm_worst == "repetition_loop":
                final = 0.5
            elif fm_worst == "truncated":
                final = min(0.4 * det + 0.6 * js, 2.0)
            else:
                final = 0.4 * det + 0.6 * js
            per_cell[img][m] = {"final": final, "judge": js}
            run_contents[(m, img)] = [r.get("content", "") for r in by_cell[(m, img)].values()]
    return per_cell, run_contents, models, images


def load_reliability():
    rel = defaultdict(lambda: {"ok": 0, "fail": 0, "ok_elapsed": [], "ok_tps": [], "fail_elapsed": [], "runaway": 0})
    with open(RAW) as f:
        for line in f:
            r = json.loads(line)
            if r.get("type") != "result":
                continue
            model = r["model"]
            variant = f"{model}|nothink" if r.get("thinking_disabled") else f"{model}|think"
            cell = rel[(variant, r["image"])]
            if r.get("ok"):
                cell["ok"] += 1
                cell["ok_elapsed"].append(float(r.get("elapsed_s", 0)))
                if r.get("tokens_per_s"):
                    cell["ok_tps"].append(float(r["tokens_per_s"]))
                if int(r.get("eval_count", 0)) >= 16384:
                    cell["runaway"] += 1
            else:
                cell["fail"] += 1
                cell["fail_elapsed"].append(float(r.get("elapsed_s", 0)))
    return rel


def aggregate_100(per_cell, models, images):
    weights = {}
    for img in images:
        js = [per_cell[img][m]["judge"] for m in models if m in per_cell[img]]
        spread = (max(js) - min(js)) if len(js) >= 5 else 0
        weights[img] = (0 if spread == 0 else 8 if spread >= 6 else 7 if spread >= 4
                         else 6 if spread >= 3 else 5 if spread >= 2 else 3 if spread >= 1 else 2)
    max_w = sum(weights.values())
    totals, done = defaultdict(float), defaultdict(float)
    for img in images:
        w = weights[img]
        for m in models:
            c = per_cell[img].get(m)
            if c:
                totals[m] += c["final"] * w / 10.0
                done[m] += w
    return {m: totals[m] / max_w * 100 for m in models if done[m] >= max_w * 0.85}


def bucket_metrics(per_cell, run_contents, rel, variant, images, k, timeout_s):
    """quality = final * run-success-rate; sigma = output divergence (1.0 if <2 ok runs)."""
    qterms, sterms, rates, lats, tps = [], [], [], [], []
    for img in images:
        c = per_cell.get(img, {}).get(variant)
        rc = rel.get((variant, img)) or {}
        n_ok, n_fail = rc.get("ok", 0), rc.get("fail", 0)
        n_att = n_ok + n_fail
        rate = (n_ok / n_att) if n_att else 0.0
        rates.append(rate)
        if c is not None and n_ok >= 1:                         # >=1 successful run + judge
            qterms.append(c["final"] * rate)                    # expected per-call quality (0-10)
            contents = run_contents.get((variant, img), [])
            sterms.append(jaccard_divergence(contents) if len(contents) >= 2 else 1.0)
            lats.append(statistics.mean(rc["ok_elapsed"]) if rc.get("ok_elapsed") else timeout_s)
            tps += rc.get("ok_tps", [])
        else:                                                   # total timeout / no success
            qterms.append(0.0)
            sterms.append(1.0)
            lats.append(statistics.mean(rc["fail_elapsed"]) if rc.get("fail_elapsed") else timeout_s)
    mean_q = statistics.mean(qterms) if qterms else 0.0          # 0-10
    sigma = statistics.mean(sterms) if sterms else 0.0
    okrate = statistics.mean(rates) if rates else 0.0
    return {
        "eff": mean_q * 10.0 * (1 - k * sigma),
        "mean": mean_q * 10.0,
        "sigma": sigma,
        "okrate": okrate,
        "lat": statistics.mean(lats) if lats else float("nan"),
        "tps": statistics.mean(tps) if tps else float("nan"),
        "n": len(images),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--k", type=float, default=0.2, help="variability discount (effective = quality*(1 - k*sigma))")
    ap.add_argument("--tiers", default="20,45,90", help="latency budgets (s), ascending")
    ap.add_argument("--timeout", type=float, default=300.0, help="latency (s) when a timeout has no recorded elapsed")
    ap.add_argument("--include-moe", action="store_true", help="allow MoE (A3B/A4B) in recommendations/menu")
    args = ap.parse_args()
    tiers = [int(x) for x in args.tiers.split(",")]

    per_cell = defaultdict(dict)
    run_contents = defaultdict(list)
    all_variants, validation = set(), []
    for mod, jdir in [(q4mod, "benchmark-results/judgments_v5"), (q8mod, "benchmark-results/judgments_q8")]:
        pc, rc, models, images = build_per_cell(mod, jdir)
        for img, vd in pc.items():
            per_cell[img].update(vd)
        for key, vals in rc.items():
            run_contents[key].extend(vals)
        all_variants |= set(models)
        validation.append(aggregate_100(pc, models, images))
    variants = sorted(all_variants)
    rel = load_reliability()
    is_moe = lambda v: MOE.search(v) is not None

    print("<!-- VALIDATION: weighted /100 per quant (raw scorecard, pre-reliability/variability; must match) -->\n")
    for i, agg in enumerate(validation):
        ranked = sorted(agg.items(), key=lambda x: -x[1])
        print(f"**{'Q4' if i == 0 else 'Q8'}** top 5: " + ", ".join(f"{short_name(m)} {s:.1f}" for m, s in ranked[:5]) + "\n")

    # ---- Master ranking: overall effective quality (reliability + variability) over all 30 images ----
    all_images = sorted({i for i in per_cell})
    raw100 = {**validation[0], **validation[1]}   # weighted scorecard /100, per quant

    def quant_mode(v):
        model, mode = v.rsplit("|", 1)
        return ("Q8" if model.endswith("-Q8") else "Q4"), mode

    overall = []
    for v in variants:
        m = bucket_metrics(per_cell, run_contents, rel, v, all_images, args.k, args.timeout)
        overall.append({"variant": v, **m, "raw": raw100.get(v)})
    overall.sort(key=lambda r: -r["eff"])
    print("## Master ranking — all variants by effective quality\n")
    print(f"effective = (v5 final × run-success-rate) × (1 − {args.k:g}·σ) over all 30 images — reliability AND across-run variability both counted. "
          "Raw = weighted scorecard /100 (pre-adjustment, for reference).\n")
    print("| # | Variant | Quant | Mode | Raw /100 | Effective | σ(text) | ok% | Latency s | Note |")
    print("|---|---|---|---|---|---|---|---|---|---|")
    for i, r in enumerate(overall, 1):
        q, mo = quant_mode(r["variant"])
        raw = f"{r['raw']:.1f}" if r["raw"] is not None else "—"
        moe = " (MoE)" if is_moe(r["variant"]) else ""
        print(f"| {i} | {short_name(r['variant'])}{moe} | {q} | {mo} | {raw} | {r['eff']:.1f} | {r['sigma']:.2f} | {r['okrate']*100:.0f} | {r['lat']:.0f} | {NOTES.get(r['variant'], '')} |")
    print()

    def recommend_from(rows):
        pool = [r for r in rows if args.include_moe or not is_moe(r["variant"])]
        return min(pool, key=lambda r: (-r["eff"], r["sigma"], r["lat"])) if pool else None

    bucket_results = {}
    for bname, bimgs in TASK_TYPES.items():
        rows = [{"variant": v, **bucket_metrics(per_cell, run_contents, rel, v, bimgs, args.k, args.timeout)} for v in variants]
        if not bimgs:
            bucket_results[bname] = (rows, None, None, None)
            continue
        rec = recommend_from(rows)
        best_dense = recommend_from([r for r in rows if not is_moe(r["variant"])])
        best_moe = max((r for r in rows if is_moe(r["variant"])), key=lambda r: r["eff"], default=None)
        bucket_results[bname] = (rows, rec, best_dense, best_moe)

    print(f"<!-- Quality: per-cell quality = v5 final * run-success-rate (expected per-call quality); "
          f"effective = quality * (1 - {args.k:g}*sigma), sigma = pairwise token-Jaccard divergence of run outputs "
          f"(1.0 if <2 ok runs). MoE shown & flagged but {'ALLOWED' if args.include_moe else 'NOT recommended (dense)'}. -->\n")

    # ---- Quality table (dense only; MoE ranked in the master table) ----
    print("## Per-tool recommendations — quality (dense models)\n")
    print("| Tool | n | Recommended | Mode | eff/100 | mean/100 | σ(text) | ok% |")
    print("|---|---|---|---|---|---|---|---|")
    for bname, (rows, rec, best_dense, best_moe) in bucket_results.items():
        n = len(TASK_TYPES[bname])
        if bname in NO_DATA:
            print(f"| `{bname}` | 0 | — | — | — | — | — | — |")
            continue
        r = rec or best_dense
        _, mo = quant_mode(r["variant"])
        print(f"| `{bname}` | {n} | **{short_name(r['variant'])}** | {mo} | {r['eff']:.1f} | {r['mean']:.1f} | {r['sigma']:.2f} | {r['okrate']*100:.0f} |")
    print()
    print("*eff = (v5 final × run-success-rate) × (1 − k·σ); mean = pre-reliability/variability; σ = across-run output divergence (1.0 if a cell had <2 successful runs); ok% = mean run-success rate. Dense models only — MoE are ranked in the master table above.*\n")

    # ---- Per-tool detail (dense only) ----
    for bname, bimgs in TASK_TYPES.items():
        if not bimgs:
            continue
        rows = [r for r in bucket_results[bname][0] if not is_moe(r["variant"])]
        rows.sort(key=lambda r: -r["eff"])
        rec = bucket_results[bname][1]
        print(f"<details><summary><b>{bname}</b> — dense variants (n={len(bimgs)})</summary>\n")
        print("| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |")
        print("|---|---|---|---|---|---|---|")
        for r in rows:
            mark = "⭐ " if rec and r["variant"] == rec["variant"] else ""
            print(f"| {mark}{short_name(r['variant'])} | {r['eff']:.1f} | {r['mean']:.1f} | {r['sigma']:.2f} | {r['okrate']*100:.0f} | {r['lat']:.0f} | {r['tps']:.0f} |")
        print("</details>\n")

    # ---- Latency menu ----
    print("## Latency-budget menu — best effective quality within a wall-clock budget\n")
    print("| Tool | " + " | ".join(f"≤{t}s" for t in tiers) + " | unlimited |")
    print("|---|" + "---|" * (len(tiers) + 1))
    for bname, bimgs in TASK_TYPES.items():
        if not bimgs:
            print(f"| `{bname}` | " + " | ".join("—" for _ in tiers) + " | — |")
            continue
        rows = bucket_results[bname][0]
        pool = [r for r in rows if args.include_moe or not is_moe(r["variant"])]
        cells = []
        for t in tiers:
            elig = [r for r in pool if r["lat"] <= t]
            cells.append(short_name(max(elig, key=lambda r: (r["eff"], -r["sigma"], -r["lat"]))["variant"]) if elig else "—")
        cells.append(short_name(max(pool, key=lambda r: (r["eff"], -r["sigma"], -r["lat"]))["variant"]))
        print(f"| `{bname}` | " + " | ".join(cells) + " |")


if __name__ == "__main__":
    main()
