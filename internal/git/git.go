package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type GitClient interface {
	HasStagedChanges() (bool, error)
	HasUnstagedChanges() (bool, error)
	HasAnyChanges() (bool, error)
	GetDiff() (string, error)
	StageChanges() error
	Commit(message string) error
	AmendCommit() error
	Push(command string) (string, error)
	HasRemotes() (bool, error)
	GetCurrentBranch() (string, error)
	GetRemoteURL(remote string) (string, error)
	GetLastCommitHash() (string, error)
	GetDiffNumStatSummary() (additions int, deletions int, err error)
	FindBranchBase(branch string) (string, error)
	GetCommitCountRange(base string) (int, error)
	GetDiffRange(base string) (string, error)
	GetDiffNumStatRange(base string) (additions int, deletions int, err error)
	ResetSoft(commit string) error
	Stash() error
	StashPop() error
	Pull() error
	GetUnpushedCommits() ([]string, error)
	GetRemoteOnlyCommits() ([]string, error)
	GetDivergenceVsOrigin(branch string) (localOnly []string, remoteOnly []string, err error)
	GetStatusShort() (string, error)
}

type ExecGitClient struct {
	RepoPath string
}

func NewExecGitClient() (*ExecGitClient, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to find git repository root: %w. Are you in a git repository?", err)
	}
	repoPath := strings.TrimSpace(out.String())
	return &ExecGitClient{RepoPath: repoPath}, nil
}

type GitError struct {
	Command  string
	Output   string
	ExitCode int
}

func (e *GitError) Error() string {
	return fmt.Sprintf("git command '%s' failed (exit code %d): %s", e.Command, e.ExitCode, strings.TrimSpace(e.Output))
}

func (c *ExecGitClient) runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.RepoPath
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command:  fmt.Sprintf("git %s", strings.Join(args, " ")),
				Output:   string(output),
				ExitCode: exitErr.ExitCode(),
			}
		}
		return "", fmt.Errorf("failed to execute git command: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (c *ExecGitClient) HasStagedChanges() (bool, error) {
	_, err := c.runGitCommand("diff", "--cached", "--no-color", "--quiet")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.ExitCode == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check for staged changes: %w", err)
	}
	return false, nil
}

func (c *ExecGitClient) HasUnstagedChanges() (bool, error) {
	_, err := c.runGitCommand("diff", "--no-color", "--quiet")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.ExitCode == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	output, err := c.runGitCommand("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, fmt.Errorf("failed to check for untracked files: %w", err)
	}

	return output != "", nil
}

func (c *ExecGitClient) HasAnyChanges() (bool, error) {
	hasUnstaged, err := c.HasUnstagedChanges()
	if err != nil {
		return false, err
	}
	if hasUnstaged {
		return true, nil
	}
	return c.HasStagedChanges()
}

func (c *ExecGitClient) GetDiff() (string, error) {
	numstatOutput, err := c.getNumstatOutput()
	if err != nil || numstatOutput == "" {
		return "", err
	}

	files := parseFilesFromNumstat(numstatOutput)
	if len(files) == 0 {
		return "", nil
	}
	return c.getDiffForFiles(files), nil
}

func (c *ExecGitClient) getNumstatOutput() (string, error) {
	numstatOutput, err := c.runGitCommand("diff", "--cached", "--numstat", "--no-color")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output != "" {
			return gitErr.Output, nil
		}
		return "", nil
	}
	return numstatOutput, nil
}

func parseFilesFromNumstat(output string) []string {
	var files []string
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) == 3 {
			files = append(files, parts[2])
		}
	}
	return files
}

func (c *ExecGitClient) getDiffForFiles(files []string) string {
	args := append([]string{"diff", "--cached", "--no-color", "--"}, files...)
	output, err := c.runGitCommand(args...)
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output != "" {
			return gitErr.Output
		}
		return ""
	}
	return output
}

func (c *ExecGitClient) StageChanges() error {
	_, err := c.runGitCommand("add", "-A")
	if err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}
	return nil
}

func (c *ExecGitClient) Commit(message string) error {
	_, err := c.runGitCommand("commit", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

func (c *ExecGitClient) AmendCommit() error {
	_, err := c.runGitCommand("commit", "--amend", "--no-edit")
	if err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	return nil
}

func (c *ExecGitClient) Push(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "git" || parts[1] != "push" {
		return "", fmt.Errorf("invalid push command format: expected 'git push ...', got '%s'", command)
	}

	cmd := exec.Command("git", parts[1:]...)
	cmd.Dir = c.RepoPath
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	cmd.Stdin = os.Stdin

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := strings.TrimSpace(buf.String())

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command:  fmt.Sprintf("git %s", strings.Join(parts[1:], " ")),
				Output:   output,
				ExitCode: exitErr.ExitCode(),
			}
		}
		return "", fmt.Errorf("failed to push changes: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) HasRemotes() (bool, error) {
	output, err := c.runGitCommand("remote", "-v")
	if err != nil {
		return false, fmt.Errorf("failed to check for remotes: %w", err)
	}
	return output != "", nil
}

func (c *ExecGitClient) GetCurrentBranch() (string, error) {
	output, err := c.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) GetRemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	output, err := c.runGitCommand("remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) GetLastCommitHash() (string, error) {
	output, err := c.runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get last commit hash: %w", err)
	}
	return output, nil
}

func sumNumstatLines(output string) (additions, deletions int) {
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		if add, err := strconv.Atoi(parts[0]); err == nil {
			additions += add
		}
		if del, err := strconv.Atoi(parts[1]); err == nil {
			deletions += del
		}
	}
	return
}

func (c *ExecGitClient) GetDiffNumStatSummary() (additions int, deletions int, err error) {
	output, err := c.getNumstatOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get diff stats: %w", err)
	}
	additions, deletions = sumNumstatLines(output)
	return
}

func (c *ExecGitClient) Pull() error {
	if _, err := c.runGitCommand("rev-parse", "--abbrev-ref", "@{u}"); err != nil {
		return nil
	}
	_, err := c.runGitCommand("pull", "--ff-only")
	if err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}
	return nil
}

func (c *ExecGitClient) getCommitList(rangeArg string) ([]string, error) {
	output, err := c.runGitCommand("log", rangeArg, "--format=%ad - %an - %s", "--date=short")
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	var commits []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

func (c *ExecGitClient) GetUnpushedCommits() ([]string, error) {
	return c.getCommitList("@{u}..HEAD")
}

func (c *ExecGitClient) GetRemoteOnlyCommits() ([]string, error) {
	return c.getCommitList("HEAD..@{u}")
}

func (c *ExecGitClient) GetStatusShort() (string, error) {
	output, err := c.runGitCommand("status", "--short")
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) GetDivergenceVsOrigin(branch string) ([]string, []string, error) {
	originRef := "origin/" + branch
	if _, err := c.runGitCommand("rev-parse", "--verify", originRef); err != nil {
		return nil, nil, nil
	}
	local, err := c.getCommitList(originRef + "..HEAD")
	if err != nil {
		return nil, nil, err
	}
	remote, err := c.getCommitList("HEAD.." + originRef)
	if err != nil {
		return nil, nil, err
	}
	return local, remote, nil
}
