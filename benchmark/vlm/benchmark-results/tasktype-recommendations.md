# Master ranking + per-tool recommendations (generated)

Regenerate: `PYTHONPATH=code python3 code/recommend_by_tasktype.py` (from `benchmark/vlm/`). Effective = (v5 final x run-success-rate) x (1 - k*sigma); sigma = across-run output divergence (1.0 if <2 ok runs). Master = all 24 incl MoE; per-tool = best effective dense.

<!-- VALIDATION: weighted /100 per quant (raw scorecard, pre-reliability/variability; must match) -->

**Q4** top 5: Q3.6-27B-nothink 79.6, Q3.6-27B 78.2, Q3.6-35B-A3B-nothink 76.4, Q3.5-4B-nothink 75.5, GLM-9B 75.1

**Q8** top 5: G4-12B-Q8 76.6, Q3VL-8B-Q8 74.4, GLM-9B-Q8 73.4, Q3.5-4B-Q8 66.1, Q3.5-4B-Q8-nothink 65.7

## Master ranking — all variants by effective quality

effective = (v5 final × run-success-rate) × (1 − 0.2·σ) over all 30 images — reliability AND across-run variability both counted. Raw = weighted scorecard /100 (pre-adjustment, for reference).

| # | Variant | Quant | Mode | Raw /100 | Effective | σ(text) | ok% | Latency s | Note |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Q3.6-27B-nothink | Q4 | nothink | 79.6 | 72.4 | 0.48 | 100 | 71 | **Champion** — best effective, 100% reliable |
| 2 | Q3.6-35B-A3B-nothink (MoE) | Q4 | nothink | 76.4 | 69.6 | 0.50 | 100 | 24 | MoE — fast + top-tier; not in per-tool recs |
| 3 | Q3VL-8B-Q8 | Q8 | think | 74.4 | 69.0 | 0.43 | 100 | 30 | **Best mid-tier** — Q8 strict win (0 fails, low σ); won describe_diagram + image_to_prompt |
| 4 | Q3.6-35B-A3B (MoE) | Q4 | think | 75.0 | 68.6 | 0.48 | 100 | 43 | MoE — think twin, slower than nothink |
| 5 | GLM-9B | Q4 | think | 75.1 | 68.5 | 0.47 | 100 | 37 | CJK-signage specialist; Q8 twin wins extract_text |
| 6 | G4-31B | Q4 | think | 74.6 | 68.5 | 0.45 | 100 | 97 | Slow (~97s); not better than Q3.6-27B — skip |
| 7 | Q3.5-4B-nothink | Q4 | nothink | 75.5 | 68.4 | 0.52 | 100 | 22 | **Best small** — fast, reliable; ≤20s menu workhorse |
| 8 | Q3VL-8B | Q4 | think | 73.1 | 68.0 | 0.41 | 100 | 28 | Sweet spot — fast, low σ; won diagnose_error |
| 9 | Q3.5-9B-nothink | Q4 | nothink | 73.1 | 67.0 | 0.48 | 100 | 32 | Solid mid; think twin wins read_image + extract_table |
| 10 | G4-26B-A4B (MoE) | Q4 | think | 72.7 | 66.7 | 0.47 | 100 | 39 | MoE — outclassed |
| 11 | GLM-9B-Q8 | Q8 | think | 73.4 | 66.6 | 0.50 | 93 | 62 | Won extract_text (CJK OCR); slight timeout risk (93%) |
| 12 | Q3.5-9B | Q4 | think | 72.7 | 66.6 | 0.50 | 100 | 61 | Won read_image + extract_table; nothink twin slightly higher |
| 13 | Q3.6-27B | Q4 | think | 78.2 | 65.1 | 0.51 | 92 | 123 | Won describe_ui + describe_chart; think mode slower/less reliable — use nothink |
| 14 | Q3.5-4B | Q4 | think | 70.6 | 64.1 | 0.53 | 100 | 47 | Think twin of best-small; nothink preferred |
| 15 | Q3VL-4B | Q4 | think | 65.9 | 61.9 | 0.42 | 100 | 28 | Tiny + fast; ≤20s menu pick; dense-scene runaways |
| 16 | Q3VL-4B-Q8 | Q8 | think | 65.3 | 60.1 | 0.43 | 94 | 36 | Unstable at Q8 (94% ok); Q4 preferred |
| 17 | G4-12B | Q4 | think | 64.1 | 58.9 | 0.52 | 100 | 63 | **Avoid** — hallucination flips ('Atomic acid' for 'Domoic acid') |
| 18 | Q3.5-4B-Q8-nothink | Q8 | nothink | 65.7 | 58.8 | 0.59 | 87 | 59 | Drop vs Q4 — timeout-prone at Q8 (87%) |
| 19 | Q3.5-9B-Q8-nothink | Q8 | nothink | — | 56.2 | 0.59 | 81 | 79 | Unusable at Q8 (81% ok) |
| 20 | G4-12B-Q8 | Q8 | think | 76.6 | 56.1 | 0.60 | 81 | 71 | ⚠ raw 76.6 looks good, fails ~1-in-5 (81% ok) |
| 21 | G4-E4B | Q4 | think | 58.8 | 54.1 | 0.54 | 100 | 22 | **Avoid** — perception fails |
| 22 | G4-E4B-Q8 | Q8 | think | 63.9 | 53.2 | 0.58 | 90 | 29 | Q8 helps, still weak (perception) |
| 23 | Q3.5-4B-Q8 | Q8 | think | 66.1 | 48.0 | 0.67 | 76 | 64 | Unusable at Q8 (76% ok, high σ) |
| 24 | Q3.5-9B-Q8 | Q8 | think | — | 45.3 | 0.67 | 62 | 138 | Unusable at Q8 (62% ok); slow |

