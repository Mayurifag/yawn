package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecGitClient_GetDefaultBranchFallsBackToOriginMaster(t *testing.T) {
	repo := newTestRepo(t)
	head := runTestGit(t, repo, "rev-parse", "HEAD")
	runTestGit(t, repo, "update-ref", "refs/remotes/origin/master", head)

	client := &ExecGitClient{RepoPath: repo}

	branch, err := client.GetDefaultBranch()

	require.NoError(t, err)
	assert.Equal(t, "master", branch)
}

func TestExecGitClient_FindBranchBaseUsesOriginMasterMergeBase(t *testing.T) {
	repo := newTestRepo(t)
	base := runTestGit(t, repo, "rev-parse", "HEAD")
	runTestGit(t, repo, "update-ref", "refs/remotes/origin/master", base)
	runTestGit(t, repo, "checkout", "-b", "opencode")
	writeTestFile(t, repo, "feature.txt", "feature\n")
	runTestGit(t, repo, "add", "feature.txt")
	runTestGit(t, repo, "commit", "-m", "feature")

	client := &ExecGitClient{RepoPath: repo}

	got, err := client.FindBranchBase("opencode")

	require.NoError(t, err)
	assert.Equal(t, base, got)
}

func TestExecGitClient_FindBranchBaseRefUsesClosestBranch(t *testing.T) {
	repo := newTestRepo(t)
	runTestGit(t, repo, "checkout", "-b", "beta")
	writeTestFile(t, repo, "beta.txt", "beta\n")
	runTestGit(t, repo, "add", "beta.txt")
	runTestGit(t, repo, "commit", "-m", "beta")
	runTestGit(t, repo, "update-ref", "refs/remotes/origin/beta", "HEAD")
	runTestGit(t, repo, "checkout", "-b", "opencode")
	writeTestFile(t, repo, "feature.txt", "feature\n")
	runTestGit(t, repo, "add", "feature.txt")
	runTestGit(t, repo, "commit", "-m", "feature")

	client := &ExecGitClient{RepoPath: repo}

	got, err := client.FindBranchBaseRef("opencode")

	require.NoError(t, err)
	assert.Equal(t, "beta", got)
}

func TestExecGitClient_GetDiffCachedRangeIncludesStagedChanges(t *testing.T) {
	repo := newTestRepo(t)
	base := runTestGit(t, repo, "rev-parse", "HEAD")
	runTestGit(t, repo, "checkout", "-b", "feature")
	writeTestFile(t, repo, "committed.txt", "committed\n")
	runTestGit(t, repo, "add", "committed.txt")
	runTestGit(t, repo, "commit", "-m", "committed")
	writeTestFile(t, repo, "dirty.txt", "dirty\n")
	runTestGit(t, repo, "add", "dirty.txt")

	client := &ExecGitClient{RepoPath: repo}

	diff, err := client.GetDiffCachedRange(base)

	require.NoError(t, err)
	assert.Contains(t, diff, "committed.txt")
	assert.Contains(t, diff, "dirty.txt")
}

func TestExecGitClient_AmendCommitUsesMessage(t *testing.T) {
	repo := newTestRepo(t)
	writeTestFile(t, repo, "feature.txt", "feature\n")
	runTestGit(t, repo, "add", "feature.txt")
	runTestGit(t, repo, "commit", "-m", "old message")

	client := &ExecGitClient{RepoPath: repo}

	err := client.AmendCommit("feat: generated title\n\nGenerated body")

	require.NoError(t, err)
	assert.Equal(t, "feat: generated title\n\nGenerated body", runTestGit(t, repo, "log", "-1", "--format=%B"))
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runTestGit(t, repo, "init", "-b", "master")
	runTestGit(t, repo, "config", "user.email", "test@example.com")
	runTestGit(t, repo, "config", "user.name", "Test User")
	writeTestFile(t, repo, "README.md", "test\n")
	runTestGit(t, repo, "add", "README.md")
	runTestGit(t, repo, "commit", "-m", "initial")
	return repo
}

func writeTestFile(t *testing.T, repo, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644))
}

func runTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
	return strings.TrimSpace(string(output))
}
