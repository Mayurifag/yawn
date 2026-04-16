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

	timeout := time.After(sshWaitTimeout)
	ticker := time.NewTicker(sshPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			keysAvailable, err = git.CheckSSHKeysAvailable()
			if err != nil {
				ui.StopSpinner(spinner)
				ui.PrintError(fmt.Sprintf("checking SSH keys: %v", err))
				return nil
			}
			if keysAvailable {
				ui.StopSpinner(spinner)
				ui.PrintSuccess("SSH keys detected.")
				return nil
			}
		case <-timeout:
			ui.StopSpinner(spinner)
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

	spinner := ui.StartSpinner(spinnerText)
	result, err := a.Pusher.ExecutePush(pushCmd)
	ui.StopSpinner(spinner)

	if err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("push failed")
	}

	ui.PrintSuccess(successMsg)
	switch {
	case result.PRLink != "":
		ui.PrintRepoLink("View pull request:", result.PRLink)
	case result.RepoLink != "":
		ui.PrintRepoLink("View repository:", result.RepoLink)
	}

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
	if !a.Config.SquashAutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Force push? (using: %s)", pushCmd), false) {
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto force-pushing (enabled via %s)...", a.Config.GetConfigSource("SquashAutoPush")))
	}

	return a.doPush(pushCmd, "Force-pushing...", "Successfully force-pushed.")
}
