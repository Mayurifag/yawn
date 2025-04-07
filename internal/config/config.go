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
)

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
	DefaultPrompt         = `Generate a commit message.

- Fully follow Conventional Commits (https://www.conventionalcommits.org/en/v1.0.0/). ALWAYS follow Conventional Commits specification. Also cheatsheet https://gist.github.com/qoomon/5dfcdf8eec66a051ecd85625518cfd13
- Do not use gitmoji
- If there are multiple changes, try to mention them all in the commit message in body, divide them into separate bullet points (one per -)
- Try to make meangingful description of the changes, think why changes were done and make it single bullet point for description line. It has to be the most important and descriptive part.
- Do not use formatting for output, just the commit message itself. Don't use ticks or other formatting symbols, only text of commit, no commentaries after or before message
- That is nice to use filenames inside body or description, if it is relevant to the changes, just be sure to use it just like text, no backticks or else
- Preserve newlines in the commit body. Each bullet point should be on its own line. Use the imperative, present tense: "Change" not "changed" nor "changes". Capitalize the first letter
- The body should be separated from the description by a blank line
- Description has to start with lowercase letter to preserve consistancy between different commits

Structure of output:
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]

Here are some example outputs (goes until ---):
fix: prevent racing of requests

- Introduce a request id and a reference to latest request. Dismiss
incoming responses other than from latest request.
- Remove timeouts which were used to mitigate the racing issue but are
obsolete now.
---
feat: allow provided config object to extend other configs

BREAKING CHANGE: 'extends' key in config file is now used for extending other config files
---

Here is the diff to analyze:

{{Diff}}`
)

// Config holds the application configuration. Fields must be exported for TOML decoding.
type Config struct {
	GeminiAPIKey          string `toml:"gemini_api_key"`
	GeminiModel           string `toml:"gemini_model"`
	MaxTokens             int    `toml:"max_tokens"`
	RequestTimeoutSeconds int    `toml:"request_timeout_seconds"`
	Prompt                string `toml:"prompt,multiline"`
	AutoStage             bool   `toml:"auto_stage"`
	AutoPush              bool   `toml:"auto_push"`
	PushCommand           string `toml:"push_command"`
	Verbose               bool   `toml:"verbose"`
	WaitForSSHKeys        bool   `toml:"wait_for_ssh_keys"`

	sources map[string]string `toml:"-"` // Key: field name, Value: source (default, user, project, env, flag)
}

// getUserConfigPath returns the path to the user's config file.
var getUserConfigPathFunc = getUserConfigPath

// loadUserConfig attempts to load configuration from the user's config file.
// Returns the loaded config, metadata, and any error encountered.
func loadUserConfig() (Config, toml.MetaData, error) {
	userConfigPath, err := getUserConfigPathFunc()
	if err != nil {
		return Config{}, toml.MetaData{}, nil // Non-fatal, just means we can't load user config
	}

	if _, err := os.Stat(userConfigPath); err != nil {
		if os.IsNotExist(err) {
			return Config{}, toml.MetaData{}, nil // File doesn't exist, not an error
		}
		return Config{}, toml.MetaData{}, fmt.Errorf("failed to check user config file %s: %w", userConfigPath, err)
	}

	var loadedCfg Config
	metadata, decodeErr := toml.DecodeFile(userConfigPath, &loadedCfg)
	if decodeErr != nil {
		return Config{}, toml.MetaData{}, fmt.Errorf("failed to load user config from %s: %w", userConfigPath, decodeErr)
	}

	return loadedCfg, metadata, nil
}

// findProjectConfig searches for .yawn.toml starting from startPath and going up.
var findProjectConfigFunc = findProjectConfig

