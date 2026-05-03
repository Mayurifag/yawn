package app

import (
	"fmt"

	"github.com/Mayurifag/yawn/internal/ui"
)

func (a *App) RunForcePush() error {
	if err := a.ensureSSHRemote(); err != nil {
		return err
	}
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

	if !a.Config.AutoPush {
		if !ui.AskYesNo(fmt.Sprintf("Force push? (using: %s)", pushCmd), false) {
			a.printSquashLinks()
			return nil
		}
	} else {
		ui.PrintInfo(fmt.Sprintf("Auto force-pushing (enabled via %s)...", a.Config.GetConfigSource("AutoPush")))
	}

	return a.doPush(pushCmd, "Force-pushing...", "Successfully force-pushed.")
}
