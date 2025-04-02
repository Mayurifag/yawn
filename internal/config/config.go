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
	AppName            = "yawn"
	ProjectConfigName  = ".yawn.toml"
	UserConfigDirName  = "yawn"
	UserConfigFileName = "config.toml"
	EnvPrefix          = "YAWN_"
	DefaultGeminiModel = "gemini-2.0-flash-lite"
	DefaultMaxTokens   = 1000000
	DefaultTimeoutSecs = 10
	DefaultAutoStage   = false
	DefaultAutoPush    = false
	DefaultPushCommand = "git push origin HEAD"
	DefaultVerbose     = false
	DefaultPrompt      = `Generate a commit message.

- Fully follow Conventional Commits (https://www.conventionalcommits.org/en/v1.0.0/). ALWAYS follow Conventional Commits specification.
- Do not use gitmoji
- If there are multiple changes, try to mention them all in the commit message in body, divide them into separate bullet points (one per -)
- Try to make meangingful description of the changes, think why changes were done and make it single bullet point for description line
- Do not use formatting for output, just the commit message itself. Don't use ticks or other formatting symbols, only text of commit, no commentaries after or before message.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

Commits MUST be prefixed with a type, which consists of a noun, feat, fix, etc., followed by the OPTIONAL scope, OPTIONAL !, and REQUIRED terminal colon and space.

The type feat MUST be used when a commit adds a new feature to your application or library.

The type fix MUST be used when a commit represents a bug fix for your application.

A scope MAY be provided after a type. A scope MUST consist of a noun describing a section of the codebase surrounded by parenthesis, e.g., fix(parser):

A description MUST immediately follow the colon and space after the type/scope prefix. The description is a short summary of the code changes, e.g., fix: array parsing issue when multiple spaces were contained in string.

A longer commit body MAY be provided after the short description, providing additional contextual information about the code changes. The body MUST begin one blank line after the description.

A commit body is free-form and MAY consist of any number of newline separated paragraphs.

One or more footers MAY be provided one blank line after the body. Each footer MUST consist of a word token, followed by either :<space> or <space>#, followed by a string value (this is inspired by the git trailer convention).

A footer's token MUST use - in place of whitespace characters, e.g., Acked-by (this helps differentiate the footer section from a multi-paragraph body). An exception is made for BREAKING CHANGE, which MAY also be used as a token.

A footer's value MAY contain spaces and newlines, and parsing MUST terminate when the next valid footer token/separator pair is observed.

Breaking changes MUST be indicated in the type/scope prefix of a commit, or as an entry in the footer.

If included as a footer, a breaking change MUST consist of the uppercase text BREAKING CHANGE, followed by a colon, space, and description, e.g., BREAKING CHANGE: environment variables now take precedence over config files.

If included in the type/scope prefix, breaking changes MUST be indicated by a ! immediately before the :. If ! is used, BREAKING CHANGE: MAY be omitted from the footer section, and the commit description SHALL be used to describe the breaking change.

Types other than feat and fix MAY be used in your commit messages, e.g., docs: updated ref docs.

The units of information that make up Conventional Commits MUST NOT be treated as case sensitive by implementors, with the exception of BREAKING CHANGE which MUST be uppercase.

BREAKING-CHANGE MUST be synonymous with BREAKING CHANGE, when used as a token in a footer.

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

{{Diff}}
`
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

	// Internal fields to track config sources
	sources map[string]string `toml:"-"` // Key: field name, Value: source (default, user, project, env, flag)
}

