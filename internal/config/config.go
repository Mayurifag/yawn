package config

import (
	"time"
)

const (
	AppName               = "yawn"
	ProjectConfigName     = ".yawn.toml"
	UserConfigDirName     = "yawn"
	UserConfigFileName    = "config.toml"
	EnvPrefix             = "YAWN_"
	DefaultGeminiModel    = "gemini-flash-latest"
	DefaultTimeoutSecs    = 15
	DefaultAutoStage      = false
	DefaultAutoPush       = false
	DefaultPushCommand    = "git push origin HEAD"
	DefaultWaitForSSHKeys = false
	DefaultSquashAutoPush = false
)

type CLIFlags struct {
	APIKey    *string
	AutoStage *bool
	AutoPush  *bool
}

type Config struct {
	GeminiAPIKey          string `toml:"gemini_api_key"`
	GeminiModel           string `toml:"gemini_model"`
	RequestTimeoutSeconds int    `toml:"request_timeout_seconds"`
	Prompt                string `toml:"prompt,multiline"`
	AutoStage             bool   `toml:"auto_stage"`
	AutoPush              bool   `toml:"auto_push"`
	PushCommand           string `toml:"push_command"`
	WaitForSSHKeys        bool   `toml:"wait_for_ssh_keys"`
	SquashAutoPush        bool   `toml:"squash_auto_push"`

	sources map[string]string `toml:"-"`
}

func defaultConfig() Config {
	return Config{
		GeminiModel:           DefaultGeminiModel,
		RequestTimeoutSeconds: DefaultTimeoutSecs,
		Prompt:                DefaultPrompt,
		AutoStage:             DefaultAutoStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		WaitForSSHKeys:        DefaultWaitForSSHKeys,
		SquashAutoPush:        DefaultSquashAutoPush,
	}
}

func (c Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

func (c Config) GetConfigSource(option string) string {
	if source, ok := c.sources[option]; ok {
		return source
	}
	return "unknown"
}
