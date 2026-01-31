---
name: agentic-translation
description: Use when translating documents ≥5K words, or when user requests agentic translation for smaller complex documents. NOT for simple sentences/paragraphs without explicit agentic request.
---

# Agentic Translation

## Overview

Multi-agent translation system delivering world-class quality through coordinated specialization: style analysis, target language adaptation, translation memories, parallel translation (3 translators per chunk), expert review, and centralized orchestration.

**WARNING**: `agentic-translation-reference.md` contains ~4,800 words. Read only when executing multi-agent translation. For simple translations, translate directly.

**Core principle**: Quality at scale requires division of labor. One agent cannot maintain context, style consistency, terminology accuracy, AND cultural adaptation across large documents.

## When to Use

**Use agentic translation when ANY of these apply**:
- Document size ≥ 5,000 words (multi-chunk coordination needed)
- User explicitly requests "agentic translation" (regardless of size)
- Literary/brand content with distinctive voice (style preservation critical)
- Specialized terminology requiring consistency (technical, medical, legal)
- Cultural adaptation important (idioms, references, social context)
- Maximum quality required (publication, executive, legal documents)
- Poetry/songs requiring form preservation
- Mixed formats (code + text, structured data)
- Multiple language pairs with consistency needs

**Direct translation OK when**:
- Simple sentence/paragraph/short text (<5K words) WITHOUT explicit "agentic" request
- User says "translate this" without mentioning "agentic"
- Style/terminology not critical
- No cultural adaptation needed
- Quick reference sufficient

## Limited Context Strategy

**When context is tight** (< 50K tokens available):

**Option 1 - Direct translation** (for simple documents):
1. Translate directly using your knowledge
2. Apply style guidance if provided
3. Review and refine

**Option 2 - Subagent delegation** (for complex documents):
1. Create task specification (source, target, requirements)
2. Dispatch subagent with the full reference: `agentic-translation-reference.md`
3. Subagent executes full workflow
4. Collect and verify output

## Quick Reference: Agent Roles

| Agent | Purpose | Context |
|-------|---------|-------------------|
| **Orchestrator** | Coordinate all work, launch all subagents | NEVER runs out—central coordinator only, MUST launch translators & reviewer |
| **Style Analyst** | Analyze source style | Samples if large; full read if < 50K words |
| **Target Stylist** | Adapt style to target language | References style analysis only |
| **Chunker** | Split into context-safe chunks | Math: (source + instructions + condensed_refs + output + buffer) ≤ 50% context |
| **Translator** (×3 per chunk) | Translate chunks | One chunk each, launched in parallel by orchestrator |
| **Reviewer** (×1 per chunk) | Compare 3 translations, combine best | Reads 3 outputs, launched by orchestrator after translators |

## Core Workflow (8 Phases)

**Phase 0**: User clarification (11 questions including format)
**Phase 1**: Style analysis → Style analysis document
**Phase 2**: Target style bible → Target style guide
**Phase 2.5**: Condensed reference guides → Compact guides for translators
**Phase 3**: Translation memories → Consistency tracker (condensed + full)
**Phase 4**: Document chunking → Context-safe chunks (with refs in formula)
**Phase 5**: Translation (3× per chunk) → Translation drafts
**Phase 6**: Review (1 per chunk) → Final chunk translation
**Phase 7**: Assembly → Complete translated document
**Phase 8**: Cleanup and archive → Clean workspace, archived guides

## Critical Constraints

**NON-NEGOTIABLE**:
- **3 translators per chunk** (never reduce)
- **Style analysis required** (never skip)
- **Review phase required** (never accept raw translator output)
- **Format preservation** (plain text→plain text, markdown→markdown, etc.)
- **Calculated chunking** (never eyeball it, includes reference sizes)
- **Task tracker updated after every phase** (no placeholders)
- **No meta-commentary in outputs** (headers/footers/synthesis reports prohibited)
- **Orchestrator launches all subagents** (subagents cannot launch other subagents)
- **3 translators launched in parallel** (single message with 3 Task calls)
- **Reviewer launched after all 3 translators complete**
- **Sequential chunk processing** (each chunk completes before next starts)
- **Condensed references for translators** (full references to reviewer only)
- **Cleanup required** (never leave artifacts scattered)

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Single agent for large document | Use multi-agent team |
| **Reviewer launching translator subagents** | Orchestrator must launch all subagents (subagents cannot launch other subagents) |
| **Orchestrator launching translators sequentially** | Launch 3 translators in parallel (single message with 3 Task calls) |
| **Orchestrator launching reviewer before translators complete** | Wait for all 3 translators to complete before launching reviewer |
| Fewer than 3 translators per chunk | 3 translators mandatory |
| Skipping style analysis | Style analysis NON-NEGOTIABLE |
| Converting format (plain text→markdown) | Output format MUST match source |
| Meta-commentary in output | Output MUST contain only translated content |
| Creating task tracker but never updating | Update tracker AFTER EVERY phase |
| Processing chunks in parallel | Sequential processing only (within chunk: translators parallel; between chunks: sequential) |
| Leaving chunk files scattered | Run Phase 8 cleanup |

## Red Flags - STOP

- Fewer than 3 translators per chunk
- **Reviewer launching translator subagents**
- **Orchestrator launching translators sequentially**
- **Orchestrator launching reviewer before translators complete**
- Format changed (plain text→markdown, etc.)
- Meta-commentary in output files
- Task tracker contains placeholders after work begins
- Orchestrator holding document content
- Processing multiple chunks simultaneously
- Chunk files left in workspace after completion

**All of these mean: Stop. Replan. Follow the workflow.**

## Format-Specific Guidelines

**Plain text (.txt)**: ALL CAPS headings, preserve spacing
**Markdown (.md)**: Preserve # headers, **bold**, *italics*
**HTML**: Translate text, keep tags
**Code**: Translate comments, keep syntax

## Bottom Line

**Maximum translation quality required?** Read `agentic-translation-reference.md` and execute full 8-phase workflow.

**Document ≥5K words OR user requests agentic translation?** Use this skill.

**Simple sentence/paragraph without "agentic" request?** Translate directly. No multi-agent system needed.

**Limited context?** Dispatch subagent with the full reference and task specification.
