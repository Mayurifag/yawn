package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

const maxCommitGenRetries = 3

var errRequestTimeout = errors.New("request timeout")

type App struct {
	Config    config.Config
	GitClient git.GitClient
	Pusher    git.PushProvider
}

func NewApp(cfg config.Config, gitClient git.GitClient) *App {
	return &App{
		Config:    cfg,
		GitClient: gitClient,
		Pusher:    git.NewPusher(gitClient),
	}
}

func (a *App) autoPull() error {
	hasRemotes, err := a.GitClient.HasRemotes()
	if err != nil || !hasRemotes {
		return nil
	}
	ui.PrintInfo("Pulling remote branch commits (to be sure the commit will be on actual codebase)...")
	return a.GitClient.Pull()
}

func (a *App) ensureAPIKey() error {
	if a.Config.GeminiAPIKey == "" {
		ui.PrintInfo("No API key found. Please provide your Google Gemini API key.")
		ui.PrintInfo("You can get one from: https://makersuite.google.com/app/apikey")
		apiKey := ui.AskForInput("Enter your Google Gemini API key: ", true)
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}
		if err := config.SaveAPIKeyToUserConfig(apiKey); err != nil {
			ui.PrintError(fmt.Sprintf("Warning: Failed to save API key to config file: %v", err))
		}
		a.Config.GeminiAPIKey = apiKey
	}
	return nil
}

func (a *App) setupAndCheckPrerequisites() (bool, error) {
	if err := a.ensureAPIKey(); err != nil {
		return false, err
	}
	hasChanges, err := a.GitClient.HasAnyChanges()
	if err != nil {
		return false, fmt.Errorf("failed to check for changes: %w", err)
	}
	return hasChanges, nil
}

func (a *App) ensureStagedChanges() error {
	hasStaged, err := a.GitClient.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}

	hasUnstaged, err := a.GitClient.HasUnstagedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	if !hasStaged && !hasUnstaged {
		return fmt.Errorf("you have no changes to commit")
	}

	shouldStage := false
	if hasUnstaged {
		if a.Config.AutoStage {
			ui.PrintInfo(fmt.Sprintf("Auto-staging changes (enabled via %s)...", a.Config.GetConfigSource("AutoStage")))
			shouldStage = true
		} else if !hasStaged {
			if !ui.AskYesNo("You have unstaged changes. Would you like to stage them?", true) {
				return fmt.Errorf("staging required to proceed")
			}
			shouldStage = true
		}
	}

	if shouldStage {
		if err := a.GitClient.StageChanges(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		ui.PrintSuccess("Successfully staged changes.")
	}

	return nil
}

func (a *App) generateAndCommitChanges(ctx context.Context) error {
	diff, err := a.GitClient.GetDiff()
	if err != nil {
		return fmt.Errorf("failed to get staged changes: %w", err)
	}
	if diff == "" {
		return fmt.Errorf("no staged changes to commit")
	}

	geminiClient, err := gemini.NewClient(a.Config.GeminiAPIKey, a.Config.GeminiModel)
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	branchName, additions, deletions := a.gatherCommitInfo()
	ui.PrintPreGenerationInfo(branchName, additions, deletions, a.Config.GeminiModel)

	message, err := a.generateCommitMessageAndStream(ctx, geminiClient, a.Config.Prompt, diff)
	if err != nil {
		return err
	}

	if err := a.GitClient.Commit(message); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	ui.PrintSuccess("Successfully committed changes.")

	return nil
}

func (a *App) gatherCommitInfo() (branchName string, additions int, deletions int) {
	branchName, err := a.GitClient.GetCurrentBranch()
	if err != nil {
		branchName = "unknown"
	}
	additions, deletions, _ = a.GitClient.GetDiffNumStatSummary()
	return
}

func (a *App) doGenerateStream(ctx context.Context, geminiClient gemini.Client, systemPrompt, userContent string) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, a.Config.GetRequestTimeout())
	defer cancel()

	spinner := ui.StartSpinner("Generating commit message...")
	stream, err := geminiClient.GenerateCommitMessageStream(ctxTimeout, systemPrompt, userContent)
	ui.StopSpinner(spinner)

	if err != nil {
		if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("commit message generation timed out after %s: %w", a.Config.GetRequestTimeout(), errRequestTimeout)
		}
		return "", fmt.Errorf("failed to start commit message generation: %w", err)
	}

	ui.PrintInfo("Generated commit message:")
	message, err := stream.Collect(func(chunk string) { fmt.Print(chunk) })
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("error receiving commit message stream: %w", err)
	}
	if message == "" {
		return "", fmt.Errorf("empty commit message received from Gemini")
	}
	return message, nil
}

func (a *App) generateCommitMessageAndStream(ctx context.Context, geminiClient gemini.Client, systemPrompt, userContent string) (string, error) {
	var lastErr error
	for attempt := range maxCommitGenRetries {
		msg, err := a.doGenerateStream(ctx, geminiClient, systemPrompt, userContent)
		if err == nil {
			return msg, nil
		}
		lastErr = err
		isRetryable := gemini.IsTransientError(err) || errors.Is(err, errRequestTimeout)
		if !isRetryable || attempt == maxCommitGenRetries-1 || ctx.Err() != nil {
			return "", err
		}
		pause := time.Duration(attempt+1) * time.Second
		ui.PrintInfo(fmt.Sprintf("Retrying in %s... (attempt %d/%d)", pause, attempt+1, maxCommitGenRetries))
		time.Sleep(pause)
	}
	return "", lastErr
}

func printLinks(repoLink, prLink, suggestPRLink string) {
	if repoLink != "" {
		ui.PrintRepoLink("View repository:", repoLink)
	}
	if prLink != "" {
		ui.PrintRepoLink("View pull request:", prLink)
	} else if suggestPRLink != "" {
		ui.PrintRepoLink("Create pull request:", suggestPRLink)
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.autoPull(); err != nil {
		return err
	}
	hasChanges, err := a.setupAndCheckPrerequisites()
	if err != nil {
		return err
	}
	if !hasChanges {
		return a.handleUnpushedCommits()
	}

	if err := a.ensureStagedChanges(); err != nil {
		return err
	}

	if err := a.generateAndCommitChanges(ctx); err != nil {
		return err
	}

	return a.handlePushOperation()
}
