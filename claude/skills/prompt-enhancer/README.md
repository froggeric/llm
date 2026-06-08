# Prompt Enhancer

A native Claude Code skill that transforms vague requests into precise, actionable prompts before execution. Context-aware, evidence-based, and universal — works for coding, research, creative writing, image generation, audio work, and any other task.

## Installation

Copy the skill into your Claude Code skills directory:

```bash
mkdir -p ~/.claude/skills/prompt-enhancer
cp SKILL.md ~/.claude/skills/prompt-enhancer/
```

Claude Code auto-discovers skills on next session start. No hooks, no scripts, no configuration.

**Verify it's loaded:** Start a Claude Code session and type a vague request like `fix the bug`. If the skill is active, Claude will present an enhanced prompt rather than immediately searching your codebase.

## Usage

### Automatic

The skill triggers automatically when it detects a vague or ambiguous request. Just type normally.

```
> fix the bug
```

The skill intercepts, diagnoses what's missing, gathers lightweight context if needed, and presents an enhanced prompt for your approval.

### Explicit

Ask Claude to enhance any prompt:

```
> enhance this prompt: write an article about AI safety
> sharpen this request: refactor the API
> improve this: make the dashboard faster
```

### Bypass

Prefix with `*` to skip enhancement and execute directly:

```
> *fix the typo in README.md
```

## Examples

These are real test results, not hypotheticals.

### Example 1: Vague Coding Request

**Input:**
```
fix the hero section
```

**Without the skill** (baseline test):
Claude searched for "hero" across the entire codebase — including a Pac-Man game that had no hero section. It couldn't identify the target, listed ambiguities ("which file? which element?"), and produced no actionable output.

**With the skill:**
Claude identified this was a front-end task, quickly located the relevant component file, and produced:

```
Fix the hero section in index.html (lines 45-67) where:
- Background image fails to load on mobile breakpoints (<768px)
- CTA button text overflows its container at 320px width
- Heading uses a deprecated font-stack

Approach: Fix CSS media queries and update font reference.
Constraints: Preserve existing desktop layout (1280px+).
Done when: Hero renders correctly at 320px, 768px, and 1280px widths.
```

### Example 2: Creative Writing Request

**Input:**
```
write something about machine learning
```

**Enhanced:**
```
Write a 1,500-word beginner-friendly explainer on how neural networks learn,
covering forward propagation, backpropagation, and gradient descent with
real-world analogies (no math notation). Tone: conversational but authoritative,
like a patient professor. Audience: professionals considering a career pivot
into ML. Format: blog post with section headers.
```

### Example 3: Research Request

**Input:**
```
research quantum computing
```

**Enhanced:**
```
Investigate the current state of quantum error correction (2024-2026), covering:
surface codes, logical qubit thresholds, and recent breakthroughs from IBM/Google.
Deliver as a structured summary with key metrics (qubit counts, error rates,
correction overhead). Focus on practical implications for cryptography timelines.
```

### Example 4: Ambiguous but Runnable (80%+ clear)

**Input:**
```
add a dark mode toggle to the settings page
```

The skill recognizes this is specific enough (target, action, and location are all clear). It enhances silently in reasoning and executes directly — no interruption.

## Philosophy

### Why not just be smarter about prompting?

Because vague prompts waste the most expensive resource: **attention**. Every token a model spends guessing your intent is a token not spent solving your problem. The skill doesn't add complexity — it removes ambiguity.

### Why a skill, not a hook or script?

Three reasons:

1. **Transparency.** Hooks run silently. The skill shows you exactly what changed and waits for approval. You stay in control.

2. **Context-awareness.** A script can't decide whether to explore your codebase. The skill reads 1-2 files when the prompt references specific code, and skips exploration entirely for creative or general requests.

3. **Zero overlap.** Hook-based prompt improvers (like severity1/claude-code-prompt-improver) include workflow nudges that overlap with existing skills and Claude Code's built-in features. This skill does one thing: prompt enhancement.

### Why evidence-based?