func findProjectConfig(startPath string) string {
	dir, err := filepath.Abs(startPath) // Start with absolute path
	if err != nil {
		// Cannot get absolute path, unlikely but possible
		return ""
	}

	for {
		configPath := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(configPath); err == nil {
			// Check if it's readable
			f, openErr := os.Open(configPath)
			if openErr == nil {
				f.Close()
				return configPath // Found readable config file
			}
			// If Stat worked but Open failed, might be permissions issue, stop searching up?
			// Let's continue searching up for now, maybe a higher level one is readable.
		} else if !os.IsNotExist(err) {
			// Error other than "not found" while checking file, stop searching.
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir { // Reached root directory
			break
		}
		dir = parent
	}
	return "" // Not found
}

// loadProjectConfig attempts to load configuration from the project's config file.
// Returns the loaded config, metadata, and any error encountered.
func loadProjectConfig(projectPath string) (Config, toml.MetaData, error) {
	projectConfigPath := findProjectConfigFunc(projectPath)
	if projectConfigPath == "" {
		return Config{}, toml.MetaData{}, nil // No project config found, not an error
	}

	var loadedCfg Config
	metadata, decodeErr := toml.DecodeFile(projectConfigPath, &loadedCfg)
	if decodeErr != nil {
		return Config{}, toml.MetaData{}, fmt.Errorf("failed to load project config from %s: %w", projectConfigPath, decodeErr)
	}

	return loadedCfg, metadata, nil
}

// applyEnvConfig applies configuration from environment variables.
func applyEnvConfig(cfg *Config) {
	loadConfigFromEnv(cfg)
}

// applyFlags applies command-line flags to the configuration.
func applyFlags(
	cfg *Config,
	verboseFlag bool,
	apiKeyFlag string,
	autoStageFlag bool,
	autoPushFlag bool,
	flagsSpecified ...string, // Names of flags that were explicitly specified
) {
	// Create a map to quickly check if a flag was specified
	specified := make(map[string]bool)
	for _, flag := range flagsSpecified {
		specified[flag] = true
	}

	// Only apply flags that were explicitly specified
	if specified["verbose"] {
		cfg.Verbose = verboseFlag
		cfg.sources["Verbose"] = "flag"
	}

	if specified["api-key"] && apiKeyFlag != "" {
		cfg.GeminiAPIKey = apiKeyFlag
		cfg.sources["GeminiAPIKey"] = "flag"
	}

	if specified["stage"] {
		cfg.AutoStage = autoStageFlag
		cfg.sources["AutoStage"] = "flag"
	}

	if specified["push"] {
		cfg.AutoPush = autoPushFlag
		cfg.sources["AutoPush"] = "flag"
	}
}

// logConfigLoadingSummary logs information about where configuration was loaded from.
func logConfigLoadingSummary(cfg *Config, projectPath string) {
	userConfigPath, err := getUserConfigPathFunc()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Could not determine user config path: %v\n", err)
	} else if userConfigPath != "" {
		if _, statErr := os.Stat(userConfigPath); statErr == nil {
			fmt.Fprintf(os.Stderr, "[CONFIG] Loaded user config: %s\n", userConfigPath)
		} else if !os.IsNotExist(statErr) {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Error checking user config %s: %v\n", userConfigPath, statErr)
		} else {
			fmt.Fprintf(os.Stderr, "[CONFIG] No user config found at %s\n", userConfigPath)
		}
	}

	projectConfigPath := findProjectConfigFunc(projectPath)
	if projectConfigPath != "" {
		fmt.Fprintf(os.Stderr, "[CONFIG] Loaded project config: %s\n", projectConfigPath)
	} else if projectPath != "" {
		fmt.Fprintf(os.Stderr, "[CONFIG] No project config found in or above %s\n", projectPath)
	}

	fmt.Fprintln(os.Stderr, "[CONFIG] Applied environment variables.")
	fmt.Fprintln(os.Stderr, "[CONFIG] Applied command-line flags.")

	logConfigSources(*cfg)
}

// LoadConfig loads configuration from defaults, user file, project file, and environment variables.
// It returns the merged configuration and an error if any occurs during loading.
func LoadConfig(
	projectPath string,
	verboseFlag bool,
	apiKeyFlag string,
	autoStageFlag bool,
	autoPushFlag bool,
	flagsSpecified ...string, // Names of flags that were explicitly specified
) (Config, error) {
	// Initialize config with defaults
	cfg, err := loadDefaults()
	if err != nil {
		return cfg, fmt.Errorf("failed to load default configuration: %w", err)
	}

	// Load and apply user config
	if err := applyUserConfig(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to apply user configuration: %w", err)
	}

	// Load and apply project config
	if err := applyProjectConfig(&cfg, projectPath); err != nil {
		return cfg, fmt.Errorf("failed to apply project configuration: %w", err)
	}

	// Apply environment variables
	applyEnvConfig(&cfg)

	// Apply command-line flags (highest precedence)
	applyFlags(&cfg, verboseFlag, apiKeyFlag, autoStageFlag, autoPushFlag, flagsSpecified...)

	// Log configuration loading process if verbose
	if cfg.Verbose {
		logConfigLoadingSummary(&cfg, projectPath)
	}

	return cfg, nil
}

