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

	target, err := a.GitClient.ResolvePushTarget(a.Config.PushCommand)
	if err != nil {
		return err
	}
	currentURL := target.URL
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
		ui.PrintInfo(fmt.Sprintf("  note: %q is a custom host. If SSH uses a non-default port, set the push URL for %q to ssh://git@%s:PORT/%s/%s.git.", info.Host, target.Remote, info.Host, info.Owner, info.Repo))
	}

	if !ui.AskYesNo(fmt.Sprintf("Convert the push URL for %q to SSH now?", target.Remote), true) {
		return errors.New("aborted: HTTPS remote not allowed; convert to SSH and retry")
	}

	if err := a.GitClient.SetRemoteURL(target.Remote, sshURL); err != nil {
		return fmt.Errorf("failed to switch remote to SSH: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Push URL for %q set to %s", target.Remote, sshURL))
	return nil
}
