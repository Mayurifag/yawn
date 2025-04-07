package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitClient defines the interface for Git operations.
// This allows for mocking in tests.
type GitClient interface {
	HasStagedChanges() (bool, error)
	HasUncommittedChanges() (bool, error)
	HasUnstagedChanges() (bool, error)
	HasAnyChanges() (bool, error)
	GetDiff() (string, error)
	StageChanges() error
	Commit(message string) error
	Push(command string) error
	HasRemotes() (bool, error)
	GetCurrentBranch() (string, error)
	GetRemoteURL(remote string) (string, error)
	GetLastCommitHash() (string, error)
}

// ExecGitClient implements GitClient using os/exec.
type ExecGitClient struct {
	RepoPath string // Path to the repository root
	Verbose  bool
}

// NewExecGitClient creates a new Git client that executes git commands.
// It tries to find the repository root automatically.
func NewExecGitClient(verbose bool) (*ExecGitClient, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to find git repository root: %w. Are you in a git repository?", err)
	}
	repoPath := strings.TrimSpace(out.String())
	return &ExecGitClient{RepoPath: repoPath, Verbose: verbose}, nil
}

// GitError represents an error from a git command execution.
type GitError struct {
	Command string
	Output  string
	Err     error
}

// Error implements the error interface for GitError.
func (e *GitError) Error() string {
	return fmt.Sprintf("git command '%s' failed: %s", e.Command, e.Err.Error())
}

// runGitCommand executes a git command and returns its output and any error.
// It handles command execution, output capture, and error wrapping.
func (c *ExecGitClient) runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.RepoPath
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &GitError{
				Command: fmt.Sprintf("git %s", strings.Join(args, " ")),
				Output:  string(output),
				Err:     fmt.Errorf("git command failed with exit code %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(output))),
			}
		}
		return "", fmt.Errorf("failed to execute git command: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// HasStagedChanges checks if there are any staged changes in the repository.
// Returns true if there are staged changes, false otherwise.
func (c *ExecGitClient) HasStagedChanges() (bool, error) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Checking for staged changes...\n")
	}
	_, err := c.runGitCommand("diff", "--cached", "--no-color", "--quiet")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output == "" {
			// Exit code 1 with no output means there are staged changes
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[GIT] Found staged changes\n")
			}
			return true, nil
		}
		return false, fmt.Errorf("failed to check for staged changes: %w", err)
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] No staged changes found\n")
	}
	return false, nil
}

// HasUncommittedChanges checks if there are any uncommitted changes in the repository.
// Returns true if there are uncommitted changes, false otherwise.
func (c *ExecGitClient) HasUncommittedChanges() (bool, error) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Checking for uncommitted changes...\n")
	}
	_, err := c.runGitCommand("diff", "--no-color", "--quiet")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output == "" {
			// Exit code 1 with no output means there are uncommitted changes
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[GIT] Found uncommitted changes\n")
			}
			return true, nil
		}
		return false, fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] No uncommitted changes found\n")
	}
	return false, nil
}

// HasUnstagedChanges checks if there are any unstaged changes in the repository.
// Returns true if there are unstaged changes, false otherwise.
func (c *ExecGitClient) HasUnstagedChanges() (bool, error) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Checking for unstaged changes...\n")
	}
	// Use git diff --quiet to check for unstaged changes
	// Exit code 1 (with empty output) means there are unstaged changes
	_, err := c.runGitCommand("diff", "--no-color", "--quiet")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output == "" {
			// Exit code 1 with no output means there are unstaged changes
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[GIT] Found unstaged changes\n")
			}
			return true, nil
		}
		return false, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}
	// Exit code 0 means no unstaged changes
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] No unstaged changes found\n")
	}
	return false, nil
}

// HasAnyChanges checks if there are any changes (staged or unstaged) in the repository.
// Returns true if there are either staged or unstaged changes, false otherwise.
func (c *ExecGitClient) HasAnyChanges() (bool, error) {
	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Checking for any changes (staged or unstaged)...\n")
	}

	// First check for unstaged changes
	hasUnstaged, err := c.HasUnstagedChanges()
	if err != nil {
		return false, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	if hasUnstaged {
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[GIT] Found unstaged changes\n")
		}
		return true, nil
	}

	// If no unstaged changes, check for staged changes
	hasStaged, err := c.HasStagedChanges()
	if err != nil {
		return false, fmt.Errorf("failed to check for staged changes: %w", err)
	}

	if hasStaged {
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[GIT] Found staged changes\n")
		}
		return true, nil
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] No changes found (neither staged nor unstaged)\n")
	}
	return false, nil
}

// GetDiff retrieves the diff of staged changes.
// Returns the diff output as a string.
func (c *ExecGitClient) GetDiff() (string, error) {
	output, err := c.runGitCommand("diff", "--cached", "--no-color")
	if err != nil {
		if gitErr, ok := err.(*GitError); ok && gitErr.Output == "" {
			// Exit code 1 with no output means no changes
			return "", nil
		}
		return "", fmt.Errorf("failed to get diff: %w", err)
	}
	return output, nil
}

