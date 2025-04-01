package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	AppName            = "yawn"
	ProjectConfigName  = ".yawn.toml"
	UserConfigDirName  = "yawn"
	UserConfigFileName = "config.toml"
	EnvPrefix          = "YAWN_"
	DefaultGeminiModel = "gemini-2.0-flash-lite"
	DefaultMaxTokens   = 800000
	DefaultTimeoutSecs = 10
	DefaultAskStage    = true
	DefaultAutoPush    = false
	DefaultPushCommand = "git push origin HEAD"
	DefaultVerbose     = false
	DefaultPrompt      = `Analyze the following Git diff and generate a concise, conventional commit message.
The message should follow the standard format: <type>(<scope>): <subject>

<body>

<footer>.
Determine the appropriate type (feat, fix, chore, refactor, style, test, docs, build, ci, perf).
The scope is optional. The subject should be imperative, present tense, and start with a lowercase letter.
The body should explain the 'what' and 'why' of the change.

Diff:
'''diff
{{Diff}}
'''

Commit Message:`
)

// Config holds the application configuration. Fields must be exported for TOML decoding.
type Config struct {
	GeminiAPIKey          string   `toml:"gemini_api_key"`
	GeminiModel           string   `toml:"gemini_model"`
	MaxTokens             int      `toml:"max_tokens"`
	RequestTimeoutSeconds int      `toml:"request_timeout_seconds"`
	Prompt                string   `toml:"prompt,multiline"`
	IgnorePatterns        []string `toml:"ignore_patterns"`
	AskStage              bool     `toml:"ask_stage"`
	AutoPush              bool     `toml:"auto_push"`
	PushCommand           string   `toml:"push_command"`
	Verbose               bool     `toml:"verbose"`

	// Internal fields to track config sources
	sources map[string]string `toml:"-"` // Key: field name, Value: source (default, user, project, env, flag)
}

// LoadConfig loads configuration from defaults, user file, project file, and environment variables.
// It returns the merged configuration and an error if any occurs during loading.
func LoadConfig(projectPath string, verboseFlag bool, apiKeyFlag string, noStageFlag bool, autoPushFlag bool) (Config, error) {
	cfg := defaultConfig()
	cfg.sources = make(map[string]string)
	for k := range toMap(cfg) {
		cfg.sources[k] = "default"
	}

	// 1. User Config File
	userConfigPath, err := getUserConfigPath()
	if err != nil {
		// Non-fatal, just means we can't load user config
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Could not determine user config path: %v\n", err)
		}
	} else {
		if _, err := os.Stat(userConfigPath); err == nil {
			if err := loadConfigFromFile(userConfigPath, &cfg, "user"); err != nil {
				return cfg, fmt.Errorf("failed to load user config from %s: %w", userConfigPath, err)
			}
			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "[CONFIG] Loaded user config: %s\n", userConfigPath)
			}
		} else if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("failed to check user config file %s: %w", userConfigPath, err)
		}
	}

	// 2. Project Config File
	projectConfigPath := findProjectConfig(projectPath)
	if projectConfigPath != "" {
		if err := loadConfigFromFile(projectConfigPath, &cfg, "project"); err != nil {
			return cfg, fmt.Errorf("failed to load project config from %s: %w", projectConfigPath, err)
		}
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "[CONFIG] Loaded project config: %s\n", projectConfigPath)
		}
	}

	// 3. Environment Variables
	loadConfigFromEnv(&cfg)
	if cfg.Verbose {
		fmt.Fprintln(os.Stderr, "[CONFIG] Loaded environment variables")
	}

	// 4. Command Line Flags (Highest priority after ENV for specific overrides)
	// Verbose flag is applied first as it affects logging during load
	if verboseFlag {
		cfg.Verbose = true
		cfg.sources["Verbose"] = "flag"
	}
	if apiKeyFlag != "" {
		cfg.GeminiAPIKey = apiKeyFlag
		cfg.sources["GeminiAPIKey"] = "flag"
	}
	if noStageFlag {
		cfg.AskStage = false // --no-stage overrides config/env to disable asking
		cfg.sources["AskStage"] = "flag"
	}
	if autoPushFlag {
		cfg.AutoPush = true // --auto-push overrides config/env to enable auto push
		cfg.sources["AutoPush"] = "flag"
	}

	if cfg.Verbose {
		logConfigSources(cfg)
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		GeminiModel:           DefaultGeminiModel,
		MaxTokens:             DefaultMaxTokens,
		RequestTimeoutSeconds: DefaultTimeoutSecs,
		Prompt:                DefaultPrompt,
		IgnorePatterns:        []string{"*.svg"},
		AskStage:              DefaultAskStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		Verbose:               DefaultVerbose,
		// API Key has no default
	}
}

