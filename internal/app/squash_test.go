package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSquashGenerationFailurePreservesBranch(t *testing.T) {
	a, original := newSquashTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.RunSquash(ctx)

	require.Error(t, err)
	assert.Equal(t, original, squashTestGit(t, a, "rev-parse", "HEAD"))
	assert.Empty(t, squashTestGit(t, a, "status", "--porcelain"))
}

func TestRunSquashPreservesTree(t *testing.T) {
	a, _ := newSquashTestApp(t)
	fakeSquashOpenCode(t)
	tree := squashTestGit(t, a, "rev-parse", "HEAD^{tree}")

	require.NoError(t, a.RunSquash(context.Background()))

	assert.Equal(t, "1", squashTestGit(t, a, "rev-list", "--count", "master..HEAD"))
	assert.Equal(t, "feat: squash branch", squashTestGit(t, a, "log", "-1", "--format=%s"))
	assert.Equal(t, tree, squashTestGit(t, a, "rev-parse", "HEAD^{tree}"))
	assert.Empty(t, squashTestGit(t, a, "status", "--porcelain"))
}

func TestRunSquashCommitFailureRestoresBranch(t *testing.T) {
	a, original := newSquashTestApp(t)
	fakeSquashOpenCode(t)
	hooks := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o700))
	squashTestGit(t, a, "config", "core.hooksPath", hooks)

	err := a.RunSquash(context.Background())

	require.ErrorContains(t, err, "failed to commit")
	assert.Equal(t, original, squashTestGit(t, a, "rev-parse", "HEAD"))
	assert.Empty(t, squashTestGit(t, a, "status", "--porcelain"))
}

func newSquashTestApp(t *testing.T) (*App, string) {
	t.Helper()
	repo := t.TempDir()
	a := NewApp(config.Config{
		MainProvider:          config.ProviderOpenCodeCLI,
		RequestTimeoutSeconds: 15,
	}, &git.ExecGitClient{RepoPath: repo})
	squashTestGit(t, a, "init", "-b", "master")
	squashTestGit(t, a, "config", "user.name", "Test User")
	squashTestGit(t, a, "config", "user.email", "test@example.com")
	squashTestGit(t, a, "commit", "--allow-empty", "-m", "base")
	squashTestGit(t, a, "checkout", "-b", "feature")
	for _, name := range []string{"one.txt", "two.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(repo, name), []byte(name+"\n"), 0o600))
		squashTestGit(t, a, "add", name)
		squashTestGit(t, a, "commit", "-m", name)
	}
	return a, squashTestGit(t, a, "rev-parse", "HEAD")
}

func squashTestGit(t *testing.T, a *App, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = a.GitClient.(*git.ExecGitClient).RepoPath
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func fakeSquashOpenCode(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake opencode executable requires a POSIX shell")
	}
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "opencode"), []byte(`#!/bin/sh
printf '%s\n' '{"type":"text","part":{"type":"text","text":"feat: squash branch"}}'
`), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
