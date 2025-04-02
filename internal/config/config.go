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
	DefaultAskStage    = true
	DefaultAutoPush    = false
	DefaultPushCommand = "git push origin HEAD"
	DefaultVerbose     = false
	DefaultPrompt      = `Generate a commit message.

- Limit subject to 50 characters
- Follow Conventional Commits standard (https://www.conventionalcommits.org/en/v1.0.0/)
- Use gitmoji on the subject line end if applicable
- Generate just a commit message, no other text, no other formatting, no other comments, no ticks
- Try to generate a commit message that is descriptive of the changes, but not too verbose
- If there are multiple changes, try to mention them all in the commit message in body and generate good subject line, descriptive of the changes

The commit message MUST be structured as follows (do not use --- symbols, thats just an example to show the format):
---
<type>[optional scope]: <subject>

[optional body]

[optional footer(s)]
---

After that line will be the diff to analyze, do not use it in your response:

{{Diff}}
`
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
	if noStageFlag {
		cfg.AskStage = false // --no-stage overrides config/env to disable asking
		cfg.sources["AskStage"] = "flag"
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
		IgnorePatterns:        []string{},
		AskStage:              DefaultAskStage,
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
	// For slices, replace if defined (toml decoder might make empty slice vs nil)
	if metadata.IsDefined("ignore_patterns") {
		baseCfg.IgnorePatterns = loadedCfg.IgnorePatterns // Replace even if empty list in file
		baseCfg.sources["IgnorePatterns"] = source
	}
	// Booleans need explicit check if they were defined in the file.
	if metadata.IsDefined("ask_stage") {
		baseCfg.AskStage = loadedCfg.AskStage
		baseCfg.sources["AskStage"] = source
	}
	if metadata.IsDefined("auto_push") {
		baseCfg.AutoPush = loadedCfg.AutoPush
		baseCfg.sources["AutoPush"] = source
	}
	if metadata.IsDefined("verbose") {
		baseCfg.Verbose = loadedCfg.Verbose
		baseCfg.sources["Verbose"] = source
	}
	if metadata.IsDefined("push_command") && loadedCfg.PushCommand != "" {
		baseCfg.PushCommand = loadedCfg.PushCommand
		baseCfg.sources["PushCommand"] = source
	}
}

