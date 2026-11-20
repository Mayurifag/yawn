package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPushTestRepo(t *testing.T) (string, string, *ExecGitClient) {
	t.Helper()
	repo := newTestRepo(t)
	remote := t.TempDir()
	runTestGit(t, remote, "init", "--bare")
	runTestGit(t, repo, "remote", "add", "origin", remote)
	runTestGit(t, repo, "push", "origin", "HEAD:refs/heads/master")
	runTestGit(t, repo, "checkout", "-b", "feature")
	runTestGit(t, repo, "commit", "--allow-empty", "-m", "feature")
	return repo, remote, &ExecGitClient{RepoPath: repo}
}

func TestPushUnpublishedBranchAndSetUpstream(t *testing.T) {
	repo, remote, client := newPushTestRepo(t)
	plan, err := client.PreparePush("git push -u origin HEAD")
	require.NoError(t, err)
	assert.Empty(t, plan.RemoteHead)
	assert.Len(t, plan.LocalCommits, 1)
	assert.Empty(t, plan.RemoteCommits)
	_, err = client.Push(plan, false)
	require.NoError(t, err)
	assert.Equal(t, plan.Head, runTestGit(t, remote, "rev-parse", "refs/heads/feature"))
	assert.Equal(t, "origin", runTestGit(t, repo, "config", "branch.feature.remote"))
	assert.Equal(t, "refs/heads/feature", runTestGit(t, repo, "config", "branch.feature.merge"))
}

func TestPushPreviewUsesPushURLAndDestination(t *testing.T) {
	repo, fetchRemote, client := newPushTestRepo(t)
	pushRemote := t.TempDir()
	runTestGit(t, pushRemote, "init", "--bare")
	base := runTestGit(t, repo, "rev-parse", "master")
	remoteHead := runTestGit(t, repo, "commit-tree", "HEAD^{tree}", "-p", base, "-m", "remote work")
	runTestGit(t, repo, "push", pushRemote, remoteHead+":refs/heads/review")
	runTestGit(t, repo, "remote", "set-url", "--push", "origin", pushRemote)
	refs := runTestGit(t, repo, "show-ref")
	plan, err := client.PreparePush("git push origin HEAD:review")
	require.NoError(t, err)
	assert.Equal(t, pushRemote, plan.Target.URL)
	assert.Equal(t, "review", plan.Target.Branch)
	assert.Equal(t, remoteHead, plan.RemoteHead)
	assert.Len(t, plan.LocalCommits, 1)
	assert.Len(t, plan.RemoteCommits, 1)
	assert.Equal(t, refs, runTestGit(t, repo, "show-ref"), "preview must not update remote-tracking refs")
	_, err = client.Push(plan, true)
	require.NoError(t, err)
	assert.Equal(t, plan.Head, runTestGit(t, pushRemote, "rev-parse", "refs/heads/review"))
	out, err := exec.Command("git", "-C", fetchRemote, "show-ref", "--verify", "refs/heads/review").CombinedOutput()
	require.Error(t, err, string(out))
}

func TestExplicitLeaseRejectsRemoteChangesAfterPreview(t *testing.T) {
	repo, remote, client := newPushTestRepo(t)
	plan, err := client.PreparePush("git push origin HEAD:master")
	require.NoError(t, err)
	newRemoteHead := runTestGit(t, repo, "commit-tree", "HEAD^{tree}", "-p", plan.RemoteHead, "-m", "new remote work")
	runTestGit(t, repo, "push", remote, newRemoteHead+":refs/heads/master")
	// An IDE's background fetch must not change the lease approved by the user.
	runTestGit(t, repo, "update-ref", "refs/remotes/origin/master", newRemoteHead)
	_, err = client.Push(plan, true)
	require.Error(t, err)
	assert.Equal(t, newRemoteHead, runTestGit(t, remote, "rev-parse", "refs/heads/master"))
}

func TestPushPinsPreviewedLocalCommit(t *testing.T) {
	repo, remote, client := newPushTestRepo(t)
	plan, err := client.PreparePush("git push origin HEAD")
	require.NoError(t, err)
	runTestGit(t, repo, "commit", "--allow-empty", "-m", "later local work")
	_, err = client.Push(plan, false)
	require.NoError(t, err)
	assert.Equal(t, plan.Head, runTestGit(t, remote, "rev-parse", "refs/heads/feature"))
	assert.NotEqual(t, plan.Head, runTestGit(t, repo, "rev-parse", "HEAD"))
}

