# Yawn

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)

**Writing Git commit messages makes you yawn?** 🥱 Here is a tool that will stage/commit/squash/push for you!

## Why Yawn?

In its most basic form, you make changes, run `yawn`, and boom – your code is staged (if needed), committed with AI-generated message, and pushed. All in one go!

But "simple" doesn't mean "limited". Under the hood, `yawn` is **super customizable**:

* Tweak the AI prompt or use different Gemini model? ✅
* Automatically stage changes, commit and push? ✅
* Squash the branch onto single commit fast? ✅
* Have a link to repository and MR? ✅
* Need to push skipping Git hooks (`git push --no-verify`)? You may even force push, if you want. ✅
* Push gets a per-attempt timeout and 3 retries with exponential backoff. ✅
* HTTPS remotes are detected and you're prompted to convert them to SSH. ✅
* Override defaults using environment variables or additional parameters? ✅
* Override config per project? ✅

It **really** adapts to your workflow, that's why I made it and why it is better
than any other Git commit message generator I've tried. It is also
cross-platform, amd64/arm64 supported and single binary.

## Installation

### Using release

~~~sh
mise use -g github:Mayurifag/yawn@latest # if you have mise installed
# or place binary from Releases in one of your `$PATH` folders, make it executable.
~~~

Generate global config. Minimal change is to add Google AI Studio API key there:

~~~sh
yawn --generate-config > ~/.config/yawn/config.toml
~~~

Pro-tip: `alias q="yawn"` is very useful, add it after first tries + config
adaptations and your workflow will be changed forever. 😉 Same goes for
`alias sq="yawn squash"` — squash your branch commits into 1 with two keystrokes.

### Building from Source

`make install`

## Commands

### `yawn` (default)

Stages, commits with AI-generated message, and pushes.

If there are no local changes but unpushed commits exist, `yawn` lists them (with date, author, and subject) and offers to push. With `auto_push: true`, it pushes automatically.

After a successful push, `yawn` prints a PR creation link (GitHub compare URL or GitLab merge request URL) when you're on a non-default branch.

### `yawn squash`

Squashes all commits on the current branch (since it diverged from `main`/`master`/`dev`) into a single AI-generated commit.

If you have uncommitted changes when squashing, `yawn` prompts: **[Enter]** cancel, **[s]** stash & restore after, **[a]** include in squash.

### `yawn force-push`

Replacement for `git push --force-with-lease` that also prints the repository link and a pull request link (or PR creation link on a non-default branch). Handy to alias (e.g. `alias gpf="yawn force-push"`, like `git push --force`, but better).

Shows a divergence preview and asks for confirmation. Pass `--auto-push` to skip the prompt.

## Customization

* **See all options:** Run `yawn --generate-config` to see a commented default configuration file (`.yawn.toml`).
* **Common tweaks:**
  * `prompt`: Rewrite the instructions for the AI.
  * `gemini_model`: Change the Gemini model (default: `gemini-flash-latest`; falls back to `gemini-flash-lite-latest` on failure).
  * `request_timeout_seconds`: API request timeout in seconds (default: `15`).
  * `auto_stage`: Set to `true` to always stage automatically.
  * `auto_push`: Set to `true` to always push after commit.
  * `push_command`: Change how `yawn` pushes (e.g., `git push --no-verify origin HEAD`).
  * `squash_auto_push`: Set to `true` to automatically force-push after `yawn squash`. Defaults to `false`.
  * `wait_for_ssh_keys`: Set to `true` to make yawn wait until SSH keys are available via `ssh-add -l` before pushing (60-second timeout). Useful for workflows involving tools like KeePassXC where the agent might not have keys immediately. Defaults to `false`.

Place your customizations in `./.yawn.toml` (project-specific, also searched in parent directories) or `~/.config/yawn/config.toml` (global), or use `YAWN_*` environment variables.

By default, `yawn` generates commit messages following the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification, which provides a standardized format for commit messages. This makes your commit history more readable and enables automated tools to parse your commit messages.

## Roadmap

* Release 1.0.0 when it will be mature enough. homebrew, AUR, else?

## CLI Flags (not meant to be used, but just in case)

| Flag                | Description                            |
| ------------------- | -------------------------------------- |
| `--api-key`         | Override Gemini API key                |
| `--auto-stage`      | Stage all changes without prompting    |
| `--auto-push`       | Push after commit without prompting    |
| `--generate-config` | Print default config template and exit |
| `--version`         | Print version and exit                 |