<!-- Quality: per-cell quality = v5 final * run-success-rate (expected per-call quality); effective = quality * (1 - 0.2*sigma), sigma = pairwise token-Jaccard divergence of run outputs (1.0 if <2 ok runs). MoE shown & flagged but NOT recommended (dense). -->

## Per-tool recommendations — quality (dense models)

| Tool | n | Recommended | Mode | eff/100 | mean/100 | σ(text) | ok% |
|---|---|---|---|---|---|---|---|
| `read_image` | 11 | **Q3.5-9B** | think | 62.9 | 69.8 | 0.50 | 100 |
| `extract_text` | 5 | **GLM-9B-Q8** | think | 74.8 | 83.0 | 0.50 | 100 |
| `extract_code` | 1 | **Q3.6-27B-nothink** | nothink | 93.4 | 100.0 | 0.33 | 100 |
| `extract_table` | 2 | **Q3.5-9B** | think | 88.5 | 94.0 | 0.29 | 100 |
| `describe_ui` | 1 | **Q3.6-27B** | think | 93.8 | 100.0 | 0.31 | 100 |
| `describe_diagram` | 3 | **Q3VL-8B-Q8** | think | 82.5 | 89.1 | 0.37 | 100 |
| `describe_chart` | 2 | **Q3.6-27B** | think | 87.0 | 94.0 | 0.37 | 100 |
| `diagnose_error` | 1 | **Q3VL-8B** | think | 94.6 | 100.0 | 0.27 | 100 |
| `image_to_prompt` | 4 | **Q3VL-8B-Q8** | think | 74.6 | 82.3 | 0.47 | 100 |
| `compare_images` | 0 | — | — | — | — | — | — |

*eff = (v5 final × run-success-rate) × (1 − k·σ); mean = pre-reliability/variability; σ = across-run output divergence (1.0 if a cell had <2 successful runs); ok% = mean run-success rate. Dense models only — MoE are ranked in the master table above.*

