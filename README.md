# Yawn 🥱 - AI Git Commiter

[![Go Version](https://img.shields.io/github/go-mod/go-version/Mayurifag/yawn)](https://github.com/Mayurifag/yawn/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/Mayurifag/yawn)](https://goreportcard.com/report/github.com/Mayurifag/yawn)
[![CI](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/ci.yml)
[![Release](https://github.com/Mayurifag/yawn/actions/workflows/release.yml/badge.svg)](https://github.com/Mayurifag/yawn/actions/workflows/release.yml)
<!-- [![codecov](https://codecov.io/gh/Mayurifag/yawn/graph/badge.svg?token=YOUR_CODECOV_TOKEN)](https://codecov.io/gh/Mayurifag/yawn) -->
<!-- Add Codecov badge after setting up Codecov token -->

`yawn` is a command-line tool that uses AI (specifically Google's Gemini models) to generate Git commit messages based on your staged changes. It aims to streamline the commit process, especially for frequent or complex changes.

## Features

*   **AI-Powered Commit Messages:** Leverages Google Gemini to suggest commit messages based on your code changes.
*   **Configurable:** Customize the AI model, prompt, ignored files, and behavior via configuration files and environment variables.
*   **Layered Configuration:** Settings are loaded from user config (`~/.config/yawn/config.toml`), project config (`./.yawn.toml`), and environment variables (e.g., `YAWN_GEMINI_API_KEY`), with environment variables taking the highest priority.
*   **Git Integration:**
    *   Checks for uncommitted changes.
    *   Optionally checks for staged changes and prompts the user to stage if necessary.
    *   Applies the generated commit message using `git commit`.
    *   Optionally pushes the commit automatically or after confirmation.
*   **Safety Checks:** Verifies diff size against model token limits before sending to the API.
*   **User-Friendly:** Provides interactive prompts with defaults and a spinner during AI processing.
*   **Verbose Mode:** Offers detailed logging about configuration loading and execution steps.

## Installation

### Using `go install`

Ensure you have Go installed (version 1.24.1 or later recommended) and your `GOPATH` is set up correctly (`$GOPATH/bin` or `$HOME/go/bin` should be in your `PATH`).

```bash
go install github.com/Mayurifag/yawn/cmd/yawn@latest
```

### Getting a Gemini API Key

1.  Go to [Google AI Studio](https://aistudio.google.com/).
2.  Sign in with your Google account.
3.  Click on "Get API key" in the top left or navigate to the API key section.
4.  Create a new API key for your project.
5.  **Important:** Keep your API key secure. Using environment variables (`YAWN_GEMINI_API_KEY`) or the user config file (`~/.config/yawn/config.toml`) with restricted permissions is recommended over storing it in the project's `.yawn.toml`.

## Development

### Prerequisites

*   Go (see `.mise.toml`)
*   `mise` (for managing Go version and linters)
*   `make`
*   `golangci-lint` (installed via `mise install`)

### Setup

1.  Clone the repository: `git clone https://github.com/Mayurifag/yawn.git`
2.  Navigate into the directory: `cd yawn`
3.  Install tools: `mise install`

### Useful Make Commands

*   `make build`: Build the binary locally.
*   `make install`: Build and install the binary to `~/.local/bin`.
*   `make test`: Run unit tests.
*   `make coverage`: Run tests and view coverage report.
*   `make lint`: Run the linter.
*   `make fmt`: Format the code.
*   `make clean`: Remove build artifacts.
*   `make generate-config`: Create a default `.yawn.toml` in the current directory.
*   `make help`: Show available commands.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

1.  Fork the repository.
2.  Create a new branch (`git checkout -b feature/your-feature-name`).
3.  Make your changes.
4.  Ensure tests pass (`make test`).
5.  Ensure code is formatted (`make fmt`) and linted (`make lint`).
6.  Commit your changes (`git commit -am 'Add some feature'`).
7.  Push to the branch (`git push origin feature/your-feature-name`).
8.  Create a new Pull Request.