// loadDefaults initializes a configuration with default values.
func loadDefaults() (Config, error) {
	cfg := defaultConfig()
	cfg.sources = make(map[string]string)

	// Mark all fields as coming from defaults
	for k := range toMap(cfg) {
		cfg.sources[k] = "default"
	}

	return cfg, nil
}

// applyUserConfig loads and applies user configuration from the user config file.
func applyUserConfig(cfg *Config) error {
	userCfg, userMeta, err := loadUserConfig()
	if err != nil {
		return err
	}

	// Only merge if we actually loaded something (check for any keys in metadata)
	if len(userMeta.Keys()) == 0 {
		return nil
	}

	mergeConfig(cfg, userCfg, userMeta, "user home config")

	return nil
}

// applyProjectConfig loads and applies project-specific configuration.
func applyProjectConfig(cfg *Config, projectPath string) error {
	projectCfg, projectMeta, err := loadProjectConfig(projectPath)
	if err != nil {
		return err
	}

	// Only merge if we actually loaded something
	if len(projectMeta.Keys()) == 0 {
		return nil
	}

	mergeConfig(cfg, projectCfg, projectMeta, "project")
	return nil
}

func defaultConfig() Config {
	return Config{
		GeminiModel:           DefaultGeminiModel,
		MaxTokens:             DefaultMaxTokens,
		RequestTimeoutSeconds: DefaultTimeoutSecs,
		Prompt:                DefaultPrompt,
		AutoStage:             DefaultAutoStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		Verbose:               DefaultVerbose,
		WaitForSSHKeys:        DefaultWaitForSSHKeys,
		// API Key has no default
	}
}

// mergeConfig merges loadedCfg into baseCfg, tracking the source, using metadata to check defined keys.
func mergeConfig(baseCfg *Config, loadedCfg Config, metadata toml.MetaData, source string) {
	// Manual approach using metadata.IsDefined for clarity:

	if metadata.IsDefined("gemini_api_key") && loadedCfg.GeminiAPIKey != "" {
		baseCfg.GeminiAPIKey = loadedCfg.GeminiAPIKey
		baseCfg.sources["GeminiAPIKey"] = source
	}
	if metadata.IsDefined("gemini_model") && loadedCfg.GeminiModel != "" {
		baseCfg.GeminiModel = loadedCfg.GeminiModel
		baseCfg.sources["GeminiModel"] = source
	}
	if metadata.IsDefined("max_tokens") && loadedCfg.MaxTokens != 0 { // Assuming 0 is not a valid user setting
		baseCfg.MaxTokens = loadedCfg.MaxTokens
		baseCfg.sources["MaxTokens"] = source
	}
	if metadata.IsDefined("request_timeout_seconds") && loadedCfg.RequestTimeoutSeconds != 0 {
		baseCfg.RequestTimeoutSeconds = loadedCfg.RequestTimeoutSeconds
		baseCfg.sources["RequestTimeoutSeconds"] = source
	}
	if metadata.IsDefined("prompt") && loadedCfg.Prompt != "" {
		baseCfg.Prompt = loadedCfg.Prompt
		baseCfg.sources["Prompt"] = source
	}
	// Booleans need explicit check if they were defined in the file.
	if metadata.IsDefined("auto_stage") {
		baseCfg.AutoStage = loadedCfg.AutoStage
		baseCfg.sources["AutoStage"] = source
	}
	if metadata.IsDefined("auto_push") {
		baseCfg.AutoPush = loadedCfg.AutoPush
		baseCfg.sources["AutoPush"] = source
	}
	if metadata.IsDefined("push_command") && loadedCfg.PushCommand != "" {
		baseCfg.PushCommand = loadedCfg.PushCommand
		baseCfg.sources["PushCommand"] = source
	}
	if metadata.IsDefined("verbose") {
		baseCfg.Verbose = loadedCfg.Verbose
		baseCfg.sources["Verbose"] = source
	}
	if metadata.IsDefined("wait_for_ssh_keys") {
		baseCfg.WaitForSSHKeys = loadedCfg.WaitForSSHKeys
		baseCfg.sources["WaitForSSHKeys"] = source
	}
}