<details><summary><b>read_image</b> — dense variants (n=11)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3.5-9B | 62.9 | 69.8 | 0.50 | 100 | 61 | 46 |
| Q3.6-27B-nothink | 60.6 | 67.5 | 0.51 | 100 | 76 | 16 |
| G4-31B | 60.0 | 66.4 | 0.48 | 100 | 103 | 15 |
| Q3.5-4B | 59.9 | 67.1 | 0.54 | 100 | 44 | 68 |
| GLM-9B | 59.3 | 65.8 | 0.50 | 100 | 42 | 44 |
| Q3.6-27B | 57.8 | 64.5 | 0.52 | 93 | 137 | 16 |
| Q3.5-4B-nothink | 57.3 | 64.0 | 0.52 | 100 | 29 | 68 |
| Q3VL-8B | 54.6 | 59.7 | 0.43 | 100 | 44 | 54 |
| Q3.5-9B-nothink | 54.5 | 60.7 | 0.51 | 100 | 43 | 47 |
| GLM-9B-Q8 | 54.3 | 61.1 | 0.55 | 91 | 75 | 33 |
| Q3VL-8B-Q8 | 54.0 | 59.6 | 0.47 | 100 | 43 | 38 |
| Q3.5-4B-Q8-nothink | 49.1 | 56.1 | 0.62 | 82 | 74 | 56 |
| Q3VL-4B-Q8 | 49.0 | 54.0 | 0.46 | 87 | 46 | 64 |
| G4-12B-Q8 | 47.5 | 54.2 | 0.61 | 80 | 75 | 25 |
| G4-12B | 45.8 | 51.4 | 0.55 | 100 | 92 | 34 |
| Q3.5-9B-Q8-nothink | 44.6 | 51.1 | 0.64 | 76 | 86 | 35 |
| Q3VL-4B | 43.6 | 47.9 | 0.44 | 100 | 55 | 80 |
| Q3.5-4B-Q8 | 41.7 | 48.7 | 0.73 | 74 | 64 | 57 |
| G4-E4B | 41.5 | 46.6 | 0.55 | 100 | 24 | 65 |
| G4-E4B-Q8 | 40.4 | 46.1 | 0.61 | 86 | 30 | 50 |
| Q3.5-9B-Q8 | 39.1 | 45.1 | 0.66 | 61 | 149 | 35 |
</details>

<details><summary><b>extract_text</b> — dense variants (n=5)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ GLM-9B-Q8 | 74.8 | 83.0 | 0.50 | 100 | 35 | 33 |
| Q3.6-27B-nothink | 73.2 | 82.5 | 0.56 | 100 | 64 | 16 |
| G4-12B-Q8 | 72.7 | 80.1 | 0.46 | 100 | 59 | 25 |
| G4-31B | 72.2 | 79.5 | 0.46 | 100 | 73 | 15 |
| Q3VL-8B-Q8 | 70.9 | 78.4 | 0.47 | 100 | 24 | 39 |
| GLM-9B | 68.6 | 76.3 | 0.50 | 100 | 31 | 44 |
| G4-12B | 67.7 | 76.4 | 0.57 | 100 | 39 | 35 |
| Q3.5-9B-Q8-nothink | 67.1 | 75.0 | 0.53 | 100 | 30 | 35 |
| G4-E4B-Q8 | 66.7 | 74.7 | 0.54 | 100 | 22 | 51 |
| Q3.5-4B | 64.5 | 72.7 | 0.57 | 100 | 29 | 67 |
| Q3VL-8B | 63.7 | 70.4 | 0.48 | 100 | 20 | 55 |
| Q3VL-4B | 62.8 | 69.0 | 0.45 | 100 | 13 | 82 |
| Q3.5-4B-Q8-nothink | 62.5 | 70.5 | 0.57 | 100 | 20 | 56 |
| Q3.5-9B-nothink | 62.0 | 69.1 | 0.52 | 100 | 27 | 47 |
| Q3.5-4B-nothink | 61.9 | 69.9 | 0.57 | 100 | 17 | 68 |
| Q3.5-9B | 59.7 | 67.1 | 0.55 | 100 | 86 | 46 |
| G4-E4B | 57.7 | 65.0 | 0.57 | 100 | 18 | 66 |
| Q3.6-27B | 56.5 | 64.8 | 0.64 | 80 | 108 | 16 |
| Q3.5-9B-Q8 | 52.1 | 59.9 | 0.65 | 73 | 106 | 35 |
| Q3.5-4B-Q8 | 50.8 | 57.5 | 0.58 | 87 | 71 | 57 |
| Q3VL-4B-Q8 | 49.1 | 54.6 | 0.51 | 100 | 54 | 64 |
</details>

