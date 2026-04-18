package git

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *ExecGitClient) FindBranchBase(branch string) (string, error) {
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
	return "", fmt.Errorf("cannot determine branch base: no checkout entry found for %q in reflog", branch)
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
	numstatOutput, err := c.runGitCommand("diff", "--numstat", "--no-color", base, "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get diff stats: %w", err)
	}
	files := parseFilesFromNumstat(numstatOutput)
	if len(files) == 0 {
		return "", nil
	}
	args := append([]string{"diff", "--no-color", base, "HEAD", "--"}, files...)
	output, err := c.runGitCommand(args...)
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output != "" {
			return gitErr.Output, nil
		}
		return "", fmt.Errorf("failed to get diff range: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) GetDiffNumStatRange(base string) (int, int, error) {
	output, err := c.runGitCommand("diff", "--numstat", "--no-color", base, "HEAD")
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