func loadConfigFromEnv(cfg *Config) {
	// Helper to get env var with prefix
	getEnv := func(key string) string {
		return os.Getenv(EnvPrefix + key)
	}

	// Helper to get bool env var
	getBoolEnv := func(key string) (bool, bool) {
		val := getEnv(key)
		if val == "" {
			return false, false
		}
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, false
		}
		return b, true
	}

	// Helper to get int env var
	getIntEnv := func(key string) (int, bool) {
		val := getEnv(key)
		if val == "" {
			return 0, false
		}
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, false
		}
		return i, true
	}

	// Load from environment variables
	if apiKey := getEnv("GEMINI_API_KEY"); apiKey != "" {
		cfg.GeminiAPIKey = apiKey
		cfg.sources["GeminiAPIKey"] = "env"
	}
	if model := getEnv("GEMINI_MODEL"); model != "" {
		cfg.GeminiModel = model
		cfg.sources["GeminiModel"] = "env"
	}
	if maxTokens, ok := getIntEnv("MAX_TOKENS"); ok {
		cfg.MaxTokens = maxTokens
		cfg.sources["MaxTokens"] = "env"
	}
	if timeout, ok := getIntEnv("REQUEST_TIMEOUT_SECONDS"); ok {
		cfg.RequestTimeoutSeconds = timeout
		cfg.sources["RequestTimeoutSeconds"] = "env"
	}
	if prompt := getEnv("PROMPT"); prompt != "" {
		cfg.Prompt = prompt
		cfg.sources["Prompt"] = "env"
	}
	if autoStage, ok := getBoolEnv("AUTO_STAGE"); ok {
		cfg.AutoStage = autoStage
		cfg.sources["AutoStage"] = "env"
	}
	if autoPush, ok := getBoolEnv("AUTO_PUSH"); ok {
		cfg.AutoPush = autoPush
		cfg.sources["AutoPush"] = "env"
	}
	if pushCommand := getEnv("PUSH_COMMAND"); pushCommand != "" {
		cfg.PushCommand = pushCommand
		cfg.sources["PushCommand"] = "env"
	}
	if verbose, ok := getBoolEnv("VERBOSE"); ok {
		cfg.Verbose = verbose
		cfg.sources["Verbose"] = "env"
	}
	if waitForSSHKeys, ok := getBoolEnv("WAIT_FOR_SSH_KEYS"); ok {
		cfg.WaitForSSHKeys = waitForSSHKeys
		cfg.sources["WaitForSSHKeys"] = "env"
	}
}

func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	// Follow XDG Base Directory Specification if possible
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}
	yawnConfigDir := filepath.Join(xdgConfigHome, UserConfigDirName)

	return filepath.Join(yawnConfigDir, UserConfigFileName), nil
}

// ensureUserConfigDir ensures the user config directory exists, creating it if necessary
func ensureUserConfigDir() (string, error) {
	configPath, err := getUserConfigPath()
	if err != nil {
		return "", err
	}

	yawnConfigDir := filepath.Dir(configPath)

	// Ensure the directory exists with appropriate permissions (0700)
	if err := os.MkdirAll(yawnConfigDir, 0700); err != nil {
		// Check if the path exists but is a file
		if stat, statErr := os.Stat(yawnConfigDir); statErr == nil && !stat.IsDir() {
			return "", fmt.Errorf("user config path %s exists but is not a directory", yawnConfigDir)
		}
		// If MkdirAll failed for other reasons (permissions?)
		return "", fmt.Errorf("failed to create user config directory %s: %w", yawnConfigDir, err)
	}

	return configPath, nil
}