// StageChanges stages all changes in the repository.
// Returns an error if staging fails.
func (c *ExecGitClient) StageChanges() error {
	_, err := c.runGitCommand("add", "-A")
	if err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}
	return nil
}

// Commit creates a commit with the given message.
// Returns an error if commit fails.
func (c *ExecGitClient) Commit(message string) error {
	_, err := c.runGitCommand("commit", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

// Push executes the provided Git push command.
func (c *ExecGitClient) Push(command string) error {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "git" {
		return fmt.Errorf("invalid push command format: expected 'git push ...', got '%s'", command)
	}

	// Remove the "git" prefix and execute the command
	_, err := c.runGitCommand(parts[1:]...)
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}
	return nil
}

// HasRemotes checks if the repository has any remote repositories configured.
// Returns true if there are remotes, false otherwise.
func (c *ExecGitClient) HasRemotes() (bool, error) {
	output, err := c.runGitCommand("remote", "-v")
	if err != nil {
		return false, fmt.Errorf("failed to check for remotes: %w", err)
	}
	return output != "", nil
}

// GetCurrentBranch returns the name of the current branch.
// Returns an error if branch name cannot be determined.
func (c *ExecGitClient) GetCurrentBranch() (string, error) {
	output, err := c.runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return output, nil
}

// GetRemoteURL returns the URL of the specified remote.
func (c *ExecGitClient) GetRemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin" // Default to origin when no remote is specified
	}
	output, err := c.runGitCommand("remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// GetLastCommitHash returns the hash of the last commit.
func (g *ExecGitClient) GetLastCommitHash() (string, error) {
	output, err := g.runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get last commit hash: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// --- Mock Client for Testing ---

// MockGitClient implements GitClient for testing purposes.
type MockGitClient struct {
	MockHasStagedChanges      func() (bool, error)
	MockHasUncommittedChanges func() (bool, error)
	MockHasUnstagedChanges    func() (bool, error)
	MockHasAnyChanges         func() (bool, error)
	MockGetDiff               func() (string, error)
	MockStageChanges          func() error
	MockCommit                func(message string) error
	MockPush                  func(command string) error
	MockHasRemotes            func() (bool, error)
	MockGetCurrentBranch      func() (string, error)
	MockGetRemoteURL          func(remoteName string) (string, error)
	MockGetLastCommitHash     func() (string, error)
}

func (m *MockGitClient) HasStagedChanges() (bool, error) {
	if m.MockHasStagedChanges != nil {
		return m.MockHasStagedChanges()
	}
	return false, nil
}

func (m *MockGitClient) HasUncommittedChanges() (bool, error) {
	if m.MockHasUncommittedChanges != nil {
		return m.MockHasUncommittedChanges()
	}
	return true, nil // Default to having changes for testing flow
}

func (m *MockGitClient) HasUnstagedChanges() (bool, error) {
	if m.MockHasUnstagedChanges != nil {
		return m.MockHasUnstagedChanges()
	}
	return false, fmt.Errorf("mock HasUnstagedChanges not implemented")
}

func (m *MockGitClient) HasAnyChanges() (bool, error) {
	if m.MockHasAnyChanges != nil {
		return m.MockHasAnyChanges()
	}
	return false, fmt.Errorf("mock HasAnyChanges not implemented")
}

func (m *MockGitClient) GetDiff() (string, error) {
	if m.MockGetDiff != nil {
		return m.MockGetDiff()
	}
	return "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new", nil
}

func (m *MockGitClient) StageChanges() error {
	if m.MockStageChanges != nil {
		return m.MockStageChanges()
	}
	return nil
}

func (m *MockGitClient) Commit(message string) error {
	if m.MockCommit != nil {
		return m.MockCommit(message)
	}
	return nil
}

func (m *MockGitClient) Push(command string) error {
	if m.MockPush != nil {
		return m.MockPush(command)
	}
	return nil
}

func (m *MockGitClient) HasRemotes() (bool, error) {
	if m.MockHasRemotes != nil {
		return m.MockHasRemotes()
	}
	return true, nil // Default to having remotes for testing flow
}

// GetCurrentBranch implements GitClient.GetCurrentBranch for MockGitClient.
func (m *MockGitClient) GetCurrentBranch() (string, error) {
	if m.MockGetCurrentBranch != nil {
		return m.MockGetCurrentBranch()
	}
	return "", fmt.Errorf("MockGetCurrentBranch not implemented")
}

// GetRemoteURL implements GitClient.GetRemoteURL for MockGitClient.
func (m *MockGitClient) GetRemoteURL(remoteName string) (string, error) {
	if m.MockGetRemoteURL != nil {
		return m.MockGetRemoteURL(remoteName)
	}
	return "", fmt.Errorf("MockGetRemoteURL not implemented")
}

// GetLastCommitHash implements GitClient.GetLastCommitHash for MockGitClient.
func (m *MockGitClient) GetLastCommitHash() (string, error) {
	if m.MockGetLastCommitHash != nil {
		return m.MockGetLastCommitHash()
	}
	return "", fmt.Errorf("MockGetLastCommitHash not implemented")
}
