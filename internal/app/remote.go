package app

import (
	"errors"
	"fmt"

	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

func (a *App) ensureSSHRemote() error {
	hasRemotes, err := a.GitClient.HasRemotes()
	if err != nil || !hasRemotes {
		return nil
	}

	currentURL, err := a.GitClient.GetRemoteURL("")
	if err != nil {
		return nil
	}
	if !git.IsHTTPSRemoteURL(currentURL) {
		return nil
	}

	sshURL, err := git.ConvertHTTPSToSSH(currentURL)
	if err != nil {
		return fmt.Errorf("remote uses HTTPS and cannot be auto-converted to SSH (%s): %w", currentURL, err)
	}

	ui.PrintInfo("Remote uses HTTPS. yawn requires SSH for reliable pushes.")
	ui.PrintInfo(fmt.Sprintf("  current: %s", currentURL))
	ui.PrintInfo(fmt.Sprintf("  proposed: %s", sshURL))

	if info, perr := git.ParseRemoteURL(currentURL); perr == nil && !git.IsKnownSSHHost(info.Host) {
		ui.PrintInfo(fmt.Sprintf("  note: %q is a custom host. If its SSH server uses a non-default port, run `git remote set-url origin ssh://git@%s:PORT/%s/%s.git` after this conversion.", info.Host, info.Host, info.Owner, info.Repo))
	}

	if !ui.AskYesNo("Convert remote 'origin' to SSH now?", true) {
		return errors.New("aborted: HTTPS remote not allowed; convert to SSH and retry")
	}

	if err := a.GitClient.SetRemoteURL("", sshURL); err != nil {
		return fmt.Errorf("failed to switch remote to SSH: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Remote 'origin' set to %s", sshURL))
	return nil
}
