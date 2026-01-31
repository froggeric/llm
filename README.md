# froggeric/llm

Prompts, skills, and shell-first workflows for working with large language models (LLMs) and agentic frameworks such as Claude Code, Gemini CLI, cline, and Kilo Code.

---

Table of contents
- About
- Repository structure (clickable links)
- Quick start
- Examples
- Contributing
- License
- Contact

About
This repository contains curated prompt templates, small shell scripts, and recommended workflows for integrating LLMs and agentic tools into CLI-based pipelines. Files are intentionally shell-centric so you can combine them easily with shell scripts, environment variables, and CLI clients.

Repository structure (clickable links)
- README.md — this file
  https://github.com/froggeric/llm/blob/main/README.md
- .gitignore — repository ignore rules
  https://github.com/froggeric/llm/blob/main/.gitignore
- claude/ — Claude-specific prompts, scripts, and example workflows (Anthropic)
  https://github.com/froggeric/llm/tree/main/claude
- prompts/ — generic and categorized prompt templates and examples for different tasks and agents
  https://github.com/froggeric/llm/tree/main/prompts

Quick start
1. Clone the repository:
   git clone https://github.com/froggeric/llm.git
2. Inspect prompts/ for templates you can adapt to your LLM or agent client.
3. See claude/ for examples tailored to Claude Code / Claude CLI.
4. Integrate prompt files into your preferred client (Gemini CLI, cline, Kilo Code, etc.) and provide credentials via environment variables or your client config.

Examples
- Reuse a prompt template:
  1. Open a file under prompts/ and replace placeholders (context, examples, constraints).
  2. Pass it as the system or user prompt to your LLM client.
- Compose a shell workflow:
  - Use here-docs, env vars, and small shell wrappers to feed prompts and collect outputs:
    my_prompt=$(<prompts/example.txt)
    echo "$my_prompt" | my-llm-client --stdin
- Claude-specific patterns:
  - See the claude/ directory for suggested invocation patterns and example scripts targeting Anthropic models.

Contributing
Contributions are welcome. Suggested workflow:
1. Fork the repository.
2. Create a feature branch for your change.
3. Add or update prompts, scripts, or documentation.
4. Open a pull request with a clear description and example usage.

Please do not add secrets, API keys, or copyrighted content.

License
No LICENSE file detected in the repository. If you want this project to be open source, add a LICENSE (for example, MIT or Apache-2.0). If you want, I can add a license file for you — tell me which license to use.

Contact / Maintainer
- Owner: froggeric — https://github.com/froggeric

Notes
If you want the README expanded with example snippets taken directly from files in claude/ or prompts/, tell me which file(s) to include and I will update the README accordingly.