func loadConfigFromFile(path string, cfg *Config, source string) error {
	tempCfg := defaultConfig()
	if _, err := toml.DecodeFile(path, &tempCfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	loadedMap := toMap(tempCfg)
	defaultMap := toMap(defaultConfig())

	for k := range loadedMap {
		// Check if the value actually changed from the default or was explicitly set
		// Special handling for slices and API key
		switch k {
		case "GeminiAPIKey":
			// Always track API key if file exists
			cfg.sources[k] = source
		case "IgnorePatterns":
			// Always track ignore patterns if file exists
			cfg.sources[k] = source
		default:
			// For other fields, check if they differ from defaults
			if loadedMap[k] != defaultMap[k] {
				cfg.sources[k] = source
			}
		}
	}

	// Copy all values from tempCfg to cfg
	cfg.GeminiAPIKey = tempCfg.GeminiAPIKey
	cfg.GeminiModel = tempCfg.GeminiModel
	cfg.MaxTokens = tempCfg.MaxTokens
	cfg.RequestTimeoutSeconds = tempCfg.RequestTimeoutSeconds
	cfg.Prompt = tempCfg.Prompt
	cfg.IgnorePatterns = make([]string, len(tempCfg.IgnorePatterns))
	copy(cfg.IgnorePatterns, tempCfg.IgnorePatterns)
	cfg.AskStage = tempCfg.AskStage
	cfg.AutoPush = tempCfg.AutoPush
	cfg.PushCommand = tempCfg.PushCommand
	cfg.Verbose = tempCfg.Verbose

	return nil
}

func loadConfigFromEnv(cfg *Config) {
	// String values
	if val, ok := os.LookupEnv(EnvPrefix + "GEMINI_API_KEY"); ok {
		cfg.GeminiAPIKey = val
		cfg.sources["GeminiAPIKey"] = "env"
	}
	if val, ok := os.LookupEnv(EnvPrefix + "GEMINI_MODEL"); ok {
		cfg.GeminiModel = val
		cfg.sources["GeminiModel"] = "env"
	}
	if val, ok := os.LookupEnv(EnvPrefix + "PROMPT"); ok {
		cfg.Prompt = val
		cfg.sources["Prompt"] = "env"
	}
	if val, ok := os.LookupEnv(EnvPrefix + "PUSH_COMMAND"); ok {
		cfg.PushCommand = val
		cfg.sources["PushCommand"] = "env"
	}
	if val, ok := os.LookupEnv(EnvPrefix + "IGNORE_PATTERNS"); ok {
		patterns := strings.Split(val, ",")
		// Trim whitespace from each pattern
		for i := range patterns {
			patterns[i] = strings.TrimSpace(patterns[i])
		}
		cfg.IgnorePatterns = patterns
		cfg.sources["IgnorePatterns"] = "env"
	}

	// Integer values
	if val, ok := os.LookupEnv(EnvPrefix + "MAX_TOKENS"); ok {
		if intVal, err := strconv.Atoi(val); err == nil {
			cfg.MaxTokens = intVal
			cfg.sources["MaxTokens"] = "env"
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "REQUEST_TIMEOUT_SECONDS"); ok {
		if intVal, err := strconv.Atoi(val); err == nil {
			cfg.RequestTimeoutSeconds = intVal
			cfg.sources["RequestTimeoutSeconds"] = "env"
		}
	}

	// Boolean values
	if val, ok := os.LookupEnv(EnvPrefix + "ASK_STAGE"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.AskStage = boolVal
			cfg.sources["AskStage"] = "env"
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "AUTO_PUSH"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.AutoPush = boolVal
			cfg.sources["AutoPush"] = "env"
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "VERBOSE"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.Verbose = boolVal
			cfg.sources["Verbose"] = "env"
		}
	}
}

func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	yawnConfigDir := filepath.Join(homeDir, ".config", UserConfigDirName)
	// Ensure the directory exists
	if err := os.MkdirAll(yawnConfigDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create user config directory %s: %w", yawnConfigDir, err)
	}
	return filepath.Join(yawnConfigDir, UserConfigFileName), nil
}