// GetRequestTimeout converts the config seconds to time.Duration.
func (c Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// GetConfigSource returns the source of a configuration option (default, user, project, env, flag)
func (c Config) GetConfigSource(option string) string {
	if source, ok := c.sources[option]; ok {
		return source
	}
	return "unknown"
}

// GenerateConfigContent generates the configuration content in TOML format.
// It accepts an optional API key. If the API key is empty, it's excluded from the output.
// The function returns a byte slice for direct file writing and an error if encoding fails.
func GenerateConfigContent(apiKey string) ([]byte, error) {
	var buf bytes.Buffer

	// Write comments first
	comments := []string{
		"# Configuration file for yawn - AI Git Commiter using Google Gemini",
		"#",
		"# This file can be placed in (or both):",
		"# - ~/.config/yawn/config.toml (user config)",
		"# - ./.yawn.toml (project config, you might want to add this to your .gitignore)",
		"#",
		"# Precedence order: command line flags > environment variables > project config > user config > defaults",
	}

	for _, comment := range comments {
		buf.WriteString(comment + "\n")
	}
	buf.WriteString("\n")

	// Create config with default values - except for prompt which we'll handle separately
	cfg := map[string]interface{}{
		"gemini_model":            DefaultGeminiModel,
		"max_tokens":              DefaultMaxTokens,
		"request_timeout_seconds": DefaultTimeoutSecs,
		"auto_stage":              DefaultAutoStage,
		"auto_push":               DefaultAutoPush,
		"push_command":            DefaultPushCommand,
		"verbose":                 DefaultVerbose,
		"wait_for_ssh_keys":       DefaultWaitForSSHKeys,
	}

	// Only include API key if it's provided
	if apiKey != "" {
		cfg["gemini_api_key"] = apiKey
	}

	// Encode config as TOML
	encoder := toml.NewEncoder(&buf)
	encoder.Indent = ""
	if err := encoder.Encode(cfg); err != nil {
		return nil, fmt.Errorf("failed to encode config: %w", err)
	}

	// Add the prompt using multiline syntax
	buf.WriteString("prompt = '''\n")
	buf.WriteString(DefaultPrompt)
	buf.WriteString("\n'''\n")

	return buf.Bytes(), nil
}

// GenerateDefaultConfig returns the default configuration as a TOML string.
// This is a wrapper for GenerateConfigContent for backward compatibility.
func GenerateDefaultConfig() (string, error) {
	content, err := GenerateConfigContent("")
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SaveAPIKeyToUserConfig saves the provided API key to the user's configuration file.
// If the file doesn't exist, it creates a new one using GenerateConfigContent.
// If the file exists, it preserves all other settings while updating the API key.
func SaveAPIKeyToUserConfig(apiKey string) error {
	// Get config path and ensure directory exists
	configPath, err := ensureUserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to prepare user config directory: %w", err)
	}

	var configContent []byte

	// Check if file exists
	_, statErr := os.Stat(configPath)
	if os.IsNotExist(statErr) {
		// Generate content for new config file
		configContent, err = GenerateConfigContent(apiKey)
		if err != nil {
			return fmt.Errorf("failed to generate new config content: %w", err)
		}
	} else if statErr == nil {
		// Read and update existing config file
		existingContent, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("failed to read existing config file %s: %w", configPath, readErr)
		}

		configContent, err = updateExistingConfigContent(existingContent, apiKey)
		if err != nil {
			return err
		}
	} else {
		// Other error checking the file
		return fmt.Errorf("failed to check user config file %s: %w", configPath, statErr)
	}

	// Write the config content atomically
	return writeConfigFileAtomically(configContent, configPath)
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
		"AutoStage":             c.AutoStage,
		"AutoPush":              c.AutoPush,
		"PushCommand":           c.PushCommand,
		"Verbose":               c.Verbose,
		"WaitForSSHKeys":        c.WaitForSSHKeys,
	}
}

