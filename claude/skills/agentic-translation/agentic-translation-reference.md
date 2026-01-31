---
name: agentic-translation-reference
description: Complete reference for multi-agent translation workflow. Read only when executing agentic translation (gateway skill: agentic-translation). Contains 8-phase workflow, agent instructions, quality gates, format preservation rules, error handling, and troubleshooting.
---

# Agentic Translation - Complete Reference

## Overview

Multi-agent translation system delivering world-class translations through coordinated specialization: style analysis, target language adaptation, translation memories, parallel translation (3 translators per chunk), expert review, and centralized orchestration. Achieves what single agents cannot—context-safe scaling, perfect consistency, and maximum quality through adversarial comparison.

**Core principle**: Quality at scale requires division of labor. One agent cannot maintain context, style consistency, terminology accuracy, AND cultural adaptation across large documents. Specialized agents, each expert in their domain, coordinated by an orchestrator, deliver superior results.

## When to Use

**Use agentic translation when ANY of these apply:**
- Document size ≥ 5,000 words (multi-chunk coordination needed)
- User explicitly requests "agentic translation" (regardless of size)
- Literary/brand content with distinctive voice (style preservation critical)
- Specialized terminology requiring consistency (technical, medical, legal)
- Cultural adaptation important (idioms, references, social context)
- Maximum quality required (publication, executive, legal documents)
- Poetry or songs requiring form preservation
- Mixed formats (code + text, structured data)
- Multiple language pairs with consistency needs

**Direct translation OK when:**
- Simple sentence/paragraph/short text (<5K words) WITHOUT explicit "agentic" request
- User says "translate this" without mentioning "agentic"
- Style/terminology not critical
- No cultural adaptation needed
- Quick reference sufficient

## Quick Reference: Agent Roles

| Agent | Purpose | Key Output | Context Management |
|-------|---------|------------|-------------------|
| **Orchestrator** | Coordinate all work, launch all subagents | Task tracker, agent instructions | NEVER runs out—central coordinator only, MUST launch translators & reviewer |
| **Style Analyst** | Analyze source document style | Style analysis document | Samples if large; full read if < 50K words |
| **Target Stylist** | Adapt style to target language | Target style bible | References style analysis only |
| **Chunker** | Split into context-safe chunks | Chunk assignments | Math: (source + instructions + output) ≤ 50% context |
| **Translator** (×3 per team) | Translate chunks (rewrite, not word-for-word) | Translation output | One chunk each, launched in parallel by orchestrator |
| **Reviewer** (×1 per team) | Compare 3 translations, combine best bits | Final translation | Reads 3 outputs, produces 1 superior output, launched by orchestrator after translators |

## Core Workflow

### Phase 0: User Clarification (CRITICAL - Orchestrator ONLY)

**Before starting ANY work, the orchestrator MUST ask:**

1. **Source document**: What file or text to translate?
2. **Source language**: What language is it currently in?
3. **Target language**: What language to translate to?
4. **Document type**: Novel, technical docs, poetry, code, marketing, legal?
5. **Purpose**: Publication, internal use, reference, executive review?
6. **Target audience**: General public, experts, executives, specific demographic?
7. **Format requirements**: Keep original format, specific output format?
8. **Source format**: What is the source file format? (.txt, .md, .html, code?)
9. **Special requirements**: Rhyme preservation, code handling, terminology constraints?
10. **Deadline**: Any time constraints?
11. **Style guidance**: Examples of desired style/tone?

**CRITICAL**: Detect source format from:
- File extension (.txt, .md, .html, .py, .js, etc.)
- Content structure (markdown headers, HTML tags, code syntax)
- User confirmation

**DO NOT proceed until ALL aspects are clear.**

### Phase 1: Style Analysis (Style Analyst Agent)

**Purpose**: Understand what makes the source style unique so it can be preserved or adapted.

**Process**:
1. Detect source document format (.txt, .md, .html, code file)
2. Read source document (full if < 50K words, strategic sampling if larger)
3. Identify and document:
   - **Source format**: Plain text, markdown, HTML, code (CRITICAL)
   - **Chapter/section structure**: Markers, separators, blank lines, visual hierarchy (CRITICAL for multi-section documents)
   - **Register**: Formal/informal, academic/conversational, technical/accessible
   - **Voice**: First person/omniscient, enthusiastic/detached, ironic/direct
   - **Sentence structure**: Long/complex, short/punchy, paratactic/hypothetic
   - **Vocabulary**: Specialized domain terms, colloquialisms, period language
   - **Literary devices**: Metaphor, irony, symbolism, allusion
   - **Rhythm/musicality**: Especially important for poetry/lyrics
   - **Cultural references**: Idioms, historical events, social norms
4. Write **Style Analysis Document** with:
   - Concrete examples from source text illustrating each style element
   - **Chapter/section separation patterns with exact examples** (CRITICAL - see "Chapter/Section Separation and Document Structure" section)
   - Explanation of WHY each element matters to the work's effect
   - Identification of what makes this style distinctive/special
   - **Source format specification** (NEW)
5. **Report back to orchestrator** with:
   - Agent ID who completed the analysis
   - Output file location
   - **Source format detected** (NEW)
   - Confirmation that analysis is complete
   - Any issues or findings

**Orchestrator action**: Update task tracker Phase 1 section:
- [x] Source format detected: [.txt / .md / .html / code]
- [x] Style analyst dispatched
- [x] Style analysis document created: [actual location]
- Record agent ID
- Change status from BLOCKED to COMPLETE