<details><summary><b>extract_code</b> — dense variants (n=1)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3.6-27B-nothink | 93.4 | 100.0 | 0.33 | 100 | 71 | 16 |
| G4-31B | 92.6 | 100.0 | 0.37 | 100 | 68 | 15 |
| Q3.6-27B | 92.1 | 100.0 | 0.40 | 100 | 92 | 16 |
| Q3.5-9B-nothink | 91.8 | 100.0 | 0.41 | 100 | 21 | 46 |
| GLM-9B | 90.7 | 100.0 | 0.46 | 100 | 28 | 45 |
| Q3.5-9B | 90.6 | 100.0 | 0.47 | 100 | 39 | 48 |
| Q3VL-4B | 90.5 | 100.0 | 0.47 | 100 | 7 | 91 |
| Q3.5-4B-nothink | 90.2 | 100.0 | 0.49 | 100 | 13 | 70 |
| G4-12B | 90.1 | 100.0 | 0.49 | 100 | 46 | 34 |
| G4-E4B | 89.5 | 100.0 | 0.52 | 100 | 20 | 66 |
| Q3.5-4B | 89.4 | 100.0 | 0.53 | 100 | 24 | 68 |
| Q3VL-8B | 88.3 | 94.0 | 0.30 | 100 | 13 | 57 |
| Q3VL-4B-Q8 | 88.1 | 94.0 | 0.31 | 100 | 9 | 70 |
| Q3.5-9B-Q8 | 85.2 | 94.0 | 0.47 | 100 | 54 | 35 |
| Q3VL-8B-Q8 | 85.2 | 94.0 | 0.47 | 100 | 17 | 40 |
| G4-12B-Q8 | 84.9 | 94.0 | 0.48 | 100 | 63 | 24 |
| G4-E4B-Q8 | 84.4 | 94.0 | 0.51 | 100 | 27 | 52 |
| Q3.5-4B-Q8 | 84.0 | 94.0 | 0.53 | 100 | 27 | 58 |
| Q3.5-4B-Q8-nothink | 83.9 | 94.0 | 0.54 | 100 | 17 | 56 |
| Q3.5-9B-Q8-nothink | 79.4 | 88.0 | 0.49 | 100 | 29 | 35 |
| GLM-9B-Q8 | 78.8 | 88.0 | 0.52 | 100 | 47 | 34 |
</details>

<details><summary><b>extract_table</b> — dense variants (n=2)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3.5-9B | 88.5 | 94.0 | 0.29 | 100 | 57 | 47 |
| Q3.5-4B | 88.3 | 94.0 | 0.30 | 100 | 41 | 67 |
| Q3VL-4B | 88.3 | 94.0 | 0.31 | 100 | 21 | 80 |
| Q3.5-9B-Q8 | 88.1 | 95.1 | 0.37 | 100 | 75 | 35 |
| Q3VL-4B-Q8 | 87.0 | 93.1 | 0.33 | 100 | 23 | 63 |
| GLM-9B-Q8 | 87.0 | 93.8 | 0.36 | 100 | 68 | 33 |
| Q3.5-4B-Q8 | 86.9 | 92.5 | 0.30 | 100 | 53 | 56 |
| Q3VL-8B | 86.6 | 92.7 | 0.33 | 100 | 31 | 53 |
| Q3.5-9B-Q8-nothink | 86.2 | 93.6 | 0.40 | 100 | 56 | 35 |
| Q3.5-9B-nothink | 86.1 | 94.0 | 0.42 | 100 | 41 | 47 |
| Q3.5-4B-nothink | 86.0 | 94.0 | 0.43 | 100 | 28 | 68 |
| G4-12B-Q8 | 84.3 | 89.8 | 0.31 | 100 | 103 | 24 |
| Q3.6-27B-nothink | 82.8 | 91.0 | 0.45 | 100 | 100 | 16 |
| Q3.6-27B | 82.2 | 87.1 | 0.28 | 100 | 166 | 16 |
| GLM-9B | 81.8 | 87.8 | 0.34 | 100 | 63 | 43 |
| Q3VL-8B-Q8 | 80.5 | 85.1 | 0.27 | 100 | 41 | 38 |
| Q3.5-4B-Q8-nothink | 79.4 | 88.0 | 0.49 | 100 | 33 | 55 |
| G4-31B | 68.2 | 74.0 | 0.39 | 100 | 183 | 14 |
| G4-E4B-Q8 | 68.0 | 75.3 | 0.49 | 100 | 42 | 50 |
| G4-E4B | 57.2 | 64.2 | 0.55 | 100 | 29 | 65 |
| G4-12B | 43.4 | 48.2 | 0.50 | 100 | 67 | 34 |
</details>

