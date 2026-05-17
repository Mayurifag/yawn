# Configuration

Config is loaded in this order:

| Source | Notes |
| ------ | ----- |
| `~/.config/yawn/config.toml` | User config. |
| `.yawn.toml` | Project config, also searched in parent directories. |
| `YAWN_*` | Environment variables. |
| CLI flags | Highest precedence. |

## Providers

Supported providers are intentionally small:

| Provider | Auth | Notes |
| -------- | ---- | ----- |
| `gemini` | Google AI Studio API key | Default direct API provider. |
| `opencode_cli` | Local OpenCode login | Uses models available in your OpenCode setup. |

OpenCode is called with `--variant low`, `--no-thinking`, and no output token limit flag.

## Options

| Key | Meaning |
| --- | ------- |
| `prompt` | Commit-message instructions. |
| `main_provider` | Primary AI provider. Default: `gemini`. |
| `fallback_provider` | Optional backup provider. Constructed lazily only after primary failure. |
| `request_timeout_seconds` | AI request timeout. Default: `15`. |
| `auto_stage` | Stage changes without prompting. |
| `auto_push` | Push after committing without prompting. |
| `push_command` | Push command. Default: `git push origin HEAD`. |
| `squash_auto_push` | Force-push automatically after `yawn squash`. |
| `wait_for_ssh_keys` | Wait for `ssh-add -l` before pushing. Useful with KeePassXC or other SSH-agent unlock flows. |

## CLI Flags

| Flag | Meaning |
| ---- | ------- |
| `--api-key` | Override the primary provider API key. |
| `--auto-stage` | Stage all changes without prompting. |
| `--auto-push` | Push after commit without prompting. |
| `--generate-config` | Print the default config template. |
| `--version` | Print version information. |
