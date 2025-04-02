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
	GetDiff(ignorePatterns []string) (string, error)
	StageChanges() error
	Commit(message string) error
	Push(command string) error
	HasRemotes() (bool, error)
}

// ExecGitClient implements GitClient using os/exec.
type ExecGitClient struct {
	RepoPath string // Path to the repository root
	Verbose  bool
}

// NewExecGitClient creates a new Git client that executes git commands.
// It tries to find the repository root automatically.
func NewExecGitClient(verbose bool) (*ExecGitClient, error) {
	// Find git repo root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to find git repository root: %w. Are you in a git repository?", err)
	}
	repoPath := strings.TrimSpace(out.String())
	return &ExecGitClient{RepoPath: repoPath, Verbose: verbose}, nil
}

func (g *ExecGitClient) runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath // Ensure command runs in the repo root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if g.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Running command: git %s\n", strings.Join(args, " "))
	}

	err := cmd.Run()
	if err != nil {
		// For some git commands, non-zero exit codes are expected and meaningful
		// Let the calling function interpret the exit code if needed
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Return the error with stderr for the caller to handle
			return stdout.String(), &GitError{
				Command:  fmt.Sprintf("git %s", strings.Join(args, " ")),
				ExitCode: exitErr.ExitCode(),
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		// For non-ExitError types (like if git is not found), return as a regular error
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		if g.Verbose {
			fmt.Fprintf(os.Stderr, "[GIT] Error: %s\n", errMsg)
		}
		return "", fmt.Errorf("git command failed: git %s: %s", strings.Join(args, " "), errMsg)
	}

	output := strings.TrimSpace(stdout.String())
	if g.Verbose && output != "" {
		fmt.Fprintf(os.Stderr, "[GIT] Output:\n%s\n", output)
	}
	if g.Verbose && strings.TrimSpace(stderr.String()) != "" {
		fmt.Fprintf(os.Stderr, "[GIT] Stderr:\n%s\n", strings.TrimSpace(stderr.String()))
	}

	return output, nil
}

// GitError represents a git command error with exit code information.
type GitError struct {
	Command  string
	ExitCode int
	Stderr   string
}

func (e *GitError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git command '%s' failed with exit code %d: %s", e.Command, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("git command '%s' failed with exit code %d", e.Command, e.ExitCode)
}

// HasStagedChanges checks if there are any staged changes.
func (g *ExecGitClient) HasStagedChanges() (bool, error) {
	_, err := g.runGitCommand("diff", "--cached", "--quiet")
	if err != nil {
		// Exit code 1 means there are staged changes
		if gitErr, ok := err.(*GitError); ok && gitErr.ExitCode == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check staged changes: %w", err)
	}
	// Exit code 0 means no staged changes
	return false, nil
}

// HasUncommittedChanges checks for any uncommitted changes (staged or unstaged).
func (g *ExecGitClient) HasUncommittedChanges() (bool, error) {
	// Use status --porcelain which is stable and easy to parse.
	// It lists changes line by line. Empty output means no changes.
	output, err := g.runGitCommand("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to get git status: %w", err)
	}
	return output != "", nil
}

// GetDiff gets the diff of staged changes that will be included in the next commit.
// It ignores files matching the provided glob patterns.
func (g *ExecGitClient) GetDiff(ignorePatterns []string) (string, error) {
	// We want the diff of staged changes only, which is what will be committed.
	// `git diff --cached` shows changes staged for the next commit.
	args := []string{"diff", "--cached"}

	// Add pathspecs to exclude ignored patterns
	// Note: Simple glob matching might not cover all gitignore capabilities.
	// Git's internal filtering is more robust, but applying it post-diff is complex.
	// Using pathspecs is a good compromise.
	if len(ignorePatterns) > 0 {
		args = append(args, "--") // Separator for pathspecs
		// Add pathspecs to exclude ignored files
		// We need to use the ':!' exclude syntax for pathspecs
		for _, pattern := range ignorePatterns {
			if pattern != "" {
				args = append(args, fmt.Sprintf(":(exclude)%s", pattern))
				// Also exclude directories matching the pattern, git pathspec behavior can vary
				args = append(args, fmt.Sprintf(":(exclude)*/%s", pattern))
			}
		}
	}

	// Enhanced verbose logging for the exact command being executed
	if g.Verbose {
		fmt.Fprintf(os.Stderr, "[GIT] Getting diff of staged changes with command: git %s\n", strings.Join(args, " "))
	}

	diff, err := g.runGitCommand(args...)
	if err != nil {
		// It's possible `git diff --cached` returns an error if there are no staged changes
		// Or if there are no changes (though usually just empty output).
		// Let's check for specific known non-fatal errors if needed.
		// For now, return the error.
		return "", fmt.Errorf("failed to get git diff of staged changes: %w", err)
	}

	// Enhanced verbose logging for the obtained diff
	if g.Verbose {
		if diff == "" {
			fmt.Fprintln(os.Stderr, "[GIT] No staged changes found after applying ignore patterns.")
		} else {
			// Log a summary of the diff to avoid excessive output
			lines := strings.Split(diff, "\n")
			if len(lines) > 10 {
				fmt.Fprintf(os.Stderr, "[GIT] Obtained diff of %d lines (showing first 10):\n%s\n...\n", len(lines), strings.Join(lines[:10], "\n"))
			} else {
				fmt.Fprintf(os.Stderr, "[GIT] Obtained diff of %d lines:\n%s\n", len(lines), diff)
			}
		}
	}

	return diff, nil
}

