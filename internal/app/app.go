package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

// App orchestrates the yawn application logic.
type App struct {
	Config    config.Config
	GitClient git.GitClient
	Pusher    git.PushProvider
}

// NewApp creates a new App instance.
func NewApp(cfg config.Config, gitClient git.GitClient) *App {
	return &App{
		Config:    cfg,
		GitClient: gitClient,
		Pusher:    git.NewPusher(gitClient),
	}
}

// setupAndCheckPrerequisites performs initial setup and checks:
// - Starts verbose logging
// - Ensures API key is available
// - Checks for uncommitted changes
// Returns whether there are changes and any error encountered.
func (a *App) setupAndCheckPrerequisites() (bool, error) {
	if a.Config.Verbose {
		fmt.Fprintln(os.Stderr, "[APP] Starting yawn - AI Git Commiter using Google Gemini")
	}

	// Check for API key
	if a.Config.GeminiAPIKey == "" {
		fmt.Fprintln(os.Stderr, "No API key found. Please provide your Google Gemini API key.")
		fmt.Fprintln(os.Stderr, "You can get one from: https://makersuite.google.com/app/apikey")
		apiKey := ui.AskForInput("Enter your Google Gemini API key: ", true)
		if apiKey == "" {
			return false, fmt.Errorf("API key is required")
		}

		// Save the API key to user config
		if err := config.SaveAPIKeyToUserConfig(apiKey); err != nil {
			// Log error but continue since we have the key in memory
			fmt.Fprintf(os.Stderr, "Warning: Failed to save API key to config file: %v\n", err)
		}
		a.Config.GeminiAPIKey = apiKey
	}

	// Check for uncommitted changes
	hasChanges, err := a.GitClient.HasUncommittedChanges()
	if err != nil {
		return false, fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	if !hasChanges {
		return false, fmt.Errorf("no changes to commit")
	}

	return true, nil
}

// ensureStagedChanges ensures that changes are staged for commit.
// If auto_stage is enabled, stages all changes automatically.
// Otherwise, prompts the user to stage changes if needed.
// Returns an error if staging fails or is declined when required.
func (a *App) ensureStagedChanges() error {
	hasStaged, err := a.GitClient.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Has staged changes: %v\n", hasStaged)
	}

	if hasStaged {
		if a.Config.Verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] Already have staged changes, skipping staging prompt\n")
		}
		return nil // Already staged, nothing to do
	}

	if a.Config.AutoStage {
		if a.Config.Verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] Auto-staging enabled, staging all changes\n")
		}
		// Auto-stage all changes
		if err := a.GitClient.StageChanges(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		return nil
	}

	// Prompt user to stage changes
	hasUnstaged, err := a.GitClient.HasUnstagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Has unstaged changes: %v\n", hasUnstaged)
	}

	if hasUnstaged {
		if !ui.AskYesNo("You have unstaged changes. Would you like to stage them?", true) {
			return fmt.Errorf("staging required to proceed")
		}
		if err := a.GitClient.StageChanges(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
	} else {
		// We've already checked for staged changes above, and now we find no unstaged changes
		// This means there are no changes at all to commit
		return fmt.Errorf("no changes to commit - nothing is staged or unstaged")
	}

	return nil
}

// generateAndCommitChanges handles the commit message generation and commit execution.
// It retrieves the staged diff, generates a message using Gemini, and commits the changes.
// Returns an error if any step fails.
func (a *App) generateAndCommitChanges(ctx context.Context) error {
	// Get staged changes for commit message generation
	diff, err := a.GitClient.GetDiff()
	if err != nil {
		return fmt.Errorf("failed to get staged changes: %w", err)
	}
	if diff == "" {
		return fmt.Errorf("no staged changes to commit")
	}

	// Create a Gemini client with the API key (which is guaranteed to exist now)
	geminiClient, err := gemini.NewClient(a.Config.GeminiAPIKey)
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Generate commit message using Gemini with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, a.Config.GetRequestTimeout())
	defer cancel()

	spinner := ui.StartSpinner("Generating commit message...")
	message, err := geminiClient.GenerateCommitMessage(ctxTimeout, a.Config.GeminiModel, a.Config.Prompt, diff, a.Config.MaxTokens)
	ui.StopSpinner(spinner)
	ui.ClearLine()

	if err != nil {
		if ctxTimeout.Err() == context.DeadlineExceeded {
			return fmt.Errorf("commit message generation timed out after %s", a.Config.GetRequestTimeout())
		}
		if strings.Contains(err.Error(), "git diff is too large") {
			return fmt.Errorf("changes are too large for the configured 'max_tokens' (%d). Consider committing smaller changes or increasing the limit", a.Config.MaxTokens)
		}
		return fmt.Errorf("failed to generate commit message: %w", err)
	}

	if message == "" {
		return fmt.Errorf("empty commit message received from Gemini")
	}

	// Display the generated message
	fmt.Println("Generated commit message:")
	fmt.Println(message)

	// Commit changes
	if err := a.GitClient.Commit(message); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

// handlePushOperation manages the push workflow:
// - Checks if push is needed (auto-push or user prompt)
// - Verifies remotes exist
// - Executes the push command
// - Reports success/failure
// Returns an error if any step fails.
func (a *App) handlePushOperation() error {
	// Check if push is needed
	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Would you like to push changes now? (using: %s)", a.Config.PushCommand), true) {
			return nil // User declined push
		}
	}

	// Check for remotes
	hasRemotes, err := a.Pusher.HasRemotes()
	if err != nil {
		return fmt.Errorf("failed to check for remote repositories: %w", err)
	}
	if !hasRemotes {
		return fmt.Errorf("no remote repositories configured. Add one using 'git remote add <name> <url>'")
	}

	// Execute push command
	spinner := ui.StartSpinner("Pushing changes...")
	result, err := a.Pusher.ExecutePush(a.Config.PushCommand)
	ui.StopSpinner(spinner)
	ui.ClearLine()

	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("push command failed")
	}

	// Report success and repository link if available
	if result.RepoLink != "" {
		fmt.Fprintf(os.Stderr, "View your repository: %s\n", result.RepoLink)
	}

	return nil
}

// Run executes the main application logic.
func (a *App) Run() error {
	// Setup and check prerequisites
	hasChanges, err := a.setupAndCheckPrerequisites()
	if err != nil {
		return err
	}
	if !hasChanges {
		return nil // No changes to commit
	}

	// Ensure changes are staged
	if err := a.ensureStagedChanges(); err != nil {
		return err
	}

	// Generate commit message and commit changes
	if err := a.generateAndCommitChanges(context.Background()); err != nil {
		return err
	}

	// Handle push operation if needed
	if err := a.handlePushOperation(); err != nil {
		return err
	}

	return nil
}
