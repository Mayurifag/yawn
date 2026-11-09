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

func TestExecGitClient_GetDiffUsesLiteralPathspecs(t *testing.T) {
	repo := newTestRepo(t)
	base := runTestGit(t, repo, "rev-parse", "HEAD")
	writeTestFile(t, repo, ".gitattributes", "a.txt yawn=skip\n")
	writeTestFile(t, repo, "[ab].txt", "public content\n")
	writeTestFile(t, repo, "a.txt", "synthetic private content\n")
	runTestGit(t, repo, "add", ".")
	client := &ExecGitClient{RepoPath: repo}

	check := func(diff string, err error) {
		t.Helper()
		require.NoError(t, err)
		assert.Contains(t, diff, "+public content")
		assert.Contains(t, diff, "a.txt: skipped")
		assert.NotContains(t, diff, "synthetic private content")
	}
	check(client.GetDiff())
	check(client.GetDiffCachedRange(base))
	runTestGit(t, repo, "commit", "-m", "add files")
	check(client.GetDiffRange(base))
}

func TestExecGitClient_FindBranchBaseIgnoresNonAncestorBranches(t *testing.T) {
	for _, kind := range []string{"backup", "descendant", "sibling"} {
		t.Run(kind, func(t *testing.T) {
			repo := newTestRepo(t)
			base := runTestGit(t, repo, "rev-parse", "HEAD")
			runTestGit(t, repo, "checkout", "-b", "feature")
			for _, name := range []string{"one.txt", "two.txt"} {
				writeTestFile(t, repo, name, name+"\n")
				runTestGit(t, repo, "add", name)
				runTestGit(t, repo, "commit", "-m", name)
			}
			start := "HEAD"
			if kind == "sibling" {
				start = "HEAD~1"
			}
			runTestGit(t, repo, "checkout", "-b", "other", start)
			if kind != "backup" {
				writeTestFile(t, repo, "other.txt", "other\n")
				runTestGit(t, repo, "add", "other.txt")
				runTestGit(t, repo, "commit", "-m", "other")
			}
			runTestGit(t, repo, "checkout", "feature")
			client := &ExecGitClient{RepoPath: repo}

			got, err := client.FindBranchBase("feature")
			require.NoError(t, err)
			assert.Equal(t, base, got)
			ref, err := client.FindBranchBaseRef("feature")
			require.NoError(t, err)
			assert.Equal(t, "master", ref)
		})
	}
}

func TestExecGitClient_StashRestoresOnlyItsOwnWork(t *testing.T) {
	repo := newTestRepo(t)
	writeTestFile(t, repo, "README.md", "unrelated old work\n")
	runTestGit(t, repo, "stash", "push", "-m", "old work")
	oldStash := runTestGit(t, repo, "rev-parse", "refs/stash")
	writeTestFile(t, repo, "untracked.txt", "new work\n")
	client := &ExecGitClient{RepoPath: repo}

	stash, err := client.Stash()
	require.NoError(t, err)
	require.NotEmpty(t, stash)
	require.NotEqual(t, oldStash, stash)
	assert.NoFileExists(t, filepath.Join(repo, "untracked.txt"))
	require.NoError(t, client.StashPop(stash))
	assert.FileExists(t, filepath.Join(repo, "untracked.txt"))
	assert.Equal(t, oldStash, runTestGit(t, repo, "rev-parse", "refs/stash"))
	assert.Equal(t, oldStash, runTestGit(t, repo, "stash", "list", "--format=%H"))
	assert.Empty(t, runTestGit(t, repo, "diff"))
}

func TestExecGitClient_StashWithoutChangesPreservesExistingStash(t *testing.T) {
	repo := newTestRepo(t)
	writeTestFile(t, repo, "README.md", "old work\n")
	runTestGit(t, repo, "stash", "push", "-m", "old work")
	oldStash := runTestGit(t, repo, "rev-parse", "refs/stash")
	client := &ExecGitClient{RepoPath: repo}

	stash, err := client.Stash()
	require.NoError(t, err)
	assert.Empty(t, stash)
	require.NoError(t, client.StashPop(stash))
	assert.Equal(t, oldStash, runTestGit(t, repo, "rev-parse", "refs/stash"))
	assert.Empty(t, runTestGit(t, repo, "status", "--porcelain"))
}

func TestExecGitClient_StashPreservesIndex(t *testing.T) {
	repo := newTestRepo(t)
	writeTestFile(t, repo, "README.md", "staged work\n")
	runTestGit(t, repo, "add", "README.md")
	writeTestFile(t, repo, "README.md", "unstaged work\n")
	client := &ExecGitClient{RepoPath: repo}
	staged := runTestGit(t, repo, "diff", "--cached")
	unstaged := runTestGit(t, repo, "diff")

	stash, err := client.Stash()
	require.NoError(t, err)
	require.NoError(t, client.StashPop(stash))
	assert.Equal(t, staged, runTestGit(t, repo, "diff", "--cached"))
	assert.Equal(t, unstaged, runTestGit(t, repo, "diff"))
	assert.Empty(t, runTestGit(t, repo, "stash", "list"))
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