<details><summary><b>describe_ui</b> — dense variants (n=1)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3.6-27B | 93.8 | 100.0 | 0.31 | 100 | 85 | 16 |
| GLM-9B | 92.3 | 100.0 | 0.39 | 100 | 27 | 45 |
| Q3.6-27B-nothink | 92.1 | 100.0 | 0.40 | 100 | 53 | 16 |
| G4-31B | 90.9 | 100.0 | 0.46 | 100 | 67 | 15 |
| Q3.5-4B-nothink | 90.8 | 100.0 | 0.46 | 100 | 13 | 70 |
| Q3VL-8B-Q8 | 89.6 | 94.0 | 0.23 | 100 | 11 | 40 |
| Q3.5-9B-Q8-nothink | 88.3 | 94.0 | 0.30 | 100 | 21 | 35 |
| G4-12B-Q8 | 87.4 | 94.0 | 0.35 | 100 | 59 | 24 |
| Q3.5-9B-Q8 | 87.4 | 94.0 | 0.35 | 100 | 38 | 35 |
| Q3VL-4B-Q8 | 87.4 | 94.0 | 0.35 | 100 | 8 | 69 |
| Q3.5-9B | 86.8 | 94.0 | 0.38 | 100 | 30 | 47 |
| G4-12B | 85.3 | 94.0 | 0.46 | 100 | 45 | 34 |
| Q3.5-4B | 85.1 | 94.0 | 0.47 | 100 | 23 | 65 |
| Q3.5-4B-Q8-nothink | 84.6 | 94.0 | 0.50 | 100 | 15 | 57 |
| GLM-9B-Q8 | 84.6 | 94.0 | 0.50 | 100 | 41 | 34 |
| Q3VL-8B | 84.0 | 88.0 | 0.23 | 100 | 7 | 58 |
| Q3.5-9B-nothink | 82.8 | 88.0 | 0.30 | 100 | 16 | 47 |
| Q3VL-4B | 80.3 | 88.0 | 0.44 | 100 | 7 | 91 |
| Q3.5-4B-Q8 | 68.5 | 76.0 | 0.50 | 100 | 25 | 58 |
| G4-E4B-Q8 | 57.9 | 64.0 | 0.48 | 100 | 27 | 50 |
| G4-E4B | 57.7 | 64.0 | 0.49 | 100 | 20 | 66 |
</details>

<details><summary><b>describe_diagram</b> — dense variants (n=3)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3VL-8B-Q8 | 82.5 | 89.1 | 0.37 | 100 | 15 | 40 |
| Q3VL-8B | 82.5 | 90.2 | 0.43 | 100 | 12 | 57 |
| GLM-9B-Q8 | 79.9 | 87.3 | 0.43 | 100 | 28 | 34 |
| Q3.6-27B-nothink | 79.8 | 86.7 | 0.40 | 100 | 57 | 16 |
| Q3.5-9B-Q8-nothink | 76.6 | 85.2 | 0.51 | 100 | 25 | 35 |
| GLM-9B | 75.5 | 83.1 | 0.46 | 100 | 21 | 46 |
| Q3.5-4B-nothink | 73.6 | 82.4 | 0.54 | 100 | 14 | 69 |
| Q3VL-4B | 72.7 | 79.2 | 0.41 | 100 | 8 | 90 |
| Q3VL-4B-Q8 | 72.4 | 79.0 | 0.41 | 100 | 10 | 69 |
| Q3.5-9B-nothink | 70.7 | 78.0 | 0.47 | 100 | 19 | 48 |
| Q3.6-27B | 70.3 | 76.8 | 0.42 | 100 | 102 | 16 |
| Q3.5-4B-Q8-nothink | 68.8 | 76.5 | 0.51 | 100 | 17 | 56 |
| G4-12B | 67.0 | 73.8 | 0.46 | 100 | 45 | 34 |
| Q3.5-9B | 65.2 | 72.3 | 0.49 | 100 | 35 | 47 |
| G4-31B | 63.8 | 69.0 | 0.38 | 100 | 76 | 15 |
| Q3.5-4B | 59.1 | 66.3 | 0.54 | 100 | 73 | 69 |
| G4-E4B | 58.0 | 65.2 | 0.55 | 100 | 20 | 64 |
| G4-E4B-Q8 | 57.2 | 63.4 | 0.49 | 100 | 25 | 51 |
| G4-12B-Q8 | 39.3 | 47.2 | 0.84 | 56 | 62 | 25 |
| Q3.5-4B-Q8 | 35.4 | 42.7 | 0.85 | 53 | 70 | 58 |
| Q3.5-9B-Q8 | 26.1 | 31.3 | 0.84 | 33 | 214 | 36 |
</details>

