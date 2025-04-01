# Yawn 🥱 - Effortless Git Commits with AI

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)
<!-- [![codecov](https://codecov.io/gh/Mayurifag/yawn/graph/badge.svg?token=YOUR_CODECOV_TOKEN)](https://codecov.io/gh/Mayurifag/yawn) -->

**Tired of writing Git commit messages? Let AI do the heavy lifting!** `yawn` analyzes your staged changes using Google Gemini and suggests conventional commit messages, making your workflow faster and more consistent.

---

## ✨ Key Features

*   🤖 **AI-Powered:** Generates commit messages based on your code diffs.
*   ⚙️ **Configurable:** Customize the AI model, prompt, push behavior, and more.
*   💾 **Smart Configuration:** Loads settings from user (`~/.config/yawn/config.toml`), project (`.yawn.toml`), and environment variables (like `YAWN_GEMINI_API_KEY`). Environment variables always win!
*   🤝 **Git Integration:** Checks for changes, optionally stages them, commits, and even pushes.
*   🛡️ **Safety First:** Checks diff size against API limits.
*   💬 **Interactive:** User-friendly prompts guide you through the process.

---

## 🚀 Installation

Make sure you have Go (1.24+) installed and `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`.

Then, simply run:

`go install github.com/Mayurifag/yawn/cmd/yawn@latest`

---

## 🔑 Getting Started: API Key & Configuration

`yawn` needs a Google Gemini API key to work.

1.  **Get your Key:** Visit [Google AI Studio](https://aistudio.google.com/), sign in, and create a new API key. Keep it secret!
2.  **Set your Key:** The easiest way is to set an environment variable:
    `export YAWN_GEMINI_API_KEY="YOUR_API_KEY_HERE"`
    (Add this to your `.bashrc`, `.zshrc`, etc. for persistence).
    Alternatively, you can add it to the configuration file.
3.  **Generate Config (Optional):** See all options and create a local config file:
    `yawn --generate-config`
    This creates a `.yawn.toml` file in your current directory. You can also have a global config at `~/.config/yawn/config.toml`.

---

## 💡 Usage

1.  Make changes to your code.
2.  Run `yawn` in your terminal within the repository.
3.  If you haven't staged changes, `yawn` might ask if you want to stage them (configurable).
4.  It analyzes the diff and generates a commit message.
5.  Review the message and confirm the commit.
6.  Decide whether to push the changes (configurable).

That's it! Your commit is done.

---

## 🛠️ Customization Highlights

You can tweak `yawn`'s behavior via the config file (`.yawn.toml` or `~/.config/yawn/config.toml`) or environment variables:

*   `gemini_model` / `YAWN_GEMINI_MODEL`: Choose a different Gemini model (default: `gemini-2.0-flash-lite`).
*   `prompt` / `YAWN_PROMPT`: Change the instructions given to the AI.
*   `ask_stage` / `YAWN_ASK_STAGE`: Control whether `yawn` asks to stage changes (`true`/`false`).
*   `auto_push` / `YAWN_AUTO_PUSH`: Automatically push after commit (`true`/`false`).
*   `push_command` / `YAWN_PUSH_COMMAND`: Customize the push command (default: `git push origin HEAD`).
*   `ignore_patterns` / `YAWN_IGNORE_PATTERNS`: Comma-separated list of file patterns to ignore in the diff (e.g., `*.log,*.tmp`).

Check the output of `yawn --generate-config` for all options and descriptions.

---

## 🧑‍💻 Development & Contributing

Interested in improving `yawn`?

*   Clone the repo: `git clone https://github.com/Mayurifag/yawn.git`
*   Use `make help` to see available development commands (build, test, lint, etc.). Requires `make` and `mise`.
*   Contributions (issues, PRs) are welcome! Please format (`make fmt`) and lint (`make lint`) your code.

---

## 📜 License

This project is released into the public domain under The Unlicense. See the [LICENSE](LICENSE) file for details.
