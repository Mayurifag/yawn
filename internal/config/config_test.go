package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupXDGConfig(t *testing.T, content string) {
	t.Helper()
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	if content == "" {
		return
	}
	userConfigPath := filepath.Join(xdgDir, UserConfigDirName, UserConfigFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(userConfigPath), 0700))
	require.NoError(t, os.WriteFile(userConfigPath, []byte(content), 0600))
}

func TestLoadConfig_Defaults(t *testing.T) {
	setupXDGConfig(t, "")
	cfg, err := LoadConfig(t.TempDir(), CLIFlags{})
	require.NoError(t, err)

	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoStage, cfg.AutoStage)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultPrompt, cfg.Prompt)
	assert.Equal(t, DefaultWaitForSSHKeys, cfg.WaitForSSHKeys)

	assert.Equal(t, "default", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["Prompt"])
	assert.Equal(t, "default", cfg.sources["WaitForSSHKeys"])
}

func TestLoadConfig_UserOverride(t *testing.T) {
	setupXDGConfig(t, `
auto_stage = true
`)
	cfg, err := LoadConfig(t.TempDir(), CLIFlags{})
	require.NoError(t, err)

	assert.Equal(t, true, cfg.AutoStage)

	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
}

func TestLoadConfig_ProjectOverride(t *testing.T) {
	setupXDGConfig(t, `
auto_stage = true
push_command = "git push user-origin"
`)

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
request_timeout_seconds = 30
push_command = "git push project-origin"
wait_for_ssh_keys = true
`), 0600))

	cfg, err := LoadConfig(projectDir, CLIFlags{})
	require.NoError(t, err)

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, "git push project-origin", cfg.PushCommand)
	assert.Equal(t, true, cfg.WaitForSSHKeys)

	assert.Equal(t, true, cfg.AutoStage)

	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)

	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "project", cfg.sources["PushCommand"])
	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	setupXDGConfig(t, `
auto_stage = true
`)

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
request_timeout_seconds = 30
`), 0600))

	t.Setenv("YAWN_AUTO_PUSH", "true")
	t.Setenv("YAWN_WAIT_FOR_SSH_KEYS", "true")

	cfg, err := LoadConfig(projectDir, CLIFlags{})
	require.NoError(t, err)

	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.WaitForSSHKeys)

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, true, cfg.AutoStage)

	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	assert.Equal(t, "env", cfg.sources["AutoPush"])
	assert.Equal(t, "env", cfg.sources["WaitForSSHKeys"])
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "user home config", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
}

func TestLoadConfig_FlagOverride(t *testing.T) {
	setupXDGConfig(t, `
auto_stage = false
`)

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
request_timeout_seconds = 30
auto_stage = true
`), 0600))

	t.Setenv("YAWN_GEMINI_API_KEY", "env-api-key")
	t.Setenv("YAWN_AUTO_PUSH", "false")

	apiKey, autoStage, autoPush := "flag-api-key", true, true
	cfg, err := LoadConfig(projectDir, CLIFlags{APIKey: &apiKey, AutoStage: &autoStage, AutoPush: &autoPush})
	require.NoError(t, err)

	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, true, cfg.AutoStage)
	assert.Equal(t, true, cfg.AutoPush)

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)

	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	assert.Equal(t, "flag", cfg.sources["GeminiAPIKey"])
	assert.Equal(t, "flag", cfg.sources["AutoStage"])
	assert.Equal(t, "flag", cfg.sources["AutoPush"])
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
}

func TestLoadConfig_AllSources(t *testing.T) {
	setupXDGConfig(t, `
request_timeout_seconds = 15
auto_stage = true
push_command = "git push user-origin"
`)

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
request_timeout_seconds = 30
push_command = "git push project-origin"
`), 0600))

	t.Setenv("YAWN_AUTO_PUSH", "true")

	apiKey, autoStage, autoPush := "flag-api-key", false, false
	cfg, err := LoadConfig(projectDir, CLIFlags{APIKey: &apiKey, AutoStage: &autoStage, AutoPush: &autoPush})
	require.NoError(t, err)

	assert.Equal(t, "flag-api-key", cfg.GeminiAPIKey)
	assert.Equal(t, false, cfg.AutoStage)
	assert.Equal(t, false, cfg.AutoPush)
	assert.Equal(t, "flag", cfg.sources["GeminiAPIKey"])
	assert.Equal(t, "flag", cfg.sources["AutoStage"])
	assert.Equal(t, "flag", cfg.sources["AutoPush"])

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, "git push project-origin", cfg.PushCommand)
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "project", cfg.sources["PushCommand"])

	assert.Equal(t, DefaultPrompt, cfg.Prompt)
	assert.Equal(t, "default", cfg.sources["Prompt"])
}