// findProjectConfig searches for .yawn.toml starting from startPath and going up.
func findProjectConfig(startPath string) string {
	dir := startPath
	for {
		configPath := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir { // Reached root directory
			break
		}
		dir = parent
	}
	return ""
}

// GetRequestTimeout converts the config seconds to time.Duration.
func (c Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// GenerateDefaultConfig returns the default configuration as a TOML string.
func GenerateDefaultConfig() (string, error) {
	cfg := defaultConfig()
	// Use a temporary struct to control output order and add comments if needed
	// For simplicity, just marshal the default config directly.
	// Note: API key will be empty in the generated file.

	var builder strings.Builder
	encoder := toml.NewEncoder(&builder)
	encoder.Indent = "" // Use default indentation

	// Manually add comments for clarity
	builder.WriteString("# " + AppName + " Configuration File\n")
	builder.WriteString("# Generated by 'yawn --generate-config'\n\n")

	builder.WriteString("# Gemini API Key. REQUIRED.\n")
	builder.WriteString("# Can also be set via YAWN_GEMINI_API_KEY environment variable.\n")
	builder.WriteString(`gemini_api_key = "" # <-- Add your key here or set ENV var` + "\n\n")

	builder.WriteString("# Gemini model to use.\n")
	builder.WriteString("# See available models: https://ai.google.dev/models/gemini\n")
	builder.WriteString(fmt.Sprintf("gemini_model = %q\n\n", cfg.GeminiModel))

	builder.WriteString("# Maximum number of tokens (input + output) allowed for the Gemini request.\n")
	builder.WriteString("# Helps prevent excessive costs and errors for large diffs.\n")
	builder.WriteString("# Roughly estimate: 1 token ~= 4 characters. Adjust based on model.\n")
	builder.WriteString(fmt.Sprintf("max_tokens = %d\n\n", cfg.MaxTokens))

	builder.WriteString("# Request timeout in seconds when calling the Gemini API.\n")
	builder.WriteString(fmt.Sprintf("request_timeout_seconds = %d\n\n", cfg.RequestTimeoutSeconds))

	builder.WriteString("# Prompt template sent to Gemini.\n")
	builder.WriteString("# Available placeholders: {{Diff}}\n")
	builder.WriteString(fmt.Sprintf("prompt = '''\n%s\n'''\n\n", cfg.Prompt)) // Use multi-line literal string

	builder.WriteString("# List of glob patterns for files to ignore when generating the diff.\n")
	builder.WriteString(fmt.Sprintf("ignore_patterns = %s\n\n", formatStringSlice(cfg.IgnorePatterns)))

	builder.WriteString("# If true and nothing is currently staged, ask the user if they want to stage changes before proceeding.\n")
	builder.WriteString("# Set to false to never ask. Can be overridden by --no-stage flag.\n")
	builder.WriteString(fmt.Sprintf("ask_stage = %t\n\n", cfg.AskStage))

	builder.WriteString("# If true, automatically push the commit after it's made using 'push_command'.\n")
	builder.WriteString("# If false, ask the user for confirmation before pushing.\n")
	builder.WriteString("# Can be overridden by --auto-push flag.\n")
	builder.WriteString(fmt.Sprintf("auto_push = %t\n\n", cfg.AutoPush))

	builder.WriteString("# The command used to push the commit. 'HEAD' ensures the current branch is pushed.\n")
	builder.WriteString(fmt.Sprintf("push_command = %q\n\n", cfg.PushCommand))

	builder.WriteString("# Enable verbose logging output. Shows config loading details and step-by-step execution.\n")
	builder.WriteString("# Can be overridden by --verbose flag or YAWN_VERBOSE=true env var.\n")
	builder.WriteString(fmt.Sprintf("verbose = %t\n", cfg.Verbose))

	return builder.String(), nil
}

// formatStringSlice formats a slice of strings for TOML output.
func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, s := range slice {
		sb.WriteString(fmt.Sprintf("%q", s))
		if i < len(slice)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// --- Helper for logging sources ---

// toMap converts Config struct to a map[string]interface{} for easier processing.
// This is basic; reflection would be more robust but adds complexity.
func toMap(c Config) map[string]interface{} {
	return map[string]interface{}{
		"GeminiAPIKey":          c.GeminiAPIKey,
		"GeminiModel":           c.GeminiModel,
		"MaxTokens":             c.MaxTokens,
		"RequestTimeoutSeconds": c.RequestTimeoutSeconds,
		"Prompt":                c.Prompt,
		"IgnorePatterns":        c.IgnorePatterns,
		"AskStage":              c.AskStage,
		"AutoPush":              c.AutoPush,
		"PushCommand":           c.PushCommand,
		"Verbose":               c.Verbose,
	}
}

func logConfigSources(cfg Config) {
	fmt.Fprintln(os.Stderr, "[CONFIG] Final configuration sources:")
	configMap := toMap(cfg)
	// Sort keys for consistent output? Optional.
	for key, source := range cfg.sources {
		// Mask API key in logs
		displayValue := configMap[key]
		if key == "GeminiAPIKey" && cfg.GeminiAPIKey != "" {
			displayValue = "***" // Mask the key
		}
		if key == "Prompt" && len(cfg.Prompt) > 50 {
			displayValue = cfg.Prompt[:50] + "..." // Truncate long prompt
		}
		fmt.Fprintf(os.Stderr, "[CONFIG]  - %-25s = %-20v (from %s)\n", key, displayValue, source)
	}
}

// SaveAPIKeyToUserConfig saves the provided API key to the user's configuration file.
// If the file doesn't exist, it creates a new one with default values.
// If the file exists, it preserves all other settings while updating the API key.
func SaveAPIKeyToUserConfig(apiKey string) error {
	configPath, err := getUserConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get user config path: %w", err)
	}

	// Create a config with default values
	cfg := defaultConfig()
	cfg.GeminiAPIKey = apiKey

	// If file exists, load it first to preserve other settings
	if _, err := os.Stat(configPath); err == nil {
		if err := loadConfigFromFile(configPath, &cfg, "user"); err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
		// Ensure the API key is set even if it was empty in the file
		cfg.GeminiAPIKey = apiKey
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check config file: %w", err)
	}

	// Ensure the directory exists (getUserConfigPath already does this, but let's be extra safe)
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write the config to a temporary file first
	tmpFile, err := os.CreateTemp(dir, "config.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up in case of failure

	// Write config to temporary file
	encoder := toml.NewEncoder(tmpFile)
	if err := encoder.Encode(cfg); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write config: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Set restrictive permissions on the file (read/write for owner only)
	if err := os.Chmod(tmpPath, 0600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomically replace the old config file
	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}
