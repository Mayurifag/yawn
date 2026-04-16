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

var defaultBranchNames = []string{"main", "master", "dev"}

func (a *App) handleDirtyState() (dirtyAction, error) {
	hasChanges, err := a.GitClient.HasAnyChanges()
	if err != nil {
		return dirtyCancel, err
	}
	if !hasChanges {
		return dirtyClean, nil
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
	for _, def := range defaultBranchNames {
		if branch == def {
			err = fmt.Errorf("squash: cannot squash on default branch %q", branch)
			return
		}
	}
	base, err = a.GitClient.FindBranchBase(branch)
	if err != nil {
		return
	}
	count, err = a.GitClient.GetCommitCountRange(base)
	return
}

func (a *App) printSquashRepoLink() {
	remoteURL, err := a.GitClient.GetRemoteURL("")
	if err != nil {
		return
	}
	remoteInfo, err := git.ParseRemoteURL(remoteURL)
	if err != nil {
		return
	}
	if link := git.GenerateRepoLink(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo); link != "" {
		ui.PrintRepoLink("View repository:", link)
	}
}

func (a *App) RunSquash(ctx context.Context) error {
	if err := a.ensureAPIKey(); err != nil {
		return err
	}

	base, count, err := a.squashSetup()
	if err != nil {
		return err
	}
	if count <= 1 {
		ui.PrintInfo(fmt.Sprintf("Only %d commit(s) on branch — nothing to squash.", count))
		return nil
	}

	action, err := a.handleDirtyState()
	if err != nil {
		return err
	}
	if action == dirtyCancel {
		return fmt.Errorf("squash: cancelled — commit or stash changes first")
	}

	if action == dirtyAdd {
		if err := a.GitClient.StageChanges(); err != nil {
			return err
		}
	}

	if action == dirtyStash {
		if err := a.GitClient.Stash(); err != nil {
			return err
		}
		defer func() {
			if popErr := a.GitClient.StashPop(); popErr != nil {
				ui.PrintError(fmt.Sprintf("stash pop failed: %v", popErr))
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

	a.printSquashRepoLink()
	return a.handleSquashPush()
}
