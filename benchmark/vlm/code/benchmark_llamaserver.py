#!/usr/bin/env python3
"""Benchmark vision models via llama-server directly (no Ollama).

For each model:
  1. Start llama-server as a subprocess with the model + mmproj
  2. Wait for /health
  3. Send each test image via the OpenAI-compatible /v1/chat/completions API
  4. Capture: response text, timing, prompt_tokens (for visual budget check),
     completion_tokens, tokens/s
  5. Kill the server
  6. Write results to JSONL (same schema as the Ollama-based benchmark)

Usage:
  python3 benchmark_llamaserver.py <model_name> <gguf_path> <mmproj_path> [--ctx N] [--gpu-layers N]

Examples:
  python3 benchmark_llamaserver.py gemma4-12b \\
    /Volumes/ssd/llm-models/gemma4-12b/gemma-4-12b-it-Q4_K_M.gguf \\
    /Volumes/ssd/llm-models/gemma4-12b/mmproj-F16.gguf

  python3 benchmark_llamaserver.py qwen3.6-27b \\
    /Volumes/ssd/llm-models/qwen3.6-27b/Qwen3.6-27B-Q4_K_M.gguf \\
    /Volumes/ssd/llm-models/qwen3.6-27b/mmproj-F16.gguf --ctx 16384
"""
import argparse
import base64
import json
import os
import re
import signal
import subprocess
import sys
import tempfile
import threading
import time
import urllib.request
import urllib.error
from pathlib import Path

LLAMA_SERVER = os.environ.get("LLAMA_SERVER", "llama-server")
_ROOT = Path(__file__).resolve().parent.parent  # code/ -> benchmark/vlm/
IMG_DIR = _ROOT / "test-images"
OUT_DIR = _ROOT / "benchmark-results"
RAW_OUT = OUT_DIR / "raw.jsonl"

UNIFORM_PROMPT = (
    "Describe this image in detail. Include: visible text (verbatim), "
    "objects, people, layout, colors, and any notable features. "
    "Use Markdown headings to organize your answer."
)

# Per-call timeout (seconds) for the HTTP request to llama-server.
# 300s is enough for normal cells; dense images + thinking models can take longer
# but we'd rather fail fast and record the timeout as data than wait 20 min.
CALL_TIMEOUT = 300

# Watchdog timeout (seconds). If a single cell exceeds this, the python wrapper
# kills the llama-server (in case it's hung mid-inference) and marks the cell as
# a timeout error. Slightly higher than CALL_TIMEOUT to allow normal cleanup.
WATCHDOG_TIMEOUT = 360


def encode_image(path):
    """Read image file and return (base64_data, format_str).

    WebP is silently dropped by llama-server's image decoder (stb_image),
    resulting in 0 image tokens and responses like "no image provided".
    Convert webp → png via sips (always available on macOS) before encoding.

    Returns:
        (b64_string, format) where format is the image format for the data URL
        (e.g. 'png', 'jpeg', 'webp').
    """
    src_fmt = path.suffix[1:].lower()

    if src_fmt == "webp":
        # Convert to PNG in a temp file. sips is macOS-native and always
        # available. PIL/Pillow would also work but adds a dependency.
        with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tmp:
            tmp_path = Path(tmp.name)
        try:
            result = subprocess.run(
                ["sips", "-s", "format", "png", str(path), "--out", str(tmp_path)],
                capture_output=True, text=True, timeout=30,
            )
            if result.returncode != 0:
                raise RuntimeError(f"sips failed: {result.stderr.strip()[:200]}")
            with open(tmp_path, "rb") as f:
                return base64.b64encode(f.read()).decode(), "png"
        finally:
            tmp_path.unlink(missing_ok=True)

    with open(path, "rb") as f:
        return base64.b64encode(f.read()).decode(), src_fmt


def list_images(pattern=None):
    """Return sorted list of test images, optionally filtered by regex.

    pattern: if given, only images whose filename matches the regex (re.search)
    are returned. Useful for re-running a subset (e.g. only new images 21-30
    via pattern '^(2[1-9]|30)-').
    """
    out = []
    rx = re.compile(pattern) if pattern else None
    for p in sorted(IMG_DIR.iterdir()):
        if p.suffix.lower() not in (".png", ".jpg", ".jpeg", ".webp"):
            continue
        if p.name.startswith("."):
            continue
        if rx and not rx.search(p.name):
            continue
        out.append(p)
    return out


def wait_for_health(port, timeout=180):
    """Poll /health until 200 or timeout."""
    url = f"http://127.0.0.1:{port}/health"
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=5) as resp:
                if resp.status == 200:
                    return True
        except (urllib.error.URLError, ConnectionError, OSError):
            pass
        time.sleep(1)
    return False


