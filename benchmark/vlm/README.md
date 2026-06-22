# benchmark/vlm

Open-weights Vision-Language Model (VLM) benchmark used to pick the catalog for the [`localvision`](../../mcp/localvision) MCP server. 30 images × 15 model variants × 3 runs at Q4_K_M, plus a Q8_0 comparison on small/mid models.

## Where to look

- **[`BENCHMARK-REPORT-v5.md`](./BENCHMARK-REPORT-v5.md)** — single authoritative report. Master ranking table, per-model analysis, hardware-tier recommendations, Q4 vs Q8 guidance.
- **[`SUMMARY.md`](./SUMMARY.md)** — one-page cheat sheet: 3-tier hardware table.
- **[`CLAUDE.md`](./CLAUDE.md)** — operational context, gotchas, and run recipes for future Claude Code sessions working in this directory.
- **[`reddit-post-v6.md`](./reddit-post-v6.md)** — published post summarizing the v6 findings.

## What's here

```
benchmark/vlm/
├── BENCHMARK-REPORT-v5.md     # authoritative report (read this first)
├── SUMMARY.md                 # 3-tier quick reference
├── CLAUDE.md                  # ops context + run recipes
├── reddit-post-v6.md
├── code/                      # Python harness + scorers + shell orchestrators
│   ├── benchmark_llamaserver.py
│   ├── score_v5.py
│   ├── score_v5_multirun.py
│   ├── score_q8_multirun.py
│   ├── dispatch_multirun_judges.py
│   └── run_*.sh
├── test-images/               # 30 hand-curated images + GROUND-TRUTH.md
└── benchmark-results/
    ├── raw.jsonl              # ~2,200 raw llama-server responses (12 MB)
    ├── judgments_v5/          # 60 LLM-judge verdicts producing v5 scores
    └── judgments_q8/          # Q8 judge verdicts
```

## Reproducing

The harness needs `llama-server` on `$PATH` and the GGUF + mmproj files for each model. Models are pinned in `mcp/localvision/internal/models/builtin.toml` — the three v6 winners are:

- `Qwen3-VL-8B-Instruct-Q8_0.gguf` — constrained tier (12–16 GB)
- `Qwen3.5-4B-Q4_K_M.gguf` (nothink) — constrained fallback (4–8 GB)
- `Qwen3.6-27B-Q4_K_M.gguf` (nothink) — mainstream tier (24+ GB)

To rerun one model across the 30 images:

```bash
cd benchmark/vlm
python3 code/benchmark_llamaserver.py <name> <gguf> <mmproj> --run-id <id> [--disable-thinking]
python3 code/score_v5_multirun.py
```

The shell orchestrators (`code/run_*.sh`) handle the `cd` themselves — invoke them from anywhere.

See [`CLAUDE.md`](./CLAUDE.md) for full recipes, including the multi-run orchestrators and the LLM-judge dispatch flow.

## Provenance

Curated from a scratch `local-vlm-research/` directory that held ~60 hours of sustained inference across Q4 + Q8 + think/nothink variants. Superseded scripts (score.py through v4, benchmark.py) and intermediate analyses were dropped; only the v5/v6 load-bearing artifacts survived the move.

The benchmark tagged in `BENCHMARK-REPORT-v5.md` is what feed the `localvision` v0.2 catalog choices — every model in `mcp/localvision/internal/models/builtin.toml` is justified by a row in the master ranking table.
