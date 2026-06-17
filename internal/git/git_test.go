package git

import (
	"errors"
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
	tests := []struct {
		name         string
		mockOutput   string
		mockError    error
		expAdditions int
		expDeletions int
		expError     bool
	}{
		{
			name:         "empty diff",
			mockOutput:   "",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "single file with additions and deletions",
			mockOutput:   "10\t5\tsome/file.go",
			mockError:    nil,
			expAdditions: 10,
			expDeletions: 5,
			expError:     false,
		},
		{
			name:         "multiple files",
			mockOutput:   "10\t5\tfile1.go\n3\t0\tfile2.go\n0\t7\tfile3.go",
			mockError:    nil,
			expAdditions: 13,
			expDeletions: 12,
			expError:     false,
		},
		{
			name:         "binary file",
			mockOutput:   "-\t-\tbinary.png",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "mixed normal and binary files",
			mockOutput:   "10\t5\tfile1.go\n-\t-\tbinary.png\n3\t2\tfile2.go",
			mockError:    nil,
			expAdditions: 13,
			expDeletions: 7,
			expError:     false,
		},
		{
			name:         "malformed line",
			mockOutput:   "not-a-number\tfile.go",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "command error",
			mockOutput:   "",
			mockError:    errors.New("git command failed"),
			expAdditions: 0,
			expDeletions: 0,
			expError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockGitClient{
				MockGetDiffNumStatSummary: func() (int, int, error) {
					if tt.mockError != nil {
						return 0, 0, tt.mockError
					}
					return tt.expAdditions, tt.expDeletions, nil
				},
			}

			additions, deletions, err := mockClient.GetDiffNumStatSummary()

			if tt.expError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expAdditions, additions)
				assert.Equal(t, tt.expDeletions, deletions)
			}
		})
	}
}

func TestMockGitClient_GetDiffNumStatSummary(t *testing.T) {
	t.Run("using custom mock function", func(t *testing.T) {
		client := &MockGitClient{
			MockGetDiffNumStatSummary: func() (int, int, error) {
				return 42, 24, nil
			},
		}

		add, del, err := client.GetDiffNumStatSummary()
		assert.NoError(t, err)
		assert.Equal(t, 42, add)
		assert.Equal(t, 24, del)
	})

	t.Run("using default implementation", func(t *testing.T) {
		client := &MockGitClient{}

		add, del, err := client.GetDiffNumStatSummary()
		assert.NoError(t, err)
		assert.Equal(t, 0, add)
		assert.Equal(t, 0, del)
	})
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

func TestHasUnstagedChanges(t *testing.T) {
	tests := []struct {
		name          string
		diffOutput    string
		diffError     error
		lsFilesOutput string
		lsFilesError  error
		expected      bool
		expectErr     bool
	}{
		{
			name:          "modified files detected",
			diffOutput:    "",
			diffError:     errors.New("exit code 1"),
			lsFilesOutput: "",
			lsFilesError:  nil,
			expected:      true,
			expectErr:     false,
		},
		{
			name:          "untracked files detected",
			diffOutput:    "",
			diffError:     nil,
			lsFilesOutput: "untracked.txt\nuntracked2.go",
			lsFilesError:  nil,
			expected:      true,
			expectErr:     false,
		},
		{
			name:          "no unstaged changes (no modified or untracked files)",
			diffOutput:    "",
			diffError:     nil,
			lsFilesOutput: "",
			lsFilesError:  nil,
			expected:      false,
			expectErr:     false,
		},
		{
			name:          "error checking for modified files",
			diffOutput:    "fatal: not a git repository",
			diffError:     errors.New("git error"),
			lsFilesOutput: "",
			lsFilesError:  nil,
			expected:      false,
			expectErr:     true,
		},
		{
			name:          "error checking for untracked files",
			diffOutput:    "",
			diffError:     nil,
			lsFilesOutput: "",
			lsFilesError:  errors.New("git error"),
			expected:      false,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockGitClient{
				MockHasUnstagedChanges: func() (bool, error) {
					if tt.diffError != nil {
						if tt.diffOutput == "" {
							return true, nil
						}
						return false, tt.diffError
					}
					if tt.lsFilesError != nil {
						return false, tt.lsFilesError
					}
					if tt.lsFilesOutput != "" {
						return true, nil
					}
					return false, nil
				},
			}

			hasChanges, err := mockClient.HasUnstagedChanges()

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, hasChanges)
			}
		})
	}
}

func TestMockGitClient_HasUnstagedChanges(t *testing.T) {
	t.Run("using custom mock function", func(t *testing.T) {
		client := &MockGitClient{
			MockHasUnstagedChanges: func() (bool, error) {
				return true, nil
			},
		}

		hasChanges, err := client.HasUnstagedChanges()
		assert.NoError(t, err)
		assert.True(t, hasChanges)
	})

	t.Run("using default implementation", func(t *testing.T) {
		client := &MockGitClient{}

		hasChanges, err := client.HasUnstagedChanges()
		assert.NoError(t, err)
		assert.False(t, hasChanges)
	})
}

func TestExecGitClient_HasUnstagedChanges(t *testing.T) {
	tests := []struct {
		name                  string
		diffReturnsError      bool
		diffErrorIsExitCode1  bool
		lsFilesHasOutput      bool
		lsFilesReturnsError   bool
		expectedResult        bool
		expectedErrorContains string
	}{
		{
			name:                 "modified files present (diff returns exit code 1)",
			diffReturnsError:     true,
			diffErrorIsExitCode1: true,
			expectedResult:       true,
		},
		{
			name:             "no modified files, but untracked files present",
			diffReturnsError: false,
			lsFilesHasOutput: true,
			expectedResult:   true,
		},
		{
			name:             "no modified files and no untracked files",
			diffReturnsError: false,
			lsFilesHasOutput: false,
			expectedResult:   false,
		},
		{
			name:                  "error checking for modified files",
			diffReturnsError:      true,
			diffErrorIsExitCode1:  false,
			expectedResult:        false,
			expectedErrorContains: "failed to check for unstaged changes",
		},
		{
			name:                  "error checking for untracked files",
			diffReturnsError:      false,
			lsFilesReturnsError:   true,
			expectedResult:        false,
			expectedErrorContains: "failed to check for untracked files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := &MockGitClient{
				MockHasUnstagedChanges: func() (bool, error) {
					if tt.diffReturnsError {
						if tt.diffErrorIsExitCode1 {
							return true, nil
						}
						return false, fmt.Errorf("failed to check for unstaged changes: some error")
					}
					if tt.lsFilesReturnsError {
						return false, fmt.Errorf("failed to check for untracked files: some error")
					}
					if tt.lsFilesHasOutput {
						return true, nil
					}
					return false, nil
				},
			}

			result, err := mockGit.HasUnstagedChanges()

			if tt.expectedErrorContains != "" {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.expectedErrorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
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
if [ "$1" = "pr" ]; then
	echo "https://github.com/owner/repo/pull/20"
	exit 0
fi
exit 1
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client := &ExecGitClient{RepoPath: repoPath}
	url, err := client.GetPullRequestURL("refs/heads/feature/test")
	assert.NoError(t, err)
	assert.Equal(t, "https://github.com/owner/repo/pull/20", url)
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
	_, err := client.GetPullRequestURL("feature/test")
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
	echo https://github.com/owner/repo/pull/20
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
