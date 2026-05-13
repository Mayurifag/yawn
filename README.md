# Yawn

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)

AI-assisted Git commits, squashes, and pushes in a single binary.

`yawn` stages changes, writes Conventional Commit messages from your diff, commits, pushes, prints PR links, squashes branches, and makes force-pushes harder to regret.

## Why

Writing commit messages is small work that still breaks flow. `yawn` compresses the usual Git loop into one command: stage, describe, commit, push, and show the next PR link.

It is tuned for fast personal workflows:

- Conventional Commit messages based on the actual diff.
- One-command branch cleanup via `yawn squash`.
- Optional auto-stage and auto-push for trusted repos.
- Global and per-project config.
- SSH remote nudges, push retries, and force-push previews.

## Install

~~~sh
mise use -g github:Mayurifag/yawn@latest
~~~

Or download a release binary, put it on `PATH`, and make it executable.

From source:

~~~sh
make install
~~~

Generate a config:

~~~sh
yawn --generate-config > ~/.config/yawn/config.toml
~~~

After setup, aliases make it feel native:

~~~sh
alias q="yawn"
alias sq="yawn squash"
alias gpf="yawn force-push"
~~~

## Commands

| Command           | What it does                                                                         |
| ----------------- | ------------------------------------------------------------------------------------ |
| `yawn`            | Stage if needed, generate a commit message, commit, and optionally push.             |
| `yawn squash`     | Squash branch commits since `main`, `master`, or `dev` into one AI-generated commit. |
| `yawn force-push` | Show divergence, ask for confirmation, then run a safer force push.                  |

## Configuration

Config is read from `~/.config/yawn/config.toml`, then project `.yawn.toml` files, environment variables, and CLI flags. Later sources win.

Supported providers are intentionally small. `gemini` and `opencode_cli` were chosen because they work well with fast free-tier or low-cost models and keep `yawn` out of provider-specific account complexity.

| Provider       | Auth                     | Notes                                         |
| -------------- | ------------------------ | --------------------------------------------- |
| `gemini`       | Google AI Studio API key | Default direct API provider.                  |
| `opencode_cli` | Local OpenCode login     | Uses models available in your OpenCode setup. |

### Gemini

~~~toml
main_provider = "gemini"

[providers.gemini]
api_key = "YOUR_GOOGLE_AI_STUDIO_KEY"
model = "gemini-flash-latest"
~~~

### OpenCode CLI

~~~toml
main_provider = "opencode_cli"
fallback_provider = "gemini"

[providers.opencode_cli]
model = "PROVIDER/MODEL"

[providers.gemini]
api_key = "YOUR_GOOGLE_AI_STUDIO_KEY"
model = "gemini-flash-latest"
~~~

Prepare OpenCode once:

~~~sh
opencode providers login
opencode models
~~~

Configure any fast model your OpenCode account is allowed to use. `yawn` calls OpenCode with `--variant low`, `--no-thinking`, and no output token limit flag.

## Options

Common config keys:

| Key                       | Meaning                                                                  |
| ------------------------- | ------------------------------------------------------------------------ |
| `prompt`                  | Commit-message instructions.                                             |
| `main_provider`           | Primary AI provider. Default: `gemini`.                                  |
| `fallback_provider`       | Optional backup provider. Constructed lazily only after primary failure. |
| `request_timeout_seconds` | AI request timeout. Default: `15`.                                       |
| `auto_stage`              | Stage changes without prompting.                                         |
| `auto_push`               | Push after committing without prompting.                                 |
| `push_command`            | Push command. Default: `git push origin HEAD`.                           |
| `squash_auto_push`        | Force-push automatically after `yawn squash`.                            |
| `wait_for_ssh_keys`       | Wait for `ssh-add -l` before pushing.                                    |

CLI flags:

| Flag                | Meaning                                |
| ------------------- | -------------------------------------- |
| `--api-key`         | Override the primary provider API key. |
| `--auto-stage`      | Stage all changes without prompting.   |
| `--auto-push`       | Push after commit without prompting.   |
| `--generate-config` | Print the default config template.     |
| `--version`         | Print version information.             |

## Safety

`yawn` redacts likely-sensitive or noisy file contents before sending diffs to the AI provider. It sends only `path: category, +adds -dels` for git-crypt files, encrypted files, lockfiles, binary files, and files marked with `.gitattributes` `yawn=skip`.

HTTPS remotes can be converted to SSH, pushes use retries with per-attempt timeouts, and force-pushes show a divergence preview before proceeding.
