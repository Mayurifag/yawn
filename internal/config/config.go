package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	koanftoml "github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Constants for configuration
const (
	AppName               = "yawn"
	ProjectConfigName     = ".yawn.toml"
	UserConfigDirName     = "yawn"
	UserConfigFileName    = "config.toml"
	EnvPrefix             = "YAWN_"
	DefaultGeminiModel    = "gemini-1.5-flash"
	DefaultMaxTokens      = 1000000
	DefaultTimeoutSecs    = 10
	DefaultAutoStage      = false
	DefaultAutoPush       = false
	DefaultPushCommand    = "git push origin HEAD"
	DefaultVerbose        = false
	DefaultWaitForSSHKeys = false
	DefaultTemperature    = 0.1
)

// Default prompt for commit message generation
const DefaultPrompt = `Generate a commit message.

- ALWAYS follow Conventional Commits specification (https://www.conventionalcommits.org/en/v1.0.0/)
- Description, type and scope must start with a lowercase letter
- Use only these types: fix, feat, docs, style, refactor, perf, test, build, ci, chore
- Scope should be a noun describing a section of the codebase (e.g., api, core, ui, auth)
- Write a precise description capturing the primary intent of the changes, explaining WHY they were made. Keep it under 50 characters, focusing on ONE main change, even if changes are unrelated. Use specific nouns and verbs relevant to the diff.
- Prefer terminology used in the diff or context for consistency.
- Body starts with a brief paragraph (1-2 sentences) explaining WHY and WHAT was done, providing context for the changes. Follow with a blank line, then list all changes as bullet points (one per -), starting with a capital letter. Each bullet should describe a specific change and, where relevant, include a brief reason (e.g., "to improve X" or "for better Y").
- Ensure the body's introductory text expands on, but does not repeat, the description line. Provide unique context or details about WHY and WHAT was done.
- Use filenames in body or description if relevant, treating them as plain text without formatting.
- Never use gitmoji
- Only output the commit message TEXT, which does NOT contain backticks, quotes, or other formatting symbols. No commentaries before or after the message.

Structure of output:
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]

Here are example outputs (until ---):
refactor(interactors): simplify strategies generation

Simplified the strategy generation process to improve maintainability and readability by using a single orchestrator.

- Replaced StrategyGeneratorInteractor with StrategyGenerationOrchestrator to centralize logic.
- Removed MultiprocessingStrategyGenerator to reduce complexity.
- Created ParallelBacktestExecutor for efficient backtesting.
- Added ResultsProcessor to handle result storage.
---
feat!: allow provided config object to extend other configs

BREAKING CHANGE: 'extends' key in config file is now used for extending other config files
---

Here is the diff to analyze:

{{Diff}}`

// Config holds the application configuration. Fields must be exported for TOML decoding.
type Config struct {
	GeminiAPIKey          string  `koanf:"gemini_api_key" toml:"gemini_api_key"`
	GeminiModel           string  `koanf:"gemini_model" toml:"gemini_model"`
	MaxTokens             int     `koanf:"max_tokens" toml:"max_tokens"`
	RequestTimeoutSeconds int     `koanf:"request_timeout_seconds" toml:"request_timeout_seconds"`
	Prompt                string  `koanf:"prompt" toml:"prompt,multiline"`
	AutoStage             bool    `koanf:"auto_stage" toml:"auto_stage"`
	AutoPush              bool    `koanf:"auto_push" toml:"auto_push"`
	PushCommand           string  `koanf:"push_command" toml:"push_command"`
	Verbose               bool    `koanf:"verbose" toml:"verbose"`
	WaitForSSHKeys        bool    `koanf:"wait_for_ssh_keys" toml:"wait_for_ssh_keys"`
	Temperature           float32 `koanf:"temperature" toml:"temperature"`
}

// setDefaults sets all default configuration values
func setDefaults(k *koanf.Koanf) error {
	defaults := map[string]interface{}{
		"gemini_model":            DefaultGeminiModel,
		"max_tokens":              DefaultMaxTokens,
		"request_timeout_seconds": DefaultTimeoutSecs,
		"prompt":                  DefaultPrompt,
		"auto_stage":              DefaultAutoStage,
		"auto_push":               DefaultAutoPush,
		"push_command":            DefaultPushCommand,
		"verbose":                 DefaultVerbose,
		"wait_for_ssh_keys":       DefaultWaitForSSHKeys,
		"temperature":             DefaultTemperature,
	}

	for key, value := range defaults {
		if err := k.Set(key, value); err != nil {
			return fmt.Errorf("failed to set default %s: %w", key, err)
		}
	}
	return nil
}

// applyFlags applies command line flags to the configuration
func applyFlags(k *koanf.Koanf, specified map[string]bool, verboseFlag bool, apiKeyFlag string, autoStageFlag bool, autoPushFlag bool) error {
	if specified["verbose"] {
		if err := k.Set("verbose", verboseFlag); err != nil {
			return fmt.Errorf("failed to set verbose flag: %w", err)
		}
	}
	if specified["api-key"] && apiKeyFlag != "" {
		if err := k.Set("gemini_api_key", apiKeyFlag); err != nil {
			return fmt.Errorf("failed to set API key: %w", err)
		}
	}
	if specified["stage"] {
		if err := k.Set("auto_stage", autoStageFlag); err != nil {
			return fmt.Errorf("failed to set auto stage flag: %w", err)
		}
	}
	if specified["push"] {
		if err := k.Set("auto_push", autoPushFlag); err != nil {
			return fmt.Errorf("failed to set auto push flag: %w", err)
		}
	}
	return nil
}

