package config

import (
	"strings"
	"time"
)

const (
	AppName                 = "yawn"
	ProjectConfigName       = ".yawn.toml"
	UserConfigDirName       = "yawn"
	UserConfigFileName      = "config.toml"
	EnvPrefix               = "YAWN_"
	ProviderGemini          = "gemini"
	ProviderOpenCodeCLI     = "opencode_cli"
	DefaultProvider         = ProviderGemini
	DefaultGeminiModel      = "gemini-flash-latest"
	DefaultOpenCodeCLIModel = "openai/gpt-5.3-codex-spark"
	DefaultTimeoutSecs      = 15
	DefaultAutoStage        = false
	DefaultAutoPush         = false
	DefaultPushCommand      = "git push origin HEAD"
	DefaultWaitForSSHKeys   = false
	DefaultSquashAutoPush   = false
)

type CLIFlags struct {
	APIKey    *string
	AutoStage *bool
	AutoPush  *bool
}

type ProviderConfig struct {
	APIKey string `toml:"api_key"`
	Model  string `toml:"model"`
}

type Config struct {
	MainProvider          string                    `toml:"main_provider"`
	FallbackProvider      string                    `toml:"fallback_provider"`
	Providers             map[string]ProviderConfig `toml:"providers"`
	RequestTimeoutSeconds int                       `toml:"request_timeout_seconds"`
	Prompt                string                    `toml:"prompt,multiline"`
	AutoStage             bool                      `toml:"auto_stage"`
	AutoPush              bool                      `toml:"auto_push"`
	PushCommand           string                    `toml:"push_command"`
	WaitForSSHKeys        bool                      `toml:"wait_for_ssh_keys"`
	SquashAutoPush        bool                      `toml:"squash_auto_push"`

	sources map[string]string `toml:"-"`
}

func defaultConfig() Config {
	return Config{
		MainProvider:          DefaultProvider,
		Providers:             map[string]ProviderConfig{},
		RequestTimeoutSeconds: DefaultTimeoutSecs,
		Prompt:                DefaultPrompt,
		AutoStage:             DefaultAutoStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		WaitForSSHKeys:        DefaultWaitForSSHKeys,
		SquashAutoPush:        DefaultSquashAutoPush,
	}
}

func NormalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return strings.ReplaceAll(provider, "-", "_")
}

func (c Config) GetMainProvider() string {
	provider := NormalizeProvider(c.MainProvider)
	if provider == "" {
		return DefaultProvider
	}
	return provider
}

func (c Config) GetFallbackProvider() string {
	return NormalizeProvider(c.FallbackProvider)
}

func (c *Config) SetProviderConfig(provider string, providerCfg ProviderConfig) {
	provider = NormalizeProvider(provider)
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
	c.Providers[provider] = providerCfg
}

func (c Config) GetProviderConfig(provider string) ProviderConfig {
	provider = NormalizeProvider(provider)
	providerCfg := defaultProviderConfig(provider)
	loadedProviderCfg := c.Providers[provider]
	if loadedProviderCfg.APIKey != "" {
		providerCfg.APIKey = loadedProviderCfg.APIKey
	}
	if loadedProviderCfg.Model != "" {
		providerCfg.Model = loadedProviderCfg.Model
	}
	if providerCfg.Model == "" {
		providerCfg.Model = defaultProviderModel(provider)
	}
	return providerCfg
}

func (c Config) GetAPIKey() string {
	return c.GetProviderConfig(c.GetMainProvider()).APIKey
}

func (c *Config) SetAPIKey(apiKey string) {
	provider := c.GetMainProvider()
	providerCfg := c.Providers[provider]
	providerCfg.APIKey = apiKey
	c.SetProviderConfig(provider, providerCfg)
}

func (c Config) GetModel() string {
	return c.GetProviderConfig(c.GetMainProvider()).Model
}

func defaultProviderConfig(provider string) ProviderConfig {
	switch provider {
	case ProviderGemini:
		return ProviderConfig{Model: DefaultGeminiModel}
	case ProviderOpenCodeCLI:
		return ProviderConfig{Model: DefaultOpenCodeCLIModel}
	default:
		return ProviderConfig{}
	}
}

func defaultProviderModel(provider string) string {
	switch provider {
	case ProviderGemini:
		return DefaultGeminiModel
	case ProviderOpenCodeCLI:
		return DefaultOpenCodeCLIModel
	default:
		return ""
	}
}

func ProviderRequiresAPIKey(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderGemini:
		return true
	default:
		return false
	}
}

func (c Config) GetModelLabel() string {
	label := c.GetMainProvider() + "/" + c.GetModel()
	if fallbackProvider := c.GetFallbackProvider(); fallbackProvider != "" {
		fallbackCfg := c.GetProviderConfig(fallbackProvider)
		label += " -> " + fallbackProvider + "/" + fallbackCfg.Model
	}
	return label
}

func ProviderDisplayName(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderGemini:
		return "Google Gemini"
	case ProviderOpenCodeCLI:
		return "OpenCode CLI"
	default:
		return NormalizeProvider(provider)
	}
}

func ProviderAPIKeyHelp(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderGemini:
		return "Get one from: https://makersuite.google.com/app/apikey"
	case ProviderOpenCodeCLI:
		return "Run: opencode providers login"
	default:
		return "Set api_key in your yawn config"
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
