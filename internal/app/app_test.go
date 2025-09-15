package app

import (
	"context"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/stretchr/testify/assert"
)

// TestWaitForSSHKeysConfig tests that the WaitForSSHKeys configuration is properly integrated
func TestWaitForSSHKeysConfig(t *testing.T) {
	// Create a test config with WaitForSSHKeys enabled
	cfg := config.Config{
		WaitForSSHKeys: true,
	}

	// Basic validation that the config field exists and is accessible
	if !cfg.WaitForSSHKeys {
		t.Error("WaitForSSHKeys should be true when set")
	}

	// Verify the config structure has the expected field with default value
	emptyCfg := config.Config{}
	if emptyCfg.WaitForSSHKeys != config.DefaultWaitForSSHKeys {
		t.Errorf("Default WaitForSSHKeys should be %v, got %v",
			config.DefaultWaitForSSHKeys, emptyCfg.WaitForSSHKeys)
	}
}

// TestGenerateAndCommitChanges tests that the generateAndCommitChanges function
// properly calls token counting, branch name retrieval, and diff stat gathering
func TestGenerateAndCommitChanges(t *testing.T) {
	// Skip this test until we can properly mock the gemini.NewClient function
	t.Skip("Skipping test that requires mocking package-level functions")

	// Create minimal test configuration
	cfg := config.Config{
		GeminiAPIKey: "test-api-key",
		MaxTokens:    1000,
		Temperature:  0.1,
		Prompt:       "Generate commit message for this diff: !YAWNDIFFPLACEHOLDER!",
	}

	// Create mock git client
	mockGit := &git.MockGitClient{
		MockGetDiff: func() (string, error) {
			return "test diff content", nil
		},
		MockGetCurrentBranch: func() (string, error) {
			return "main", nil
		},
		MockGetDiffNumStatSummary: func() (int, int, error) {
			return 42, 10, nil
		},
		MockCommit: func(message string) error {
			// Verify the message is not empty
			assert.NotEmpty(t, message)
			return nil
		},
	}

	// Create the app with our mocks
	app := &App{
		Config:    cfg,
		GitClient: mockGit,
	}

	// Call the function being tested
	err := app.generateAndCommitChanges(context.Background())

	// Verify no error occurred
	assert.NoError(t, err)
}
