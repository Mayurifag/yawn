package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

const (
	sshWaitTimeout  = 60 * time.Second
	sshPollInterval = 500 * time.Millisecond
)

func (a *App) waitForSSHKeys() error {
	keysAvailable, err := git.CheckSSHKeysAvailable()
	if err != nil {
		if errors.Is(err, git.ErrSSHAddNotFound) {
			ui.PrintError(err.Error())
			ui.PrintInfo("Please install ssh-add or disable the wait_for_ssh_keys option.")
			return err
		}
		ui.PrintError(fmt.Sprintf("checking SSH keys: %v", err))
		ui.PrintInfo("Continuing with push operation...")
		return nil
	}
	if keysAvailable {
		return nil
	}

	ui.PrintInfo(fmt.Sprintf("Waiting for SSH keys to become available (enabled via %s)... Press CTRL+C to cancel.", a.Config.GetConfigSource("WaitForSSHKeys")))
	spinner := ui.StartSpinner("Checking for SSH keys...")
	defer ui.StopSpinner(spinner)

	timeout := time.After(sshWaitTimeout)
	ticker := time.NewTicker(sshPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			keysAvailable, err = git.CheckSSHKeysAvailable()
			if err != nil {
				ui.PrintError(fmt.Sprintf("checking SSH keys: %v", err))
				return nil
			}
			if keysAvailable {
				ui.PrintSuccess("SSH keys detected.")
				return nil
			}
		case <-timeout:
			return fmt.Errorf("timed out waiting for SSH keys after %s", sshWaitTimeout)
		}
	}
}

func (a *App) doPush(plan *git.PushPlan, force bool) error {
	var result *git.PushResult
	var err error
	for attempt := range networkMaxRetries {
		spinner := ui.StartSpinner("Pushing changes...")
		result, err = a.Pusher.ExecutePush(plan, force)
		ui.StopSpinner(spinner)
		if err == nil {
			break
		}
		var gitErr *git.GitError
		if errors.As(err, &gitErr) && strings.Contains(gitErr.Output, "non-fast-forward") && !force && !plan.Target.Force {
			fresh, previewErr := a.GitClient.PreparePush(a.Config.PushCommand)
			if previewErr != nil {
				return previewErr
			}
			return a.confirmAndPush(fresh, true, false)
		}
		if !isRetryableNetworkErr(err) {
			break
		}
		if attempt < networkMaxRetries-1 {
			pause := time.Duration(1<<attempt) * time.Second
			ui.PrintInfo(fmt.Sprintf("Push failed (%v), retrying in %s... (attempt %d/%d)", err, pause, attempt+1, networkMaxRetries))
			time.Sleep(pause)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("push failed")
	}

	ui.PrintSuccess("Successfully pushed changes.")
	printLinks(result.RepoLink, result.PRLink, result.SuggestPRLink)

	return nil
}

func (a *App) preparePush() (*git.PushPlan, error) {
	hasRemotes, err := a.Pusher.HasRemotes()
	if err != nil {
		return nil, fmt.Errorf("failed to check for remote repositories: %w", err)
	}
	if !hasRemotes {
		ui.PrintInfo("No remote repositories configured. Push operation will be skipped.")
		return nil, nil
	}
	if a.Config.WaitForSSHKeys {
		if err := a.waitForSSHKeys(); err != nil {
			return nil, err
		}
	}
	return a.GitClient.PreparePush(a.Config.PushCommand)
}

func (a *App) pushChanges(force, auto bool) error {
	plan, err := a.preparePush()
	if err != nil || plan == nil {
		return err
	}
	return a.confirmAndPush(plan, force, auto)
}

func (a *App) handlePushOperation() error {
	return a.pushChanges(false, a.Config.AutoPush)
}

func (a *App) handleUnpushedCommits() error {
	plan, err := a.preparePush()
	if err != nil || plan == nil {
		return err
	}
	if len(plan.LocalCommits) == 0 && plan.RemoteHead != "" {
		ui.PrintInfo("No unpushed commits detected.")
		a.printSquashLinks()
		return nil
	}
	ui.PrintInfo(fmt.Sprintf("%d unpushed commit(s) found:", len(plan.LocalCommits)))
	for _, commit := range plan.LocalCommits {
		fmt.Printf("  %s\n", commit)
	}
	return a.confirmAndPush(plan, false, a.Config.AutoPush)
}

func (a *App) confirmAndPush(plan *git.PushPlan, force, auto bool) error {
	needsForce := force || plan.Target.Force || len(plan.RemoteCommits) > 0
	ui.PrintInfo(fmt.Sprintf("Push destination: %s → %s", plan.Target.Remote, plan.Target.Branch))
	prompt := "Push changes?"
	if needsForce {
		ui.PrintForcePushPreview(plan.RemoteCommits, plan.LocalCommits)
		prompt = "Overwrite the destination branch using the previewed lease?"
	}
	if !auto || (!force && needsForce) {
		if !ui.AskYesNo(prompt, !needsForce) {
			return nil
		}
	}
	return a.doPush(plan, needsForce)
}

func (a *App) handleSquashPush() error {
	return a.pushChanges(true, a.Config.SquashAutoPush)
}
