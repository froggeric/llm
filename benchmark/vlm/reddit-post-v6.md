# Best local model for vision - 2nd benchmark update - 21 Jun 2026

I previously posted the first results of my [VLM benchmark](https://www.reddit.com/r/LocalLLaMA/comments/1u5oydc/which_is_the_best_local_vlm_benchmark_results/). There were a few useful comments and observations I took into account, to revise and expand my benchmark:

* I initially did not take into account the Gemma 4 **vision budget** which defaults to 280, essentially making it useless. I have increased it to maximum level, with the following optimal setttings which were posted here recently: `--image-min-tokens 560 --image-max-tokens 2240`
* I used the `-b 4096 -ub 4096` parameters to **avoid splitting the image tokens** into multiple blocks (default value is 512)
* Switched from ollama to **llama.cpp**
* I expanded my dataset from 20 to **30 images,** to cover more use cases
* I expanded the benchmark to test the impact of **thinking vs non-thinking**
* The first benchmark only included Q4 quants; I expanded it to **Q8 quants for small models**
* The first benchmark only tested each image once; now **3x tests per image**

In total, 23 models x 30 images x 3 tests = **2,070 tests** (not including failures, tunings, re-runs), **60 to 70 inference hours**.

# I have three recommendations this time, one per hardware tier:

|VRAM tier|Pick|Size|Score|Speed|
|:-|:-|:-|:-|:-|
|**4–8 GB**|**Qwen3.5 4B (nothink) @ Q4**|3.2 GB|75.5/100|20 s/img|
|**12–16 GB**|**Qwen3-VL 8B @ Q8** (not Q4)|8.1 GB|74.4/100|26 s/img|
|**24+ GB**|**Qwen3.6 27B (nothink) @ Q4**|16.9 GB|79.6/100|70 s/img|

I noticed a few interesting outcomes, which I did not expect:

**Thinking mode hurts vision.** Every Qwen hybrid thinker scored higher with `enable_thinking=false`. This is because *vision is perception, not reasoning.* Thinking adds instability, timeouts, and empty outputs.

**MoE size is misleading for vision.** MoE models tie with much smaller dense models, and perform worse than equivalent dense models. It makes sense in retrospect if when you see that a MoE is a collection of small models. Their big total parameter count buys knowledge breadth, not perception depth which scales with density.

**Q8 is not a guaranteed improvement.** It improves Gemma 4 (more consistent, less hallucinations), cripples Qwen hybrid thinkers (they spend too long thinking, resulting in frequent timeouts). The only Q8 that's a strict win is Qwen3-VL 8B-Q8.

# Here are the full quality ranking, sorted by effective score (raw × completion rate). σ = stability across 3 runs.

|\#|Variant|Quant|Mode|Score|σ|Successful|Note|
|:-|:-|:-|:-|:-|:-|:-|:-|
|1|Qwen3.6 27B|Q4|nothink|**79.6**|0.24|90/90|Champion|
|2|Qwen3.6 27B|Q4|think|78.2|0.26|81/90|Same model, slower|
|3|Qwen3.6 35B-A3B|Q4|nothink|76.4|0.55|90/90|MoE|
|4|Qwen3.5 4B|Q4|nothink|75.5|0.48|90/90|Best pts/GB|
|5|GLM-4.6V-Flash 9B|Q4|—|75.1|0.53|90/90|Best for chinese OCR|
|6|Qwen3.6 35B-A3B|Q4|think|75.0|0.31|90/90|MoE|
|7|Gemma 4 31B|Q4|—|74.6|0.45|90/90|Slow (93 s)|
|8|Qwen3-VL 8B|Q8|—|74.4|0.33|90/90|Only perfect Q8|
|9|Qwen3-VL 8B|Q4|—|73.1|0.52|90/90||
|10|Qwen3.5 9B|Q4|nothink|73.1|0.58|90/90||
|11|Gemma 4 26B-A4B|Q4|—|72.7|0.51|90/90||
|12|Qwen3.5 9B|Q4|think|72.7|0.52|90/90||
|13|GLM-9B|Q8|—|73.4 raw / 68.5 eff|0.51|84/90|Drop vs Q4|
|14|Qwen3.5 4B|Q4|think|70.6|0.77|90/90|Unstable|
|15|Qwen3-VL 4B|Q4|—|65.9|0.76|90/90|Degenerates|
|16|Qwen3.5 4B|Q8|nothink|65.7|0.51|partial|Drop vs Q4|
|17|Qwen3-VL 4B|Q8|—|65.3|1.03|87/93|Worst σ|
|18|Gemma 4 12B|Q8|—|76.6 raw / 59.7 eff|0.28|74/95|22% timeouts|
|19|Gemma 4 12B|Q4|—|64.1|0.66|90/90|Hallucinations|
|20|Gemma 4 E4B|Q8|—|63.9|0.46|78/90||
|21|Gemma 4 E4B|Q4|—|58.8|0.60|90/90|Wrong counts|
|22|Qwen3.5 9B|Q8|nothink|partial|—|\~85% fail|Unusable|
|23|Qwen3.5 9B|Q8|think|partial|—|\~60% fail|Unusable|

Here is bit more info about some of those models, that the above numbers cannot express, based on reading their actual output:

**Qwen3.6-27B** (Q4=16.9GB) : Best quality, best stability, no failures with thinking disabled. The no-thinking mode has a huge beneficial on speed, and avoids the timeouts due to reasoning too long. Gives very direct answers.

**Qwen3.6-35B-A3B** (Q4=21.9GB) : Based on the numbers it might appear like a good speedy alternatives, but it rarely performs better than smaller models. Biggest problem, apart from its size, is the huge variance and unpredictability of its responses. Skip it, not worth using MoE for vision.

**Qwen3-VL-8B-Instruct (Q4=5.8GB Q8=8.1GB)** : The only model with 100% reliability on Q8. Q8 brings big over Q4, for both quality and consistency.

**Qwen3.5-4B** (Q4=3.2GB) : Use with thinking disabled; when enabled, on dense images, it can easily exhaust its token budget and error, or timeout. Q8 was a lot worse than Q4, with again timeouts on dense images. None of those problems with Q4 non-thinking. 


### Test methodology

* specs: Apple M2 Max, 96GB RAM
* runtime: llama.cpp b9690 via llama-server
* models: 11 base models, Q4\_K\_M; Q8\_0 added for 7 of the smaller ones
* hybrid thinking models (Qwen3.5/3.6) tested both with and without thinking enabled
* 30 images across screenshots, photos, posters, art, medical, scientific graphs, dense scenes, and multilingual content
* 3 runs per (model × image), median run scored
* hybrid scoring: 40% deterministic probes (OCR, counts, hallucination checks) + 60% LLM judge based on human created detailed ground truth description for each image
* timeout: 300s per call (fail fast on runaway thinking)


### More info on Gemma 4 vision token budget

> In llama.cpp, you can configure Gemma 4's vision budget with 2 parameters `--image-min-tokens` and `--image-max-tokens`. The engine will try to fit the image within those bounds. I believe the default is 40 and 280 respectively. This is Gemma 4's default from Google's side but it's way too low.

> I like to run them at 560 and 2240 respectively and it's able to pick up very minute and hazy details within images.
Why 2240 - isn't that double of the max from Google (1120)? In my testing, 2240 for some reason works better than 1120. I suspect this might be because of llama.cpp's implementation where it tries to fit the image between min and max tokens.

> Also, weirdly, 560 and 2240 was outperforming 1120 and 1120 in my testing. I suspect this is because the model is capable of more than 1120 max tokens.

Someone asked why not put both `--image-min-tokens` and `--image-max-tokens` to 1120

> This will upscale anything that is less than 1120 (~2.6M pixels). If you want the original size of the image to be maintained, ideally should provide a lower and upper bound.

Source: https://www.reddit.com/r/LocalLLaMA/comments/1srrhi5/gemma_4_vision/