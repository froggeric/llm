#!/usr/bin/env python3
"""Score the per-category self-consistency sweep (run_id=cat-*).

Question: which localvision tool CATEGORIES benefit from multi-sampling +
correlation, and at what temperature / precision cost?

For each (category, temperature) we compare, over 5 repeats:
  single   = mean per-run score (one random run)
  union@5  = score of the UNION over all 5 runs (≥1 run has it)
  maj@5    = score of the per-fact/per-token MAJORITY vote (≥3/5)

Two scoring shapes:
  EXTRACTION (extract_code, extract_table): token-level P/R/F1 vs a GT text
    block — this is where union's precision drop (hallucinated tokens) shows up.
  COVERAGE (describe_ui, describe_diagram, describe_chart, read_image,
    diagnose_error, extract_text): key-FACT recall vs a curated list, plus a
    hallucination count where the GT lists things NOT in the image.

Baseline = single @ temp 0.1 (production). Verdict per category: does
union@5 @ temp 0.7 beat it on the primary metric without a precision collapse?

Run from benchmark/vlm/ :  python3 code/score_category.py
"""
import json
import re
import statistics
import sys
from collections import defaultdict
from pathlib import Path

RAW = Path("benchmark-results/raw.jsonl")
GT = Path("test-images/REFINE-GROUND-TRUTH.md")
WORD = re.compile(r"[A-Za-z0-9_'.À-ɏ]+")

IMG_CAT = {
    "30-where-is-waldo.webp": "read_image",
    "ocr-test-4.png": "extract_text",
    "extract-code-test-1.png": "extract_code",
    "08-poster-class-schedule.jpg": "extract_table",
    "ui-test-1.png": "describe_ui",
    "04_architecture.png": "describe_diagram",
    "26-graph-ocean-acidification-hawaii.jpg": "describe_chart",
    "03_error_trace.png": "diagnose_error",
}
CAT_IMG = {v: k for k, v in IMG_CAT.items()}
SHORTM = {
    "qwen3-vl-8b": "Q3VL-8B-Q8", "qwen3.5-4b": "Q3.5-4B-nt", "qwen3.5-9b": "Q3.5-9B-nt",
    "glm-4.6v-flash-9b": "GLM-9B", "qwen3.5-4b-Q8": "Q3.5-4B-Q8",
    "gemma4-e4b": "G4-E4B", "qwen3.6-35b-a3b": "Q3.6-35B-A3B",
}
# Reps to score at. 3 is the sweet spot (REPEAT-REPORT.md): union@3 ~= union@5, so
# 5 costs ~60% more wall time for ~0 extra quality. Cells with >NREP reps use the
# first NREP (so the 8B/4B 5-rep runs re-score at 3 for an apples-to-apples view).
NREP = 3

UI1 = ["New Chat", "New Projects", "Search", "Hub", "Settings", "Chats", "INTEGRATIONS",
       "Experimental", "MCP Servers", "Claude Code", "MODEL PROVIDERS", "Llama.cpp", "MLX",
       "OpenAI", "Azure", "Anthropic", "OpenRouter", "Mistral", "Groq", "vAI", "General",
       "App Version", "v0.8.2", "Automatic Update Check", "Check for Updates", "Language",
       "English", "Data Folder", "App Data", "Change Location", "App Logs", "Show in Finder",
       "Open Logs", "Advanced", "Jan CLI", "Uninstall", "Reset To Factory Settings"]

