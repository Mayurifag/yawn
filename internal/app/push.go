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

func squashPushCommand(pushCommand string) string {
	if strings.Contains(pushCommand, "--force") {
		return pushCommand
	}
	return pushCommand + " --force-with-lease"
}

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

func (a *App) doPush(pushCmd, spinnerText, successMsg string) error {
	if a.Config.WaitForSSHKeys {
		if err := a.waitForSSHKeys(); err != nil {
			return err
		}
	}

	var result *git.PushResult
	var err error
	for attempt := range networkMaxRetries {
		spinner := ui.StartSpinner(spinnerText)
		result, err = a.Pusher.ExecutePush(pushCmd)
		ui.StopSpinner(spinner)
		if err == nil {
			break
		}
		var gitErr *git.GitError
		if errors.As(err, &gitErr) && strings.Contains(gitErr.Output, "non-fast-forward") {
			return a.handleNonFastForwardPush(pushCmd, spinnerText, successMsg)
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

	ui.PrintSuccess(successMsg)
	printLinks(result.RepoLink, result.PRLink, result.SuggestPRLink)

	return nil
}

func (a *App) handlePushOperation() error {
	hasRemotes, err := a.Pusher.HasRemotes()
	if err != nil {
		return fmt.Errorf("failed to check for remote repositories: %w", err)
	}
	if !hasRemotes {
		ui.PrintInfo("No remote repositories configured. Push operation will be skipped.")
		return nil
	}

	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Would you like to push changes now? (using: %s)", a.Config.PushCommand), true) {
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto-pushing changes (enabled via %s)...", a.Config.GetConfigSource("AutoPush")))
	}

	return a.doPush(a.Config.PushCommand, "Pushing changes...", "Successfully pushed changes.")
}

func (a *App) handleUnpushedCommits() error {
	commits, err := a.GitClient.GetUnpushedCommits()
	if err != nil {
		ui.PrintInfo("No changes detected for commit.")
		a.printSquashLinks()
		return nil
	}
	if len(commits) == 0 {
		branch, bErr := a.GitClient.GetCurrentBranch()
		if bErr == nil {
			localOnly, remoteOnly, _ := a.GitClient.GetDivergenceVsOrigin(branch)
			if len(localOnly) > 0 {
				return a.handleOriginDivergence(localOnly, remoteOnly)
			}
		}
		ui.PrintInfo("No changes detected for commit.")
		a.printSquashLinks()
		return nil
	}

	ui.PrintInfo(fmt.Sprintf("%d unpushed commit(s) found:", len(commits)))
	for _, c := range commits {
		fmt.Printf("  %s\n", c)
	}

	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Push %d commit(s)? (using: %s)", len(commits), a.Config.PushCommand), true) {
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto-pushing %d commit(s) (enabled via %s)...", len(commits), a.Config.GetConfigSource("AutoPush")))
	}

	return a.doPush(a.Config.PushCommand, "Pushing...", "Successfully pushed.")
}

func (a *App) handleOriginDivergence(localOnly, remoteOnly []string) error {
	ui.PrintForcePushPreview(remoteOnly, localOnly)
	needsForce := len(remoteOnly) > 0
	pushCmd := a.Config.PushCommand
	if needsForce {
		pushCmd = squashPushCommand(pushCmd)
	}
	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Push %d commit(s)? (using: %s)", len(localOnly), pushCmd), !needsForce) {
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto-pushing %d commit(s) (enabled via %s)...", len(localOnly), a.Config.GetConfigSource("AutoPush")))
	}
	return a.doPush(pushCmd, "Pushing...", "Successfully pushed.")
}

func (a *App) handleNonFastForwardPush(pushCmd, spinnerText, successMsg string) error {
	localCommits, _ := a.GitClient.GetUnpushedCommits()
	remoteCommits, _ := a.GitClient.GetRemoteOnlyCommits()
	ui.PrintForcePushPreview(remoteCommits, localCommits)
	forcePushCmd := squashPushCommand(pushCmd)
	if !ui.AskYesNo(fmt.Sprintf("Overwrite remote? (using: %s)", forcePushCmd), false) {
		return nil
	}
	return a.doPush(forcePushCmd, spinnerText, successMsg)
}

func (a *App) handleSquashPush() error {
	hasRemotes, err := a.Pusher.HasRemotes()
	if err != nil {
		return fmt.Errorf("failed to check for remote repositories: %w", err)
	}
	if !hasRemotes {
		ui.PrintInfo("No remote repositories configured. Push skipped.")
		return nil
	}

	pushCmd := squashPushCommand(a.Config.PushCommand)
	localCommits, _ := a.GitClient.GetUnpushedCommits()
	remoteCommits, _ := a.GitClient.GetRemoteOnlyCommits()
	ui.PrintForcePushPreview(remoteCommits, localCommits)
	if !a.Config.SquashAutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Force push? (using: %s)", pushCmd), false) {
			a.printSquashLinks()
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto force-pushing (enabled via %s)...", a.Config.GetConfigSource("SquashAutoPush")))
	}

	return a.doPush(pushCmd, "Force-pushing...", "Successfully force-pushed.")
}
