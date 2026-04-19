package app

import (
	"context"
	"fmt"

	"github.com/Mayurifag/yawn/internal/git"
	"github.com/Mayurifag/yawn/internal/ui"
)

type dirtyAction int

const (
	dirtyClean dirtyAction = iota
	dirtyCancel
	dirtyStash
	dirtyAdd
)

var defaultBranchNames = map[string]bool{"main": true, "master": true, "dev": true}

func (a *App) handleDirtyState() (dirtyAction, error) {
	hasChanges, err := a.GitClient.HasAnyChanges()
	if err != nil {
		return dirtyCancel, err
	}
	if !hasChanges {
		return dirtyClean, nil
	}
	if status, err := a.GitClient.GetStatusShort(); err == nil && status != "" {
		ui.PrintDirtyChanges(status)
	}
	choice := ui.AskSquashDirtyAction()
	switch choice {
	case "s":
		return dirtyStash, nil
	case "a":
		return dirtyAdd, nil
	default:
		return dirtyCancel, nil
	}
}

func (a *App) squashSetup() (base string, count int, err error) {
	branch, err := a.GitClient.GetCurrentBranch()
	if err != nil {
		return
	}
	defaultBranch, _ := a.GitClient.GetDefaultBranch()
	if defaultBranchNames[branch] || (defaultBranch != "" && branch == defaultBranch) {
		err = fmt.Errorf("squash: cannot squash on default branch %q", branch)
		return
	}
	base, err = a.GitClient.FindBranchBase(branch)
	if err != nil {
		return
	}
	count, err = a.GitClient.GetCommitCountRange(base)
	return
}

func (a *App) printSquashLinks() {
	remoteURL, err := a.GitClient.GetRemoteURL("")
	if err != nil {
		return
	}
	remoteInfo, err := git.ParseRemoteURL(remoteURL)
	if err != nil {
		return
	}
	repoLink := git.GenerateRepoLink(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo)
	var suggestPRLink string
	if branch, err := a.GitClient.GetCurrentBranch(); err == nil {
		defaultBranch, _ := a.GitClient.GetDefaultBranch()
		if branch != defaultBranch {
			suggestPRLink = git.GeneratePRURL(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo, branch)
		}
	}
	printLinks(repoLink, "", suggestPRLink)
}

func (a *App) handleSingleCommit() error {
	hasChanges, err := a.GitClient.HasAnyChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		ui.PrintInfo("Only 1 commit on branch — nothing to squash.")
		a.printSquashLinks()
		return nil
	}
	if status, err := a.GitClient.GetStatusShort(); err == nil && status != "" {
		ui.PrintDirtyChanges(status)
	}
	if ui.AskAmendDirtyAction() != "a" {
		return fmt.Errorf("squash: cancelled")
	}
	ui.PrintInfo("Staging all changes and amending commit...")
	if err := a.GitClient.StageChanges(); err != nil {
		return err
	}
	if err := a.GitClient.AmendCommit(); err != nil {
		return err
	}
	return a.handleSquashPush()
}

func (a *App) handleMultiCommitSquash(ctx context.Context, base string, count int) (err error) {
	if err := a.ensureAPIKey(); err != nil {
		return err
	}

	action, err := a.handleDirtyState()
	if err != nil {
		return err
	}
	if action == dirtyCancel {
		return fmt.Errorf("squash: cancelled — commit or stash changes first")
	}

	if action == dirtyAdd {
		ui.PrintInfo("Staging all changes...")
		if err := a.GitClient.StageChanges(); err != nil {
			return err
		}
	}

	if action == dirtyStash {
		if err = a.GitClient.Stash(); err != nil {
			return err
		}
		defer func() {
			if popErr := a.GitClient.StashPop(); popErr != nil {
				ui.PrintError(fmt.Sprintf("stash pop failed: %v", popErr))
				if err == nil {
					err = popErr
				}
			}
		}()
	}

	if err := a.GitClient.ResetSoft(base); err != nil {
		return err
	}

	ui.PrintInfo(fmt.Sprintf("Squashing %d commits into 1...", count))
	if err := a.generateAndCommitChanges(ctx); err != nil {
		return err
	}

	return a.handleSquashPush()
}

func (a *App) RunSquash(ctx context.Context) error {
	if err := a.autoPull(); err != nil {
		return err
	}

	base, count, err := a.squashSetup()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("squash: no commits on branch")
	}
	if count == 1 {
		return a.handleSingleCommit()
	}
	return a.handleMultiCommitSquash(ctx, base, count)
}
