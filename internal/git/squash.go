package git

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *ExecGitClient) FindBranchBase(branch, ref string) (string, error) {
	if ref == "" {
		var err error
		ref, err = c.branchBaseRef(branch)
		if err != nil {
			return "", err
		}
		if base, err := c.runGitCommand("merge-base", "--fork-point", ref, "HEAD"); err == nil {
			return base, nil
		}
	}
	commit, err := c.runGitCommand("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("invalid squash base %q: %w", ref, err)
	}
	return c.runGitCommand("merge-base", commit, "HEAD")
}

func (c *ExecGitClient) FindBranchBaseRef(branch string) (string, error) {
	ref, err := c.branchBaseRef(branch)
	if err != nil {
		return "", err
	}
	if remote, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
		_, name, _ := strings.Cut(remote, "/")
		return name, nil
	}
	return strings.TrimPrefix(ref, "refs/heads/"), nil
}

func (c *ExecGitClient) branchBaseRef(branch string) (string, error) {
	parent := c.branchCreationParent(branch)
	if parent != "" && parent != branch && parent != "origin/"+branch {
		for _, ref := range []string{"refs/heads/" + parent, "refs/remotes/" + parent, "refs/remotes/origin/" + parent} {
			if _, err := c.runGitCommand("show-ref", "--verify", ref); err == nil {
				return ref, nil
			}
		}
	}
	return "", fmt.Errorf("cannot determine the creation parent of %q; use yawn squash --base REF", branch)
}

func (c *ExecGitClient) branchCreationParent(branch string) string {
	creation, err := c.runGitCommand("reflog", "show", "-1", "--format=%H %gs", "--grep-reflog=^branch: Created from ", "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	hash, source, ok := strings.Cut(creation, " branch: Created from ")
	if !ok {
		return ""
	}
	if source != "HEAD" {
		return strings.TrimPrefix(strings.TrimPrefix(source, "refs/heads/"), "refs/remotes/")
	}
	output, err := c.runGitCommand("reflog", "HEAD", "--format=%H %gs")
	if err != nil {
		return ""
	}
	lines := strings.Split(output, "\n")
	// Only the creation checkout identifies a parent; later switches can come from siblings.
	for i := len(lines) - 1; i >= 0; i-- {
		if source, ok := strings.CutPrefix(lines[i], hash+" checkout: moving from "); ok && strings.HasSuffix(source, " to "+branch) {
			return strings.TrimSuffix(source, " to "+branch)
		}
	}
	return ""
}

func (c *ExecGitClient) GetCommitCountRange(base string) (int, error) {
	output, err := c.runGitCommand("rev-list", "--count", base+"..HEAD")
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}
	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("invalid commit count %q: %w", output, err)
	}
	return count, nil
}

func (c *ExecGitClient) GetDiffRange(base string) (string, error) {
	numstatOutput, err := c.runGitCommand("diff", "--numstat", "-z", "--no-renames", "--no-color", base, "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get diff stats: %w", err)
	}
	return c.buildFilteredDiff(numstatOutput, []string{"diff", "--no-color", base, "HEAD"}, base)
}

func (c *ExecGitClient) GetDiffCachedRange(base string) (string, error) {
	numstatOutput, err := c.runGitCommand("diff", "--cached", "--numstat", "-z", "--no-renames", "--no-color", base)
	if err != nil {
		return "", fmt.Errorf("failed to get diff stats: %w", err)
	}
	return c.buildFilteredDiff(numstatOutput, []string{"diff", "--cached", "--no-color", base}, base)
}

func (c *ExecGitClient) GetDiffNumStatCachedRange(base string) (int, int, error) {
	output, err := c.runGitCommand("diff", "--cached", "--numstat", "-z", "--no-renames", "--no-color", base)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get diff stats: %w", err)
	}
	add, del := sumNumstatLines(output)
	return add, del, nil
}

func (c *ExecGitClient) ResetSoft(commit string) error {
	if _, err := c.runGitCommand("reset", "--soft", commit); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}
	return nil
}

func (c *ExecGitClient) Stash() (string, error) {
	before, err := c.runGitCommand("stash", "list", "-1", "--format=%H")
	if err != nil {
		return "", err
	}
	if _, err := c.runGitCommand("stash", "push", "--include-untracked"); err != nil {
		return "", fmt.Errorf("failed to stash: %w", err)
	}
	after, err := c.runGitCommand("stash", "list", "-1", "--format=%H")
	if err != nil {
		return "", err
	}
	if after == before {
		return "", nil
	}
	return after, nil
}

func (c *ExecGitClient) StashPop(stash string) error {
	if stash == "" {
		return nil
	}
	if _, err := c.runGitCommand("stash", "apply", "--index", stash); err != nil {
		return fmt.Errorf("failed to restore stash %s: %w", stash, err)
	}
	output, err := c.runGitCommand("stash", "list", "--format=%gd %H")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(output, "\n") {
		ref, hash, _ := strings.Cut(line, " ")
		if hash == stash {
			_, err := c.runGitCommand("stash", "drop", ref)
			return err
		}
	}
	return nil
}
