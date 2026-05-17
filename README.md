# Yawn

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)

Writing Git commit messages is small work, but it breaks flow.

`yawn` turns the end of a coding session into one command: stage the diff, ask 
**FREE** AI for a commit message, commit, push, and print the PR link.

## Why install it?

Because this is annoying:

~~~sh
git add .
git diff --cached
git commit -m "fix: something probably"
git push origin HEAD
open the repo
find the branch
open the PR page
~~~

This is less annoying: `q` (alias from `yawn`). Imagine effort you save every 
day compounded.

`yawn` handles the common Git chores:

- writes commit messages from the actual diff
- stages files when you let it
- pushes when you let it
- prints GitHub or GitLab PR links after push
- squashes a branch into one AI-named commit
- previews force-push divergence before pushing
- redacts secrets, lockfiles, binaries, and skipped paths before asking AI
- reads global config, project config, env vars, and CLI flags

It also handles the little workflow annoyances:

- waits for SSH keys so you can unlock KeePassXC instead of racing `git push`
- nudges HTTPS remotes toward SSH
- retries flaky pushes with per-attempt timeouts
- shows unpushed commits when there is nothing new to commit
- asks what to do with dirty files before squashing

It is a single Go binary. No daemon, no editor plugin, no extra service to keep 
alive.

## Install

~~~sh
mise use -g github:Mayurifag/yawn@latest
~~~

Or download a release binary, put it somewhere on `PATH`, and make it executable.

From source:

~~~sh
make install
~~~

Generate a config:

~~~sh
mkdir -p ~/.config/yawn
yawn --generate-config > ~/.config/yawn/config.toml
~~~

Add a provider, then try it in a repo with changes:

~~~sh
yawn
~~~

Useful aliases:

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

If there are no local changes but unpushed commits exist, `yawn` lists them and offers to push. After a successful push from a non-default branch, it prints a PR creation link.

## Configuration

Config is read from `~/.config/yawn/config.toml`, then project `.yawn.toml` files, environment variables, and CLI flags. Later sources win.

Default setup uses Gemini:

~~~toml
main_provider = "gemini"

[providers.gemini]
api_key = "YOUR_GOOGLE_AI_STUDIO_KEY"
model = "gemini-flash-latest"
~~~

Or use your local OpenCode login:

~~~toml
main_provider = "opencode_cli"
fallback_provider = "gemini"

[providers.opencode_cli]
model = "PROVIDER/MODEL"

[providers.gemini]
api_key = "YOUR_GOOGLE_AI_STUDIO_KEY"
model = "gemini-flash-latest"
~~~

Prepare OpenCode once with `opencode providers login` and `opencode models`.

For every option, run `yawn --generate-config` or see [configuration reference](docs/configuration.md).

## Safety

`yawn` redacts likely-sensitive or noisy file contents before sending diffs to the AI provider. It sends only `path: category, +adds -dels` for git-crypt files, encrypted files, lockfiles, binary files, and files marked with `.gitattributes` `yawn=skip`.

HTTPS remotes can be converted to SSH, pushes use retries with per-attempt timeouts, and force-pushes show a divergence preview before proceeding.

It still lets you do dangerous Git things. It just makes you look first.