The prompt engineering field has a replication crisis. A 2024 arXiv study (2409.20303) found that many prompting techniques don't replicate across models and tasks. The skill's design is grounded in what actually works on frontier models, not what went viral on Twitter.

## How It Works

```
User request
    │
    ▼
┌─────────────┐    specific enough    ┌──────────────┐
│  Diagnose   │ ─────────────────────►│    Execute    │
│             │                       │   directly    │
│ What's      │    ambiguous          └──────────────┘
│ missing?    │ ─────────┐
└─────────────┘          │
                         ▼
               ┌─────────────────┐
               │    Context      │
               │  (if needed)    │
               │                 │
               │ Code refs? →    │
               │  quick grep     │
               │ Prior work? →   │
               │  check history  │
               │ General? → skip │
               └────────┬────────┘
                        │
                        ▼
               ┌─────────────────┐
               │    Enhance      │
               │                 │
               │ [INTENT]        │
               │ [SCOPE]         │
               │ [APPROACH]      │
               │ [CONSTRAINTS]   │
               │ [DELIVERABLE]   │
               │ [DONE WHEN]     │
               └────────┬────────┘
                        │
                        ▼
               ┌─────────────────┐
               │    Present      │
               │                 │
               │ Enhanced prompt │
               │ + what changed  │
               │ + max 3 questions│
               │                 │
               │ Wait for approval│
               └─────────────────┘
```

### The Enhancement Framework