# Coverage categories: curated key facts (each entry = a required substring, or a
# tuple of acceptable alternatives). Recall = fraction of entries present.
COVERAGE = {
    "describe_ui": UI1,
    "extract_text": [  # ocr-test-4 handwritten/printed fields (owner-verified)
        "Joan", ("Almiñana", "Almiana"), "Moropsa", "20041185C", "635889002",
        ("n°19", "nº19", "n° 19"), "piso 3A", "joansnobc@gmail.com", "estevet",
        "Corralejo", "La Oliva", "Fuerteventura", "Cabra Rojo", "928 537 272",
        "Bajo Blanco", "Hubara", "35660", "NOTA SIMPLE", "42", "11656",
        "32840", "Muñoz", "50183525D"],
    "describe_diagram": [  # 04-architecture
        "Web Client", "API Gateway", "FastAPI", "Auth Service", "Worker Queue",
        "Postgres", "Redis", "HTTPS", "gRPC", ("phantom", "dangling", "nowhere"),
        "System Architecture"],
    "describe_chart": [  # 26-graph-ocean-acidification
        "Ocean acidification", "1958", "2018", ("CO₂", "CO2", "carbon dioxide"),
        "315", "415", "pH", "Mauna Loa", "ALOHA", "Atmospheric", ("pCO₂", "pCO2"),
        "8.12", "8.06"],
    "read_image": [  # 30-where-is-waldo
        "Waldo", "Odlaw", "Wizard Whitebeard", ("beach hut", "beach huts"),
        "submarine", "hovercraft", ("sailboat", "sail boat", "sailboats"),
        "Martin Handford", ("horse", "horses"), "volleyball"],
    "diagnose_error": [  # 03_error_trace
        "psycopg2", "OperationalError", "ConnectionError", "password authentication",
        "main.py", "42", "db.py", "18", ("exit code", "exited with", "exit 1"),
        "5432", "db.internal", "app", "FATAL"],
}
# Things NOT in the image (from GT) — counting these = hallucination proxy.
HALLUC = {
    "read_image": ["pier", "jetty", "lighthouse", "Dick Bruna"],
}

# Extraction categories: token P/R/F1 vs a GT text block.
def code_gt():
    txt = GT.read_text()
    m = re.search(r"^## extract-code-test-1\.png.*?```\w*\n(.*?)```", txt, re.S | re.M)
    return m.group(1) if m else ""

TABLE_GT = (  # 08-poster-class-schedule (owner-verified 11-class schedule)
    "VIC Health Club Weekly Activities Schedule "
    "Monday Shaolin QIGONG 12:00-13:00 YOGA 13:00-14:00 Embassy of India "
    "OSTEOPOROSIS Prevention 13:10-14:00 Tuesday YOGASTHENICS 12:00-13:00 "
    "TAIJI 24 forms Gabi Wednesday CALISTHENICS VIC Runners Thursday "
    "Shaolin KUNG FU Friday PRANAYAMA Register online https vichealth.club")
EXTRACTION_GT = {"extract_code": None, "extract_table": TABLE_GT}  # code filled at runtime


def norm(s):
    s = (s or "").lower()
    repl = {"₂": "2", "₃": "3", "–": "-", "—": "-", "’": "'", "‘": "'",
            "“": '"', "”": '"', "´": "'", "ö": "o", "ü": "u", "ñ": "n", "á": "a"}
    for k, v in repl.items():
        s = s.replace(k, v)
    return s


def present(fact, text):
    """fact = str or tuple-of-alts. Boundary-aware substring match on normed text."""
    alts = fact if isinstance(fact, tuple) else (fact,)
    for a in alts:
        a = norm(a)
        if re.search(rf"(?<![a-z0-9]){re.escape(a)}(?![a-z0-9])", text):
            return True
    return False


def fact_recall(contents, facts):
    """recall of the UNION of contents over the fact list."""
    joined = norm(" ".join(contents))
    return sum(1 for f in facts if present(f, joined)) / len(facts) if facts else 0.0


def single_fact_recall(contents, facts):
    """mean per-run recall."""
    if not contents:
        return 0.0
    return sum(fact_recall([c], facts) for c in contents) / len(contents)


def maj_fact_recall(contents, facts, k):
    """per-fact majority over first k runs (≥ceil(k/2))."""
    sub = contents[:k]
    thr = k // 2 + 1
    hit = sum(1 for f in facts if sum(present(f, norm(c)) for c in sub) >= thr)
    return hit / len(facts) if facts else 0.0


