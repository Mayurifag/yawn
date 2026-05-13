package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Mayurifag/yawn/internal/ai"
	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

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

const networkMaxRetries = 3

func isRetryableNetworkErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, git.ErrNetworkTimeout) {
		return true
	}
	var gitErr *git.GitError
	if errors.As(err, &gitErr) {
		if git.IsAuthError(gitErr.Output) {
			return false
		}
		return !containsAny(gitErr.Output, "non-fast-forward", "rejected", "fatal: refusing")
	}
	return false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func (a *App) ensureAPIKey() error {
	if !config.ProviderRequiresAPIKey(a.Config.GetMainProvider()) {
		return nil
	}
	if a.Config.GetAPIKey() == "" {
		provider := a.Config.GetMainProvider()
		providerName := config.ProviderDisplayName(provider)
		ui.PrintInfo(fmt.Sprintf("No API key found for %s.", providerName))
		ui.PrintInfo(config.ProviderAPIKeyHelp(provider))
		apiKey := ui.AskForInput(fmt.Sprintf("Enter your %s API key: ", providerName), true)
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}
		if err := config.SaveProviderAPIKeyToUserConfig(provider, apiKey); err != nil {
			ui.PrintError(fmt.Sprintf("Warning: Failed to save API key to config file: %v", err))
		}
		a.Config.SetAPIKey(apiKey)
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

	aiClient, err := ai.NewClient(a.Config)
	if err != nil {
		return fmt.Errorf("failed to create AI client: %w", err)
	}

	branchName, additions, deletions := a.gatherCommitInfo()
	ui.PrintPreGenerationInfo(branchName, additions, deletions, a.Config.GetModelLabel())

	message, err := a.generateCommitMessageAndStream(ctx, aiClient, a.Config.Prompt, diff)
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
	if err := a.ensureSSHRemote(); err != nil {
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