**Output**: Style analysis document (detailed enough that someone who hasn't read the original could write in the same style)

### Phase 2: Target Style Bible (Target Language Stylist Agent)

**Purpose**: Mirror the style analysis in the target language, adapting for cultural/linguistic differences.

**Process**:
1. Read the style analysis document
2. Using expert knowledge of target language, determine how to achieve equivalent effect:
   - **Register equivalents**: How does formality work in target language?
   - **Voice adaptation**: What narrative devices exist in target language?
   - **Sentence patterns**: Natural target language structures for similar effect
   - **Vocabulary mapping**: Target language terms for source domain vocabulary
   - **Literary device equivalents**: Target language conventions for metaphor, irony, etc.
   - **Cultural adaptation**: Target language cultural touchpoints for source references
3. Write **Target Style Bible** with:
   - Specific guidance for each style element from analysis
   - Examples in target language showing desired style
   - Do's and don'ts for maintaining this style in target language
   - Cultural adaptation guidelines
4. **Report back to orchestrator** with:
   - Agent ID who created the style bible
   - Output file location
   - Confirmation that style bible is complete
   - Any adaptation challenges or decisions

**Orchestrator action**: Update task tracker Phase 2 section:
- [x] Target stylist dispatched
- [x] Target style bible created: [actual location]
- Record agent ID
- Change status from BLOCKED to COMPLETE

**Output**: Target style bible (actionable guide for translators)

### Phase 2.5: Condensed Reference Guides (Target Stylist + Orchestrator)

**Purpose**: Create lightweight reference materials for translators to minimize context overhead while maintaining consistency. Full references go to reviewer.

**CRITICAL CONTEXT PROBLEM**: When translators have full reference materials (~40K tokens), chunk sizes must be very small to stay within context limits. By creating condensed versions for translators, we can use larger, more efficient chunks while the reviewer still catches all consistency issues.

**Process**:

1. **Target Stylist creates TWO versions of style guidance**:
   - **Full Style Bible** (~14K tokens): Comprehensive guide with all nuances, examples, and rationale → **Reviewer only**
   - **Condensed Style Guide** (~3K tokens): Core elements only → **Translators A/B/C**

2. **Orchestrator creates TWO versions of translation memories** (if memories are needed):
   - **Full Translation Memories** (~18K tokens): All recurring terms → **Reviewer only**
   - **Quick Reference** (~3K tokens): Top 50 most critical terms → **Translators A/B/C**

3. **Condensed Style Guide Contents** (what to include):
   - Register level (formal/informal/academic/conversational)
   - Character voice guidelines (if applicable - who talks how)
   - Vocabulary dos and don'ts (10-15 key rules)
   - Sentence structure patterns (brief examples)
   - Critical style rules only (skip extensive rationale)

4. **Quick Reference Contents** (top 50 terms selection):
   - **Frequency**: Characters/places appearing most often (top 20 names)
   - **Criticality**: Terms where consistency is essential (mottos, titles)
   - **Complexity**: Terms with non-obvious translations
   - Skip: Standard vocabulary with one correct translation

5. **Report back to orchestrator** with:
   - Both file locations (condensed and full versions)
   - Agent ID(s) who created them
   - Token counts for each version
   - Confirmation that phase is complete
   - List of what was condensed (summary of differences)

**Orchestrator action**: Update task tracker Phase 2.5 section:
```markdown
- [x] Condensed references created
- [x] Condensed style guide: /path/to/{project}-style-guide-compact.md (~3K tokens)
- [x] Full style bible: /path/to/{project}-style-guide-full.md (~14K tokens)
- [x] Quick reference: /path/to/{project}-terms-quickref.md (~3K tokens)
- [x] Full memories: /path/to/{project}-terms-full.md (~18K tokens) [if applicable]
- Record agent ID(s)
- Change status from BLOCKED to COMPLETE
```

**Output Files**:
- `{project-name}-style-guide-compact.md` (~3K tokens) → For translators
- `{project-name}-style-guide-full.md` (~14K tokens) → For reviewer
- `{project-name}-terms-quickref.md` (~3K tokens) → For translators
- `{project-name}-terms-full.md` (~18K tokens) → For reviewer

**Quality Assurance**:
- Condensed guides contain all **critical** information (no essential rules omitted)
- Quick reference includes most frequent 50 terms (covers 80%+ of occurrences)
- Full versions preserve all nuance and rationale for reviewer

### Phase 3: Translation Memories (Orchestrator + Style Analyst)

**Purpose**: Ensure consistent translation of recurring elements.

**Process**:
1. FIRST, test whether translation memories are needed by asking:
   1. Are there recurring narrative constants? (names, places, organizations appearing 10+ times)
   2. Are there repeated phrases/lyrics/quotes?
   3. Is terminology non-standard or proprietary requiring specific approved translation?
   4. Will consistency be a problem without explicit tracking?
2. If answer to ALL is NO: Skip translation memories (use style guide only), note decision in tracker
3. If ANY answer is YES: Work with Style Analyst or handle yourself:
   - Extract only recurring items (names, places, repeated phrases, proprietary terms)
   - DO NOT include standard vocabulary, code elements, or single-occurrence items
   - Create translation memory entries with approved translations
   - Document rationale for each translation choice
   - **Select top 50 terms** for quick reference:
     - Frequency: Most frequently occurring characters, places, organizations
     - Criticality: Terms where consistency is essential (titles, mottos)
     - Complexity: Terms with non-obvious translations
     - Skip: Standard vocabulary with automatic consistency
4. **Create TWO versions** (coordinated with Phase 2.5):
   - **Quick Reference** (~3K tokens): Top 50 critical terms → Translators A/B/C
   - **Full Memories** (~18K tokens): All recurring terms → Reviewer
5. **Report back to orchestrator** with:
   - Agent ID who created the memories (if applicable)
   - Both file locations (quick reference + full version)
   - Token counts for each version
   - Confirmation that phase is complete
   - Count of total memory entries and quick reference entries

**Orchestrator action**: Update task tracker Phase 3 section:
```markdown
- [x] Extracted recurring items (or note "N/A - not needed")
- [x] Translation memories created:
  - Quick reference: /path/to/{project}-terms-quickref.md (~3K tokens, [N] entries)
  - Full version: /path/to/{project}-terms-full.md (~18K tokens, [N] entries)
  OR "SKIPPED - rationale"
- Record agent ID (if applicable) or note "Orchestrator decision"
- Change status from BLOCKED to COMPLETE
```

**Output**: Translation memories file (living document, updated as new items found) OR documented decision that memories are not needed

---

## Translation Memories vs Style Guides - Critical Distinction

### What Translation Memories Are For

**Translation memories ARE for:**
- **Recurring narrative constants**: Items that appear repeatedly throughout the document and MUST be translated consistently
- Character names in novels (appears 10+ times, often 100+ times)
- Location/place names in stories (appears repeatedly)
- Organization names when specific approved translation required
- **Specific repeated phrases**: Lyrics, quotes, mottos, slogans, repeated expressions
- Complex terms with context-specific, non-standard translations
- Proprietary terminology requiring specific approved translations
- Narrative constants that would damage consistency if translated differently

**Translation memories are NOT for:**
- Standard vocabulary (Algorithm → Algoritmo, Model → Modelo - any competent translator knows these)
- Basic domain terminology (industry-standard terms are automatically consistent)
- Code/library names (sklearn, pandas - these are "do not translate" rules, not memories)
- Function/method names (fit(), predict() - keep in English, not translation memories)
- Things appearing only once (nothing to "remember" across the document)
- Single-occurrence items (section headers, individual comments, one-time phrases)
- Common prepositions and connectors (subset of, based on, from - standard translations)
- Technical terms with only one correct translation (supervisionado, regresión - automatically consistent)

**Rationale**: Translation memories are for CONSISTENCY across recurrences. If an item appears once, there's nothing to be consistent with. If a term has only one correct translation, translators will automatically be consistent.

### Style Guides Serve Different Purposes

**Style guides (Target Style Bible) are for:**
- Register and tone decisions (formal/informal, academic/conversational)
- Voice and perspective guidelines
- Sentence structure patterns for natural flow
- Vocabulary choices (word selection, period language, colloquialisms)
- Literary device usage (metaphor, irony, symbolism conventions)
- Cultural adaptation guidelines
- Format and presentation decisions
- Do's and don'ts with examples

**Key distinction**: Style guides guide HOW to write. Translation memories dictate WHAT to write for specific recurring items.

### When Are Translation Memories Needed?

**Create translation memories when:**
- Literary works with recurring characters/locations (novels, stories, series)
- Documents with repeated quotes/lyrics/slogans/mottos
- Multiple documents in same series requiring cross-document consistency
- Proprietary terminology with specific approved translations required
- Complex terms where context affects translation (same term, different translations in different contexts)
- Documents with brand-critical terminology requiring legal approval

**Skip translation memories when:**
- Technical documents with standard terminology (ML, programming, scientific - terms have one correct translation)
- One-time translations (no recurring elements to remember)
- Documents where all terms are unambiguous and industry-standard
- Short documents (<10K words) with simple vocabulary
- API documentation (code elements + standard technical terms)
- Blog posts or articles with no recurring characters/places/concepts

**Examples**:

✅ **Needs translation memories**:
- Novel: "Elizabeth Bennet" appears 500+ times
- Song: Chorus repeats 8 times with identical lyrics
- Technical manual: Proprietary "HyperFlow" technology must be "HyperFlow" (not "Hiperflujo") per branding guidelines
- Series: 10 technical manuals with shared proprietary terminology

❌ **Does NOT need translation memories** (use style guide only):
- ML tutorial: Standard ML terms (supervisionado, regresión, sobreajuste) - any competent translator uses correct standard term
- Single technical document with common vocabulary
- API documentation: Code elements (keep in English) + standard technical terms (automatically consistent)
- Blog post: No recurring narrative constants

**Testing whether translation memories are needed**:

Ask these questions:

1. **Are there recurring narrative constants?** (names, places, organizations appearing 10+ times)
   - Yes → Create translation memories
   - No → Maybe not needed

2. **Are there repeated phrases/lyrics/quotes?**
   - Yes → Create translation memories
   - No → Maybe not needed

3. **Is terminology non-standard or proprietary?** (requires specific approved translation)
   - Yes → Create translation memories
   - No → Standard terminology automatically consistent

4. **Will consistency across recurrences be a problem without explicit tracking?**
   - Yes → Create translation memories
   - No → Style guide sufficient

**If answer to all questions is "No"**: Translation memories not needed. Use style guide only.

**Reality check from testing**: The ML tutorial demonstration (article_en.md → article_es-TRANSLATED.md) did NOT require translation memories. All ML terminology is industry-standard with only one correct Spanish translation. Style guide alone was sufficient. Creating translation memories for "Algorithm → Algoritmo" and "Data → Datos" was overkill that provided no value.

**Common rationalizations to avoid**:

| Rationalization | Reality |
|----------------|---------|
| "Better safe than sorry - create memories for everything" | Over-inclusive memories create noise without benefit. Focus on what actually needs tracking. |
| "Translation memories are always part of professional workflow" | Not true. Professional translators use memories when there's something to remember. Standard terminology doesn't need tracking. |
| "What if the same term appears again in another document?" | If it's standard terminology, it will be translated correctly in the other document too. No memory needed. |
| "Creating memories shows thoroughness" | No, it shows confusion about purpose. Thoroughness comes from correct process, not unnecessary artifacts. |

**Bottom line**: Translation memories prevent inconsistency in recurrences. No recurrences = no memories needed. Standard terminology = automatic consistency. Focus memories on actual consistency risks.

---

### Phase 4: Document Chunking (Chunker Agent)

**Purpose**: Split document into context-safe pieces that no agent will overflow.

**Critical Constraint**: Plan for (source text + instructions + condensed_refs + thinking + output + buffer) ≤ 50% of context window

**CRITICAL**: Formula MUST include condensed reference size:
```
safe_chunk = (50% × context_limit) - instructions - condensed_refs - buffer
```

**Calculation Example** (for 200K context with reference-heavy work):
- Safe per agent: 100K tokens total (50% of context)
- Instructions: ~10K tokens
- Condensed references: ~6K tokens (style guide + quick reference)
- Thinking buffer: ~10K tokens
- Output estimate: ~20K tokens (translation similar size to source)
- Safety buffer: ~10K tokens
- Remaining for source: ~44K tokens
- Therefore: Max chunk size ≈ 11K words (assuming 4 tokens/word) OR ~8K words for token-dense languages like Chinese (~5.5 tokens/word)

**Calculation Example** (for 200K context with light-reference work):
- Safe per agent: 100K tokens total
- Instructions: ~10K tokens
- Condensed style guide only: ~3K tokens (no translation memories)
- Thinking buffer: ~10K tokens
- Output estimate: ~20K tokens
- Safety buffer: ~10K tokens
- Remaining for source: ~47K tokens
- Therefore: Max chunk size ≈ 12K words

**Process**:
1. Analyze document structure to find logical break points:
   - Literary: Chapters, scenes, sections
   - Technical: Modules, functions, documentation sections
   - Poetry: Individual poems, stanzas (only if form-independent)
   - Code: Functions, classes, files
   - Mixed content: By logical section, not arbitrary size
2. Create chunks that:
   - Respect logical boundaries (don't break mid-sentence/chapter/function)
   - Are roughly equal in size (within 20% variance)
   - Can be translated independently (minimal forward references)
3. **CRITICAL: Processing model**
   - **Within each chunk**: 3 translators work in PARALLEL (same chunk, simultaneous)
   - **Between chunks**: Sequential (Chunk N+1 starts only after Chunk N's reviewer completes)
4. Assign each chunk to a translation team (3 translators + 1 reviewer) in sequential order
5. Create **Translation Task Tracker** with:
   - Chunk ID, source text location
   - Assigned team members (3 translators, 1 reviewer)
   - Status (pending/in-progress/completed)
   - Sequential processing order (Chunk 1 → Chunk 2 → Chunk 3...)
6. **Report back to orchestrator** with:
   - Agent ID who performed chunking
   - Total number of chunks created
   - Chunk assignments (which team for which chunk)
   - Context safety calculations for each chunk
   - Confirmation that chunking is complete
   - Processing plan: parallel translators within each chunk, sequential chunks

**Orchestrator action**: Update task tracker Phase 4 section:
- [x] Chunker dispatched
- [x] Total chunks: [N]
- [x] Chunk assignments created
- Record agent ID
- Change status from BLOCKED to COMPLETE
- Populate all Translation Team sections with chunk details and "[PENDING ASSIGNMENT]" placeholders

**Output**: Chunk assignments + task tracker

### Phase 5: Translation (Translator Agents - 3 per chunk)

**Purpose**: Rewrite each chunk in target language following style bible and translation memories.

**CRITICAL CONSTRAINT**: Subagents cannot launch other subagents. The orchestrator (main thread) must launch all translator agents.

**Critical Principles**:
1. **Rewrite, don't translate word-for-word**: The goal is target text that reads as if originally written in target language
2. **Context awareness**: Understand how this chunk fits into the whole work
3. **Follow style bible**: Use target language style guidance
4. **Honor translation memories**: Use approved translations consistently
5. **Respect format constraints**:
   - Poetry: Maintain form (rhyme, rhythm, meter) as much as possible without sacrificing meaning
   - Code: Translate comments/docstrings only, keep code syntax unchanged
   - JSON/HTML: Translate content, keep structure/tags/attributes unchanged
   - Markdown: Translate text content, keep formatting, code blocks, URLs
   - **Plain text**: Output plain text, NO markdown syntax (NEW)

**For each translator, provide**:
- Chunk to translate (source text)
- Source format information (CRITICAL)
- **Condensed style guide** (target language guidance - ~3K tokens)
- **Quick reference** (top 50 terms - ~3K tokens, if translation memories exist)
- Context notes (where this fits in the whole)
- **Format preservation requirement** (CRITICAL)

**NOTE**: Translators receive CONDENSED references only. Full references go to reviewer.

**Reference Materials Template**:
```markdown
**Reference Materials** (Condensed versions for translators):
1. Condensed Style Guide: /path/to/{project}-style-guide-compact.md
2. Quick Reference: /path/to/{project}-terms-quickref.md

These contain essential style guidance and the most critical terms. Focus on natural flow and core consistency. The reviewer will verify your work against full references.
```

**Format preservation instructions**:
- "Source format is [FORMAT]. You must output in the same format."
- "Plain text source → Output plain text (NO markdown # headers, **bold**, etc.)"
- "Markdown source → Output markdown with # headers, **bold**, etc."
- "HTML source → Output HTML with tags preserved"
- "Code source → Output code with translated comments, syntax preserved"

**ORCHESTRATOR WORKFLOW** (CRITICAL - Main Thread Only):

**Step 1: Launch 3 translators in PARALLEL**
- Use a single message with 3 Task tool calls (one per translator)
- All 3 translators work on the same chunk simultaneously
- Each translator produces independent output

**Step 2: Wait for all 3 translators to complete**
- Monitor task tracker for completion status
- Collect output file locations from all 3 translators
- Verify all 3 outputs are present before proceeding

**Step 3: Verify outputs before launching reviewer**
- Check each translator's output for basic format compliance
- Ensure all 3 files exist and are non-empty
- Note any issues for reviewer to address

**Process** for each translator (subagent executes):
1. Read chunk source text
2. Read **condensed style guide** (target language guidance)
3. Read **quick reference** (top 50 terms, if translation memories exist)
4. Read chunk-specific instructions (context from orchestrator)
5. Rewrite chunk in target language:
   - First pass: Capture all meaning accurately
   - Second pass: Refine style to match style bible
   - Third pass: Verify translation memories used correctly
6. Output translation
7. **Report back to orchestrator** with:
   - Agent ID (Translator A/B/C)
   - Chunk ID translated
   - Output file location
   - Confirmation that translation is complete
   - Any issues or uncertainties

**Orchestrator action**: For each translator completion, update task tracker Translation Team section:
- Replace "[PENDING ASSIGNMENT]" with actual agent ID
- Change status from `[ ]` to `[x]` (completed)
- Record output file location
- Update status from "PENDING" to "in-progress" → "completed" as each translator finishes
- **DO NOT launch reviewer until all 3 translators are complete**

**Output**: 3 translations per chunk (one from each translator)

**NOTE**: After all 3 translators complete, orchestrator proceeds to Phase 6 (Review).

### Phase 6: Review and Synthesis (Reviewer Agent)

**Purpose**: Compare 3 translations, take best parts from each, create one superior translation.

**CRITICAL CONSTRAINT**: Subagents cannot launch other subagents. The reviewer is launched by the orchestrator AFTER all 3 translators complete.

**ORCHESTRATOR WORKFLOW** (CRITICAL - Main Thread Only):

**Prerequisite**: All 3 translators for this chunk must be complete before launching reviewer.

**Step 1: Launch reviewer (single Task call)**
- Provide reviewer with:
  - Source text for the chunk
  - All 3 translation outputs (file paths)
  - **Full style bible** (comprehensive guidance)
  - **Full translation memories** (all recurring terms)
  - Format preservation requirements
  - Any issues noted from translator outputs

**CRITICAL**: Reviewer receives FULL references to catch any consistency issues that translators may have missed with condensed guides.

**Reference Materials Template**:
```markdown
**Reference Materials** (Full versions for reviewer):
1. Full Style Bible: /path/to/{project}-style-guide-full.md
2. Full Translation Memories: /path/to/{project}-terms-full.md

Verify translator outputs against comprehensive guidance. Check:
- All translation memories honored (not just top 50)
- Style bible fully followed (all nuances)
- Consistency across all recurring terms

Correct any issues by selecting best translation or creating hybrid that honors full references.
```

**Step 2: Wait for reviewer to complete**
- Monitor task tracker for completion
- Collect final output file location

**Step 3: Verify reviewer output**
- Confirm output file exists and is non-empty
- Verify format matches source format (quick check)
- Proceed to Phase 7 (Assembly) when all chunks complete

**Process** (reviewer subagent executes):
1. Read all 3 translations of the assigned chunk
2. Read source text for comparison
3. Read **full style bible** and **full translation memories**
4. For each segment (paragraph/stanza/section):
   - Compare all 3 translations
   - Identify which does the best job:
     - Accuracy to source meaning
     - Style consistency with style bible
     - Natural flow in target language
     - Cultural adaptation
     - Format adherence (for poetry/code)
   - Select the best version OR create hybrid combining best elements
5. Assemble final translation from selected segments
6. **Quality assessment** (rate each criterion 1-10):
   - **Naturalness**: Native speaker flow (≥8.8 target)
   - **Cultural Appropriateness**: Cultural fitting (≥8.5 target)
   - **Meaning Preservation**: Core intent maintained (≥9.0 target)
   - **Style Fidelity**: Original voice preserved (≥8.5 for literary content)
   - **Communicative Effectiveness**: Achieves communication goal (≥8.7 target)
   - **Audience Appropriateness**: Suitable for target audience (≥8.7 target)
   - **Any score < 8.5**: Immediate refinement required
7. Quality check:
   - Translation memories all used correctly
   - No inconsistencies within chunk
   - Style matches style bible
   - Format requirements met
   - Chapter/section separation matches source exactly
   - No meta-commentary in output
8. **Report back to orchestrator** with:
   - Agent ID (Reviewer)
   - Chunk ID reviewed
   - Final output file location
   - Confirmation that review is complete
   - Quality assessment scores (in task tracker, NOT in output file)
   - Summary of synthesis decisions (which translations were used)
   - Any quality issues found and resolved

**Orchestrator action**: Update task tracker Translation Team section:
- Replace "[PENDING ASSIGNMENT]" with actual reviewer agent ID
- Mark `[x]` Reviewer status as completed
- Record final output file location
- Change team status from "PENDING" to "COMPLETED"
- Record timestamp of completion

**CRITICAL: Sequential Processing** - After reviewer completes chunk, orchestrator must:
1. Verify chunk is complete before starting next chunk
2. Move to next chunk (Chunk N+1) only after current chunk's reviewer finishes
3. Never have multiple chunks in progress simultaneously

**Output**: 1 final polished translation per chunk

### Phase 7: Assembly (Orchestrator)

**Purpose**: Combine all chunks into final translated document.

**Process**:
1. Collect all completed chunks from reviewers
2. **CRITICAL VERIFICATION** - Before concatenating, verify each chunk:
   - **Format check**: Does output format match source format?
     - Plain text source → No markdown syntax (#, **, *)?
     - Markdown source → Markdown syntax present?
     - HTML source → Tags preserved?
     - Code source → Syntax preserved?
   - **Meta-content check**: No meta-headers, meta-footers, synthesis reports
   - Check first 10 lines: Should start with actual content
   - Check last 10 lines: Should end with actual content
   - Search for meta-patterns: "TRADUCTION FINALE", "NOTE DE SYNTHÈSE", "RAPPORT DE SYNTHÈSE", "FIN DU CHUNK", "Reviewed by", "Synthesis prepared by"
   - Search for format-conversion markers: "# CHAPTER" when source is plain text with "CHAPTER"
   - **If ANY issues found**: DO NOT use that file. Request corrected output from reviewer.
3. Assemble verified chunks in correct order
4. Verify transitions between chunks are smooth
5. Final consistency check:
   - Spot check translation memory adherence across chunk boundaries
   - Verify style consistency across work
   - Check format consistency
6. Create final output file with **same extension as source**:
   - source.txt → output.txt
   - source.md → output.md
   - source.html → output.html
   - source.py → source_translated.py (code with translated comments)
7. Update task tracker Phase 7 section

**Orchestrator action**: Update task tracker Phase 7 section:
- [x] All chunks completed (N/N)
- [x] Format verified: all chunks match [source format]
- [x] Meta-commentary check: no headers/footers/synthesis reports found
- [x] Assembled into final document
- [x] Final consistency check
- [x] Output location: [path]
- [x] Output format: [same as source]
- Overall project status: "COMPLETE"
- Completion timestamp: [YYYY-MM-DD HH:MM:SS]

**Output**: Complete translated document(s)

### Phase 8: Cleanup and Archive (Orchestrator)

**Purpose**: Remove temporary artifacts, preserve valuable guides and reports.

**Process**:
1. Create archive directory: `translation-artifacts/[project-name]/`
2. DELETE all chunk files:
   - Translator outputs (chunk-N-translator-A/B/C.txt, chunk-N-translator-A/B/C.md, etc.)
   - Reviewer drafts (chunk-N-reviewed.txt, chunk-N-reviewed.md, etc.)
   - Any intermediate chunk translation files
3. ARCHIVE to `translation-artifacts/[project-name]/`:
   - Style analysis document (source style guide)
   - Target style bible (target language style guide)
   - Translation memories file (if created)
   - Translation task tracker (complete execution log)
4. Verify final translated document is in root directory (not archived)
5. Update task tracker Phase 8 section

**Orchestrator action**: Update task tracker Phase 8 section:
- [x] Archive directory created: translation-artifacts/[project-name]/
- [x] Deleted: N chunk files (translator outputs + reviewer drafts)
- [x] Archived: style analysis, target style bible, translation memories, task tracker
- [x] Final document location: [path]
- [x] Workspace clean: no chunk files remaining

**Output**: Clean workspace with archived guides and final translated document

**What to DELETE vs ARCHIVE**:

| File Type | Action | Rationale |
|-----------|--------|-----------|
| Translator outputs (chunk-N-translator-A/B/C) | DELETE | Intermediate artifacts, no reuse value |
| Reviewer drafts (chunk-N-reviewed) | DELETE | Superseded by final assembled document |
| Source document | KEEP | Original input, user's file |
| Final translated document | KEEP | The deliverable, output of translation |
| Style analysis document | ARCHIVE | Valuable reference for future similar documents |
| Target style bible | ARCHIVE | Valuable reference for future translations in same language/style |
| Translation memories | ARCHIVE | Valuable for consistency across documents or series |
| Translation task tracker | ARCHIVE | Complete execution log, useful for review and troubleshooting |

## Error Handling and Recovery

### Agent Failures

**Agent times out**:
1. Note which agent/phase failed
2. Dispatch new agent with identical instructions
3. Update task tracker with new agent ID
4. Continue workflow

**Agent produces invalid output**:
1. Identify specific issue (format error, meta-commentary, memory violations)
2. Return output to agent with explicit correction request
3. Specify what's wrong and what's required
4. Re-verify before proceeding

**Agent ignores instructions**:
1. Include explicit reminder in next dispatch
2. Reference specific requirement from skill
3. If repeat failure, consider different agent model

### Output Verification Failures

**Meta-commentary found**:
- Return to reviewer: "Output contains meta-commentary (headers, footers, synthesis reports). Output MUST contain ONLY translated content. Remove all meta-content and resubmit."
- Verify correction before accepting

**Format conversion detected**:
- Return to agent: "Source format is [X] but output is [Y]. Format conversion violates requirements. Resubmit in correct format."
- Provide format rules from skill
- Verify correction before accepting

**Translation memory violations**:
- Return to translator: "Translation memory not used correctly. [Term] should be [approved translation]. Correct and resubmit."
- Verify correction before accepting

### Recovery Process

1. **Identify failure point**: Which agent, which phase, what went wrong
2. **Document specific issue**: Be precise about what's wrong
3. **Request correction**: Provide explicit instructions for fix
4. **Verify correction**: Don't proceed until output is correct
5. **Update tracker**: Document recovery actions taken

## Success Criteria

A successful agentic translation project achieves:

### Quality Metrics
- ✅ All agent outputs verified (no meta-commentary, correct format)
- ✅ Format preserved exactly (plain text→plain text, markdown→markdown, etc.)
- ✅ Chapter/section separation matches source exactly (blank lines, separators, headings)
- ✅ No meta-commentary in output files (no headers, footers, synthesis reports, statistics)
- ✅ Translation memories honored throughout (consistent terminology)
- ✅ Style bible guidance followed (tone, voice, register)
- ✅ No inconsistencies in character names, locations, concepts
- ✅ Natural target language flow (reads like original, not translation)
- ✅ Cultural references appropriately adapted
- ✅ Quality assessment scores met (≥8.5 on all criteria, ≥9.0 for meaning preservation)
- ✅ Form preserved (poetry rhyme/rhythm, code syntax)

### Process Metrics
- ✅ All 8 phases completed in order
- ✅ All agents completed successfully (no failures unaddressed)
- ✅ Task tracker fully updated (no placeholders remain)
- ✅ All verification checks passed (format, meta-content, quality)
- ✅ Source format documented and preserved
- ✅ All chunks assembled in correct order
- ✅ Orchestrator launched all subagents (translators and reviewers)
- ✅ 3 translators per chunk launched in parallel
- ✅ Reviewer launched only after all 3 translators completed
- ✅ Sequential processing maintained (no parallel chunk execution)
- ✅ Workspace clean (no chunk files remaining)

### Deliverables
- ✅ Final translated document (in source format, with source file extension)
- ✅ Task tracker (complete execution log with actual agent IDs, timestamps)
- ✅ Style analysis document (comprehensive source style guide)
- ✅ Target style bible (actionable target language style guide)
- ✅ Translation memories file (if created, with approved translations)

## Critical Constraints

### Context Management (NON-NEGOTIABLE)

**Orchestrator**: NEVER runs out of context
- Only coordinates, doesn't translate
- Maintains task tracker, not document content
- Can use task tracker to query agents about progress
- **MUST launch all subagents** (translators and reviewers) - subagents cannot launch other subagents

**Style Analyst**: Samples if document too large
- For > 100K words: read representative sections (beginning, middle, end + key chapters)
- Document sampling strategy in style analysis

**Target Stylist**: References only
- Only reads style analysis, doesn't read source document
- Writes target style bible from analysis + target language expertise

**Chunker**: Calculates safe chunk sizes
- MUST do the math: (source + instructions + thinking + output) ≤ 50% context
- Document calculation in task tracker

**Translators**: One chunk each
- Each translator handles exactly one chunk at a time
- Clear input/output boundaries

**Reviewer**: Reads 3, writes 1
- Input: 3 translations of same chunk
- Output: 1 superior translation
- Manageable context: 3 × chunk size

### "Rewrite Don't Translate" Principle (NON-NEGOTIABLE)

Translation is not word-for-word substitution. It's rewriting the content in the target language so it reads naturally while preserving the meaning, style, and effect of the original.

**What this means**:
- Prioritize natural target language flow over literal accuracy
- Adapt idioms to target language equivalents
- Preserve the EFFECT more than the exact words
- For literature: Voice and atmosphere matter more than sentence-by-sentence fidelity
- For poetry: Form and feeling over literal meaning
- For technical: Accuracy and clarity over elegance

### Translation Memory Enforcement (NON-NEGOTIABLE)

When an item appears in translation memories:
- **MUST** use the approved translation
- **NO exceptions** without updating the memory entry first
- If translation seems wrong for context: Update memory with new context, then use updated version

### Quality Gates (NON-NEGOTIABLE)

**NO shortcuts allowed**, even under time pressure:
- 3 translators per chunk (never reduce to 1 or 2)
- Full style analysis phase (never skip)
- Target style bible required (never omit)
- Translation memories for all recurring items (never "wing it")
- Review phase required (never accept raw translator output)
- Calculated chunking (never eyeball it)
- **Orchestrator launches all subagents** (translators and reviewers - subagents cannot launch other subagents)
- **3 translators launched in parallel** (single message with 3 Task calls)
- **Reviewer launched only after all 3 translators complete** (not before)
- Sequential chunk processing (each chunk completes before next starts)
- Phase 8 cleanup required (never leave artifacts scattered)

## Common Mistakes

| Mistake | Why It's Wrong | Fix |
|---------|---------------|-----|
| Single agent translates entire document | Context overflow, inconsistent quality, no cross-checking | Use multi-agent team with orchestrator |
| **Reviewer launches translator subagents** | Subagents cannot launch other subagents (Claude Code limitation) | Orchestrator (main thread) must launch all translators and reviewers |
| **Orchestrator doesn't launch translators in parallel** | Sequential translator execution is slower than necessary | Launch 3 translators in parallel (single message with 3 Task calls) |
| **Orchestrator launches reviewer before translators complete** | Reviewer needs all 3 translation outputs to compare | Wait for all 3 translators to complete before launching reviewer |
| Skipping style analysis "to save time" | Style drifts, voice lost, cultural adaptation fails | Style analysis is NON-NEGOTIABLE when style matters |
| Reducing translators from 3 to 1 | Lose adversarial quality comparison benefits | 3 translators mandatory |
| Translating code/syntax | Breaks functionality, creates confusion | Only translate comments/docstrings |
| Word-for-word poetry translation | Destroys form, loses poetic qualities | Prioritize form + feeling over literal meaning |
| Not creating translation memories | Inconsistent terminology, character names change | Translation memories mandatory for recurring narrative constants (names, places, repeated phrases) |
| Creating over-inclusive translation memories | Wastes time, creates noise, provides no value for standard terminology | Only create memories for recurring items or proprietary terminology requiring specific approval |
| Chunking by size only | Breaks logical units, creates disjointed translation | Chunk by logical boundaries + size |
| Orchestrator runs out of context | Can't coordinate, loses cohesion | Orchestrator only coordinates, never holds document content |
| Skipping review phase | No quality check, translator errors propagate | Review phase NON-NEGOTIABLE |
| Not asking user for clarification | Wrong assumptions, wasted effort, poor fit | Orchestrator MUST clarify ALL aspects before starting |
| Creating task tracker but never updating it | No execution log, placeholders remain, can't verify completion | Update tracker AFTER EVERY PHASE with actual agent IDs, file locations, timestamps, and completion status |
| **Reviewer includes meta-commentary in output file** | Meta-commentary (headers, footers, synthesis reports, "Reviewed by" notes) contaminates final document, requires manual cleanup | Reviewer output MUST contain ONLY translated content. Meta-analysis goes in task tracker, NOT in output file. Orchestrator must verify outputs are clean before concatenating. |
| **Converting document format (plain text→markdown, HTML→text, etc.)** | Changes document structure, breaks compatibility, violates user requirements | Output format MUST match source format exactly. Plain text→plain text, markdown→markdown, HTML→HTML, code→code. Orchestrator must verify format preservation before concatenating. |
| **Chapter/section separation not matching source** | Output has different blank lines, missing separators, wrong headings than source | Match source document structure exactly. Same number of blank lines, same horizontal rules, same heading formats. Style analyst must document separation patterns in Phase 1. |
| **Processing chunks in parallel** | Cascading errors, no cross-chunk learning, inconsistent quality | Sequential processing only: each chunk (3 translators + 1 reviewer) must complete before next chunk starts |
| **Leaving chunk files scattered after completion** | Clutters workspace, confuses future sessions, unprofessional | Run Phase 8 cleanup: delete chunk files, archive valuable guides |

## Rationalization Table

| Excuse | Reality |
|--------|---------|
| "This document is small enough for one agent" | Small documents still benefit from style analysis, translation memories, and review. Quality is not about size alone. |
| "Style analysis takes too long" | Style analysis prevents having to redo the entire translation when style drifts. It's faster to do it right than to fix it later. |
| "Three translators is overkill" | Three translators provide adversarial quality comparison. The reviewer combines best elements from all three. One translation = single blind spot. |
| "I can combine chunks to save time" | Chunks calculated for context safety. Exceeding 50% context = agents run out = quality degrades = rework required. |
| "Translation memories aren't needed for this" | Any recurring term (character, location, concept) needs consistency. Translation memories prevent errors. BUT: Standard terminology doesn't need memories. Use the testing questions. |
| "We're on a tight deadline, skip review" | Review catches errors that would require rework. It's faster to review once than to fix and re-translate later. |
| "I'll handle context carefully without calculating" | Without calculation, you WILL exceed context. The math is required, not optional. |
| "Direct translation is faster than rewriting" | Direct translation creates awkward, unnatural text that requires more editing. Rewriting gets it right the first time. |
| "Cultural adaptation isn't necessary" | Literal translation of cultural references confuses readers. Adaptation is required for accessibility. |
| "The user didn't specify style, so any style is fine" | Wrong. If style not specified, ASK. Don't assume. |
| "The task tracker is just for planning, I don't need to update it" | Wrong. The tracker is a living execution log. Without updates, you can't verify completion or track progress. Update after EVERY phase. |
| "Reviewer can launch the 3 translators" | Wrong. Subagents cannot launch other subagents. The orchestrator (main thread) must launch all translators and reviewers. |
| "Launching translators sequentially is simpler" | Wrong. Parallel translator execution is required for efficiency. Launch 3 translators in a single message. |
| "I can launch the reviewer as soon as the first translator finishes" | Wrong. The reviewer needs all 3 translation outputs to compare. Wait for all 3 to complete before launching reviewer. |
| "Markdown is cleaner than plain text" | Wrong. Format preservation is CRITICAL. You cannot change the document format. Plain text source must produce plain text output. Period. |
| "It's just a format change, the content is the same" | Wrong. Format changes break compatibility, alter structure, violate user requirements. The format IS part of the content. |
| "Extra blank lines between chapters make it more readable" | Wrong. Document structure must match source exactly. Extra blank lines change the document organization. Match source spacing exactly. |
| "Quality scores help the user understand the translation quality" | Wrong. Scores belong in task tracker or separate report. Output files contain only translated content. |
| "Marking the end of translation is helpful" | Wrong. If source doesn't have end markers, don't add them. Match source exactly. |
| "Parallel processing is faster" | Wrong. Sequential processing prevents cascading errors. Each chunk informs the next. Parallel processing creates inconsistencies that require rework. |
| "Cleanup is optional, I'll leave the chunks for reference" | Wrong. Chunks clutter workspace and confuse future sessions. Cleanup is required. Archive valuable guides, delete intermediate artifacts. |

## Red Flags - STOP and Start Over

- Single agent assigned to large document
- Style analysis skipped or compressed
- Fewer than 3 translators per chunk
- Chunks not calculated for context safety
- Translation memories not created (when needed)
- Review phase skipped
- Code/syntax being translated
- Orchestrator holding document content
- User requirements not clarified
- Word-for-word poetry translation
- Task tracker contains placeholders like "[PENDING ASSIGNMENT]" or "[TO BE CLARIFIED]" after work has begun
- Task tracker shows all statuses as "BLOCKED" when phases are actually complete
- **Reviewer launching translator subagents**
- **Orchestrator launching translators sequentially instead of in parallel**
- **Orchestrator launching reviewer before all 3 translators complete**
- **Reviewer output files contain meta-commentary (headers, footers, synthesis reports)**
- **Orchestrator concatenates reviewer outputs without verifying they contain only translated content**
- **Document format changed (plain text→markdown, HTML→text, etc.)**
- **Output file extension doesn't match source file extension**
- **Chapter/section separation doesn't match source (different blank lines, missing separators)**
- **Style analysis didn't document chapter/section separation patterns**
- **Processing multiple chunks simultaneously**
- **Chunk files left in workspace after translation completes**

**All of these mean: Stop. Replan. Follow the workflow correctly.**

## CRITICAL: Chapter/Section Separation and Document Structure

**Universal Rule**: Chapter/Section separation in output MUST match source document format exactly.

### Why This Matters

When translating documents with chapters, sections, or distinct parts, the separation between sections is part of the document structure. Adding, removing, or modifying separators changes how readers perceive the document organization.

### Detection Process (Phase 1: Style Analysis)

**Style Analyst MUST identify and document:**

1. **Section/chapter markers**:
   - "Chapter X", "第X回", "Chapter X:", "Part X", "Section X", etc.
   - Exact format used in source

2. **Separator patterns**:
   - Number of blank lines between sections (0, 1, 2, or more)
   - Horizontal rules (---, ===, ***)
   - Page break markers
   - Decorative separators

3. **Visual hierarchy**:
   - ALL CAPS headings vs Title Case
   - Centered vs left-aligned
   - Underline styles (====, ----)
   - Indentation patterns

4. **Document the exact pattern with examples** from source

### Preservation Requirements (Phase 5-6: Translation & Review)

**Translators MUST:**
- Preserve exact section markers (translate text content, keep formatting)
- Match separator patterns exactly (same number of blank lines, same horizontal rules)
- Translate section titles while keeping structural format

**Reviewer MUST verify:**
- Section count matches source (same number of chapters/sections)
- Separators match source pattern exactly
- No extra blank lines added or removed
- Visual hierarchy preserved

### Examples

**Example 1: Chinese Novel with Chapter Headers**
```
Source format:
第一回：宴桃園豪傑三結義，斬黃巾英雄首立功

　　詞曰：

　　滾滾長江東逝水...

Correct output format:
Chapter 1: Three Heroes Sworn Brotherhood at the Peach Garden; Yellow Turban Rebels First Defeated

    The opening verse:

    The empire, long divided, must unite...

Incorrect (blank line mismatch):
Chapter 1: Three Heroes...

    The opening verse:

    The empire... (missing blank line before poem)
```

**Example 2: Technical Manual with Section Headers**
```
Source format:
## Getting Started

To begin installation...

Next Steps...

## Configuration

After installation...

Correct output format:
## Getting Started

Pour commencer...

Next Steps...

## Configuration

Après l'installation...

Incorrect (missing blank line between sections):
## Getting Started

Pour commencer...

Next Steps...
## Configuration (missing blank line before next section)
```

### Universal Verification Checklist

After translation, verify:
- [ ] Section/chapter count matches source exactly
- [ ] Section marker format matches source (capitals, colons, numbering)
- [ ] Blank line count between sections matches source
- [ ] Horizontal rules/patterns preserved exactly
- [ ] Heading indentation/alignment preserved
- [ ] No extra separators added (no "=== TRANSLATION COMPLETE ===" unless in source)

### Common Mistakes

| Mistake | Symptom | Fix |
|---------|---------|-----|
| Adding extra blank lines between chapters | Output has different spacing than source | Match source blank line count exactly |
| Removing separator patterns | Document appears "compressed" | Preserve all horizontal rules, asterisks, etc. |
| Converting ALL CAPS headings | "CHAPTER I" becomes "Chapter 1" | Preserve heading case format |
| Centering/left-aligning changes | Visual hierarchy changes | Maintain alignment from source |
| Adding section numbers | Source has no numbers, output adds them | Never add structure not present in source |

---

## CRITICAL: Prohibited Meta-Commentary

**Universal Rule**: Output files MUST contain ONLY the translated content, matching the source structure exactly. Meta-commentary, analysis reports, and methodology explanations are PROHIBITED in translation output files.

### What Constitutes Meta-Commentary

**PROHIBITED in output files:**

1. **Translation methodology explanations**
   - "This translation was created using..."
   - "Translation approach:..."
   - "Methodology notes:..."

2. **Cultural adaptation notes**
   - "Translator's note:..."
   - "Cultural context:..."
   - "Adaptation rationale:..."

3. **Quality assessment scores**
   - "Naturalness: 9.2/10"
   - "Quality metrics:..."
   - "Assessment results:..."

4. **Style analysis explanations**
   - "The source text uses..."
   - "Style decisions:..."
   - "Voice preservation notes:..."

5. **End-of-translation reports**
   - "TRANSLATION COMPLETE"
   - "END OF DOCUMENT"
   - "Translation statistics:..."
   - "Word count:..."
   - "Completion date:..."

6. **Project metadata in output**
   - "Translated by: Agent..."
   - "Date: 2026-01-31"
   - "Version: 2.0"
   - "Chunk: 1 of 10"

7. **Synthesis reports**
   - "Reviewer's synthesis:..."
   - "Best elements from each translation:..."
   - "Translation choices explained:..."

### Where Meta-Content Belongs

**Meta-commentary belongs in:**
- Task tracker (execution log, synthesis decisions, agent IDs)
- Separate report files (if user explicitly requests them)
- Translation summary documents (after completion, separate from output)
- Style analysis documents (reference materials, not output)

**Meta-commentary does NOT belong in:**
- Translation output files (.txt, .md, .html, etc.)
- Final concatenated document
- Files the user will read as the translation

### Correct vs Incorrect Examples

**❌ INCORRECT - Meta-commentary in output file:**
```
TRANSLATION: Romance of the Three Kingdoms
Translator: Claude Sonnet 4.5
Date: January 31, 2026
Methodology: Agentic translation with 3 translators per chapter

Chapter 1: Three Heroes Sworn Brotherhood at the Peach Garden

The empire, long divided, must unite...

[Translation continues]

END OF TRANSLATION
Total words: 68,000
Chapters: 10
Quality: High
```

**✅ CORRECT - Clean output file:**
```
Chapter 1: Three Heroes Sworn Brotherhood at the Peach Garden

The empire, long divided, must unite...

[Translation continues]

Chapter 2: Zhang Fei Whips the Inspector

[Next chapter]
```

### Enforcement in Workflow

**Orchestrator (Phase 7: Assembly) MUST verify before concatenating:**

1. **Check first 10 lines**: Should start with actual content, not meta-headers
2. **Check last 10 lines**: Should end with actual content, not completion reports
3. **Search for meta-patterns**:
   - "TRANSLATION COMPLETE", "END OF", "FIN DU DOCUMENT"
   - "Translated by", "Translator: ", "Reviewer: "
   - "Methodology:", "Approach:", "Quality:"
   - Word counts, statistics, dates
4. **If ANY meta-commentary found**: DO NOT concatenate. Request corrected output from reviewer.

**Reviewer (Phase 6) MUST understand:**
- Output file is for translated content ONLY
- Synthesis decisions go in task tracker, not output file
- Quality assessments go in task tracker, not output file
- Any meta-content in output = failed output that must be corrected

### Common Rationalizations to Avoid

| Rationalization | Reality |
|-----------------|---------|
| "The user needs to know the translation methodology" | Put methodology in a separate report document, not in the translation itself. |
| "Quality scores show the translation is excellent" | Scores belong in task tracker or separate report, not in output file. |
| "It's helpful to mark the end of the document" | If source doesn't have end markers, don't add them. Match source exactly. |
| "Meta-commentary is just a few lines, it doesn't matter" | Any meta-content violates format preservation and requires manual cleanup. |
| "The user can just ignore the meta-content" | Wrong. User shouldn't have to ignore anything. Output should be clean. |

---

## Format-Specific Guidelines

### Plain Text Documents (.txt)

**Translate**: All text content

**Keep**:
- ALL CAPS headings (CHAPTER I, SECTION II)
- Underline characters (====, ----)
- Spacing and indentation
- Paragraph breaks
- Special characters (Chinese characters, symbols, etc.)

**Prohibited**:
- ❌ Markdown headers (# ## ###)
- ❌ Markdown bold (**text**)
- ❌ Markdown italics (*text*)
- ❌ Markdown links ([text](url))
- ❌ Any markdown syntax

**Example**:
- Source: `CHAPTER I\nINTRODUCTION\n=========\nThis is...`
- Output: `CHAPITRE I\nINTRODUCTION\n==========\nC'est...`

### Literary Works (Novels, Stories)
- Chunk by chapter (never mid-chapter)
- Character names in translation memories
- Cultural references adapted with footnotes if needed
- Dialogue voice preserved for each character
- Maintain narrative distance (first person vs omniscient)

### Poetry and Songs
- Form preservation critical (rhyme, rhythm, meter)
- Prioritize musicality and feeling over literal meaning
- Use translator's note if form must be sacrificed
- Count syllables/feet for metric poetry
- Parallel structures for Chinese poetry

### Code and Documentation
- Translate: Comments, docstrings, error messages, user-facing text
- Keep: Syntax, keywords, variable/function names, imports, type annotations
- Technical terminology: Use standard target language terms or keep English if standard
- Code examples: Keep unchanged, translate explanatory text around them

### JSON/API Documentation
- Translate: Descriptions, messages, user guidance
- Keep: Keys, property names, error codes, HTTP methods, URLs, data types
- Error messages: User-facing translation, technical codes unchanged

### HTML/Web Pages
- Translate: All visible text, navigation, labels, placeholders, buttons
- Keep: Tags, attributes, URLs, IDs, classes, email addresses, phone numbers
- Meta content: Translate description and title, keep lang attribute codes
- Form elements: Translate labels and options, keep names and IDs

### Markdown Documents
- Translate: Headers, paragraphs, lists, table content, blockquotes
- Keep: Code blocks (except comments), URLs, image paths, formatting syntax
- Links: Translate link text, keep URLs
- Code examples: Keep code unchanged, translate explanations

## Cultural Adaptation Strategies

**Universal principles for adapting content across cultural boundaries while preserving meaning and intent.**

### Core Cultural Dimensions

When translating between cultures with different characteristics, adapt accordingly:

| Dimension | High-Context Examples | Low-Context Examples | Adaptation Strategy |
|-----------|----------------------|----------------------|-------------------|
| **Context** | Japanese, Chinese, Arabic | English, German, Scandinavian | Make implicit meanings explicit for low-context targets; add contextual layers for high-context targets |
| **Individualism** | USA, UK, Australia | China, Japan, Korea | Emphasize group harmony for collectivistic targets; emphasize individual achievement for individualistic targets |
| **Communication** | Japan, Thailand, Mexico | Israel, Netherlands, USA | Soften confrontational language for indirect cultures; be direct for low-context cultures |
| **Power Distance** | Malaysia, Russia, China | Austria, Israel, Denmark | Add hierarchical markers for high-power-distance targets; reduce formality for low-power-distance targets |

### Specific Adaptation Strategies

**High-Context → Low-Context (e.g., Japanese → English)**
- Make implicit meanings explicit
- Add context that would be understood culturally in source
- Explain cultural references within the text (not footnotes)
- Preserve subtlety where possible but ensure clarity

**Low-Context → High-Context (e.g., English → Japanese)**
- Add layers of politeness and formality
- Use indirect expressions for direct requests
- Emphasize relationship-building language
- Preserve directness where critical for meaning

**Individualistic → Collectivistic (e.g., US English → Chinese)**
- Emphasize group success over individual achievement
- Add collective pronouns (we, our) instead of individual (I, my)
- Highlight harmony and cooperation
- Adapt praise to recognize group contributions

**Collectivistic → Individualistic (e.g., Chinese → US English)**
- Emphasize individual agency and choice
- Use active voice instead of passive constructions
- Highlight personal achievement where appropriate
- Adapt collective language to individualistic norms

**Direct → Indirect (e.g., German → Japanese)**
- Soften confrontational language
- Add face-saving elements
- Use suggestions instead of commands
- Preserve directness where critical for clarity

**Indirect → Direct (e.g., Japanese → German)**
- Make requests explicit
- Remove excessive politeness where it obscures meaning
- Use direct statements instead of implications
- Preserve courtesy where essential for relationship

### Genre-Specific Cultural Adaptation

**Literary Fiction**
- Preserve cultural setting (don't Westernize non-Western settings)
- Adapt cultural references to be understandable without footnotes
- Keep social structures authentic to source culture
- Translate emotional expressions to target-culture equivalents

**Business/Professional**
- Adapt formality levels to target culture's business norms
- Translate titles and honorifics to appropriate equivalents
- Adjust directness for cultural communication preferences
- Preserve professional relationships and hierarchies

**Marketing/Advertising**
- Adapt cultural touchpoints to target culture's values
- Use culturally resonant metaphors and imagery
- Adjust emotional appeals for cultural relevance
- Preserve brand voice while adapting cultural references

**Technical/Scientific**
- Minimal cultural adaptation needed
- Ensure examples are internationally relatable
- Adapt measurements and formats to local conventions
- Preserve technical precision above all

### Idiom and Cultural Reference Translation

**Principle: Translate meaning, not words**

**Source: "It's raining cats and dogs" (English)**
- ❌ Literal: "Il pleut des chats et des chiens" (French) - meaningless
- ✅ Cultural equivalent: "Il pleut des cordes" (French - "raining ropes")
- ✅ Descriptive: "Il pleut très fort" (French - "raining very hard")

**Source: "知己知彼，百戰不殆" (Chinese)**
- ❌ Literal: "Know yourself know enemy, hundred battles no danger" - awkward
- ✅ Cultural equivalent: "Know your enemy and know yourself..." (maintain wisdom tradition)
- ✅ Adapted meaning: "Thorough understanding leads to certain victory"

**Source: "Break a leg" (English theater idiom)**
- ❌ Literal: "Casarse une jambe" (French) - confusing
- ✅ Cultural equivalent: "Merde" (French theater tradition)
- ✅ Descriptive: "Good luck with the performance"

### Cultural Sensitivity Guidelines

**DO:**
- Adapt cultural references to be understandable
- Preserve the source culture's authenticity (don't over-Westernize)
- Use target-culture equivalents for proverbs and idioms
- Explain cultural context within the narrative flow
- Research cultural norms when uncertain

**DON'T:**
- Add footnotes for cultural explanations (integrate into text instead)
- Erase source culture's distinctiveness
- Assume all cultural concepts have direct equivalents
- Stereotype or exoticize source cultures
- Make assumptions without cultural knowledge

### When to Adapt vs. Preserve

**Adapt when:**
- Literal meaning would be confusing to target audience
- Cultural reference has no equivalent in target culture
- Idiom/proverb requires localization for impact
- Social convention differs significantly between cultures

**Preserve when:**
- Cultural distinctiveness is part of the work's appeal
- Source culture is unfamiliarity that adds richness
- Story depends on specific cultural setting
- Exact cultural concept is central to meaning

**Balance:**
- Literary works: Preserve authenticity while ensuring accessibility
- Technical works: Prioritize clarity and understanding
- Marketing: Adapt fully for cultural resonance
- Poetry: Preserve feeling and imagery over literal meaning

---

## Implementation Notes

### Starting a Translation Project

**As orchestrator**, your first step is ALWAYS:

1. Ask the user ALL clarification questions (Phase 0)
2. Get clear answers before proceeding
3. Document requirements in task tracker

Only then should you dispatch the style analyst agent.

### Agent Communication

**Orchestrator to agents**: Always provide:
- Clear task description
- Relevant context (what stage, what dependencies)
- Input file locations
- Expected output
- Quality criteria

**CRITICAL: Orchestrator launch patterns**:
- **Translators**: Launch 3 translators in PARALLEL using single message with 3 Task tool calls
- **Reviewer**: Launch 1 reviewer using single Task call AFTER all 3 translators for that chunk complete
- **Never**: Have a subagent launch other subagents (Claude Code limitation)

**Agent to orchestrator**: Always provide:
- Confirmation of task understanding
- Progress updates
- Output location
- Any issues or clarifications needed

### Task Tracker Format

```markdown
# Translation Task Tracker

## Project Info
- Source: [file location]
- Source Language: [language]
- Source Format: [.txt / .md / .html / code]
- Target Language: [language]
- Target Format: [same as source]
- Document Type: [type]
- Start Date: [date]

## Requirements
- Purpose: [from user]
- Audience: [from user]
- Special Requirements: [from user]
- Format Preservation: [CRITICAL - must preserve source format]

## Phase 0: User Clarification
- [x] All requirements clarified
- Document location: [path]
- Source format: [.txt / .md / .html / code]
- Target language: [language]
- Style guidance: [notes]

## Phase 1: Style Analysis
- [x] Source format detected: [.txt / .md / .html / code]
- [ ] Style analyst dispatched
- [ ] Style analysis document created: [location]
- [ ] Agent ID: [agent-id]

## Phase 2: Target Style Bible
- [ ] Target stylist dispatched
- [ ] Target style bible created: [location]
- [ ] Agent ID: [agent-id]

## Phase 3: Translation Memories
- [ ] Extracted recurring items OR N/A - not needed
- [ ] Translation memories created: [location] OR SKIPPED
- [ ] Agent ID: [agent-id] OR Orchestrator decision

## Phase 4: Chunking
- [ ] Chunker dispatched
- [ ] Total chunks: [N]
- [ ] Chunk assignments created
- [ ] Agent ID: [agent-id]

## Phase 5-6: Translation Teams

**NOTE**: For each chunk, orchestrator launches 3 translators in PARALLEL (single message), waits for all 3 to complete, THEN launches reviewer.

### Team 1 (Chunk [N])
- Source format: [format]
- Translator A: [agent-id] - [status]
- Translator B: [agent-id] - [status]
- Translator C: [agent-id] - [status]
- Reviewer: [agent-id] - [status] (launched after all 3 translators complete)
- Format verified: [yes/no]

[Repeat for each team...]

## Phase 7: Assembly
- [x] All chunks completed (N/N)
- [x] Format verified: all chunks match [source format]
- [x] Meta-commentary check: no headers/footers/synthesis reports
- [x] Assembled into final document
- [x] Final consistency check
- [x] Output location: [path]
- [x] Output format: [same as source]

## Phase 8: Cleanup and Archive
- [x] Archive directory created: translation-artifacts/[project-name]/
- [x] Deleted: N chunk files (translator outputs + reviewer drafts)
- [x] Archived: style analysis, target style bible, translation memories, task tracker
- [x] Final document location: [path]
- [x] Workspace clean: no chunk files remaining
- Overall project status: COMPLETE
- Completion timestamp: [YYYY-MM-DD HH:MM:SS]

## Translation Statistics
- Source word count: [N words]
- Target word count: [N words]
- Expansion ratio: [X.XX]
- Chunks: [N]
- Agents: [N translators + N reviewers]
- Duration: [HH:MM]
```

---

**Remember**: Maximum translation quality requires discipline. Every phase exists for a reason. Every agent has a purpose. Skip nothing. Cut no corners. The result will be worth it.