def start_server(gguf, mmproj, port, ctx, gpu_layers, max_vision_budget=False):
    """Start llama-server as a subprocess. Returns the Popen object.

    max_vision_budget: if True, sets --image-min-tokens 560 --image-max-tokens 2240.
        Only meaningful for models with dynamic resolution support (Gemma 4).
        Other models (Qwen3-VL, Qwen3.6, etc.) ignore these flags.
    """
    args = [
        LLAMA_SERVER,
        "-m", str(gguf),
        "--mmproj", str(mmproj),
        "--host", "127.0.0.1",
        "--port", str(port),
        "-ngl", str(gpu_layers),
        "-c", str(ctx),
        "--temp", "0.1",
        "--top-p", "0.95",
        "--top-k", "64",
        "-np", "1",  # single-slot (we benchmark sequentially)
        # Batch sizes: set high enough that image tokens (up to ~2240)
        # never get split across multiple physical batches. Default -ub 512
        # would split a 548-token Qwen3-VL image into 2 batches. With 4096,
        # everything fits in one pass regardless of model or budget.
        "-b", "4096",
        "-ub", "4096",
    ]

    if max_vision_budget:
        # Gemma 4 dynamic resolution: default 280 tokens is "essentially blind"
        # per community reports. 560/2240 gives 2-3x more visual detail.
        # Only affects models WITH dynamic resolution (Gemma 4 family).
        # Other models ignore these flags entirely.
        args.extend([
            "--image-min-tokens", "560",
            "--image-max-tokens", "2240",
        ])
    proc = subprocess.Popen(
        args,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        # Put the process in its own process group so we can kill it cleanly
        start_new_session=True,
    )
    return proc


def kill_server(proc):
    """Kill the llama-server subprocess and its children."""
    if proc.poll() is not None:
        return  # already exited
    try:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
        proc.wait(timeout=10)
    except (ProcessLookupError, subprocess.TimeoutExpired):
        try:
            os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
        except ProcessLookupError:
            pass


