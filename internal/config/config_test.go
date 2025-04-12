package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_Defaults tests that default configuration values are loaded correctly
// when no other configuration sources are present.
func TestLoadConfig_Defaults(t *testing.T) {
	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override functions to ensure no configs are found
	getUserConfigPathFunc = func() (string, error) {
		return "", nil
	}
	findProjectConfigFunc = func(startPath string) string {
		return ""
	}

	// Call LoadConfig with parameters indicating no overrides
	cfg, err := LoadConfig("", false, "", false, false)
	require.NoError(t, err)

	// Assert default values
	assert.Equal(t, DefaultGeminiModel, cfg.GeminiModel)
	assert.Equal(t, DefaultMaxTokens, cfg.MaxTokens)
	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoStage, cfg.AutoStage)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)
	assert.Equal(t, DefaultPrompt, cfg.Prompt)
	assert.Equal(t, DefaultWaitForSSHKeys, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(DefaultTemperature), cfg.Temperature)
}

// TestLoadConfig_UserOverride tests that user configuration overrides defaults.
func TestLoadConfig_UserOverride(t *testing.T) {
	// Create a temporary directory to simulate user config
	tempUserDir := t.TempDir()

	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override getUserConfigPath to return our temp path
	userConfigPath := filepath.Join(tempUserDir, UserConfigFileName)
	getUserConfigPathFunc = func() (string, error) {
		return userConfigPath, nil
	}

	// Ensure no project config is found
	findProjectConfigFunc = func(startPath string) string {
		return ""
	}

	// Create user config file with non-default values
	userConfigContent := `
gemini_model = "gemini-user-model"
max_tokens = 500
auto_stage = true
temperature = 0.8
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Call LoadConfig with no project path or flags
	cfg, err := LoadConfig("", false, "", false, false)
	require.NoError(t, err)

	// Assert user-overridden values
	assert.Equal(t, "gemini-user-model", cfg.GeminiModel)
	assert.Equal(t, 500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.AutoStage)
	assert.Equal(t, float32(0.8), cfg.Temperature)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)
}

// TestLoadConfig_ProjectOverride tests that project configuration overrides both
// user configuration and defaults.
func TestLoadConfig_ProjectOverride(t *testing.T) {
	// Create temporary directories
	tempUserDir := t.TempDir()
	tempProjectDir := t.TempDir()

	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override getUserConfigPath to return our temp path
	userConfigPath := filepath.Join(tempUserDir, UserConfigFileName)
	getUserConfigPathFunc = func() (string, error) {
		return userConfigPath, nil
	}

	// Create user config file
	userConfigContent := `
gemini_model = "gemini-user-model"
max_tokens = 500
auto_stage = true
push_command = "git push user-origin"
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
gemini_model = "gemini-project-model"
request_timeout_seconds = 30
push_command = "git push project-origin"
wait_for_ssh_keys = true
temperature = 0.5
`
	err = os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0600)
	require.NoError(t, err)

	// Override findProjectConfig to return our temp project config path
	findProjectConfigFunc = func(startPath string) string {
		return projectConfigPath
	}

	// Call LoadConfig with project path but no flags
	cfg, err := LoadConfig(tempProjectDir, false, "", false, false)
	require.NoError(t, err)

	// Assert project-overridden values
	assert.Equal(t, "gemini-project-model", cfg.GeminiModel)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, "git push project-origin", cfg.PushCommand)
	assert.Equal(t, true, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(0.5), cfg.Temperature)

	// Assert user-overridden values (not overridden by project)
	assert.Equal(t, 500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.AutoStage)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)
}

// TestLoadConfig_EnvOverride tests that environment variables override project, user,
// and default configurations.
func TestLoadConfig_EnvOverride(t *testing.T) {
	// Create temporary directories
	tempUserDir := t.TempDir()
	tempProjectDir := t.TempDir()

	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override getUserConfigPath to return our temp path
	userConfigPath := filepath.Join(tempUserDir, UserConfigFileName)
	getUserConfigPathFunc = func() (string, error) {
		return userConfigPath, nil
	}

	// Create user config file
	userConfigContent := `
gemini_model = "gemini-user-model"
max_tokens = 500
auto_stage = true
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
gemini_model = "gemini-project-model"
request_timeout_seconds = 30
`
	err = os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0600)
	require.NoError(t, err)

	// Override findProjectConfig to return our temp project config path
	findProjectConfigFunc = func(startPath string) string {
		return projectConfigPath
	}

	// Set environment variables
	t.Setenv("YAWN_GEMINI_MODEL", "gemini-env-model")
	t.Setenv("YAWN_MAX_TOKENS", "1500")
	t.Setenv("YAWN_AUTO_PUSH", "true")
	t.Setenv("YAWN_WAIT_FOR_SSH_KEYS", "true")
	t.Setenv("YAWN_TEMPERATURE", "0.7")

	// Call LoadConfig with project path but no flags
	cfg, err := LoadConfig(tempProjectDir, false, "", false, false)
	require.NoError(t, err)

	// Assert environment-overridden values
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel)
	assert.Equal(t, 1500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(0.7), cfg.Temperature)

	// Assert project-overridden values (not overridden by env)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)

	// Assert user-overridden values (not overridden by env or project)
	assert.Equal(t, true, cfg.AutoStage)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)
}

// TestLoadConfig_FlagOverride tests that command-line flags override all other
// configuration sources.
func TestLoadConfig_FlagOverride(t *testing.T) {
	// Create temporary directories
	tempUserDir := t.TempDir()
	tempProjectDir := t.TempDir()

	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override getUserConfigPath to return our temp path
	userConfigPath := filepath.Join(tempUserDir, UserConfigFileName)
	getUserConfigPathFunc = func() (string, error) {
		return userConfigPath, nil
	}

	// Create user config file
	userConfigContent := `
gemini_model = "gemini-user-model"
max_tokens = 500
auto_stage = true
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
gemini_model = "gemini-project-model"
request_timeout_seconds = 30
`
	err = os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0600)
	require.NoError(t, err)

	// Override findProjectConfig to return our temp project config path
	findProjectConfigFunc = func(startPath string) string {
		return projectConfigPath
	}

	// Set environment variables
	t.Setenv("YAWN_GEMINI_MODEL", "gemini-env-model")
	t.Setenv("YAWN_MAX_TOKENS", "1500")
	t.Setenv("YAWN_AUTO_PUSH", "true")

	// Call LoadConfig with flags
	cfg, err := LoadConfig(
		tempProjectDir, // projectPath
		true,           // verboseFlag
		"flag-api-key", // apiKeyFlag
		false,          // autoStageFlag
		true,           // autoPushFlag
		"verbose",      // flagsSpecified...
		"api-key",
		"stage",
		"push",
	)
	require.NoError(t, err)

	// Assert flag-overridden values
	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, false, cfg.AutoStage)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.Verbose)

	// Assert environment-overridden values (not overridden by flags)
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel)
	assert.Equal(t, 1500, cfg.MaxTokens)

	// Assert project-overridden values (not overridden by flags or env)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultWaitForSSHKeys, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(DefaultTemperature), cfg.Temperature)
}

// TestLoadConfig_AllSources tests that configuration values are loaded correctly
// from all sources with proper precedence.
func TestLoadConfig_AllSources(t *testing.T) {
	// Create temporary directories
	tempUserDir := t.TempDir()
	tempProjectDir := t.TempDir()

	// Save the original functions and restore them after the test
	originalGetUserConfigPath := getUserConfigPathFunc
	originalFindProjectConfig := findProjectConfigFunc
	defer func() {
		getUserConfigPathFunc = originalGetUserConfigPath
		findProjectConfigFunc = originalFindProjectConfig
	}()

	// Override getUserConfigPath to return our temp path
	userConfigPath := filepath.Join(tempUserDir, UserConfigFileName)
	getUserConfigPathFunc = func() (string, error) {
		return userConfigPath, nil
	}

	// Create user config file
	userConfigContent := `
gemini_model = "gemini-user-model"
max_tokens = 500
auto_stage = true
push_command = "git push user-origin"
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
gemini_model = "gemini-project-model"
request_timeout_seconds = 30
push_command = "git push project-origin"
wait_for_ssh_keys = true
temperature = 0.5
`
	err = os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0600)
	require.NoError(t, err)

	// Override findProjectConfig to return our temp project config path
	findProjectConfigFunc = func(startPath string) string {
		return projectConfigPath
	}

	// Set environment variables
	t.Setenv("YAWN_GEMINI_MODEL", "gemini-env-model")
	t.Setenv("YAWN_MAX_TOKENS", "1500")
	t.Setenv("YAWN_AUTO_PUSH", "true")
	t.Setenv("YAWN_WAIT_FOR_SSH_KEYS", "true")
	t.Setenv("YAWN_TEMPERATURE", "0.7")

	// Call LoadConfig with flags
	cfg, err := LoadConfig(
		tempProjectDir, // projectPath
		true,           // verboseFlag
		"flag-api-key", // apiKeyFlag
		false,          // autoStageFlag
		true,           // autoPushFlag
		"verbose",      // flagsSpecified...
		"api-key",
		"stage",
		"push",
	)
	require.NoError(t, err)

	// Assert flag-overridden values (highest precedence)
	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, false, cfg.AutoStage)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.Verbose)

	// Assert environment-overridden values (second highest precedence)
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel)
	assert.Equal(t, 1500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(0.7), cfg.Temperature)

	// Assert project-overridden values (third highest precedence)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, "git push project-origin", cfg.PushCommand)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultPrompt, cfg.Prompt)
}

func TestGenerateConfigContent(t *testing.T) {
	content, err := GenerateConfigContent("test_api_key")
	require.NoError(t, err)

	// Verify the content contains the API key
	assert.Contains(t, string(content), "test_api_key")
}

func TestSaveAPIKeyToUserConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "yawn-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Override the getConfigDirFunc function for testing
	originalGetConfigDir := getConfigDirFunc
	defer func() {
		getConfigDirFunc = originalGetConfigDir
	}()

	// Mock getConfigDirFunc to return our temp directory
	getConfigDirFunc = func() (string, error) {
		return tempDir, nil
	}

	// Test saving API key to a new config file
	err = SaveAPIKeyToUserConfig("test_api_key")
	require.NoError(t, err)

	// Verify the file was created and contains the API key
	configPath := filepath.Join(tempDir, UserConfigFileName)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test_api_key")

	// Test updating an existing config file
	err = SaveAPIKeyToUserConfig("new_api_key")
	require.NoError(t, err)

	// Verify the file was updated with the new API key
	content, err = os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new_api_key")
}
