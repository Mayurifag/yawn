# Project Notes

- Any change to yawn configuration behavior, defaults, or examples must also be reflected in the user's chezmoi-managed yawn config. Use `chezmoi cd` to locate the source, edit only the yawn config source file, then run `chezmoi apply` for the yawn config target only. Never apply the full chezmoi state for this project.
- Supported AI providers are intentionally limited to `gemini` and `opencode_cli`; do not reintroduce direct OpenAI API or OpenCode Go API providers unless explicitly requested.
- `opencode_cli` requires OpenCode V2 and uses the user's local login with `provider/model#low` unless a variant is configured. Run a private server with the inline `yawn` agent denying all tools; reasoning output is off by default. Do not pass removed `--pure` or `--variant` flags or an output token limit. `openai/gpt-5.3-codex-spark` does not support reasoning effort `none`.