def run_one_call(port, image_path, prompt, max_tokens=16384, disable_thinking=False,
                 call_timeout=None, watchdog_timeout=None):
    """Send one image through llama-server. Returns dict with results.

    max_tokens default raised from 4096 to 16384: hybrid thinking models
    (Qwen3.5/3.6) spend tokens in a hidden reasoning phase before emitting
    visible output, and the cap applies to the COMBINED budget. At 4096,
    Q3.5-9B exhausted its budget thinking on dense images (manga, logo)
    and returned empty content with eval_count=4096 — scored wrongly as
    a quality failure.

    disable_thinking: if True, injects chat_template_kwargs.enable_thinking=false
    into the request body. Qwen3-family chat template honors this flag and skips
    the reasoning phase entirely. Use for Q3.5-9B / Q3.6-27B which otherwise
    exhibit runaway thinking on dense images (1200s timeouts, 0 visible chars).

    call_timeout: HTTP request timeout in seconds (default CALL_TIMEOUT).
    watchdog_timeout: hard cap in seconds; if exceeded, raises TimeoutError.
        Belt-and-suspenders for the case where llama-server hangs mid-inference
        and the urllib timeout doesn't fire.
    """
    if call_timeout is None:
        call_timeout = CALL_TIMEOUT
    if watchdog_timeout is None:
        watchdog_timeout = WATCHDOG_TIMEOUT

    img_b64, img_fmt = encode_image(image_path)

    payload = {
        "model": "local",
        "max_tokens": max_tokens,
        "temperature": 0.1,
        "messages": [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": prompt},
                    {
                        "type": "image_url",
                        "image_url": {
                            "url": f"data:image/{img_fmt};base64,{img_b64}"
                        },
                    },
                ],
            }
        ],
        "stream": False,
    }

    if disable_thinking:
        # Qwen3-family chat template honors enable_thinking via chat_template_kwargs.
        # Setting false skips the reasoning phase entirely, preventing runaway thinking
        # (Q3.5-9B: 0 visible chars on logo @ 16k token cap; Q3.6-27B: 1200s timeouts
        # on dense images). Other model families ignore this flag.
        payload["chat_template_kwargs"] = {"enable_thinking": False}

    url = f"http://127.0.0.1:{port}/v1/chat/completions"
    req = urllib.request.Request(
        url,
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    t0 = time.time()

    # Watchdog: force-kill if total cell time exceeds watchdog_timeout.
    # Uses a background thread that calls urllib timeout via raising exception
    # in the main thread. signal.alarm would be simpler but only works on Unix
    # main thread, and we may want this on workers later.
    watchdog_fired = threading.Event()

    def watchdog():
        if watchdog_timeout > 0:
            time.sleep(watchdog_timeout)
            watchdog_fired.set()

    wd_thread = threading.Thread(target=watchdog, daemon=True)
    wd_thread.start()

    try:
        with urllib.request.urlopen(req, timeout=call_timeout) as resp:
            data = json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode()[:500]
        return {
            "ok": False,
            "error": f"HTTP {e.code}: {body}",
            "elapsed_s": time.time() - t0,
        }
    except (urllib.error.URLError, TimeoutError, OSError) as e:
        elapsed = time.time() - t0
        if watchdog_fired.is_set():
            return {
                "ok": False,
                "error": f"watchdog timeout after {elapsed:.0f}s",
                "elapsed_s": elapsed,
            }
        return {
            "ok": False,
            "error": str(e),
            "elapsed_s": elapsed,
        }

    elapsed = time.time() - t0
    choice = data.get("choices", [{}])[0]
    message = choice.get("message", {})
    content = message.get("content", "") or ""
    reasoning_content = message.get("reasoning_content", "") or ""  # Qwen thinking phase
    thinking = message.get("thinking", "") or ""  # alt field some servers use
    finish_reason = choice.get("finish_reason", "")
    usage = data.get("usage", {})
    timings = data.get("timings", {})

    prompt_tokens = usage.get("prompt_tokens", 0)
    completion_tokens = usage.get("completion_tokens", 0)

    # Calculate tok/s from timings if available, else from elapsed
    predicted_ms = timings.get("predicted_ms", 0)
    if predicted_ms > 0 and completion_tokens > 0:
        tokens_per_s = completion_tokens / (predicted_ms / 1000)
    elif elapsed > 0 and completion_tokens > 0:
        tokens_per_s = completion_tokens / elapsed
    else:
        tokens_per_s = 0

    # Truncation flag: thinking model exhausted max_tokens before visible answer
    truncated = finish_reason == "length"

    return {
        "ok": True,
        "content": content,
        "reasoning_content": reasoning_content,
        "thinking": thinking,
        "finish_reason": finish_reason,
        "truncated": truncated,
        "elapsed_s": elapsed,
        "eval_count": completion_tokens,
        "prompt_eval_count": prompt_tokens,
        "tokens_per_s": tokens_per_s,
        "load_duration_s": 0,  # model already loaded; not per-call
    }


def load_done_combos(run_id=None):
    """Return set of (model, image) tuples already done successfully.
    If run_id is given, only counts results with that run_id (or missing run_id, which is treated as run '1').
    """
    done = set()
    if not RAW_OUT.exists():
        return done
    with open(RAW_OUT) as f:
        for line in f:
            try:
                rec = json.loads(line)
                if rec.get("type") == "result" and rec.get("ok"):
                    rec_run = rec.get("run_id", "1")  # default old records to run 1
                    # When checking for run_id X, skip records from other runs
                    if run_id is not None and rec_run != run_id:
                        continue
                    done.add((rec["model"], rec["image"]))
            except json.JSONDecodeError:
                continue
    return done


def main():
    parser = argparse.ArgumentParser(description="Benchmark VLM via llama-server")
    parser.add_argument("model_name", help="Model identifier (e.g. gemma4-12b)")
    parser.add_argument("gguf", help="Path to the model GGUF file")
    parser.add_argument("mmproj", help="Path to the mmproj projector file")
    parser.add_argument("--ctx", type=int, default=32768, help="Context window")
    parser.add_argument("--gpu-layers", type=int, default=-1, help="GPU layers (-1=all)")
    parser.add_argument("--port", type=int, default=8842, help="Base port for llama-server")
    parser.add_argument("--max-vision-budget", action="store_true",
                        help="Enable max visual token budget (560/2240). "
                             "Only for models with dynamic resolution (Gemma 4).")
    parser.add_argument("--max-tokens", type=int, default=16384,
                        help="max_tokens for completion request. "
                             "Hybrid thinking models (Qwen3.5/3.6) need >=8192 "
                             "because the cap applies to thinking + visible output combined.")
    parser.add_argument("--run-id", type=str, default="1",
                        help="Run identifier (1, 2, 3, ...). Used to differentiate "
                             "multi-pass runs. Default '1' for original single-pass.")
    parser.add_argument("--disable-thinking", action="store_true",
                        help="Disable thinking phase for Qwen3-family hybrid models. "
                             "Sets chat_template_kwargs.enable_thinking=false. Use for "
                             "Q3.5-9B / Q3.6-27B which exhibit runaway thinking on dense images.")
    parser.add_argument("--image-pattern", type=str, default=None,
                        help="Regex filter for image filenames (re.search). E.g. "
                             "'^(2[1-9]|30)-' runs only images 21-30. Used to re-run "
                             "a subset without re-running already-scored images.")
    parser.add_argument("--call-timeout", type=int, default=CALL_TIMEOUT,
                        help=f"HTTP request timeout per cell in seconds (default {CALL_TIMEOUT}). "
                             "Set lower to fail fast on slow/timeout-prone cells.")
    parser.add_argument("--watchdog-timeout", type=int, default=WATCHDOG_TIMEOUT,
                        help=f"Hard watchdog timeout per cell in seconds (default {WATCHDOG_TIMEOUT}). "
                             "If a cell exceeds this, the request is force-failed even if "
                             "llama-server is hung mid-inference.")
    args = parser.parse_args()

    gguf = Path(args.gguf)
    mmproj = Path(args.mmproj)

    if not gguf.exists():
        print(f"ERROR: GGUF not found: {gguf}", file=sys.stderr)
        sys.exit(1)
    if not mmproj.exists():
        print(f"ERROR: mmproj not found: {mmproj}", file=sys.stderr)
        sys.exit(1)

    images = list_images(pattern=args.image_pattern)
    done = load_done_combos(run_id=args.run_id)
    todo = [(img,) for img in images if (args.model_name, img.name) not in done]

    print(f"Model: {args.model_name}  (run_id={args.run_id})")
    print(f"GGUF:  {gguf} ({gguf.stat().st_size // (1024*1024)} MB)")
    print(f"MMPROJ: {mmproj} ({mmproj.stat().st_size // (1024*1024)} MB)")
    print(f"Images: {len(images)} total, {len(todo)} to do for run_id={args.run_id}")
    print(f"Output: {RAW_OUT}")
    print("=" * 78)

    if not todo:
        print("Nothing to do for this model.")
        return

    # Start llama-server
    print(f"\nStarting llama-server on port {args.port}...")
    t_start = time.time()
    proc = start_server(gguf, mmproj, args.port, args.ctx, args.gpu_layers,
                        max_vision_budget=args.max_vision_budget)
    load_time = 0

    print("Waiting for health...", end="", flush=True)
    if not wait_for_health(args.port, timeout=300):
        print(" FAILED")
        kill_server(proc)
        stderr = proc.stderr.read().decode()[:1000] if proc.stderr else ""
        print(f"llama-server did not become healthy.\nstderr:\n{stderr}", file=sys.stderr)
        sys.exit(1)
    load_time = time.time() - t_start
    print(f" healthy ({load_time:.1f}s to load)")

    # Run benchmark
    try:
        with open(RAW_OUT, "a") as out:
            for idx, (img,) in enumerate(todo, 1):
                print(f"\n>>> [{idx}/{len(todo)}] {args.model_name} × {img.name}", flush=True)
                t0 = time.time()
                result = run_one_call(args.port, img, UNIFORM_PROMPT,
                                      max_tokens=args.max_tokens,
                                      disable_thinking=args.disable_thinking,
                                      call_timeout=args.call_timeout,
                                      watchdog_timeout=args.watchdog_timeout)
                wall = time.time() - t0

                record = {
                    "type": "result",
                    "model": args.model_name,
                    "image": img.name,
                    "image_size_kb": img.stat().st_size // 1024,
                    "max_tokens_budget": args.max_tokens,
                    "run_id": args.run_id,
                    "thinking_disabled": args.disable_thinking,
                    **result,
                }

                if result["ok"]:
                    ptoks = result.get("prompt_eval_count", 0)
                    ctoks = result.get("eval_count", 0)
                    tps = result.get("tokens_per_s", 0)
                    trunc = result.get("truncated", False)
                    finish = result.get("finish_reason", "")
                    rlen = len(result.get("reasoning_content", "")) + len(result.get("thinking", ""))
                    flag = f" [TRUNCATED finish={finish} reasoning={rlen}c]" if trunc else ""
                    preview = result["content"][:100].replace("\n", " ")
                    print(f"    OK  [{result['elapsed_s']:.1f}s | {tps:.1f} tok/s | "
                          f"prompt={ptoks} out={ctoks} reasoning={rlen}c]{flag}  {preview}...")
                else:
                    print(f"    ERR [{result.get('elapsed_s', 0):.1f}s] {result.get('error', '')[:120]}")

                out.write(json.dumps(record) + "\n")
                out.flush()
    finally:
        print("\nShutting down llama-server...", end="", flush=True)
        kill_server(proc)
        print(" done.")

    print("\n" + "=" * 78)
    print(f"Done. Results appended to {RAW_OUT}")


if __name__ == "__main__":
    main()
