package app

import (
	"context"
	"errors"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCleanRepositoryDoesNotNeedAPIKey(t *testing.T) {
	a := NewApp(config.Config{}, &git.MockGitClient{MockHasRemotes: func() (bool, error) { return false, nil }})
	require.NoError(t, a.Run(context.Background()))
}

func TestHandleUnpushedCommitsWithoutUpstream(t *testing.T) {
	checked := false
	pushed := false
	a := NewApp(config.Config{AutoPush: true, PushCommand: config.DefaultPushCommand}, &git.MockGitClient{
		MockPreparePush: func(command string) (*git.PushPlan, error) {
			checked = true
			assert.Equal(t, config.DefaultPushCommand, command)
			return &git.PushPlan{Target: git.PushTarget{Remote: "origin", Branch: "feature"}, LocalCommits: []string{"local commit"}}, nil
		},
		MockPush: func(plan *git.PushPlan, force bool) (string, error) {
			pushed = true
			assert.Equal(t, "feature", plan.Target.Branch)
			assert.False(t, force)
			return "", nil
		},
	})

	require.NoError(t, a.handleUnpushedCommits())
	assert.True(t, checked, "unpublished branches must be inspected")
	assert.True(t, pushed)
}

func TestRejectedForcePushDoesNotPromptAgain(t *testing.T) {
	calls := 0
	a := NewApp(config.Config{}, &git.MockGitClient{
		MockPush: func(*git.PushPlan, bool) (string, error) {
			calls++
			return "", &git.GitError{Output: "rejected (non-fast-forward)"}
		},
	})

	err := a.doPush(&git.PushPlan{}, true)

	require.ErrorContains(t, err, "non-fast-forward")
	assert.Equal(t, 1, calls)
}

func TestPreviewFailurePreventsPush(t *testing.T) {
	failure := errors.New("destination cannot be read")
	a := NewApp(config.Config{AutoPush: true}, &git.MockGitClient{
		MockPreparePush: func(string) (*git.PushPlan, error) { return nil, failure },
		MockPush: func(*git.PushPlan, bool) (string, error) {
			t.Fatal("must not push without a successful preview")
			return "", nil
		},
	})
	require.ErrorIs(t, a.handlePushOperation(), failure)
}

func TestEnsureStagedChanges(t *testing.T) {
	tests := []struct {
		name        string
		autoStage   bool
		hasStaged   bool
		hasUnstaged bool
		expectStage bool
		expectErr   bool
	}{
		{"auto_stage + staged + unstaged stages all", true, true, true, true, false},
		{"auto_stage + no staged + unstaged stages all", true, false, true, true, false},
		{"no auto_stage + staged + unstaged skips staging", false, true, true, false, false},
		{"no auto_stage + staged + no unstaged proceeds", false, true, false, false, false},
		{"no changes returns error", false, false, false, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			staged := false
			mockGit := &git.MockGitClient{
				MockHasStagedChanges:   func() (bool, error) { return tc.hasStaged, nil },
				MockHasUnstagedChanges: func() (bool, error) { return tc.hasUnstaged, nil },
				MockStageChanges: func() error {
					staged = true
					return nil
				},
			}
			a := &App{Config: config.Config{AutoStage: tc.autoStage}, GitClient: mockGit}

			err := a.ensureStagedChanges()

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectStage, staged)
			}
		})
	}
}