// LoadConfig loads configuration from defaults, user file, project file, and environment variables.
// It returns the merged configuration and an error if any occurs during loading.
func LoadConfig(projectPath string, verboseFlag bool, apiKeyFlag string, autoStageFlag bool, autoPushFlag bool) (Config, error) {
	cfg := defaultConfig()
	cfg.sources = make(map[string]string)
	for k := range toMap(cfg) {
		cfg.sources[k] = "default"
	}

	// 1. User Config File
	userConfigPath, err := getUserConfigPath()
	if err != nil {
		// Non-fatal, just means we can't load user config
		// Verbosity check needs to happen *after* flags are potentially applied
	} else {
		if _, err := os.Stat(userConfigPath); err == nil {
			var loadedCfg Config
			// Use DecodeFile to load into a temporary struct first
			metadata, decodeErr := toml.DecodeFile(userConfigPath, &loadedCfg)
			if decodeErr != nil {
				return cfg, fmt.Errorf("failed to load user config from %s: %w", userConfigPath, decodeErr)
			}
			mergeConfig(&cfg, loadedCfg, metadata, "user")
			// Defer verbose logging until flags are processed
		} else if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("failed to check user config file %s: %w", userConfigPath, err)
		}
	}

	// 2. Project Config File
	projectConfigPath := findProjectConfig(projectPath)
	if projectConfigPath != "" {
		var loadedCfg Config
		metadata, decodeErr := toml.DecodeFile(projectConfigPath, &loadedCfg)
		if decodeErr != nil {
			return cfg, fmt.Errorf("failed to load project config from %s: %w", projectConfigPath, decodeErr)
		}
		mergeConfig(&cfg, loadedCfg, metadata, "project")
		// Defer verbose logging
	}

	// 3. Environment Variables
	loadConfigFromEnv(&cfg)
	// Defer verbose logging

	// 4. Command Line Flags (Highest priority)
	// Verbose flag applied first
	if verboseFlag {
		cfg.Verbose = true
		cfg.sources["Verbose"] = "flag"
	}
	if apiKeyFlag != "" {
		cfg.GeminiAPIKey = apiKeyFlag
		cfg.sources["GeminiAPIKey"] = "flag"
	}
	if autoStageFlag {
		cfg.AutoStage = true // --auto-stage overrides config/env to enable auto stage
		cfg.sources["AutoStage"] = "flag"
	}
	if autoPushFlag {
		cfg.AutoPush = true // --auto-push overrides config/env to enable auto push
		cfg.sources["AutoPush"] = "flag"
	}

	// Now that flags are processed, check verbosity for initial load messages
	if cfg.Verbose {
		if userConfigPath != "" {
			if _, err := os.Stat(userConfigPath); err == nil {
				fmt.Fprintf(os.Stderr, "[CONFIG] Loaded user config: %s\n", userConfigPath)
			} else if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Error checking user config %s: %v\n", userConfigPath, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Could not determine user config path.\n")
		}
		if projectConfigPath != "" {
			fmt.Fprintf(os.Stderr, "[CONFIG] Loaded project config: %s\n", projectConfigPath)
		}
		// Log env var loading if verbose
		fmt.Fprintln(os.Stderr, "[CONFIG] Applied environment variables.")
		// Log flag application if verbose
		fmt.Fprintln(os.Stderr, "[CONFIG] Applied command-line flags.")

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
		AutoStage:             DefaultAutoStage,
		AutoPush:              DefaultAutoPush,
		PushCommand:           DefaultPushCommand,
		Verbose:               DefaultVerbose,
		// API Key has no default
	}
}

// loadSpecificConfigFromFile is deprecated by using DecodeFile directly in LoadConfig
// func loadSpecificConfigFromFile(path string, targetCfg *Config) error { ... }

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
}

func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Follow XDG Base Directory Specification if possible
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}
	yawnConfigDir := filepath.Join(xdgConfigHome, UserConfigDirName)

	// Ensure the directory exists with appropriate permissions (0700)
	// MkdirAll is safe to call even if the directory exists
	if err := os.MkdirAll(yawnConfigDir, 0700); err != nil {
		// Check if the path exists but is a file
		if stat, statErr := os.Stat(yawnConfigDir); statErr == nil && !stat.IsDir() {
			return "", fmt.Errorf("user config path %s exists but is not a directory", yawnConfigDir)
		}
		// If MkdirAll failed for other reasons (permissions?)
		return "", fmt.Errorf("failed to create user config directory %s: %w", yawnConfigDir, err)
	}
	return filepath.Join(yawnConfigDir, UserConfigFileName), nil
}

