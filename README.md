# Yawn - Never write a Git commit message again

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)

**Writing Git commit messages makes you yawn?** 🥱 Here is a tool that will stage/commit/push for you with a single yolo push button!

---

## Why Yawn?

`yawn` is designed for speed and simplicity. In its most basic form, you make changes, run `yawn`, and boom – your code is staged (if needed), committed with a smart AI-generated message, and optionally pushed. All in one go!

But "simple" doesn't mean "limited". Under the hood, `yawn` is **super customizable**:

*   Tweak the AI prompt? ✅
*   Want to automatically stage *all* changes? ✅
*   Ignore specific files in the diff for the AI besides the .gitignore? ✅
*   Override defaults using environment variables or additional parameters? ✅
*   Override config per project? ✅
*   Avoid Gemini API limits? ✅
*   Interactive? ✅
*   Sensible defaults? ✅
*   Change the Gemini model? ✅
*   Need to push skipping Git hooks (`git push --no-verify`)? ✅ Yea, you may even force push, if you want.

It **really** adapts to your workflow, that's why I made it and why it is better than any other Git commit message generator I've tried.

---

## 🚀 Installation

Requires Go 1.24+. Make sure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`.

Run: `go install github.com/Mayurifag/yawn/cmd/yawn@latest`

There are also pre-compiled binaries in packages, yet I am too lazy to write instructions to install them in Windows, MacOS and Linux.

Pro-tip: `alias q="yawn"` is very useful, add it after first tries + config adaptations and your workflow will be changed forever. 😉

---

## 🛠️ Customization

Want to tweak things? `yawn` is flexible!

*   **See all options:** Run `yawn --generate-config` to see a commented default configuration file (`.yawn.toml`).
*   **Common tweaks:**
    *   `gemini_model`: Use a different Gemini model.
    *   `prompt`: Rewrite the instructions for the AI.
    *   `ask_stage`: Set to `false` to never stage automatically.
    *   `auto_push`: Set to `true` to always push after commit.
    *   `push_command`: Change how `yawn` pushes (e.g., `git push --no-verify origin HEAD`).
    *   `ignore_patterns`: List file patterns (`*.log`, `*.tmp`) to exclude from the diff sent to the AI.

Place your customizations in `./.yawn.toml` (project-specific) or `~/.config/yawn/config.toml` (global), or use `YAWN_*` environment variables.

---

## 🤝 Contributing

Found a bug or have an idea? Issues and Pull Requests are welcome on the [GitHub repository](https://github.com/Mayurifag/yawn)!

---

## 📜 License

This project is released into the public domain under The Unlicense. See the [LICENSE](LICENSE) file for details.

---

## Roadmap

* [ ] Release 1.0.0 when it will be mature enough
* [ ] Make installation easier for all OSes (i.e. homebrew installation) and README.md better

Actually, I do not plan any more features, except adding free AI API providers except of Gemini. Project considered as done.
