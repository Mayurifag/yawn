package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	NetworkPushTimeout = 60 * time.Second
	PRLookupTimeout    = 10 * time.Second
	MaxDiffBytes       = 120000
)

var (
	ErrNetworkTimeout = errors.New("git network operation timed out")
	ErrNotRepository  = errors.New("not a git repository: run yawn from inside a git work tree")
)

type GitClient interface {
	HasStagedChanges() (bool, error)
	HasUnstagedChanges() (bool, error)
	HasAnyChanges() (bool, error)
	GetDiff() (string, error)
	StageChanges() error
	Commit(message string) error
	AmendCommit(message string) error
	ResolvePushTarget(command string) (PushTarget, error)
	PreparePush(command string) (*PushPlan, error)
	Push(plan *PushPlan, force bool) (string, error)
	HasRemotes() (bool, error)
	GetCurrentBranch() (string, error)
	SetRemoteURL(remote, newURL string) error
	GetLastCommitHash() (string, error)
	GetDiffNumStatSummary() (additions int, deletions int, err error)
	FindBranchBase(branch, baseRef string) (string, error)
	FindBranchBaseRef(branch string) (string, error)
	GetCommitCountRange(base string) (int, error)
	GetDiffRange(base string) (string, error)
	GetDiffCachedRange(base string) (string, error)
	GetDiffNumStatCachedRange(base string) (additions int, deletions int, err error)
	ResetSoft(commit string) error
	Stash() (string, error)
	StashPop(stash string) error
	GetStatusShort() (string, error)
	GetDefaultBranch(remote string) (string, error)
	GetPullRequestURL(remoteURL, branch string) (string, error)
}

type ExecGitClient struct {
	RepoPath string
}

func NewExecGitClient() (*ExecGitClient, error) {
	repoPath, err := RepositoryRoot()
	if err != nil {
		return nil, err
	}
	return &ExecGitClient{RepoPath: repoPath}, nil
}

func RepositoryRoot() (string, error) {
	insideCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	insideOutput, err := insideCmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", ErrNotRepository
		}
		return "", fmt.Errorf("failed to check git repository: %w", err)
	}
	if strings.TrimSpace(string(insideOutput)) != "true" {
		return "", ErrNotRepository
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to find git repository root: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
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
	output, err := c.runGitCommandRaw(args...)
	return strings.TrimSpace(output), err
}

func (c *ExecGitClient) runGitCommandRaw(args ...string) (string, error) {
	return c.runGitCommandInput("", args...)
}

func (c *ExecGitClient) runGitCommandInput(input string, args ...string) (string, error) {
	return c.runGitCommandContext(context.Background(), input, args...)
}

func (c *ExecGitClient) runGitCommandContext(ctx context.Context, input string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.RepoPath
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("%w: git %s", ErrNetworkTimeout, args[0])
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command:  fmt.Sprintf("git %s", strings.Join(args, " ")),
				Output:   string(output) + string(exitErr.Stderr),
				ExitCode: exitErr.ExitCode(),
			}
		}
		return "", fmt.Errorf("failed to execute git command: %w", err)
	}

	return string(output), nil
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
	return c.buildFilteredDiff(numstatOutput, []string{"diff", "--cached", "--no-color"}, "")
}

func (c *ExecGitClient) getNumstatOutput() (string, error) {
	return c.runGitCommandRaw("diff", "--cached", "--numstat", "-z", "--no-renames", "--no-color")
}

