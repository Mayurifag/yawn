package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/Mayurifag/yawn/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runGitCommand(t *testing.T, args ...string) {
	cmd := exec.Command("git", args...)
	err := cmd.Run()
	require.NoError(t, err)
}

func setupTestRepo(t *testing.T) (string, func()) {
	// Create a temporary directory for the test repository
	tmpDir, err := os.MkdirTemp("", "yawn-e2e-test-*")
	require.NoError(t, err)

	// Initialize git repository
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Initialize git repo
	runGitCommand(t, "init")

	// Configure git user
	runGitCommand(t, "config", "user.name", "Test User")
	runGitCommand(t, "config", "user.email", "test@example.com")

	// Create git client after repository is initialized
	gitClient, err := git.NewExecGitClient(true)
	require.NoError(t, err)

	// Create a test file and commit it
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)

	// Stage and commit
	err = gitClient.StageChanges()
	require.NoError(t, err)
	err = gitClient.Commit("initial commit")
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestE2ECommitMessageGeneration(t *testing.T) {
	// Skip if no API key is set
	apiKey := os.Getenv("YAWN_GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping E2E test: YAWN_GEMINI_API_KEY not set")
	}

	// Setup test repository
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create test changes
	testFile := filepath.Join(repoDir, "test.txt")
	err := os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Initialize configuration
	cfg, err := config.LoadConfig(repoDir, true, apiKey, false, false, "verbose", "api-key")
	require.NoError(t, err)

	// Create Gemini client
	client, err := gemini.NewClient(apiKey)
	require.NoError(t, err)

	// Create git client
	gitClient, err := git.NewExecGitClient(true)
	require.NoError(t, err)

	// Stage changes
	err = gitClient.StageChanges()
	require.NoError(t, err)

	// Get diff
	diff, err := gitClient.GetDiff()
	require.NoError(t, err)
	assert.Contains(t, diff, "modified content")

	// Generate commit message
	msg, err := client.GenerateCommitMessage(context.Background(), cfg.GeminiModel, cfg.Prompt, diff, cfg.MaxTokens, cfg.Temperature)
	require.NoError(t, err)
	assert.NotEmpty(t, msg)

	// Verify commit message format
	assert.Regexp(t, `^(fix|feat|docs|style|refactor|perf|test|build|ci|chore)(\([a-z]+\))?: [a-z]`, msg)

	// Create commit
	err = gitClient.Commit(msg)
	require.NoError(t, err)

	// Verify commit was created
	hash, err := gitClient.GetLastCommitHash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestE2EConfigurationOverrides(t *testing.T) {
	// Save original environment
	origEnv := make(map[string]string)
	for _, env := range os.Environ() {
		if key, value, found := strings.Cut(env, "="); found && strings.HasPrefix(key, "YAWN_") {
			origEnv[key] = value
			os.Unsetenv(key)
		}
	}
	// Restore environment after test
	defer func() {
		for key, value := range origEnv {
			os.Setenv(key, value)
		}
	}()

	// Create a temporary directory for test configs
	tmpDir, err := os.MkdirTemp("", "yawn-test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Override XDG_CONFIG_HOME to prevent loading real user config
	origXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", origXDGConfigHome)

	// Setup test repository
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create project-specific config
	projectConfig := filepath.Join(repoDir, ".yawn.toml")
	err = os.WriteFile(projectConfig, []byte(`
gemini_model = "gemini-1.5-pro"
max_tokens = 2000
request_timeout_seconds = 20
`), 0644)
	require.NoError(t, err)

	// Set environment variable
	os.Setenv("YAWN_GEMINI_MODEL", "gemini-env-model")
	defer os.Unsetenv("YAWN_GEMINI_MODEL")

	// Load configuration with flag override
	cfg, err := config.LoadConfig(repoDir, true, "test-api-key", true, true, "verbose", "api-key", "stage", "push")
	require.NoError(t, err)

	// Verify configuration precedence
	assert.Equal(t, "test-api-key", cfg.GeminiAPIKey)    // From flag
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel) // From env
	assert.Equal(t, 2000, cfg.MaxTokens)                 // From project config
	assert.Equal(t, 20, cfg.RequestTimeoutSeconds)       // From project config
	assert.True(t, cfg.AutoStage)                        // From flag
	assert.True(t, cfg.AutoPush)                         // From flag
}