func TestPushPreservesHookRemoteName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX hook")
	}
	repo, remote, client := newPushTestRepo(t)
	hooks := t.TempDir()
	runTestGit(t, repo, "config", "core.hooksPath", hooks)
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("#!/bin/sh\n[ \"$1\" = origin ]\n"), 0o700))
	plan, err := client.PreparePush("git push origin HEAD")
	require.NoError(t, err)

	_, err = client.Push(plan, false)

	require.NoError(t, err)
	assert.Equal(t, plan.Head, runTestGit(t, remote, "rev-parse", "refs/heads/feature"))
}

func TestPushPreservesRemoteReceivePack(t *testing.T) {
	repo, remote, client := newPushTestRepo(t)
	runTestGit(t, repo, "config", "remote.origin.receivepack", "git yawn-receive-pack-does-not-exist")
	plan, err := client.PreparePush("git push origin HEAD")
	require.NoError(t, err)

	_, err = client.Push(plan, false)

	require.ErrorContains(t, err, "yawn-receive-pack-does-not-exist")
	out, err := exec.Command("git", "-C", remote, "show-ref", "--verify", "refs/heads/feature").CombinedOutput()
	require.Error(t, err, string(out))
}

func TestPushRejectsChangedDestinationURL(t *testing.T) {
	repo, _, client := newPushTestRepo(t)
	plan, err := client.PreparePush("git push origin HEAD")
	require.NoError(t, err)
	other := t.TempDir()
	runTestGit(t, other, "init", "--bare")
	runTestGit(t, repo, "remote", "set-url", "--push", "origin", other)

	_, err = client.Push(plan, false)

	require.ErrorContains(t, err, "push URL changed since preview")
	out, err := exec.Command("git", "-C", other, "show-ref").CombinedOutput()
	require.Error(t, err, string(out))
}

func TestPushLinksRenamedBranchPreservesStackedParent(t *testing.T) {
	repo := newTestRepo(t)
	runTestGit(t, repo, "checkout", "-b", "parent-feature")
	runTestGit(t, repo, "commit", "--allow-empty", "-m", "parent work")
	runTestGit(t, repo, "checkout", "-b", "child-feature")

	links := PushLinks(&ExecGitClient{RepoPath: repo}, PushTarget{
		Remote: "origin", URL: "git@gitlab.com:owner/repo.git",
		Branch: "review", LocalBranch: "child-feature", DefaultBranch: "master",
	})

	assert.Contains(t, links.SuggestPRLink, "merge_request%5Btarget_branch%5D=parent-feature")
	assert.Contains(t, links.SuggestPRLink, "merge_request%5Bsource_branch%5D=review")
}

func TestResolvePushTargetRejectsAmbiguousCommands(t *testing.T) {
	repo, _, client := newPushTestRepo(t)
	for _, command := range []string{
		"git push --all origin", "git push --mirror origin", "git push --delete origin feature",
		"git push origin HEAD other", "git push origin master", "git push --force-with-lease=master:old origin HEAD",
		"git push origin 'HEAD'", "git push origin HEAD:",
	} {
		t.Run(command, func(t *testing.T) {
			_, err := client.ResolvePushTarget(command)
			require.Error(t, err)
		})
	}
	target, err := client.ResolvePushTarget("git push --force origin HEAD:review")
	require.NoError(t, err)
	assert.True(t, target.Force)
	assert.Equal(t, "review", target.Branch)
	runTestGit(t, repo, "remote", "set-url", "--add", "--push", "origin", t.TempDir())
	runTestGit(t, repo, "remote", "set-url", "--add", "--push", "origin", t.TempDir())
	_, err = client.ResolvePushTarget("git push origin HEAD")
	require.ErrorContains(t, err, "exactly one push URL")
}

func TestPushPreviewFailureIsReturned(t *testing.T) {
	repo, _, client := newPushTestRepo(t)
	runTestGit(t, repo, "remote", "set-url", "--push", "origin", t.TempDir()+"/missing")
	_, err := client.PreparePush("git push origin HEAD")
	require.ErrorContains(t, err, "cannot inspect push destination")
}

func TestGitLabNestedNamespace(t *testing.T) {
	for _, remote := range []string{"git@gitlab.com:group/subgroup/repo.git", "https://gitlab.com/group/subgroup/repo.git"} {
		info, err := ParseRemoteURL(remote)
		require.NoError(t, err)
		assert.Equal(t, "group/subgroup", info.Owner)
		assert.Equal(t, "repo", info.Repo)
		assert.True(t, strings.HasPrefix(GeneratePRURL(info.Host, info.Owner, info.Repo, "feature", ""), "https://gitlab.com/group/subgroup/repo/-/merge_requests/new?"))
	}
}