func loadConfigFromEnv(cfg *Config) {
	source := "env"
	// String values
	if val, ok := os.LookupEnv(EnvPrefix + "GEMINI_API_KEY"); ok {
		cfg.GeminiAPIKey = val
		cfg.sources["GeminiAPIKey"] = source
	}
	if val, ok := os.LookupEnv(EnvPrefix + "GEMINI_MODEL"); ok {
		cfg.GeminiModel = val
		cfg.sources["GeminiModel"] = source
	}
	if val, ok := os.LookupEnv(EnvPrefix + "PROMPT"); ok {
		// Handle potential escaped newlines from env var
		val = strings.ReplaceAll(val, "\\n", "\n")
		cfg.Prompt = val
		cfg.sources["Prompt"] = source
	}
	if val, ok := os.LookupEnv(EnvPrefix + "PUSH_COMMAND"); ok {
		cfg.PushCommand = val
		cfg.sources["PushCommand"] = source
	}
	if val, ok := os.LookupEnv(EnvPrefix + "IGNORE_PATTERNS"); ok {
		patterns := strings.Split(val, ",")
		// Trim whitespace from each pattern
		validPatterns := make([]string, 0, len(patterns))
		for _, p := range patterns {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				validPatterns = append(validPatterns, trimmed)
			}
		}
		if len(validPatterns) > 0 { // Only override if env var is not empty/just commas
			cfg.IgnorePatterns = validPatterns
			cfg.sources["IgnorePatterns"] = source
		}
	}

	// Integer values
	if val, ok := os.LookupEnv(EnvPrefix + "MAX_TOKENS"); ok {
		if intVal, err := strconv.Atoi(val); err == nil && intVal > 0 { // Ensure positive value
			cfg.MaxTokens = intVal
			cfg.sources["MaxTokens"] = source
		} else if cfg.Verbose { // Check verbosity *after* potential env override
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Invalid integer value for %sMAX_TOKENS: %s\n", EnvPrefix, val)
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "REQUEST_TIMEOUT_SECONDS"); ok {
		if intVal, err := strconv.Atoi(val); err == nil && intVal > 0 {
			cfg.RequestTimeoutSeconds = intVal
			cfg.sources["RequestTimeoutSeconds"] = source
		} else if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Invalid integer value for %sREQUEST_TIMEOUT_SECONDS: %s\n", EnvPrefix, val)
		}
	}

	// Boolean values
	if val, ok := os.LookupEnv(EnvPrefix + "ASK_STAGE"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.AskStage = boolVal
			cfg.sources["AskStage"] = source
		} else if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Invalid boolean value for %sASK_STAGE: %s\n", EnvPrefix, val)
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "AUTO_PUSH"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.AutoPush = boolVal
			cfg.sources["AutoPush"] = source
		} else if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Invalid boolean value for %sAUTO_PUSH: %s\n", EnvPrefix, val)
		}
	}
	if val, ok := os.LookupEnv(EnvPrefix + "VERBOSE"); ok {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			cfg.Verbose = boolVal
			cfg.sources["Verbose"] = source
		} else if cfg.Verbose { // Check cfg.Verbose because this might be the var enabling it
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: Invalid boolean value for %sVERBOSE: %s\n", EnvPrefix, val)
		}
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
	cfg := defaultConfig()
	var builder strings.Builder

	builder.WriteString("# " + AppName + " Configuration File (`~/.config/yawn/config.toml` or `./.yawn.toml`)\n")
	builder.WriteString("# Generated by 'yawn --generate-config'\n\n")

	builder.WriteString("# Gemini API Key. REQUIRED.\n")
	builder.WriteString("# Get one from Google AI Studio: https://aistudio.google.com/app/apikey\n")
	builder.WriteString("# Can also be set via YAWN_GEMINI_API_KEY environment variable.\n")
	builder.WriteString(`gemini_api_key = "" # <-- Add your key here or set ENV var` + "\n\n")

	builder.WriteString("# Gemini model to use for generating commit messages.\n")
	builder.WriteString("# See available models: https://ai.google.dev/models/gemini\n")
	builder.WriteString(fmt.Sprintf("gemini_model = %q\n\n", cfg.GeminiModel))

	builder.WriteString("# Maximum number of tokens (input + output) allowed for the Gemini request.\n")
	builder.WriteString("# Helps prevent excessive costs and errors for large diffs.\n")
	builder.WriteString("# Models like Flash have large context windows (e.g., 1M tokens), but limiting can save costs.\n")
	builder.WriteString("# Estimate: ~4 chars/token. Diff + Prompt + Output must be <= max_tokens.\n")
	builder.WriteString(fmt.Sprintf("max_tokens = %d\n\n", cfg.MaxTokens))

	builder.WriteString("# Request timeout in seconds when calling the Gemini API.\n")
	builder.WriteString("# Increase if you have large diffs or slow network.\n")
	builder.WriteString(fmt.Sprintf("request_timeout_seconds = %d\n\n", cfg.RequestTimeoutSeconds))

	builder.WriteString("# Prompt template sent to Gemini.\n")
	builder.WriteString("# Use {{Diff}} as the placeholder for the git diff.\n")
	builder.WriteString("# Ensure the prompt guides the AI towards the desired commit message format (e.g., Conventional Commits).\n")
	// Use %q with backticks for multi-line string literal representation in Go source
	// but write it out with ''' for the TOML file. Ensure the prompt string itself doesn't contain '''
	tomlPrompt := strings.ReplaceAll(cfg.Prompt, "'''", `''"`) // Basic escaping if needed
	builder.WriteString(fmt.Sprintf("prompt = '''\n%s\n'''\n\n", tomlPrompt))

	builder.WriteString("# List of glob patterns for files/paths to exclude from the diff sent to the AI.\n")
	builder.WriteString("# Uses git's pathspec matching (e.g., '*.log', 'dist/', ':(exclude)vendor/**').\n")
	builder.WriteString("# See: https://git-scm.com/docs/gitglossary#Documentation/gitglossary.txt-aiddefpathspecapathspec\n")
	builder.WriteString(fmt.Sprintf("ignore_patterns = %s\n\n", formatStringSlice(cfg.IgnorePatterns)))

	builder.WriteString("# If true and no changes are staged, ask interactively whether to stage all changes (`git add -A`).\n")
	builder.WriteString("# If false, proceed only if changes are already staged (or fail if none).\n")
	builder.WriteString("# Can be overridden by the --no-stage flag (sets ask_stage=false for that run).\n")
	builder.WriteString(fmt.Sprintf("ask_stage = %t\n\n", cfg.AskStage))

	builder.WriteString("# If true, automatically push the commit after it's successfully created using 'push_command'.\n")
	builder.WriteString("# If false, ask the user for confirmation before pushing.\n")
	builder.WriteString("# Can be overridden by the --auto-push flag (sets auto_push=true for that run).\n")
	builder.WriteString(fmt.Sprintf("auto_push = %t\n\n", cfg.AutoPush))

	builder.WriteString("# The exact command used to push the commit when auto_push is true or confirmed interactively.\n")
	builder.WriteString("# Examples: \"git push\", \"git push --no-verify\", \"git push origin main\"\n")
	builder.WriteString(fmt.Sprintf("push_command = %q\n\n", cfg.PushCommand))

	builder.WriteString("# Enable verbose logging output.\n")
	builder.WriteString("# Shows config loading details, git commands being run, etc.\n")
	builder.WriteString("# Can be overridden by the --verbose flag or YAWN_VERBOSE=true env var.\n")
	builder.WriteString(fmt.Sprintf("verbose = %t\n", cfg.Verbose))

	return builder.String(), nil
}

