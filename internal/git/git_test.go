package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDiffNumStatSummary(t *testing.T) {
	repo := newTestRepo(t)
	client := &ExecGitClient{RepoPath: repo}
	add, del, err := client.GetDiffNumStatSummary()
	assert.NoError(t, err)
	assert.Zero(t, add)
	assert.Zero(t, del)
	writeTestFile(t, repo, "README.md", "one\ntwo\n")
	writeTestFile(t, repo, "extra.txt", "three\n")
	writeTestFile(t, repo, "binary.dat", "\x00binary")
	runTestGit(t, repo, "add", ".")
	add, del, err = client.GetDiffNumStatSummary()
	assert.NoError(t, err)
	assert.Equal(t, 3, add)
	assert.Equal(t, 1, del)
	_, _, err = (&ExecGitClient{RepoPath: t.TempDir()}).GetDiffNumStatSummary()
	assert.Error(t, err)
}

func TestRepositoryRoot(t *testing.T) {
	repoPath := t.TempDir()
	runGitTestCommand(t, repoPath, "init")
	subdir := filepath.Join(repoPath, "subdir")
	assert.NoError(t, os.Mkdir(subdir, 0755))
	t.Chdir(subdir)

	root, err := RepositoryRoot()
	expectedRoot, symlinkErr := filepath.EvalSymlinks(repoPath)

	assert.NoError(t, err)
	assert.NoError(t, symlinkErr)
	assert.Equal(t, expectedRoot, root)
}

func TestRepositoryRootOutsideGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())

	root, err := RepositoryRoot()

	assert.Empty(t, root)
	assert.ErrorIs(t, err, ErrNotRepository)
}

func TestExecGitClient_HasUnstagedChanges(t *testing.T) {
	for _, state := range []string{"clean", "modified", "untracked", "staged"} {
		t.Run(state, func(t *testing.T) {
			repo := newTestRepo(t)
			switch state {
			case "modified", "staged":
				writeTestFile(t, repo, "README.md", "changed\n")
			case "untracked":
				writeTestFile(t, repo, "new.txt", "new\n")
			}
			if state == "staged" {
				runTestGit(t, repo, "add", ".")
			}
			client := &ExecGitClient{RepoPath: repo}
			unstaged, err := client.HasUnstagedChanges()
			assert.NoError(t, err)
			assert.Equal(t, state == "modified" || state == "untracked", unstaged)
			staged, err := client.HasStagedChanges()
			assert.NoError(t, err)
			assert.Equal(t, state == "staged", staged)
			anyChanges, err := client.HasAnyChanges()
			assert.NoError(t, err)
			assert.Equal(t, state != "clean", anyChanges)
		})
	}
	_, err := (&ExecGitClient{RepoPath: t.TempDir()}).HasUnstagedChanges()
	assert.Error(t, err)
}

func TestExecGitClient_GetPullRequestURLUsesGHWhenAvailable(t *testing.T) {
	repoPath := t.TempDir()
	runGitTestCommand(t, repoPath, "init")
	runGitTestCommand(t, repoPath, "remote", "add", "origin", "git@github.com:owner/repo.git")

	binDir := t.TempDir()
	writeFakeGH(t, binDir, `#!/bin/sh
if [ "$1" = "auth" ]; then
	exit 0
fi
if [ "$*" = 'pr list --repo github.com/other-owner/other-repo --state open --head feature/test --json url --jq .[0].url // empty' ]; then
	echo "https://github.com/other-owner/other-repo/pull/20"
	exit 0
fi
exit 1
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client := &ExecGitClient{RepoPath: repoPath}
	url, err := client.GetPullRequestURL("git@github.com:other-owner/other-repo.git", "refs/heads/feature/test")
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/other-owner/other-repo/pull/20", url)
}

func TestExecGitClient_GetDiffSummarizesOverBudgetFiles(t *testing.T) {
	repoPath := t.TempDir()
	runGitTestCommand(t, repoPath, "init")
	runGitTestCommand(t, repoPath, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repoPath, "config", "user.name", "Test User")

	big := strings.Repeat("a\n", MaxDiffBytes)
	if err := os.WriteFile(filepath.Join(repoPath, "big.txt"), []byte(big), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "small.txt"), []byte("small\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitTestCommand(t, repoPath, "add", ".")

	client := &ExecGitClient{RepoPath: repoPath}
	diff, err := client.GetDiff()

	assert.NoError(t, err)
	assert.Contains(t, diff, "large diff omitted")
	assert.Contains(t, diff, "big.txt")
	assert.Contains(t, diff, "small.txt")
	assert.LessOrEqual(t, len(diff), MaxDiffBytes)
}

func TestExecGitClient_GetPullRequestURLChecksGHAuthBeforePRLookup(t *testing.T) {
	repoPath := t.TempDir()
	runGitTestCommand(t, repoPath, "init")
	runGitTestCommand(t, repoPath, "remote", "add", "origin", "git@github.com:owner/repo.git")

	binDir := t.TempDir()
	markerPath := filepath.Join(t.TempDir(), "pr-called")
	writeFakeGH(t, binDir, `#!/bin/sh
if [ "$1" = "auth" ]; then
	echo "not logged in"
	exit 1
fi
if [ "$1" = "pr" ]; then
	touch "$GH_PR_MARKER"
	exit 0
fi
exit 1
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_PR_MARKER", markerPath)

	client := &ExecGitClient{RepoPath: repoPath}
	_, err := client.GetPullRequestURL("git@github.com:owner/repo.git", "feature/test")
	assert.ErrorContains(t, err, "gh is not authenticated")
	assert.NoFileExists(t, markerPath)
}

func writeFakeGH(t *testing.T, dir, script string) {
	t.Helper()
	name := "gh"
	if runtime.GOOS == "windows" {
		name = "gh.bat"
		if strings.Contains(script, "not logged in") {
			script = `@echo off
if "%1"=="auth" (
	echo not logged in
	exit /b 1
)
if "%1"=="pr" (
	type nul > "%GH_PR_MARKER%"
	exit /b 0
)
exit /b 1
`
		} else {
			script = `@echo off
if "%1"=="auth" exit /b 0
if "%1"=="pr" (
	if not "%2"=="list" exit /b 1
	if not "%4"=="github.com/other-owner/other-repo" exit /b 1
	if not "%6"=="open" exit /b 1
	echo https://github.com/other-owner/other-repo/pull/20
	exit /b 0
)
exit /b 1
`
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func BenchmarkGetDiffManyFiles(b *testing.B) {
	repo := b.TempDir()
	cmd := exec.Command("git", "init", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git init: %v: %s", err, out)
	}
	for i := range 200 {
		if err := os.WriteFile(filepath.Join(repo, fmt.Sprintf("file-%03d.txt", i)), []byte("public content\n"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	cmd = exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git add: %v: %s", err, out)
	}
	client := &ExecGitClient{RepoPath: repo}
	b.ResetTimer()
	for b.Loop() {
		if _, err := client.GetDiff(); err != nil {
			b.Fatal(err)
		}
	}
}