// loadUserConfig loads the user configuration from ~/.config/yawn/config.toml
func loadUserConfig(k *koanf.Koanf) error {
	userConfigPath, err := getUserConfigPathFunc()
	if err != nil {
		return fmt.Errorf("failed to get user config path: %w", err)
	}

	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		return nil // User config does not exist, skip
	}

	if err := k.Load(file.Provider(userConfigPath), koanftoml.Parser()); err != nil {
		return fmt.Errorf("failed to load user config: %w", err)
	}

	return nil
}

// loadProjectConfig loads the project configuration from .yawn.toml
func loadProjectConfig(k *koanf.Koanf, projectPath string) error {
	if projectPath == "" {
		return nil // No project path provided, skip
	}

	projectConfigPath := findProjectConfigFunc(projectPath)
	if projectConfigPath == "" {
		return nil // Project config not found, skip
	}

	if err := k.Load(file.Provider(projectConfigPath), koanftoml.Parser()); err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	return nil
}

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv(k *koanf.Koanf) error {
	// Extract environment variables that start with YAWN_
	for _, env := range os.Environ() {
		// Check if the environment variable starts with YAWN_
		if len(env) > len(EnvPrefix) && env[:len(EnvPrefix)] == EnvPrefix {
			// Extract the key and value (format: KEY=VALUE)
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue // Invalid format, skip
			}

			// Convert from env format to koanf format (lowercase with underscores)
			key := strings.ToLower(strings.TrimPrefix(parts[0], EnvPrefix))
			key = strings.ReplaceAll(key, "_", "_")
			value := parts[1]

			// Set the value in koanf
			// For simplicity, we'll try to convert to appropriate types
			// In a real implementation, this would be more robust
			if value == "true" || value == "false" {
				// Boolean
				boolValue := value == "true"
				if err := k.Set(key, boolValue); err != nil {
					return fmt.Errorf("failed to set %s from env: %w", key, err)
				}
			} else if intValue, err := strconv.Atoi(value); err == nil {
				// Integer
				if err := k.Set(key, intValue); err != nil {
					return fmt.Errorf("failed to set %s from env: %w", key, err)
				}
			} else if floatValue, err := strconv.ParseFloat(value, 32); err == nil {
				// Float
				if err := k.Set(key, float32(floatValue)); err != nil {
					return fmt.Errorf("failed to set %s from env: %w", key, err)
				}
			} else {
				// String
				if err := k.Set(key, value); err != nil {
					return fmt.Errorf("failed to set %s from env: %w", key, err)
				}
			}
		}
	}

	return nil
}

// LoadConfig loads the configuration from various sources
func LoadConfig(
	projectPath string,
	verboseFlag bool,
	apiKeyFlag string,
	autoStageFlag bool,
	autoPushFlag bool,
	flagsSpecified ...string, // Names of flags that were explicitly specified
) (Config, error) {
	k := koanf.New(".")

	// Set defaults
	if err := setDefaults(k); err != nil {
		return Config{}, err
	}

	// Load user config
	if err := loadUserConfig(k); err != nil {
		return Config{}, fmt.Errorf("error loading user config: %w", err)
	}

	// Load project config
	if err := loadProjectConfig(k, projectPath); err != nil {
		return Config{}, fmt.Errorf("error loading project config: %w", err)
	}

	// Load from environment variables
	if err := loadConfigFromEnv(k); err != nil {
		return Config{}, fmt.Errorf("error loading from environment variables: %w", err)
	}

	// Load flags
	specified := make(map[string]bool)
	for _, flag := range flagsSpecified {
		specified[flag] = true
	}

	if err := applyFlags(k, specified, verboseFlag, apiKeyFlag, autoStageFlag, autoPushFlag); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// For testing purposes
var (
	getUserConfigPathFunc = getUserConfigPath
	findProjectConfigFunc = findProjectConfig
	getConfigDirFunc      = getConfigDir
)

// getConfigDir returns the configuration directory path
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "yawn"), nil
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	return getConfigDirFunc()
}

// GetRequestTimeout returns the request timeout duration
func (c Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// GetConfigSource returns the source of a configuration option
func (c Config) GetConfigSource(option string) string {
	return ""
}

// GenerateConfigContent generates the default configuration content with the provided API key
func GenerateConfigContent(apiKey string) ([]byte, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(Config{
		GeminiAPIKey:          apiKey,
		GeminiModel:           DefaultGeminiModel,
		MaxTokens:             DefaultMaxTokens,
		RequestTimeoutSeconds: DefaultTimeoutSecs,
		Prompt:                DefaultPrompt,
		AutoStage:             DefaultAutoStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		Verbose:               DefaultVerbose,
		WaitForSSHKeys:        DefaultWaitForSSHKeys,
		Temperature:           DefaultTemperature,
	}); err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateDefaultConfig generates the default configuration content
func GenerateDefaultConfig() (string, error) {
	content, err := GenerateConfigContent("")
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SaveAPIKeyToUserConfig saves the API key to the user's config file
func SaveAPIKeyToUserConfig(apiKey string) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(configDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, UserConfigFileName)
	content, err := GenerateConfigContent(apiKey)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getUserConfigPath returns the path to the user's config file
func getUserConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, UserConfigFileName), nil
}

// findProjectConfig searches for .yawn.toml starting from startPath and going up
func findProjectConfig(startPath string) string {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}