func writeLimited(b *strings.Builder, s string, limit int) bool {
	if s == "" {
		return true
	}
	remaining := limit - b.Len()
	if remaining <= 0 {
		return false
	}
	if len(s) <= remaining {
		b.WriteString(s)
		return true
	}
	for remaining > 0 && !utf8.RuneStart(s[remaining]) {
		remaining--
	}
	b.WriteString(s[:remaining])
	return false
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

func (c *ExecGitClient) AmendCommit(message string) error {
	_, err := c.runGitCommand("commit", "--amend", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	return nil
}

func (c *ExecGitClient) HasRemotes() (bool, error) {
	output, err := c.runGitCommand("remote", "-v")
	if err != nil {
		return false, fmt.Errorf("failed to check for remotes: %w", err)
	}
	return output != "", nil
}

func (c *ExecGitClient) GetCurrentBranch() (string, error) {
	if output, err := c.runGitCommand("branch", "--show-current"); err == nil && output != "" {
		return normalizeBranchName(output), nil
	}
	output, err := c.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return normalizeBranchName(output), nil
}

func (c *ExecGitClient) GetPullRequestURL(remoteURL, branch string) (string, error) {
	branch = normalizeBranchName(branch)
	if branch == "" {
		return "", fmt.Errorf("branch is empty")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh is not available: %w", err)
	}

	remoteInfo, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return "", err
	}
	if !isGitHubHost(remoteInfo.Host) {
		return "", fmt.Errorf("not a GitHub remote: %s", remoteInfo.Host)
	}
	if err := c.checkGHAuth(remoteInfo.Host); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), PRLookupTimeout)
	defer cancel()

	output, err := c.runGHCommandContext(ctx, "pr", "list", "--repo", remoteInfo.Host+"/"+remoteInfo.Owner+"/"+remoteInfo.Repo,
		"--state", "open", "--head", branch, "--json", "url", "--jq", ".[0].url // empty")
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("pull request lookup timed out for branch %q", branch)
		}
		return "", fmt.Errorf("failed to find pull request for branch %q: %s", branch, strings.TrimSpace(output))
	}
	return strings.TrimSpace(output), nil
}

func isGitHubHost(host string) bool {
	return strings.Contains(strings.ToLower(host), "github")
}

func (c *ExecGitClient) checkGHAuth(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), PRLookupTimeout)
	defer cancel()
	output, err := c.runGHCommandContext(ctx, "auth", "status", "--hostname", host)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("gh auth check timed out for host %q", host)
		}
		return fmt.Errorf("gh is not authenticated for host %q: %s", host, strings.TrimSpace(output))
	}
	return nil
}

func (c *ExecGitClient) runGHCommandContext(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = c.RepoPath
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (c *ExecGitClient) SetRemoteURL(remote, newURL string) error {
	if remote == "" {
		remote = "origin"
	}
	if _, err := c.runGitCommand("remote", "set-url", "--push", remote, newURL); err != nil {
		return fmt.Errorf("failed to set remote URL: %w", err)
	}
	return nil
}

func (c *ExecGitClient) GetLastCommitHash() (string, error) {
	output, err := c.runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get last commit hash: %w", err)
	}
	return output, nil
}

func sumNumstatLines(output string) (additions, deletions int) {
	for _, record := range splitNumstatRecords(output) {
		parts := strings.Split(record, "\t")
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

func (c *ExecGitClient) getCommitList(revisions ...string) ([]string, error) {
	args := append([]string{"log", "--format=%ad - %an - %s", "--date=short"}, revisions...)
	output, err := c.runGitCommand(args...)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	var commits []string
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

func (c *ExecGitClient) GetStatusShort() (string, error) {
	output, err := c.runGitCommand("status", "--short")
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}
	return output, nil
}

func (c *ExecGitClient) GetDefaultBranch(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	prefix := "refs/remotes/" + remote
	output, err := c.runGitCommand("symbolic-ref", prefix+"/HEAD")
	if err == nil {
		return strings.TrimPrefix(output, prefix+"/"), nil
	}

	for _, namespace := range []string{prefix, "refs/heads"} {
		output, listErr := c.runGitCommand("for-each-ref", "--format=%(refname)", namespace)
		if listErr != nil {
			return "", listErr
		}
		var branches []string
		for _, ref := range strings.Split(output, "\n") {
			branch := strings.TrimPrefix(ref, namespace+"/")
			if branch != "" && branch != "HEAD" {
				branches = append(branches, branch)
			}
		}
		for _, candidate := range []string{"main", "master", "dev"} {
			for _, branch := range branches {
				if branch == candidate {
					return branch, nil
				}
			}
		}
		if len(branches) == 1 {
			return branches[0], nil
		}
	}
	return "", err
}
