package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
// - Checks for changes (staged or unstaged)
// Returns whether there are changes and any error encountered.
func (a *App) setupAndCheckPrerequisites() (bool, error) {
	if a.Config.Verbose {
		fmt.Fprintln(os.Stderr, "[APP] Starting yawn - AI Git Commiter using Google Gemini")
	}

	// Check for API key
	if a.Config.GeminiAPIKey == "" {
		ui.PrintInfo("No API key found. Please provide your Google Gemini API key.")
		fmt.Fprintln(os.Stderr, "You can get one from: https://makersuite.google.com/app/apikey")
		apiKey := ui.AskForInput("Enter your Google Gemini API key: ", true)
		if apiKey == "" {
			return false, fmt.Errorf("API key is required")
		}

		// Save the API key to user config
		if err := config.SaveAPIKeyToUserConfig(apiKey); err != nil {
			// Log error but continue since we have the key in memory
			ui.PrintError(fmt.Sprintf("Warning: Failed to save API key to config file: %v", err))
		}
		a.Config.GeminiAPIKey = apiKey
	}

	// Check for any changes (staged or unstaged)
	hasChanges, err := a.GitClient.HasAnyChanges()
	if err != nil {
		return false, fmt.Errorf("failed to check for changes: %w", err)
	}
	if !hasChanges {
		return false, fmt.Errorf("no changes to commit")
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[APP] Found changes to commit\n")
	}

	return true, nil
}

// ensureStagedChanges ensures that changes are staged for commit.
// If auto_stage is enabled, stages all changes automatically.
// Otherwise, prompts the user to stage changes if needed.
// Returns an error if staging fails or is declined when required.
func (a *App) ensureStagedChanges() error {
	// First check if there are already staged changes
	hasStaged, err := a.GitClient.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Initial check - Has staged changes: %v\n", hasStaged)
	}

	hasUnstaged, err := a.GitClient.HasUnstagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Has unstaged changes: %v\n", hasUnstaged)
	}

	if hasUnstaged {
		// Handle unstaged changes based on AutoStage setting
		if a.Config.AutoStage {
			if a.Config.Verbose {
				fmt.Fprintf(os.Stderr, "[DEBUG] Auto-staging enabled, staging all changes\n")
			}
			ui.PrintInfo(fmt.Sprintf("Auto-staging changes (enabled via %s)...", a.Config.GetConfigSource("AutoStage")))
			if err := a.GitClient.StageChanges(); err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}
			ui.PrintSuccess("Successfully staged changes.")
		} else {
			if !ui.AskYesNo("You have unstaged changes. Would you like to stage them?", true) {
				return fmt.Errorf("staging required to proceed")
			}
			if err := a.GitClient.StageChanges(); err != nil {
				return fmt.Errorf("failed to stage changes: %w", err)
			}
			ui.PrintSuccess("Successfully staged changes.")
		}
	}

	// Verify we have staged changes after potential staging
	hasStaged, err = a.GitClient.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}

	if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] Final check - Has staged changes: %v\n", hasStaged)
	}

	if !hasStaged {
		return fmt.Errorf("You have no changes to commit!")
	}

	return nil
}

// generateAndCommitChanges handles the commit message generation and commit execution.
// It retrieves the staged diff, generates a message using Gemini, and commits the changes.
// Returns an error if any step fails.
func (a *App) generateAndCommitChanges(ctx context.Context) error {
	// Get and validate staged changes
	diff, err := a.getAndValidateDiff()
	if err != nil {
		return err
	}

	// Create Gemini client
	geminiClient, err := gemini.NewClient(a.Config.GeminiAPIKey)
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Gather pre-generation information
	branchName, additions, deletions := a.gatherCommitInfo()

	// Display token count and prepare for generation
	tokenCountStr := a.getTokenCount(ctx, geminiClient, diff)

	// Display pre-generation info to the user
	ui.PrintPreGenerationInfo(tokenCountStr, a.Config.MaxTokens, branchName, additions, deletions)

	// Generate and process commit message
	message, err := a.generateCommitMessage(ctx, geminiClient, diff)
	if err != nil {
		return err
	}

	// Commit changes
	if err := a.GitClient.Commit(message); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	ui.PrintSuccess("Successfully committed changes.")

	return nil
}

// getAndValidateDiff retrieves the diff of staged changes and validates it.
func (a *App) getAndValidateDiff() (string, error) {
	diff, err := a.GitClient.GetDiff()
	if err != nil {
		return "", fmt.Errorf("failed to get staged changes: %w", err)
	}
	if diff == "" {
		return "", fmt.Errorf("no staged changes to commit")
	}
	return diff, nil
}

// gatherCommitInfo collects information about the current branch and diff stats.
func (a *App) gatherCommitInfo() (branchName string, additions int, deletions int) {
	// Get current branch name
	branchName, err := a.GitClient.GetCurrentBranch()
	if err != nil {
		branchName = "unknown" // Fallback if we can't get the branch name
		if a.Config.Verbose {
			fmt.Fprintf(os.Stderr, "[APP] Failed to get current branch: %v\n", err)
		}
	}

	// Get diff stats (additions and deletions)
	additions, deletions, err = a.GitClient.GetDiffNumStatSummary()
	if err != nil {
		additions, deletions = 0, 0 // Fallback if we can't get the stats
		if a.Config.Verbose {
			fmt.Fprintf(os.Stderr, "[APP] Failed to get diff stats: %v\n", err)
		}
	}

	return branchName, additions, deletions
}