// formatStringSlice formats a slice of strings for TOML array output.
func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, s := range slice {
		// Use %q to ensure proper quoting and escaping within the string literal
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
	// Convert slice to string for simpler display if needed, or handle slice display
	ignorePatternsStr := formatStringSlice(c.IgnorePatterns) // Use TOML-like format

	return map[string]interface{}{
		"GeminiAPIKey":          c.GeminiAPIKey, // Will be masked below
		"GeminiModel":           c.GeminiModel,
		"MaxTokens":             c.MaxTokens,
		"RequestTimeoutSeconds": c.RequestTimeoutSeconds,
		"Prompt":                c.Prompt, // Will be truncated below
		"IgnorePatterns":        ignorePatternsStr,
		"AskStage":              c.AskStage,
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
		"Prompt", "IgnorePatterns", "AskStage", "AutoPush", "PushCommand", "Verbose",
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
		case "IgnorePatterns":
			// Already formatted as TOML-like string in toMap
			valueStr = configMap[key].(string)
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
		// --- Create NEW config file using GenerateDefaultConfig format ---
		defaultToml, genErr := GenerateDefaultConfig()
		if genErr != nil {
			return fmt.Errorf("failed to generate default config content: %w", genErr)
		}

		// Replace the placeholder API key line using a more robust method
		lines := strings.Split(defaultToml, "\n")
		found := false
		for i, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, `gemini_api_key =`) {
				lines[i] = fmt.Sprintf(`gemini_api_key = %q`, apiKey)
				found = true
				break
			}
		}

		if !found {
			// Fallback if placeholder format changes: prepend the key if not found
			fmt.Fprintf(os.Stderr, "[CONFIG] Warning: API key placeholder line not found in generated config, prepending key.\n")
			configContent = []byte(fmt.Sprintf("gemini_api_key = %q\n\n%s", apiKey, defaultToml))
		} else {
			configContent = []byte(strings.Join(lines, "\n"))
		}

	} else if statErr == nil {
		// --- UPDATE existing config file ---
		// Read the existing file content
		existingContent, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("failed to read existing config file %s: %w", configPath, readErr)
		}

		// Decode into a generic map to preserve structure and comments as much as possible
		// Note: BurntSushi/toml doesn't perfectly preserve comments on encode, but it preserves structure.
		var cfgMap map[string]interface{}
		if _, err := toml.Decode(string(existingContent), &cfgMap); err != nil {
			// If decoding fails, maybe the file is corrupt? Overwrite might be an option, but safer to error out.
			return fmt.Errorf("failed to decode existing config file %s for update: %w. Please check the file format", configPath, err)
		}

		// Update the API key in the map
		cfgMap["gemini_api_key"] = apiKey

		// Encode the updated map back to TOML
		var buf bytes.Buffer
		encoder := toml.NewEncoder(&buf)
		// Try to preserve indentation if possible (though often defaults are fine)
		// encoder.Indent = "    " // Example: 4 spaces
		if err := encoder.Encode(cfgMap); err != nil {
			return fmt.Errorf("failed to encode updated config: %w", err)
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