def halluc_count(contents, bads):
    joined = norm(" ".join(contents))
    return sum(1 for b in bads if present(b, joined))


def toks(s):
    return set(t.lower() for t in WORD.findall(s))


def prf(out_tok, gt_tok):
    if not out_tok:
        return 0.0, 0.0, 0.0
    inter = len(out_tok & gt_tok)
    p = inter / len(out_tok)
    r = inter / len(gt_tok) if gt_tok else 0.0
    f = 2 * p * r / (p + r) if (p + r) else 0.0
    return p, r, f


def maj_tokens(contents, k):
    sub = contents[:k]
    thr = k // 2 + 1
    sets = [toks(c) for c in sub]
    allt = set().union(*sets) if sets else set()
    return {t for t in allt if sum(t in s for s in sets) >= thr}


def load_cat():
    """(model, image, temp) -> list[(rep, content)] sorted."""
    cells = defaultdict(list)
    for line in open(RAW):
        try:
            r = json.loads(line)
        except json.JSONDecodeError:
            continue
        if r.get("type") != "result" or not str(r.get("run_id", "")).startswith("cat-"):
            continue
        if "repeat_idx" not in r:
            continue
        cells[(r["model"], r["image"], r.get("temperature", 0.1))].append(
            (r["repeat_idx"], r.get("content", "") if r.get("ok") else ""))
    for k in cells:
        cells[k].sort()
    return cells