// getTokenCount counts tokens in the diff and returns a formatted string.
func (a *App) getTokenCount(ctx context.Context, geminiClient gemini.Client, diff string) string {
	tokenCountStr := "?"
	tokenCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Short timeout for token counting
	defer cancel()

	// Prepare the prompt content for token counting
	finalPrompt := strings.Replace(a.Config.Prompt, "{{Diff}}", diff, 1)
	tokenCount, err := geminiClient.CountTokensForText(tokenCtx, a.Config.GeminiModel, finalPrompt)
	if err == nil {
		tokenCountStr = fmt.Sprintf("%d", tokenCount)
	} else if a.Config.Verbose {
		fmt.Fprintf(os.Stderr, "[APP] Failed to count tokens: %v\n", err)
	}

	return tokenCountStr
}

// generateCommitMessage generates a commit message using the Gemini API.
func (a *App) generateCommitMessage(ctx context.Context, geminiClient gemini.Client, diff string) (string, error) {
	// Generate commit message using Gemini with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, a.Config.GetRequestTimeout())
	defer cancel()

	spinner := ui.StartSpinner("Generating commit message...")
	message, err := geminiClient.GenerateCommitMessage(ctxTimeout, a.Config.GeminiModel, a.Config.Prompt, diff, a.Config.MaxTokens, a.Config.Temperature)
	ui.StopSpinner(spinner)
	ui.ClearLine()

	if err != nil {
		if ctxTimeout.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("commit message generation timed out after %s", a.Config.GetRequestTimeout())
		}
		if strings.Contains(err.Error(), "token count") && strings.Contains(err.Error(), "exceeds limit") {
			return "", fmt.Errorf("changes are too large for the configured 'max_tokens' (%d). Consider committing smaller changes or increasing the limit", a.Config.MaxTokens)
		}
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	if message == "" {
		return "", fmt.Errorf("empty commit message received from Gemini")
	}

	// Display the generated message
	ui.PrintInfo("Generated commit message:")
	fmt.Println(message)

	return message, nil
}

// handlePushOperation manages the push workflow:
// - Checks if push is needed (auto-push or user prompt)
// - Verifies remotes exist
// - Executes the push command
// - Reports success/failure
// Returns an error if any step fails.
func (a *App) handlePushOperation() error {
	// Check for remotes
	hasRemotes, err := a.Pusher.HasRemotes()
	if err != nil {
		return fmt.Errorf("failed to check for remote repositories: %w", err)
	}
	if !hasRemotes {
		ui.PrintInfo("No remote repositories configured. Push operation will be skipped.")
		return nil
	}

	// Check if push is needed
	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Would you like to push changes now? (using: %s)", a.Config.PushCommand), true) {
			return nil // User declined push
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto-pushing changes (enabled via %s)...", a.Config.GetConfigSource("AutoPush")))
	}

	// Wait for SSH keys if configured
	if a.Config.WaitForSSHKeys {
		// Check if SSH keys are available
		keysAvailable, err := git.CheckSSHKeysAvailable()
		if err != nil {
			if strings.Contains(err.Error(), "ssh-add command not found") {
				ui.PrintError(fmt.Sprintf("Error: %v", err))
				ui.PrintInfo("Please install ssh-add or disable the wait_for_ssh_keys option.")
				return err
			}
			ui.PrintError(fmt.Sprintf("Error checking SSH keys: %v", err))
			ui.PrintInfo("Continuing with push operation...")
		} else if !keysAvailable {
			ui.PrintInfo(fmt.Sprintf("Waiting for SSH keys to become available (enabled via %s)... Press CTRL+C to cancel.", a.Config.GetConfigSource("WaitForSSHKeys")))

			// Start a spinner
			spinner := ui.StartSpinner("Checking for SSH keys...")

			// Wait until keys become available
			for !keysAvailable {
				// Wait for 0.5 seconds before checking again
				time.Sleep(500 * time.Millisecond)

				// Check if keys are now available
				keysAvailable, err = git.CheckSSHKeysAvailable()
				if err != nil {
					ui.StopSpinner(spinner)
					ui.PrintError(fmt.Sprintf("Error checking SSH keys: %v", err))
					break
				}

				if keysAvailable {
					ui.StopSpinner(spinner)
					ui.PrintSuccess("SSH keys detected.")
					break
				}
			}
		}
	}

	// Execute push command
	spinner := ui.StartSpinner("Pushing changes...")
	result, err := a.Pusher.ExecutePush(a.Config.PushCommand)
	ui.StopSpinner(spinner)

	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("push command failed")
	}

	// Report success and repository link if available
	ui.PrintSuccess("Successfully pushed changes.")
	if result.RepoLink != "" {
		ui.PrintRepoLink("View repository:", result.RepoLink)
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
		ui.PrintInfo("No changes detected for commit.")
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
