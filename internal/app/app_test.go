package app

import (
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForSSHKeysConfig(t *testing.T) {
	cfg := config.Config{WaitForSSHKeys: true}
	if !cfg.WaitForSSHKeys {
		t.Error("WaitForSSHKeys should be true when set")
	}

	emptyCfg := config.Config{}
	if emptyCfg.WaitForSSHKeys != config.DefaultWaitForSSHKeys {
		t.Errorf("Default WaitForSSHKeys should be %v, got %v", config.DefaultWaitForSSHKeys, emptyCfg.WaitForSSHKeys)
	}
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