def main():
    # Optional model filter (substring of the model name) so a multi-model
    # raw.jsonl doesn't mix results. Default = the recommended default model.
    model_filter = sys.argv[1] if len(sys.argv) > 1 else "qwen3-vl-8b"
    cells = {k: v for k, v in load_cat().items() if model_filter in k[0]}
    if not cells:
        print(f"No cat-* rows found for model matching '{model_filter}'. "
              f"Run: ./code/run_category.sh <model>"); return
    EXTRACTION_GT["extract_code"] = code_gt()
    TEMPS = sorted({t for _, _, t in cells})
    model_name = SHORTM.get(next(iter(cells))[0], next(iter(cells))[0])

    print(f"# Per-category self-consistency sweep — {model_name} (run_id=cat-*)\n")
    print(f"> {NREP} reps per (category × temp). **single** = mean per-run; **union@{NREP}** = "
          f"≥1 run has it; **maj@{NREP}** = per-fact/token majority (≥{NREP//2+1}/{NREP}).\n")

    # ---- per-category detail ----
    for cat in ["read_image", "extract_text", "extract_code", "extract_table",
                "describe_ui", "describe_diagram", "describe_chart", "diagnose_error"]:
        img = CAT_IMG[cat]
        # collect across temps for this category (any model — here the default)
        by_temp = {t: [] for t in TEMPS}
        for (m, im, t), runs in cells.items():
            if im == img:
                by_temp[t] = [c for _, c in runs][:NREP]
        if not any(by_temp.values()):
            continue
        print(f"## {cat}  (`{img}`)\n")
        is_extr = cat in EXTRACTION_GT
        if is_extr:
            gt_tok = toks(EXTRACTION_GT[cat])
            print(f"| temp | single R | single P | single F1 | union@{NREP} R | union@{NREP} P | union@{NREP} F1 | maj@{NREP} R | maj@{NREP} P | maj@{NREP} F1 |")
            print("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
            for t in TEMPS:
                cs = by_temp[t]
                if not cs:
                    continue
                # single = mean per-run F1 (R/P)
                sp, sr, sf = [], [], []
                for c in cs:
                    p, r, f = prf(toks(c), gt_tok); sp.append(p); sr.append(r); sf.append(f)
                up, ur, uf = prf(set().union(*(toks(c) for c in cs)), gt_tok)
                mp, mr, mf = prf(maj_tokens(cs, NREP), gt_tok)
                print(f"| {t} | {statistics.mean(sr)*100:.0f}% | {statistics.mean(sp)*100:.0f}% | "
                      f"{statistics.mean(sf)*100:.0f}% | {ur*100:.0f}% | {up*100:.0f}% | {uf*100:.0f}% | "
                      f"{mr*100:.0f}% | {mp*100:.0f}% | {mf*100:.0f}% |")
        else:
            facts = COVERAGE[cat]
            bads = HALLUC.get(cat, [])
            print(f"| temp | single recall | union@{NREP} recall | maj@{NREP} recall | halluc single | halluc union@{NREP} |")
            print("|---|---:|---:|---:|---:|---:|")
            for t in TEMPS:
                cs = by_temp[t]
                if not cs:
                    continue
                s = single_fact_recall(cs, facts) * 100
                u = fact_recall(cs, facts) * 100
                mj = maj_fact_recall(cs, facts, NREP) * 100
                hs = statistics.mean([halluc_count([c], bads) for c in cs]) if cs else 0
                hu = halluc_count(cs, bads)
                print(f"| {t} | {s:.0f}% | {u:.0f}% | {mj:.0f}% | {hs:.1f} | {hu} |")
        print()

    # ---- verdict matrix ----
    print(f"## Verdict — best correlated @ temp 0.7 vs single @ temp 0.1 (production, {NREP} reps)\n")
    print("> Best = **majority** for extraction (code/table), **union** for coverage. "
          "Primary metric = F1 for extraction, recall for coverage. BENEFIT = ≥+2 pts.\n")
    print(f"| Category | baseline single@0.1 | best@{NREP}@0.7 | Δ | precision/halluc Δ | verdict |")
    print("|---|---:|---:|---:|---:|---|")
    for cat in ["read_image", "extract_text", "extract_code", "extract_table",
                "describe_ui", "describe_diagram", "describe_chart", "diagnose_error"]:
        img = CAT_IMG[cat]
        c01 = [c for (m, im, t), rs in cells.items() if im == img and abs(t - 0.1) < 1e-9 for _, c in rs][:NREP]
        c07 = [c for (m, im, t), rs in cells.items() if im == img and abs(t - 0.7) < 1e-9 for _, c in rs][:NREP]
        if not c01 or not c07:
            print(f"| {cat} | — | — | — | — | (missing) |"); continue
        if cat in EXTRACTION_GT:
            gt_tok = toks(EXTRACTION_GT[cat])
            base = statistics.mean([prf(toks(c), gt_tok)[2] for c in c01]) * 100
            base_p = statistics.mean([prf(toks(c), gt_tok)[0] for c in c01]) * 100
            best_toks = maj_tokens(c07, NREP)
            best = prf(best_toks, gt_tok)[2] * 100
            best_p = prf(best_toks, gt_tok)[0] * 100
            d = best - base; dp = best_p - base_p
            note = f"P {base_p:.0f}→{best_p:.0f}% ({dp:+.0f})"
        else:
            facts = COVERAGE[cat]; bads = HALLUC.get(cat, [])
            base = single_fact_recall(c01, facts) * 100
            best = fact_recall(c07, facts) * 100
            d = best - base
            h0 = statistics.mean([halluc_count([c], bads) for c in c01])
            h7 = halluc_count(c07, bads)
            note = f"halluc {h0:.1f}→{h7}" if bads else "—"
        if cat in EXTRACTION_GT:
            verdict = ("✅ BENEFIT" if (d >= 2 and best_p >= base_p - 3)
                       else "⚠️ recall↑ prec↓" if d >= 2
                       else "❌ HURTS" if d <= -2 else "➖ neutral")
        else:
            verdict = "✅ BENEFIT" if d >= 2 else ("❌ HURTS" if d <= -2 else "➖ neutral")
        print(f"| {cat} | {base:.0f}% | {best:.0f}% | {d:+.0f} | {note} | {verdict} |")


if __name__ == "__main__":
    main()
