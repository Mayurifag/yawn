# Project Notes

- Any change to yawn configuration behavior, defaults, or examples must also be reflected in the user's chezmoi-managed yawn config. Use `chezmoi cd` to locate the source, edit only the yawn config source file, then run `chezmoi apply` for the yawn config target only. Never apply the full chezmoi state for this project.
- Supported AI providers are intentionally limited to `gemini` and `opencode_cli`; do not reintroduce direct OpenAI API or OpenCode Go API providers unless explicitly requested.
- `opencode_cli` uses the user's local OpenCode login with `--variant low`, `--no-thinking`, and no output token limit flag. `openai/gpt-5.3-codex-spark` does not support reasoning effort `none`.
