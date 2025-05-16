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
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// Verify sources map
	assert.Equal(t, "default", cfg.sources["GeminiModel"])
	assert.Equal(t, "default", cfg.sources["MaxTokens"])
	assert.Equal(t, "default", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["Verbose"])
	assert.Equal(t, "default", cfg.sources["Prompt"])
	assert.Equal(t, "default", cfg.sources["WaitForSSHKeys"])
	assert.Equal(t, "default", cfg.sources["Temperature"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
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
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)

	// Verify sources map
	assert.Equal(t, "user home config", cfg.sources["GeminiModel"])
	assert.Equal(t, "user home config", cfg.sources["MaxTokens"])
	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "user home config", cfg.sources["Temperature"])
	assert.Equal(t, "default", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["Verbose"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
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
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// Assert user-overridden values (not overridden by project)
	assert.Equal(t, 500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.AutoStage)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)

	// Verify sources map
	assert.Equal(t, "project", cfg.sources["GeminiModel"])
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "project", cfg.sources["PushCommand"])
	assert.Equal(t, "project", cfg.sources["Temperature"])
	assert.Equal(t, "user home config", cfg.sources["MaxTokens"])
	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["Verbose"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
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

	// Assert env-overridden values
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel)
	assert.Equal(t, 1500, cfg.MaxTokens)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.WaitForSSHKeys)
	assert.Equal(t, float32(0.7), cfg.Temperature)
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// Assert project-overridden values (not overridden by env)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)

	// Assert user-overridden values (not overridden by project or env)
	assert.Equal(t, true, cfg.AutoStage)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultVerbose, cfg.Verbose)

	// Verify sources map
	assert.Equal(t, "env", cfg.sources["GeminiModel"])
	assert.Equal(t, "env", cfg.sources["MaxTokens"])
	assert.Equal(t, "env", cfg.sources["AutoPush"])
	assert.Equal(t, "env", cfg.sources["WaitForSSHKeys"])
	assert.Equal(t, "env", cfg.sources["Temperature"])
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["Verbose"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
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
auto_stage = false
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
gemini_model = "gemini-project-model"
request_timeout_seconds = 30
auto_stage = true
`
	err = os.WriteFile(projectConfigPath, []byte(projectConfigContent), 0600)
	require.NoError(t, err)

	// Override findProjectConfig to return our temp project config path
	findProjectConfigFunc = func(startPath string) string {
		return projectConfigPath
	}

	// Set environment variables
	t.Setenv("YAWN_GEMINI_API_KEY", "env-api-key")
	t.Setenv("YAWN_GEMINI_MODEL", "gemini-env-model")
	t.Setenv("YAWN_AUTO_PUSH", "false")

	// Call LoadConfig with project path and flags
	cfg, err := LoadConfig(tempProjectDir, true, "flag-api-key", true, true, "verbose", "api-key", "stage", "push")
	require.NoError(t, err)

	// Assert flag-overridden values
	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, true, cfg.Verbose)
	assert.Equal(t, true, cfg.AutoStage)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// Assert env-overridden values (not overridden by flags)
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel)

	// Assert project-overridden values (not overridden by env or flags)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)

	// Assert user-overridden values (not overridden by project, env, or flags)
	assert.Equal(t, 500, cfg.MaxTokens)

	// Assert defaults for non-overridden values
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	// Verify sources map
	assert.Equal(t, "flag", cfg.sources["GeminiAPIKey"])
	assert.Equal(t, "flag", cfg.sources["Verbose"])
	assert.Equal(t, "flag", cfg.sources["AutoStage"])
	assert.Equal(t, "flag", cfg.sources["AutoPush"])
	assert.Equal(t, "env", cfg.sources["GeminiModel"])
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "user home config", cfg.sources["MaxTokens"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
}

// TestLoadConfig_AllSources tests the correct precedence across all configuration sources:
// Defaults < User Config < Project Config < Environment Variables < Command-line Flags
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
# This is a test user config file
gemini_model = "gemini-user-model"
max_tokens = 500
request_timeout_seconds = 15
auto_stage = true
push_command = "git push user-origin"
verbose = true
`
	err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0600)
	require.NoError(t, err)

	// Create project config file
	projectConfigPath := filepath.Join(tempProjectDir, ProjectConfigName)
	projectConfigContent := `
# This is a test project config file
gemini_model = "gemini-project-model"
max_tokens = 700
request_timeout_seconds = 30
push_command = "git push project-origin"
verbose = false
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

	// Call LoadConfig with explicit flag values
	cfg, err := LoadConfig(tempProjectDir, true, "flag-api-key", false, false, "verbose", "api-key", "stage", "push")
	require.NoError(t, err)

	// Test the full precedence chain:

	// 1. Flag values should override everything
	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, true, cfg.Verbose)    // Overrides project's false
	assert.Equal(t, false, cfg.AutoStage) // Explicitly set to false, overrides user's true
	assert.Equal(t, false, cfg.AutoPush)  // Explicitly set to false, overrides env's true
	assert.Equal(t, "flag", cfg.sources["GeminiAPIKey"])
	assert.Equal(t, "flag", cfg.sources["Verbose"])
	assert.Equal(t, "flag", cfg.sources["AutoStage"])
	assert.Equal(t, "flag", cfg.sources["AutoPush"])
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// 2. Environment values should override project and user configs
	assert.Equal(t, "gemini-env-model", cfg.GeminiModel) // Overrides project's value
	assert.Equal(t, 1500, cfg.MaxTokens)                 // Overrides project's value
	assert.Equal(t, "env", cfg.sources["GeminiModel"])
	assert.Equal(t, "env", cfg.sources["MaxTokens"])
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)

	// 3. Project config values should override user config
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)              // Overrides user's value
	assert.Equal(t, "git push project-origin", cfg.PushCommand) // Overrides user's value
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "project", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])

	// 4. User config values should override defaults
	// No values exclusively from user config in this test

	// 5. Defaults should be used for values not specified in any other source
	assert.Equal(t, DefaultPrompt, cfg.Prompt) // Not specified in any config
	assert.Equal(t, "default", cfg.sources["Prompt"])
	assert.Equal(t, DefaultFallbackGeminiModel, cfg.FallbackGeminiModel)
	assert.Equal(t, "default", cfg.sources["FallbackGeminiModel"])
}
