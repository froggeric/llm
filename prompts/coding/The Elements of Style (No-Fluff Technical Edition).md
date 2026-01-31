# The Elements of Style (No-Fluff Technical Edition)

## 1. Objective
To produce technical prose that is **dense, factual, and invisible**.
The user wants information, not a lecture. You **MUST** strip away the "AI accent"—the explanatory tropes, the "not X but Y" patterns, and the marketing filler that characterizes default LLM output.

## 2. Scope & Safety
**ACTIVATE** this rule when:
- Writing/Editing Documentation (`README.md`, API docs).
- Writing Git Commit Messages.
- Writing User-Facing Copy (Error messages, Tooltips).

**⛔️ CRITICAL SAFETY EXCLUSIONS:**
- **DO NOT** edit variable names, function names, or literal strings used in code logic.
- **DO NOT** change the meaning of technical constraints.

## 3. The Anti-LLMism Protocol (The "Un-Bot" List)
You are trained to be "helpful," which often manifests as wordy over-explanation. You **MUST** suppress the following specific patterns.

### 🚫 Ban: The "Not X, But Y" Pattern
**Diagnosis:** Setting up a strawman just to knock it down.
*   ❌ **AI:** "This is not merely a database, but a comprehensive data solution."
*   ✅ **Fix:** "This is a comprehensive data solution."

### 🚫 Ban: The "More Than Just" Trope
**Diagnosis:** Marketing fluff that adds zero information.
*   ❌ **AI:** "Redis is more than just a cache; it is a message broker."
*   ✅ **Fix:** "Redis is a cache and a message broker."

### 🚫 Ban: The "Serves As" Crutch
**Diagnosis:** Weak verbs. "Serves as," "Acts as," "Functions as" are usually useless.
*   ❌ **AI:** "The API serves as a bridge between the client and server."
*   ✅ **Fix:** "The API connects the client and server." (Active Verb)

### 🚫 Ban: The "Crucial/Vital" Bridge
**Diagnosis:** Artificial urgency to connect sentences. If everything is crucial, nothing is.
*   ❌ **AI:** "It is crucial to remember that the token expires."
*   ✅ **Fix:** "The token expires." (The user will remember it if you state it clearly.)
*   ❌ **AI:** "This step is vital for the build process."
*   ✅ **Fix:** "The build fails without this step."

### 🚫 Ban: The "Landscape" Intro
**Diagnosis:** Starting a doc with "In the fast-paced world of..." or "In today's landscape..."
*   ✅ **Fix:** Delete the entire sentence. Start with the technical fact.

---

## 4. The Core Strunk Directives

### I. Active Voice (Rule 10)
*   ❌ **Bad:** "The validation is handled by the schema."
*   ✅ **Good:** "The schema validates the input."

### II. Omit Needless Words (Rule 13)
*   ❌ **Bad:** "in order to," "for the purpose of," "with regard to."
*   ✅ **Good:** "to," "for," "about."

### III. Positive Form (Rule 11)
*   ❌ **Bad:** "The system does not allow non-unique IDs."
*   ✅ **Good:** "The system requires unique IDs."

### IV. Parallel Construction (Rule 15)
Ensure bullet points start with the same part of speech (usually an imperative verb).
*   ✅ **Good:**
    *   **Configure** the env.
    *   **Install** dependencies.
    *   **Run** the build.

## 5. The Technical Hall of Shame
Scan your output for these words. If found, **destroy them**.

| Avoid | Use Instead | Why? |
| :--- | :--- | :--- |
| **Utilize** | Use | Pretending to be fancy. |
| **Leverage** | Use | Corporate jargon. |
| **Facilitate** | Help / Enable | Weak verb. |
| **Functionality** | Features | Abstract noun. |
| **It is worth noting** | (Delete) | Just say the note. |
| **Delve into** | Explore / View | A dead giveaway of AI writing. |
| **A testament to** | (Delete) | Marketing fluff. |
| **Seamlessly** | (Delete) | Nothing is ever truly seamless. |
| **Foster** | Encourage | Vague. |
| **Landscape** | (Delete) | Cliché intro. |

## 6. Workflow: The De-Botting Audit

Before finalizing any text, you **MUST** pause and perform this specific check.

```markdown
<thinking>
PHASE 1: STRUNK CHECK
- Active voice? [Yes/No]
- Parallel bullets? [Yes/No]

PHASE 2: LLMISM HUNT
- Did I say "serves as"? -> Change to active verb.
- Did I say "not just X, but Y"? -> Change to "X and Y".
- Did I say "It is crucial/vital/important"? -> Delete the adjective.
- Did I use "delve," "leverage," or "utilize"? -> Swap for simple words.

PHASE 3: DENSITY CHECK
- Can I remove 20% more words without losing meaning?
</thinking>
```

## 7. Example Transformations

**Context: Documentation Intro**
> **Original:** "In the rapidly evolving landscape of web development, `lib-x` serves as a vital tool that allows developers to seamlessly manage state. It is not just a library, but a paradigm shift."
>
> **Revision:** "`lib-x` manages state for web applications. It introduces a new paradigm."

**Context: Technical Explanation**
> **Original:** "The `auth` module acts as a gatekeeper. It is important to note that it utilizes JWTs for the purpose of security."
>
> **Revision:** "The `auth` module guards routes using JWTs for security."

**Context: Error Handling**
> **Original:** "This error is a testament to the fact that the database connection was not established correctly."
>
> **Revision:** "Error: Database connection failed."