// findProjectConfig searches for .yawn.toml starting from startPath and going up.
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

// GetRequestTimeout converts the config seconds to time.Duration.
func (c Config) GetRequestTimeout() time.Duration {
	if c.RequestTimeoutSeconds <= 0 {
		return time.Duration(DefaultTimeoutSecs) * time.Second // Fallback to default if invalid
	}
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// GenerateDefaultConfig returns the default configuration as a TOML string
// with comments and proper formatting.
func GenerateDefaultConfig() (string, error) {
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	encoder.Indent = ""

	// Create a map with default values
	defaults := map[string]interface{}{
		"gemini_api_key":          "", // No default, must be provided
		"gemini_model":            DefaultGeminiModel,
		"max_tokens":              DefaultMaxTokens,
		"request_timeout_seconds": DefaultTimeoutSecs,
		"auto_stage":              DefaultAutoStage,
		"auto_push":               DefaultAutoPush,
		"push_command":            DefaultPushCommand,
		"verbose":                 DefaultVerbose,
		"prompt":                  DefaultPrompt,
	}

	// Write comments before encoding
	comments := []string{
		"# Configuration file for yawn - AI Git Commiter using Google Gemini",
		"#",
		"# This file can be placed in (or both):",
		"# - ~/.config/yawn/config.toml (user config)",
		"# - ./.yawn.toml (project config, you might want to add this to your .gitignore)",
		"#",
		"# Precedence order: command line flags > environment variables > project config > user config",
		"#",
		"# When auto_stage is true, all unstaged changes will be automatically staged",
		"# When auto_stage is false and there are unstaged changes, you will be prompted to stage them",
	}

	for _, comment := range comments {
		buf.WriteString(comment + "\n")
	}
	buf.WriteString("\n")

	err := encoder.Encode(defaults)
	if err != nil {
		return "", fmt.Errorf("failed to encode default config: %w", err)
	}

	return buf.String(), nil
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
		"Prompt", "AutoStage", "AutoPush", "PushCommand", "Verbose",
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

// SaveAPIKeyToUserConfig saves the provided API key to the user's configuration file.
// If the file doesn't exist, it creates a new one using GenerateDefaultConfig format.
// If the file exists, it preserves all other settings while updating the API key.
func SaveAPIKeyToUserConfig(apiKey string) error {
	configPath, err := getUserConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get user config path: %w", err)
	}

	dir := filepath.Dir(configPath)
	// Ensure the directory exists (getUserConfigPath should do this, but double-check)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to ensure config directory exists: %w", err)
	}

	var configContent []byte

	// Check if file exists
	_, statErr := os.Stat(configPath)
	if os.IsNotExist(statErr) {
		// Create a buffer for the TOML content
		var buf bytes.Buffer

		// Write comments first
		comments := []string{
			"# Configuration file for yawn - AI Git Commiter using Google Gemini",
			"#",
			"# This file can be placed in:",
			"# - ~/.config/yawn/config.toml (user config)",
			"# - ./.yawn.toml (project config)",
			"#",
			"# Environment variables (YAWN_*) take precedence over this file",
			"# Command line flags take precedence over environment variables",
			"#",
			"# Model configuration",
			"#",
			"# Git workflow configuration",
			"#",
			"# When auto_stage is true, all unstaged changes will be automatically staged",
			"# When auto_stage is false and there are unstaged changes, you will be prompted to stage them",
			"#",
			"# Logging configuration",
			"#",
			"# Custom prompt for commit message generation",
		}

		for _, comment := range comments {
			buf.WriteString(comment + "\n")
		}
		buf.WriteString("\n")

		// Write the configuration values manually to ensure proper multiline string formatting
		buf.WriteString(fmt.Sprintf("gemini_api_key = %q\n", apiKey))
		buf.WriteString(fmt.Sprintf("gemini_model = %q\n", DefaultGeminiModel))
		buf.WriteString(fmt.Sprintf("max_tokens = %d\n", DefaultMaxTokens))
		buf.WriteString(fmt.Sprintf("request_timeout_seconds = %d\n", DefaultTimeoutSecs))
		buf.WriteString(fmt.Sprintf("auto_stage = %v\n", DefaultAutoStage))
		buf.WriteString(fmt.Sprintf("auto_push = %v\n", DefaultAutoPush))
		buf.WriteString(fmt.Sprintf("push_command = %q\n", DefaultPushCommand))
		buf.WriteString(fmt.Sprintf("verbose = %v\n", DefaultVerbose))

		// Write the multiline prompt using TOML's multiline string syntax
		buf.WriteString("prompt = '''\n")
		buf.WriteString(DefaultPrompt)
		buf.WriteString("\n'''\n")

		configContent = buf.Bytes()

	} else if statErr == nil {
		// --- UPDATE existing config file ---
		// Read the existing file content
		existingContent, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("failed to read existing config file %s: %w", configPath, readErr)
		}

		// Decode into a generic map to preserve structure and comments as much as possible
		var cfgMap map[string]interface{}
		if _, err := toml.Decode(string(existingContent), &cfgMap); err != nil {
			return fmt.Errorf("failed to decode existing config file %s for update: %w. Please check the file format", configPath, err)
		}

		// Update the API key in the map
		cfgMap["gemini_api_key"] = apiKey

		// Create a buffer for the updated TOML content
		var buf bytes.Buffer

		// Write the configuration values manually to ensure proper multiline string formatting
		buf.WriteString(fmt.Sprintf("gemini_api_key = %q\n", apiKey))
		if model, ok := cfgMap["gemini_model"].(string); ok {
			buf.WriteString(fmt.Sprintf("gemini_model = %q\n", model))
		}
		if maxTokens, ok := cfgMap["max_tokens"].(int64); ok {
			buf.WriteString(fmt.Sprintf("max_tokens = %d\n", maxTokens))
		}
		if timeout, ok := cfgMap["request_timeout_seconds"].(int64); ok {
			buf.WriteString(fmt.Sprintf("request_timeout_seconds = %d\n", timeout))
		}
		if autoStage, ok := cfgMap["auto_stage"].(bool); ok {
			buf.WriteString(fmt.Sprintf("auto_stage = %v\n", autoStage))
		}
		if autoPush, ok := cfgMap["auto_push"].(bool); ok {
			buf.WriteString(fmt.Sprintf("auto_push = %v\n", autoPush))
		}
		if pushCommand, ok := cfgMap["push_command"].(string); ok {
			buf.WriteString(fmt.Sprintf("push_command = %q\n", pushCommand))
		}
		if verbose, ok := cfgMap["verbose"].(bool); ok {
			buf.WriteString(fmt.Sprintf("verbose = %v\n", verbose))
		}

		// Write the multiline prompt using TOML's multiline string syntax
		if prompt, ok := cfgMap["prompt"].(string); ok {
			buf.WriteString("prompt = '''\n")
			buf.WriteString(prompt)
			buf.WriteString("\n'''\n")
		}

		configContent = buf.Bytes()

	} else {
		// Other error checking the file
		return fmt.Errorf("failed to check user config file %s: %w", configPath, statErr)
	}

	// --- Write the config content (either new or updated) ---
	// Write to a temporary file first for atomicity
	tmpFile, err := os.CreateTemp(dir, filepath.Base(configPath)+".*.tmp")
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
	if _, err = tmpFile.Write(configContent); err != nil {
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
	if err = os.Rename(tmpPath, configPath); err != nil {
		// Attempt to remove the temp file if rename fails
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save config file (rename failed): %w", err)
	}

	// Reset err to nil on success before defer cleanup check
	err = nil
	return nil
}