// StageChanges stages all current changes including untracked files.
func (g *ExecGitClient) StageChanges() error {
	// First stage all tracked files
	_, err := g.runGitCommand("add", "-u")
	if err != nil {
		return fmt.Errorf("failed to stage tracked changes: %w", err)
	}

	// Then stage untracked files
	_, err = g.runGitCommand("add", ".")
	if err != nil {
		return fmt.Errorf("failed to stage untracked files: %w", err)
	}

	return nil
}

// Commit creates a commit with the given message.
// It commits only the currently staged changes.
// The caller is responsible for ensuring changes are staged before calling this method.
func (g *ExecGitClient) Commit(message string) error {
	// Commit the staged changes
	_, err := g.runGitCommand("commit", "-m", message)
	if err != nil {
		// Check for "nothing to commit" error, which might happen if nothing is staged
		if strings.Contains(err.Error(), "nothing to commit") || strings.Contains(err.Error(), "no changes added to commit") {
			fmt.Fprintln(os.Stderr, "[GIT] Warning: No changes were staged for commit.")
			return nil // Not a fatal error for yawn's flow
		}
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

// Push executes the configured push command.
func (g *ExecGitClient) Push(command string) error {
	// Split the command string into parts for exec.Command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("push command is empty")
	}
	// Assume the first part is "git", replace if necessary or prepend if missing?
	// Let's assume user provides full command like "git push origin HEAD"
	if len(parts) < 2 || parts[0] != "git" {
		// Maybe prepend git? Or error out? Let's error out for clarity.
		return fmt.Errorf("invalid push command format: expected 'git push ...', got '%s'", command)
		// parts = append([]string{"git"}, parts...)
	}

	_, err := g.runGitCommand(parts[1:]...) // Pass arguments after "git"
	if err != nil {
		return fmt.Errorf("failed to push changes using command '%s': %w", command, err)
	}
	return nil
}

// HasRemotes checks if the repository has any remote repositories configured.
func (g *ExecGitClient) HasRemotes() (bool, error) {
	if g.Verbose {
		fmt.Fprintln(os.Stderr, "[GIT] Checking for remote repositories...")
	}

	output, err := g.runGitCommand("remote")
	if err != nil {
		// If the error is a GitError with exit code 128, it might mean we're not in a git repo
		if gitErr, ok := err.(*GitError); ok && gitErr.ExitCode == 128 {
			return false, fmt.Errorf("not a git repository: %w", err)
		}
		return false, fmt.Errorf("failed to check for remotes: %w", err)
	}

	// If output is empty, no remotes exist
	hasRemotes := output != ""
	if g.Verbose {
		if hasRemotes {
			fmt.Fprintf(os.Stderr, "[GIT] Found remote repositories: %s\n", output)
		} else {
			fmt.Fprintln(os.Stderr, "[GIT] No remote repositories found")
		}
	}

	return hasRemotes, nil
}

// --- Mock Client for Testing ---

// MockGitClient is a mock implementation of GitClient.
type MockGitClient struct {
	MockHasStagedChanges      func() (bool, error)
	MockHasUncommittedChanges func() (bool, error)
	MockGetDiff               func(ignorePatterns []string) (string, error)
	MockStageChanges          func() error
	MockCommit                func(message string) error
	MockPush                  func(command string) error
	MockHasRemotes            func() (bool, error)
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

func (m *MockGitClient) GetDiff(ignorePatterns []string) (string, error) {
	if m.MockGetDiff != nil {
		return m.MockGetDiff(ignorePatterns)
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
