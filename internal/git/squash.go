package git

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *ExecGitClient) FindBranchBase(branch string) (string, error) {
	if _, base := c.findBranchBaseFromRefs(branch); base != "" {
		return base, nil
	}

	output, err := c.runGitCommand("reflog", "HEAD", "--format=%H %gs")
	if err != nil {
		return "", fmt.Errorf("failed to read reflog: %w", err)
	}
	suffix := " to " + branch
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "checkout:") && strings.HasSuffix(line, suffix) {
			if parts := strings.Fields(line); len(parts) > 0 {
				return parts[0], nil
			}
		}
	}
	return "", fmt.Errorf("cannot determine branch base for %q: no merge-base with default branch and no checkout entry in reflog", branch)
}

func (c *ExecGitClient) FindBranchBaseRef(branch string) (string, error) {
	ref, _ := c.findBranchBaseFromRefs(branch)
	if ref == "" {
		return "", fmt.Errorf("cannot determine branch base ref for %q", branch)
	}
	return strings.TrimPrefix(ref, "origin/"), nil
}

func (c *ExecGitClient) findBranchBaseFromRefs(branch string) (string, string) {
	bestRef := ""
	bestBase := ""
	bestDistance := -1
	for _, ref := range c.branchBaseCandidateRefs(branch) {
		base := c.mergeBase(ref)
		distance, err := c.GetCommitCountRange(base)
		if base == "" || err != nil {
			continue
		}
		if bestDistance == -1 || distance < bestDistance {
			bestRef = ref
			bestBase = base
			bestDistance = distance
		}
	}
	return bestRef, bestBase
}

func (c *ExecGitClient) mergeBase(ref string) string {
	if _, err := c.runGitCommand("rev-parse", "--verify", ref); err != nil {
		return ""
	}
	base, err := c.runGitCommand("merge-base", "--fork-point", ref, "HEAD")
	if err == nil && base != "" {
		return base
	}
	base, err = c.runGitCommand("merge-base", ref, "HEAD")
	if err != nil {
		return ""
	}
	return base
}

func (c *ExecGitClient) branchBaseCandidateRefs(branch string) []string {
	refs := []string{"origin/HEAD"}
	if defaultBranch, err := c.GetDefaultBranch(); err == nil && defaultBranch != "" {
		refs = append(refs, "origin/"+defaultBranch, defaultBranch)
	}
	if output, err := c.runGitCommand("for-each-ref", "--format=%(refname:short)", "refs/remotes/origin", "refs/heads"); err == nil {
		for _, ref := range strings.Split(output, "\n") {
			ref = strings.TrimSpace(ref)
			if ref != "" && ref != "origin/HEAD" && ref != branch && ref != "origin/"+branch {
				refs = append(refs, ref)
			}
		}
	}

	seen := make(map[string]bool, len(refs))
	var unique []string
	for _, ref := range refs {
		if !seen[ref] {
			seen[ref] = true
			unique = append(unique, ref)
		}
	}
	return unique
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
	return c.buildFilteredDiff(numstatOutput, []string{"diff", "--no-color", base, "HEAD"}), nil
}

func (c *ExecGitClient) GetDiffCachedRange(base string) (string, error) {
	numstatOutput, err := c.runGitCommand("diff", "--cached", "--numstat", "-z", "--no-renames", "--no-color", base)
	if err != nil {
		return "", fmt.Errorf("failed to get diff stats: %w", err)
	}
	return c.buildFilteredDiff(numstatOutput, []string{"diff", "--cached", "--no-color", base}), nil
}

func (c *ExecGitClient) GetDiffNumStatRange(base string) (int, int, error) {
	output, err := c.runGitCommand("diff", "--numstat", "-z", "--no-renames", "--no-color", base, "HEAD")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get diff stats: %w", err)
	}
	add, del := sumNumstatLines(output)
	return add, del, nil
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

func (c *ExecGitClient) Stash() error {
	if _, err := c.runGitCommand("stash"); err != nil {
		return fmt.Errorf("failed to stash: %w", err)
	}
	return nil
}

func (c *ExecGitClient) StashPop() error {
	if _, err := c.runGitCommand("stash", "pop"); err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}
	return nil
}
