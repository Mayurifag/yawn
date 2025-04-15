# Yawn

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)

**Writing Git commit messages makes you yawn?** 🥱 Here is a tool that will stage/commit/push for you!

---

## Why Yawn?

In its most basic form, you make changes, run `yawn`, and boom – your code is staged (if needed), committed with AI-generated message, and pushed. All in one go!

But "simple" doesn't mean "limited". Under the hood, `yawn` is **super customizable**:

*   Tweak the AI prompt or use different Gemini model? ✅
*   Automatically stage changes, commit and push? ✅
*   Override defaults using environment variables or additional parameters? ✅
*   Override config per project? ✅
*   Avoid Gemini API limits? ✅
*   Sensible defaults? ✅
*   Need to push skipping Git hooks (`git push --no-verify`)? You may even force push, if you want. ✅

It **really** adapts to your workflow, that's why I made it and why it is better
than any other Git commit message generator I've tried.

---

## Installation

Requires Go 1.24+. Make sure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`.

Run: `go install github.com/Mayurifag/yawn/cmd/yawn@latest`

There are also pre-compiled binaries in packages, yet I am too lazy to write
instructions to install them in Windows, MacOS and Linux. Yeah, those `curl`
ones.

Pro-tip: `alias q="yawn"` is very useful, add it after first tries + config
adaptations and your workflow will be changed forever. 😉

---

## Customization

Want to tweak things? `yawn` is flexible!

*   **See all options:** Run `yawn --generate-config` to see a commented default configuration file (`.yawn.toml`).
*   **Common tweaks:**
    *   `gemini_model`: Use a different Gemini model.
    *   `prompt`: Rewrite the instructions for the AI.
    *   `ask_stage`: Set to `false` to never stage automatically.
    *   `auto_push`: Set to `true` to always push after commit.
    *   `push_command`: Change how `yawn` pushes (e.g., `git push --no-verify origin HEAD`).
    *   `wait_for_ssh_keys`: Set to `true` to make yawn wait until SSH keys are available via `ssh-add -l` before pushing. Useful for workflows involving tools like KeePassXC where the agent might not have keys immediately. Defaults to `false`.

Place your customizations in `./.yawn.toml` (project-specific) or `~/.config/yawn/config.toml` (global), or use `YAWN_*` environment variables.

By default, `yawn` generates commit messages following the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification, which provides a standardized format for commit messages. This makes your commit history more readable and enables automated tools to parse your commit messages.

---

## Contributing

Found a bug or have an idea? Issues and Pull Requests are welcome on the [GitHub repository](https://github.com/Mayurifag/yawn)!

---

## License

This project is released into the public domain under The Unlicense. See the [LICENSE](LICENSE) file for details.

---

## Roadmap

* In final result give link to user's PR or project page. Make that for github, gitlab (also private ones)
* Replace EstimateTokenCount implementation in internal/gemini/client.go with calls to c.client.CountTokens(ctx, modelName, genai.Text(text))
* Show token count for user, changed files, changed lines probably, beautiful output colored green/red with arrows. Show the branch name in the outputs
* Remove verbose mode - it is not needed and complicates code
* Think of better config handling. Current solution is complex. Though I also need source of config, koanf seems missing this functionality. Plus better init file handling.
* Add more CI commands
* Rewrite README.md
* Dockerfile -> release it -> add alias to README.md
* Make installation easier for all OSes (i.e. homebrew installation) and README.md better
* Release 1.0.0 when it will be mature enough