The skill uses a structured template adapted from the CO-STAR framework (winner of Singapore's GPT-4 prompt engineering competition):

| Section | Purpose | When to include |
|---------|---------|-----------------|
| **[INTENT]** | What the user actually wants | Always |
| **[SCOPE]** | What's in and out of bounds | When ambiguous |
| **[APPROACH]** | How to do it | When the method matters |
| **[CONSTRAINTS]** | Must-follow rules | When restrictions exist |
| **[DELIVERABLE]** | Expected output format | When format is unclear |
| **[APPROACH]** | How to verify success | When criteria is missing |

Sections that add clarity are included. Sections that would be padding are dropped. A 3-line enhanced prompt beats a 20-line one if it captures the same intent.

### Key Design Decisions

| Decision | Reasoning |
|----------|-----------|
| Max 3 questions to user | More questions = user stops reading. Present your best interpretation instead. |
| 1-2 file exploration limit | Context rot research: more tokens ≠ better results. Stop early. |
| Skip at 80%+ clarity | Unnecessary structuring can hurt (CoT research on frontier models). |
| Positive framing only | "Use tabs" beats "Don't use spaces" (Anthropic + OpenAI recommendation). |
| Bypass with `*` prefix | Respects user agency. Sometimes you just want it done. |

## Prompt Engineering Guide

Based on a comprehensive review of 2024-2026 research. What actually works on frontier models (Claude 4, GPT-4.1) — and what's mythology.

### What Works

#### 1. Structure, not magic phrases

The single most effective technique. Organize your prompt with clear sections (intent, scope, constraints, deliverable) rather than searching for the perfect incantation. XML tags work particularly well with Claude — it's fine-tuned to parse them.

**Evidence:** Anthropic's official recommendation. The CO-STAR framework won Singapore's national GPT-4 competition.

#### 2. Positive framing

Say what TO do, not what NOT to do. "Use tab indentation" beats "Don't use spaces." This avoids the "pink elephant paradox" — telling a model to avoid X requires it to process X.

**Evidence:** Official Anthropic and OpenAI guidance. eval.16x.engineer's analysis of negative instruction effectiveness.

#### 3. Permission for uncertainty

Explicitly allow "I don't know." LLMs default to confident answers because benchmarks reward confidence over honesty. Giving permission to express uncertainty reduces hallucination.

**Evidence:** Anthropic docs report "drastically reduced false information." Academic research (TruthRL) formalizes this.

#### 4. Specificity over verbosity

Better structure, not more words. "Fix the off-by-one error in the pagination loop" beats a 200-word explanation of why pagination matters.

**Evidence:** Anthropic's context engineering paradigm: the smallest set of high-signal tokens wins. The "attention budget" is finite.

#### 5. Diverse canonical examples

A few high-quality examples demonstrating expected behavior beat exhaustive edge case lists. Quality over quantity.

**Evidence:** Anthropic recommends "diverse, canonical examples." The "few-shot dilemma" paper (arXiv 2509.13196) shows too many examples paradoxically degrades performance.

#### 6. Explicit workflow for agents

For coding agents, structured workflow guidance (plan → implement → verify) outperforms ad-hoc instructions. OpenAI's GPT-4.1 guide shows three agentic reminders (persistence, tool-calling, planning) boosted SWE-bench by ~20%.

**Evidence:** OpenAI's GPT-4.1 prompting guide. Anthropic's "Building Effective Agents."

### What Doesn't Work

#### 1. "You are an expert" framing

No measurable accuracy improvement on frontier models. May reduce accuracy by making the model overconfident.

**Debunked by:** Google DeepMind's 162-persona study across four LLM families and 2,410 questions. IEEE Spectrum's "Prompt Engineering Is Dead."

#### 2. Emotional manipulation and threats

"This is very important to my career" and "you will be fired" add noise without improving output quality. Emotional prompting can amplify disinformation production.

**Debunked by:** 2025 Frontiers in AI study. Anthropic's RLHF specifically avoids responding to coercive framing.

#### 3. "Take a deep breath"

Worked on GPT-3.5 because it combined an emotional cue with a CoT directive. On frontier models with built-in thinking, it's obsolete.

**Debunked by:** Wharton School's "Decreasing Value of Chain of Thought" (SSRN 5285532). Google DeepMind's own "Prompting Considered Harmful."

#### 4. "Think step by step" (on its own)

Diminishing returns on models that already reason internally. Inflates token costs 2-5x with no measurable accuracy gain.

**Debunked by:** Wharton School study. arXiv research on reasoning model CoT controllability (scores 0.1% to 15.4%).

**Note:** Structured planning (a more deliberate form of CoT) still helps. The blanket "think step by step" incantation does not.

#### 5. Negative instructions

"Do NOT do X" is less reliable than "Do Y instead." The model must process the concept of X to avoid it, paradoxically increasing the chance of producing X.

**Debunked by:** Anthropic's explicit recommendation against negative framing. Ironic Process Theory applied to LLMs.

### The Shift: Prompt Engineering → Context Engineering

Both Anthropic and Google DeepMind argue the field has moved past "finding the right words." What matters is **what you put in context**: examples, tools, retrieved information, and structured inputs. The prompt enhancer skill embodies this principle — it adds structure and specificity (context) rather than emotional phrases or magic words.

## Comparison with Alternatives

| Feature | This Skill | severity1/prompt-improver | Hashaam101/prompt-optimizer | KiloCode Enhance |
|---------|-----------|--------------------------|----------------------------|-------------------|
| Type | Native skill | Hook engine (JSON) | Native skill | Built-in feature |
| Context-aware | Yes (1-2 files) | No | No | No (LLM rewrite only) |
| Code exploration | When needed | Never | Never | Never |
| Workflow overlap | None | High (7/9 nudges overlap with superpowers) | None | N/A |
| Transparency | Shows changes, waits for approval | Silent background nudges | Silent enhancement | Transparent |
| Universal (not just coding) | Yes | Coding-focused | Coding-focused | Coding-focused |
| Requires config | No | Yes (JSON rules) | No | Yes (model selection) |
| Bypass mechanism | `*` prefix | Disable hook | N/A | Don't click button |
| Evidence-based | Yes | Community-driven | Community-driven | N/A |

## File Structure

```
prompt-enhancer/
├── SKILL.md    # The skill (973 words)
└── README.md   # This file
```

## Uninstall

```bash
rm -rf ~/.claude/skills/prompt-enhancer
```

Gone on next session. No cleanup needed.

## License

Use freely. No warranty. The research cited is public; the design decisions are evidence-based.
