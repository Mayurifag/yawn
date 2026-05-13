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

	providerCfg := cfg.GetProviderConfig(ProviderGemini)
	assert.Equal(t, DefaultProvider, cfg.GetMainProvider())
	assert.Equal(t, "", cfg.GetFallbackProvider())
	assert.Equal(t, DefaultGeminiModel, providerCfg.Model)
	assert.Equal(t, DefaultTimeoutSecs, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultAutoStage, cfg.AutoStage)
	assert.Equal(t, DefaultAutoPush, cfg.AutoPush)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)
	assert.Equal(t, DefaultPrompt, cfg.Prompt)
	assert.Equal(t, DefaultWaitForSSHKeys, cfg.WaitForSSHKeys)

	assert.Equal(t, "default", cfg.sources["MainProvider"])
	assert.Equal(t, "default", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "default", cfg.sources["AutoStage"])
	assert.Equal(t, "default", cfg.sources["AutoPush"])
	assert.Equal(t, "default", cfg.sources["PushCommand"])
	assert.Equal(t, "default", cfg.sources["Prompt"])
	assert.Equal(t, "default", cfg.sources["WaitForSSHKeys"])
}

func TestConfig_ProviderDefaults(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{"gemini", ProviderGemini, DefaultGeminiModel},
		{"opencode_cli", ProviderOpenCodeCLI, DefaultOpenCodeCLIModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{MainProvider: tt.provider}
			providerCfg := cfg.GetProviderConfig(cfg.GetMainProvider())
			assert.Equal(t, NormalizeProvider(tt.provider), cfg.GetMainProvider())
			assert.Equal(t, tt.model, providerCfg.Model)
		})
	}
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

func TestLoadConfig_ProviderMapOverride(t *testing.T) {
	setupXDGConfig(t, `
main_provider = "opencode_cli"
fallback_provider = "gemini"

[providers.opencode_cli]
model = "openai/gpt-5.3-codex-spark"

[providers.gemini]
api_key = "gemini-key"
`)

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ProjectConfigName), []byte(`
[providers.opencode_cli]
model = "openai/gpt-5.5"
`), 0600))

	cfg, err := LoadConfig(projectDir, CLIFlags{})
	require.NoError(t, err)

	openCodeCLICfg := cfg.GetProviderConfig(ProviderOpenCodeCLI)
	geminiCfg := cfg.GetProviderConfig(ProviderGemini)
	assert.Equal(t, ProviderOpenCodeCLI, cfg.GetMainProvider())
	assert.Equal(t, ProviderGemini, cfg.GetFallbackProvider())
	assert.Equal(t, "openai/gpt-5.5", openCodeCLICfg.Model)
	assert.Equal(t, "gemini-key", geminiCfg.APIKey)
	assert.Equal(t, DefaultGeminiModel, geminiCfg.Model)
	assert.Equal(t, "project", cfg.sources["Providers"])
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
	t.Setenv("YAWN_MAIN_PROVIDER", "opencode_cli")
	t.Setenv("YAWN_FALLBACK_PROVIDER", "gemini")
	t.Setenv("YAWN_GEMINI_API_KEY", "env-provider-key")
	t.Setenv("YAWN_GEMINI_MODEL", "gemini-2.5-flash")
	t.Setenv("YAWN_OPENCODE_CLI_MODEL", "openai/gpt-5.3-codex-spark")

	cfg, err := LoadConfig(projectDir, CLIFlags{})
	require.NoError(t, err)

	geminiCfg := cfg.GetProviderConfig(ProviderGemini)
	openCodeCLICfg := cfg.GetProviderConfig(ProviderOpenCodeCLI)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, true, cfg.WaitForSSHKeys)
	assert.Equal(t, ProviderOpenCodeCLI, cfg.GetMainProvider())
	assert.Equal(t, ProviderGemini, cfg.GetFallbackProvider())
	assert.Equal(t, "env-provider-key", geminiCfg.APIKey)
	assert.Equal(t, "gemini-2.5-flash", geminiCfg.Model)
	assert.Equal(t, "openai/gpt-5.3-codex-spark", openCodeCLICfg.Model)

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, true, cfg.AutoStage)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	assert.Equal(t, "env", cfg.sources["AutoPush"])
	assert.Equal(t, "env", cfg.sources["WaitForSSHKeys"])
	assert.Equal(t, "env", cfg.sources["MainProvider"])
	assert.Equal(t, "env", cfg.sources["FallbackProvider"])
	assert.Equal(t, "env", cfg.sources["Providers"])
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

	assert.Equal(t, "flag-api-key", cfg.GetAPIKey())
	assert.Equal(t, true, cfg.AutoStage)
	assert.Equal(t, true, cfg.AutoPush)
	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, DefaultPushCommand, cfg.PushCommand)

	assert.Equal(t, "flag", cfg.sources["Providers"])
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

	assert.Equal(t, "flag-api-key", cfg.GetAPIKey())
	assert.Equal(t, false, cfg.AutoStage)
	assert.Equal(t, false, cfg.AutoPush)
	assert.Equal(t, "flag", cfg.sources["Providers"])
	assert.Equal(t, "flag", cfg.sources["AutoStage"])
	assert.Equal(t, "flag", cfg.sources["AutoPush"])

	assert.Equal(t, 30, cfg.RequestTimeoutSeconds)
	assert.Equal(t, "git push project-origin", cfg.PushCommand)
	assert.Equal(t, "project", cfg.sources["RequestTimeoutSeconds"])
	assert.Equal(t, "project", cfg.sources["PushCommand"])

	assert.Equal(t, DefaultPrompt, cfg.Prompt)
	assert.Equal(t, "default", cfg.sources["Prompt"])
}