<details><summary><b>describe_chart</b> — dense variants (n=2)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3.6-27B | 87.0 | 94.0 | 0.37 | 100 | 147 | 16 |
| Q3.6-27B-nothink | 84.0 | 91.0 | 0.38 | 100 | 78 | 16 |
| Q3.5-9B | 83.9 | 91.0 | 0.39 | 100 | 64 | 47 |
| Q3.5-9B-nothink | 80.2 | 88.0 | 0.44 | 100 | 24 | 47 |
| Q3.5-4B-nothink | 79.0 | 88.0 | 0.51 | 100 | 23 | 67 |
| Q3VL-8B | 76.3 | 82.0 | 0.34 | 100 | 17 | 57 |
| G4-31B | 75.6 | 82.2 | 0.40 | 100 | 115 | 15 |
| Q3VL-8B-Q8 | 74.5 | 80.5 | 0.37 | 100 | 25 | 40 |
| GLM-9B | 74.3 | 80.9 | 0.41 | 100 | 37 | 46 |
| Q3VL-4B | 73.8 | 79.0 | 0.33 | 100 | 14 | 87 |
| GLM-9B-Q8 | 70.0 | 76.1 | 0.40 | 100 | 52 | 34 |
| G4-E4B | 67.8 | 74.6 | 0.46 | 100 | 26 | 64 |
| G4-12B | 64.8 | 71.1 | 0.44 | 100 | 57 | 34 |
| Q3.5-4B | 44.9 | 51.0 | 0.60 | 100 | 77 | 70 |
| Q3VL-4B-Q8 | 44.7 | 48.0 | 0.35 | 100 | 62 | 65 |
| G4-E4B-Q8 | 40.2 | 47.0 | 0.72 | 62 | 36 | 51 |
| Q3.5-4B-Q8-nothink | 34.7 | 41.0 | 0.77 | 50 | 162 | 56 |
| G4-12B-Q8 | 18.0 | 22.5 | 1.00 | 25 | 83 | 25 |
| Q3.5-4B-Q8 | 10.9 | 13.7 | 1.00 | 17 | 170 | 57 |
| Q3.5-9B-Q8-nothink | 0.0 | 0.0 | 1.00 | 0 | 300 | nan |
| Q3.5-9B-Q8 | 0.0 | 0.0 | 1.00 | 0 | 300 | nan |
</details>

<details><summary><b>diagnose_error</b> — dense variants (n=1)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3VL-8B | 94.6 | 100.0 | 0.27 | 100 | 9 | 58 |
| Q3VL-8B-Q8 | 93.1 | 100.0 | 0.34 | 100 | 15 | 41 |
| GLM-9B | 92.9 | 100.0 | 0.35 | 100 | 24 | 46 |
| G4-12B | 92.7 | 100.0 | 0.36 | 100 | 43 | 33 |
| G4-E4B | 92.5 | 100.0 | 0.37 | 100 | 21 | 66 |
| GLM-9B-Q8 | 92.0 | 100.0 | 0.40 | 100 | 35 | 35 |
| G4-31B | 91.7 | 100.0 | 0.41 | 100 | 66 | 15 |
| Q3.6-27B | 91.3 | 100.0 | 0.44 | 100 | 101 | 16 |
| G4-12B-Q8 | 91.0 | 100.0 | 0.45 | 100 | 63 | 25 |
| Q3.5-9B-Q8-nothink | 90.9 | 100.0 | 0.46 | 100 | 26 | 35 |
| Q3.5-9B-nothink | 90.7 | 100.0 | 0.46 | 100 | 23 | 47 |
| Q3.5-4B-nothink | 89.9 | 100.0 | 0.51 | 100 | 9 | 69 |
| Q3.6-27B-nothink | 89.8 | 100.0 | 0.51 | 100 | 59 | 16 |
| Q3.5-4B | 88.5 | 97.1 | 0.45 | 100 | 21 | 71 |
| Q3.5-4B-Q8-nothink | 88.0 | 97.1 | 0.47 | 100 | 18 | 57 |
| Q3VL-4B-Q8 | 87.7 | 94.0 | 0.34 | 100 | 9 | 70 |
| G4-E4B-Q8 | 83.6 | 91.1 | 0.41 | 100 | 25 | 52 |
| Q3VL-4B | 76.2 | 82.0 | 0.35 | 100 | 7 | 90 |
| Q3.5-4B-Q8 | 50.5 | 55.4 | 0.44 | 100 | 28 | 59 |
| Q3.5-9B-Q8 | 26.7 | 33.3 | 1.00 | 33 | 51 | 36 |
| Q3.5-9B | 16.6 | 20.0 | 0.84 | 100 | 148 | 47 |
</details>