func logConfigSources(cfg Config) {
	fmt.Fprintln(os.Stderr, "[CONFIG] Final configuration values and sources:")
	configMap := toMap(cfg) // Get current values
	maxKeyLen := 0
	keys := make([]string, 0, len(cfg.sources))

	// Define desired order
	orderedKeys := []string{
		"GeminiAPIKey", "GeminiModel", "MaxTokens", "RequestTimeoutSeconds",
		"Prompt", "AutoStage", "AutoPush", "PushCommand", "Verbose", "WaitForSSHKeys",
	}
	// Use ordered keys if they exist in sources
	processedKeys := make(map[string]bool)
	for _, key := range orderedKeys {
		if _, exists := cfg.sources[key]; exists {
			keys = append(keys, key)
			processedKeys[key] = true
			if len(key) > maxKeyLen {
				maxKeyLen = len(key)
			}
		}
	}
	// Add any remaining keys from sources that weren't in the ordered list
	for k := range cfg.sources {
		if !processedKeys[k] {
			keys = append(keys, k)
			if len(k) > maxKeyLen {
				maxKeyLen = len(k)
			}
		}
	}

	for _, key := range keys {
		source := cfg.sources[key]
		displayValue := configMap[key]
		valueStr := fmt.Sprintf("%v", displayValue)

		// Special handling for display
		switch key {
		case "GeminiAPIKey":
			if cfg.GeminiAPIKey != "" {
				valueStr = "***" // Mask the key
			} else {
				valueStr = `"" (Not Set)`
			}
		case "Prompt":
			// Truncate long prompt, show first line or ~60 chars - for Verbose mode
			firstLine := strings.SplitN(cfg.Prompt, "\n", 2)[0]
			if len(firstLine) > 60 {
				valueStr = fmt.Sprintf("%q...", firstLine[:60])
			} else if len(cfg.Prompt) > 80 { // Check total length too
				valueStr = fmt.Sprintf("%q...", firstLine)
			} else {
				valueStr = fmt.Sprintf("%q", firstLine) // Quote the single line
			}
		}

		// Simple alignment
		fmt.Fprintf(os.Stderr, "[CONFIG]  - %-*s : %-30s (from %s)\n", maxKeyLen, key, valueStr, source)
	}
}

// writeConfigFileAtomically writes content to a file atomically with proper permissions.
// It creates a temporary file, writes content, sets permissions, and renames it to the target path.
func writeConfigFileAtomically(content []byte, targetPath string) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	// Ensure temp file is cleaned up on error
	defer func() {
		if err != nil { // Only remove if there was an error during write/rename
			os.Remove(tmpPath)
		}
	}()

	// Write content to temp file
	if _, err = tmpFile.Write(content); err != nil {
		tmpFile.Close() // Close even on write error
		return fmt.Errorf("failed to write to temporary config file: %w", err)
	}

	// Close the temp file before renaming
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}

	// Set restrictive permissions (read/write for owner only: 0600)
	if err = os.Chmod(tmpPath, 0600); err != nil {
		// Attempt to remove the temp file if chmod fails
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on temporary config file: %w", err)
	}

	// Atomically replace the actual config file with the temporary file
	if err = os.Rename(tmpPath, targetPath); err != nil {
		// Attempt to remove the temp file if rename fails
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save config file (rename failed): %w", err)
	}

	return nil
}

// updateExistingConfigContent updates an existing config file's content with a new API key.
func updateExistingConfigContent(existingContent []byte, apiKey string) ([]byte, error) {
	// Decode into a generic map to preserve structure and comments
	var cfgMap map[string]interface{}
	if _, err := toml.Decode(string(existingContent), &cfgMap); err != nil {
		return nil, fmt.Errorf("failed to decode existing config file for update: %w", err)
	}

	// Update the API key in the map
	cfgMap["gemini_api_key"] = apiKey

	// Create a buffer for the updated TOML content
	var buf bytes.Buffer

	// Write the configuration values
	configKeys := []string{
		"gemini_api_key", "gemini_model", "max_tokens", "request_timeout_seconds",
		"auto_stage", "auto_push", "push_command", "verbose", "prompt", "wait_for_ssh_keys",
	}

	for _, key := range configKeys {
		value, exists := cfgMap[key]
		if !exists {
			continue
		}

		switch v := value.(type) {
		case string:
			if key == "prompt" {
				buf.WriteString("prompt = '''\n")
				buf.WriteString(v)
				buf.WriteString("\n'''\n")
			} else {
				buf.WriteString(fmt.Sprintf("%s = %q\n", key, v))
			}
		case int64:
			buf.WriteString(fmt.Sprintf("%s = %d\n", key, v))
		case bool:
			buf.WriteString(fmt.Sprintf("%s = %v\n", key, v))
		}
	}

	return buf.Bytes(), nil
}
