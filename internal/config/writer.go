package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

func ensureUserConfigDir() (string, error) {
	configPath, err := getUserConfigPath()
	if err != nil {
		return "", err
	}

	yawnConfigDir := filepath.Dir(configPath)

	if err := os.MkdirAll(yawnConfigDir, 0700); err != nil {
		if stat, statErr := os.Stat(yawnConfigDir); statErr == nil && !stat.IsDir() {
			return "", fmt.Errorf("user config path %s exists but is not a directory", yawnConfigDir)
		}
		return "", fmt.Errorf("failed to create user config directory %s: %w", yawnConfigDir, err)
	}

	return configPath, nil
}

func GenerateConfigContent() []byte {
	var buf bytes.Buffer

	buf.WriteString("# Configuration file for yawn - AI Git Committer\n#\n# Placement:\n#   ~/.config/yawn/config.toml (user config)\n#   ./.yawn.toml (project config, add to .gitignore)\n#\n# Precedence: CLI flags > env vars > project config > user config > defaults\n# Uncomment and change only the values you want to override.\n\n")

	fmt.Fprintf(&buf, "main_provider = %q\n", DefaultProvider)
	buf.WriteString("# fallback_provider = \"opencode_cli\"\n\n")

	fmt.Fprintf(&buf, "# request_timeout_seconds = %d # Timeout per provider attempt\n", DefaultTimeoutSecs)
	fmt.Fprintf(&buf, "# auto_stage = %v\n", DefaultAutoStage)
	fmt.Fprintf(&buf, "# auto_push = %v\n", DefaultAutoPush)
	fmt.Fprintf(&buf, "# push_command = %q\n", DefaultPushCommand)
	fmt.Fprintf(&buf, "# wait_for_ssh_keys = %v\n", DefaultWaitForSSHKeys)
	fmt.Fprintf(&buf, "# squash_auto_push = %v # Ask before overwriting commits outside the pre-squash history\n", DefaultSquashAutoPush)
	buf.WriteString("\n")

	buf.WriteString("# prompt = '''\n")
	for _, line := range strings.Split(DefaultPrompt, "\n") {
		fmt.Fprintf(&buf, "# %s\n", line)
	}
	buf.WriteString("# '''\n")
	buf.WriteString("\n")

	buf.WriteString("# Provider examples:\n")
	fmt.Fprintf(&buf, "#   main_provider = %q\n", ProviderOpenCodeCLI)
	fmt.Fprintf(&buf, "#   fallback_provider = %q\n\n", ProviderGemini)

	buf.WriteString("[providers.gemini]\n")
	buf.WriteString("api_key = \"\"\n")
	fmt.Fprintf(&buf, "# model = %q\n\n", DefaultGeminiModel)

	buf.WriteString("# [providers.opencode_cli]\n")
	fmt.Fprintf(&buf, "# model = %q\n", DefaultOpenCodeCLIModel)

	return buf.Bytes()
}

func SaveProviderAPIKeyToUserConfig(provider, apiKey string) error {
	configPath, err := ensureUserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to prepare user config directory: %w", err)
	}

	var configContent []byte
	_, statErr := os.Stat(configPath)
	if os.IsNotExist(statErr) {
		configContent = GenerateConfigContent()
	} else if statErr == nil {
		configContent, err = os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config file %s: %w", configPath, err)
		}
	} else {
		return fmt.Errorf("failed to check user config file %s: %w", configPath, statErr)
	}

	configContent, err = updateExistingConfigContent(configContent, provider, apiKey)
	if err != nil {
		return err
	}
	return writeConfigFileAtomically(configContent, configPath)
}

func updateExistingConfigContent(existingContent []byte, provider, apiKey string) ([]byte, error) {
	provider = NormalizeProvider(provider)
	var expected map[string]any
	if _, err := toml.Decode(string(existingContent), &expected); err != nil {
		return nil, fmt.Errorf("failed to parse existing config: %w", err)
	}
	expected["main_provider"] = provider
	if expected["providers"] == nil {
		expected["providers"] = map[string]any{}
	}
	providers, ok := expected["providers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("providers must be a TOML table")
	}
	if providers[provider] == nil {
		providers[provider] = map[string]any{}
	}
	providerValues, ok := providers[provider].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider %q must be a TOML table", provider)
	}
	providerValues["api_key"] = apiKey

	content := updateConfigKeyContent(existingContent, "main_provider", provider)
	content = updateProviderConfigKeyContent(content, provider, "api_key", apiKey)
	var updated map[string]any
	if _, err := toml.Decode(string(content), &updated); err != nil || !reflect.DeepEqual(expected, updated) {
		return nil, fmt.Errorf("cannot safely update this TOML layout; set providers.%s.api_key manually", provider)
	}
	return content, nil
}

func updateConfigKeyContent(existingContent []byte, key, value string) []byte {
	newLine := fmt.Sprintf("%s = %q", key, value)
	lines := strings.Split(string(existingContent), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = newLine
			return []byte(strings.Join(lines, "\n"))
		}
	}
	return []byte(newLine + "\n" + string(existingContent))
}

func updateProviderConfigKeyContent(existingContent []byte, provider, key, value string) []byte {
	table := fmt.Sprintf("[providers.%s]", NormalizeProvider(provider))
	newLine := fmt.Sprintf("%s = %q", key, value)
	lines := strings.Split(string(existingContent), "\n")
	inTable := false
	insertAt := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inTable {
				insertAt = i
				break
			}
			inTable = trimmed == table
			if inTable {
				insertAt = i + 1
			}
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = newLine
			return []byte(strings.Join(lines, "\n"))
		}
		insertAt = i + 1
	}

	if !inTable {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, table, newLine)
		return []byte(strings.Join(lines, "\n"))
	}

	lines = append(lines[:insertAt], append([]string{newLine}, lines[insertAt:]...)...)
	return []byte(strings.Join(lines, "\n"))
}

func writeConfigFileAtomically(content []byte, targetPath string) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err = tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temporary config file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}

	if err = os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to save config file (rename failed): %w", err)
	}

	return nil
}