<details><summary><b>image_to_prompt</b> — dense variants (n=4)</summary>

| variant | eff/100 | mean/100 | σ(text) | ok% | lat s | tok/s |
|---|---|---|---|---|---|---|
| ⭐ Q3VL-8B-Q8 | 74.6 | 82.3 | 0.47 | 100 | 25 | 39 |
| Q3.5-9B-nothink | 73.6 | 81.9 | 0.51 | 100 | 28 | 47 |
| Q3.6-27B-nothink | 73.3 | 80.8 | 0.46 | 100 | 62 | 16 |
| Q3.5-4B-nothink | 73.0 | 81.1 | 0.50 | 100 | 19 | 69 |
| Q3VL-8B | 70.6 | 77.9 | 0.46 | 100 | 21 | 55 |
| G4-31B | 70.1 | 77.8 | 0.50 | 100 | 98 | 15 |
| Q3.5-9B | 70.0 | 78.5 | 0.54 | 100 | 43 | 46 |
| Q3VL-4B | 69.2 | 76.0 | 0.45 | 100 | 15 | 83 |
| Q3VL-4B-Q8 | 69.2 | 75.8 | 0.44 | 94 | 17 | 65 |
| GLM-9B | 62.0 | 69.2 | 0.52 | 100 | 41 | 44 |
| G4-12B | 60.2 | 67.0 | 0.50 | 100 | 46 | 34 |
| Q3.5-4B | 59.9 | 67.5 | 0.56 | 100 | 67 | 68 |
| Q3.5-4B-Q8 | 59.6 | 68.2 | 0.63 | 83 | 32 | 58 |
| Q3.5-9B-Q8 | 58.6 | 66.5 | 0.59 | 75 | 110 | 35 |
| G4-12B-Q8 | 57.4 | 65.3 | 0.60 | 81 | 68 | 25 |
| Q3.5-4B-Q8-nothink | 56.7 | 64.8 | 0.63 | 75 | 90 | 56 |
| GLM-9B-Q8 | 55.8 | 63.1 | 0.58 | 75 | 105 | 34 |
| Q3.5-9B-Q8-nothink | 54.7 | 62.6 | 0.63 | 75 | 99 | 35 |
| G4-E4B | 54.2 | 60.9 | 0.56 | 100 | 23 | 65 |
| Q3.6-27B | 53.9 | 62.0 | 0.65 | 81 | 111 | 16 |
| G4-E4B-Q8 | 52.1 | 59.8 | 0.64 | 81 | 30 | 51 |
</details>

## Latency-budget menu — best effective quality within a wall-clock budget

| Tool | ≤20s | ≤45s | ≤90s | unlimited |
|---|---|---|---|---|
| `read_image` | — | Q3.5-4B | Q3.5-9B | Q3.5-9B |
| `extract_text` | Q3VL-4B | GLM-9B-Q8 | GLM-9B-Q8 | GLM-9B-Q8 |
| `extract_code` | Q3VL-4B | Q3.5-9B-nothink | Q3.6-27B-nothink | Q3.6-27B-nothink |
| `extract_table` | — | Q3.5-4B | Q3.5-9B | Q3.5-9B |
| `describe_ui` | Q3.5-4B-nothink | GLM-9B | Q3.6-27B | Q3.6-27B |
| `describe_diagram` | Q3VL-8B-Q8 | Q3VL-8B-Q8 | Q3VL-8B-Q8 | Q3VL-8B-Q8 |
| `describe_chart` | Q3VL-8B | Q3.5-9B-nothink | Q3.6-27B-nothink | Q3.6-27B |
| `diagnose_error` | Q3VL-8B | Q3VL-8B | Q3VL-8B | Q3VL-8B |
| `image_to_prompt` | Q3.5-4B-nothink | Q3VL-8B-Q8 | Q3VL-8B-Q8 | Q3VL-8B-Q8 |
| `compare_images` | — | — | — | — |
