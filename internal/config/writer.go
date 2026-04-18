package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func GenerateConfigContent(apiKey string) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("# Configuration file for yawn - AI Git Committer using Google Gemini\n#\n# Placement:\n#   ~/.config/yawn/config.toml (user config)\n#   ./.yawn.toml (project config, add to .gitignore)\n#\n# Precedence: CLI flags > env vars > project config > user config > defaults\n# Uncomment and change only the values you want to override.\n\n")

	fmt.Fprintf(&buf, "gemini_api_key = %q\n\n", apiKey)

	fmt.Fprintf(&buf, "# gemini_model = %q\n", DefaultGeminiModel)
	fmt.Fprintf(&buf, "# request_timeout_seconds = %d\n", DefaultTimeoutSecs)
	fmt.Fprintf(&buf, "# auto_stage = %v\n", DefaultAutoStage)
	fmt.Fprintf(&buf, "# auto_push = %v\n", DefaultAutoPush)
	fmt.Fprintf(&buf, "# push_command = %q\n", DefaultPushCommand)
	fmt.Fprintf(&buf, "# wait_for_ssh_keys = %v\n", DefaultWaitForSSHKeys)
	fmt.Fprintf(&buf, "# squash_auto_push = %v\n", DefaultSquashAutoPush)
	buf.WriteString("\n")

	buf.WriteString("# prompt = '''\n")
	for _, line := range strings.Split(DefaultPrompt, "\n") {
		fmt.Fprintf(&buf, "# %s\n", line)
	}
	buf.WriteString("# '''\n")

	return buf.Bytes(), nil
}

func SaveAPIKeyToUserConfig(apiKey string) error {
	configPath, err := ensureUserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to prepare user config directory: %w", err)
	}

	var configContent []byte

	_, statErr := os.Stat(configPath)
	if os.IsNotExist(statErr) {
		configContent, err = GenerateConfigContent(apiKey)
		if err != nil {
			return fmt.Errorf("failed to generate new config content: %w", err)
		}
	} else if statErr == nil {
		existingContent, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("failed to read existing config file %s: %w", configPath, readErr)
		}
		configContent, err = updateExistingConfigContent(existingContent, apiKey)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("failed to check user config file %s: %w", configPath, statErr)
	}

	return writeConfigFileAtomically(configContent, configPath)
}

func updateExistingConfigContent(existingContent []byte, apiKey string) ([]byte, error) {
	newLine := fmt.Sprintf("gemini_api_key = %q", apiKey)
	lines := strings.Split(string(existingContent), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "gemini_api_key") {
			lines[i] = newLine
			return []byte(strings.Join(lines, "\n")), nil
		}
	}
	return []byte(newLine + "\n" + string(existingContent)), nil
}

func writeConfigFileAtomically(content []byte, targetPath string) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary config file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temporary config file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary config file: %w", err)
	}

	if err = os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions on temporary config file: %w", err)
	}

	if err = os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to save config file (rename failed): %w", err)
	}

	return nil
}
