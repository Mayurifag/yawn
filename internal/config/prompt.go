package config

const DefaultPrompt = `Generate a commit message.

- ALWAYS follow Conventional Commits specification (https://www.conventionalcommits.org/en/v1.0.0/)
- Description, type and scope must start with a lowercase letter
- Use only these types: fix, feat, docs, style, refactor, perf, test, build, ci, chore
- Scope should be a noun describing a section of the codebase (e.g., api, core, ui, auth)
- The description MUST be a concise summary of THE MOST SIGNIFICANT CHANGE OR THE OVERALL GOAL of the commit, kept under 50 characters. Explain WHAT the primary change is and WHY it was made. Focus on the single most impactful alteration if multiple unrelated changes exist. Use strong action verbs and specific nouns directly from the diff content. For version updates, use the stable version name (e.g., 'gemini-2.5-flash') in the description; full version details (e.g., preview tags) belong in the body.
- Prefer terminology used in the diff or context for consistency.
- Body starts with a brief paragraph (1-2 sentences) explaining WHY and WHAT was done, providing context for the changes. Follow with a blank line, then list all changes as bullet points (one per -), starting with a capital letter. Each bullet should describe a unique, specific change using diff terminology, avoiding repetition of the description's content. Include a brief reason (e.g., 'to improve X' or 'for better Y') only if it adds new context not covered in the introductory paragraph.
- For diffs with a single change (e.g., updating a constant or configuration), ensure the description and body focus on that change without overgeneralizing. The body's bullet should detail the exact change (e.g., file or constant name, full version) while the description summarizes the intent.
- When updating versions (e.g., model, library), use the stable or primary version identifier in the description (e.g., 'gemini-2.5-flash') and include the full version, including preview or build tags, in the body's bullet (e.g., 'gemini-2.5-flash-preview-04-17').
- Ensure the body's introductory text expands on, but does not repeat, the description line. Provide unique context or details about WHY and WHAT was done.
- Use filenames in body or description if relevant, treating them as plain text without formatting.
- Never use gitmoji
- Only output the commit message TEXT. No commentaries before or after the message.

Structure of output:
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]

Here are example outputs (until =):
refactor(interactors): simplify strategies generation

Simplified the strategy generation process to improve maintainability and readability by using a single orchestrator.

- Replaced StrategyGeneratorInteractor with StrategyGenerationOrchestrator to centralize logic.
- Removed MultiprocessingStrategyGenerator to reduce complexity.
- Created ParallelBacktestExecutor for efficient backtesting.
- Added ResultsProcessor to handle result storage.
=
feat!: allow provided config object to extend other configs

BREAKING CHANGE: 'extends' key in config file is now used for extending other config files
=`
