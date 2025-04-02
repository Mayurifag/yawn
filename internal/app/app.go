package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

// App orchestrates the yawn application logic.
type App struct {
	Config       config.Config
	GitClient    git.GitClient
	GeminiClient gemini.Client
	Pusher       git.PushProvider
}

// NewApp creates a new App instance.
func NewApp(cfg config.Config, gitClient git.GitClient, geminiClient gemini.Client) *App {
	return &App{
		Config:       cfg,
		GitClient:    gitClient,
		GeminiClient: geminiClient,
		Pusher:       git.NewPusher(gitClient),
	}
}

// Run executes the main logic of the yawn application.
func (a *App) Run(ctx context.Context) error {
	if a.Config.Verbose {
		ui.PrintInfo("Starting Yawn...")
	}

	// 1. Check API Key first
	apiKey := a.Config.GeminiAPIKey
	if apiKey == "" {
		ui.PrintInfo("Gemini API key is missing.")
		ui.PrintInfo("You can set it using:")
		ui.PrintInfo("  - YAWN_GEMINI_API_KEY environment variable")
		ui.PrintInfo("  - 'gemini_api_key' in .yawn.toml (project) or ~/.config/yawn/config.toml (user)")
		ui.PrintInfo("  - --api-key flag")
		ui.PrintInfo("Get a key from: https://aistudio.google.com/app/apikey")
		a.Config.GeminiAPIKey = ui.AskForInput("Please enter your Gemini API key:", true)
		if a.Config.GeminiAPIKey == "" {
			return fmt.Errorf("API key is required to proceed")
		}
		if err := config.SaveAPIKeyToUserConfig(a.Config.GeminiAPIKey); err != nil {
			// Log error but continue since we have the key in memory
			ui.PrintError("Failed to save API key to configuration file")
			ui.PrintInfo("The current session will continue, but you'll need to provide the key again next time")
			if a.Config.Verbose {
				ui.PrintError(fmt.Sprintf("Error details: %v", err))
			}
		} else {
			ui.PrintSuccess("API key saved to ~/.config/yawn/config.toml")
		}
		if a.Config.Verbose {
			ui.PrintInfo("API Key provided interactively.")
		}

		// Update the Gemini client with the new API key
		a.GeminiClient.SetAPIKey(a.Config.GeminiAPIKey)
	}

	// 2. Check for uncommitted changes
	hasChanges, err := a.GitClient.HasUncommittedChanges()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check git status: %v", err))
		return err
	}
	if !hasChanges {
		ui.PrintInfo("No uncommitted changes detected. Nothing to do.")
		return nil
	}
	if a.Config.Verbose {
		ui.PrintInfo("Uncommitted changes detected.")
	}

	// 3. Check for staged changes
	hasStaged, err := a.GitClient.HasStagedChanges()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to check for staged changes: %v", err))
		return err
	}

	// 4. Handle staging if needed
	if !hasStaged {
		if a.Config.Verbose {
			ui.PrintInfo("No changes staged. Checking if staging is needed/requested.")
		}

		if a.Config.AutoStage {
			if a.Config.Verbose {
				ui.PrintInfo("Auto-staging all changes...")
			}
			err := a.GitClient.StageChanges()
			if err != nil {
				ui.PrintError(fmt.Sprintf("Failed to stage changes: %v", err))
				return err
			}
			ui.PrintSuccess("All changes staged successfully.")
		} else {
			if !ui.AskYesNo("Would you like to stage all changes for commit? (This will run 'git add -A')", true) {
				ui.PrintInfo("Staging declined.")
				ui.PrintInfo("To stage changes, either:")
				ui.PrintInfo("  1. Stage changes manually using 'git add'")
				ui.PrintInfo("  2. Run yawn again and choose to stage changes")
				return nil
			}

			if a.Config.Verbose {
				ui.PrintInfo("Staging all changes...")
			}
			err := a.GitClient.StageChanges()
			if err != nil {
				ui.PrintError(fmt.Sprintf("Failed to stage changes: %v", err))
				return err
			}
			ui.PrintSuccess("All changes staged successfully.")
		}
		ui.PrintInfo("Your changes are now staged and ready to commit.")
	} else if a.Config.Verbose {
		ui.PrintInfo("Changes already staged. Proceeding with commit.")
	}

	// 5. Get Git Diff
	if a.Config.Verbose {
		ui.PrintInfo("Getting diff of staged changes...")
	}
	diff, err := a.GitClient.GetDiff()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to get git diff: %v", err))
		return err
	}
	if diff == "" {
		ui.PrintInfo("No staged changes detected. Nothing to commit.")
		return nil
	}
	if a.Config.Verbose {
		ui.PrintInfo("Diff of staged changes obtained successfully.")
	}

	// 6. Generate Commit Message
	ui.PrintInfo("Generating commit message with Gemini...")
	spinner := ui.StartSpinner("Waiting for AI")
	ctxTimeout, cancel := context.WithTimeout(ctx, a.Config.GetRequestTimeout())
	defer cancel()

	commitMessage, err := a.GeminiClient.GenerateCommitMessage(ctxTimeout, a.Config.GeminiModel, a.Config.Prompt, diff, a.Config.MaxTokens)

	ui.StopSpinner(spinner)
	ui.ClearLine() // Clean up the spinner line

	if err != nil {
		// Check for context deadline exceeded
		if ctxTimeout.Err() == context.DeadlineExceeded {
			ui.PrintError(fmt.Sprintf("Gemini request timed out after %s.", a.Config.GetRequestTimeout()))
		} else {
			ui.PrintError(fmt.Sprintf("Failed to generate commit message: %v", err))
		}
		// Check if it was the token limit error specifically
		if strings.Contains(err.Error(), "git diff is too large") {
			// Specific message already printed by Gemini client potentially, but reiterate here.
			ui.PrintError("The changes are too large for the configured 'max_tokens'.")
			ui.PrintInfo("Consider committing smaller changes or increasing 'max_tokens' in your configuration.")
		}
		return err
	}

	if commitMessage == "" {
		ui.PrintError("Gemini returned an empty commit message.")
		return fmt.Errorf("empty commit message received")
	}

	ui.PrintSuccess("Generated Commit Message:")
	fmt.Println("---")
	fmt.Println(commitMessage)
	fmt.Println("---")

	// 7. Commit
	if a.Config.Verbose {
		ui.PrintInfo("Creating commit with generated message...")
	}
	err = a.GitClient.Commit(commitMessage)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to commit changes: %v", err))
		return err
	}
	ui.PrintSuccess("Changes committed successfully.")

	// 8. Push
	shouldPush := a.Config.AutoPush
	if !shouldPush {
		if ui.AskYesNo(fmt.Sprintf("Would you like to push changes now? (using: %s)", a.Config.PushCommand), false) { // Default No
			shouldPush = true
		}
	}

	if shouldPush {
		// Check for remotes before attempting to push
		hasRemotes, err := a.Pusher.HasRemotes()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to check for remote repositories: %v", err))
			// Continue without pushing, but log the error
		} else if !hasRemotes {
			ui.PrintInfo("No remote repositories configured. Skipping push.")
			if a.Config.Verbose {
				ui.PrintInfo("To push changes, add a remote repository using 'git remote add <name> <url>'")
			}
		} else {
			if a.Config.Verbose {
				ui.PrintInfo(fmt.Sprintf("Pushing changes using command: %s", a.Config.PushCommand))
			}
			spinner := ui.StartSpinner("Pushing changes")
			result, err := a.Pusher.ExecutePush(a.Config.PushCommand)
			ui.StopSpinner(spinner)
			ui.ClearLine()

			if err != nil {
				ui.PrintError(fmt.Sprintf("Failed to push changes: %v", err))
				// Don't return error here, commit succeeded. Push failure is less critical.
			} else if result.Success {
				ui.PrintSuccess("Changes pushed successfully.")
				if result.RepoLink != "" {
					ui.PrintInfo(fmt.Sprintf("View your repository: %s", result.RepoLink))
				}
			}
		}
	} else if a.Config.Verbose {
		ui.PrintInfo("Skipping push based on config or user choice.")
	}

	return nil
}
