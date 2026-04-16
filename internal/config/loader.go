package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type envField struct {
	envKey string
	srcKey string
	apply  func(cfg *Config, val string) bool
}

var envFields = []envField{
	{EnvPrefix + "GEMINI_API_KEY", "GeminiAPIKey", func(c *Config, v string) bool {
		c.GeminiAPIKey = v
		return true
	}},
	{EnvPrefix + "GEMINI_MODEL", "GeminiModel", func(c *Config, v string) bool {
		c.GeminiModel = v
		return true
	}},
	{EnvPrefix + "PROMPT", "Prompt", func(c *Config, v string) bool {
		c.Prompt = v
		return true
	}},
	{EnvPrefix + "PUSH_COMMAND", "PushCommand", func(c *Config, v string) bool {
		c.PushCommand = v
		return true
	}},
	{EnvPrefix + "REQUEST_TIMEOUT_SECONDS", "RequestTimeoutSeconds", func(c *Config, v string) bool {
		n, err := strconv.Atoi(v)
		if err != nil {
			return false
		}
		c.RequestTimeoutSeconds = n
		return true
	}},
	{EnvPrefix + "AUTO_STAGE", "AutoStage", func(c *Config, v string) bool {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		c.AutoStage = b
		return true
	}},
	{EnvPrefix + "AUTO_PUSH", "AutoPush", func(c *Config, v string) bool {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		c.AutoPush = b
		return true
	}},
	{EnvPrefix + "WAIT_FOR_SSH_KEYS", "WaitForSSHKeys", func(c *Config, v string) bool {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		c.WaitForSSHKeys = b
		return true
	}},
	{EnvPrefix + "SQUASH_AUTO_PUSH", "SquashAutoPush", func(c *Config, v string) bool {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		c.SquashAutoPush = b
		return true
	}},
}

func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(xdgConfigHome, UserConfigDirName, UserConfigFileName), nil
}

func loadUserConfig() (Config, toml.MetaData, error) {
	userConfigPath, err := getUserConfigPath()
	if err != nil {
		return Config{}, toml.MetaData{}, nil
	}

	if _, err := os.Stat(userConfigPath); err != nil {
		if os.IsNotExist(err) {
			return Config{}, toml.MetaData{}, nil
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

func findProjectConfig(startPath string) string {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, ProjectConfigName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		} else if !os.IsNotExist(err) {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func loadProjectConfig(projectPath string) (Config, toml.MetaData, error) {
	projectConfigPath := findProjectConfig(projectPath)
	if projectConfigPath == "" {
		return Config{}, toml.MetaData{}, nil
	}

	var loadedCfg Config
	metadata, decodeErr := toml.DecodeFile(projectConfigPath, &loadedCfg)
	if decodeErr != nil {
		return Config{}, toml.MetaData{}, fmt.Errorf("failed to load project config from %s: %w", projectConfigPath, decodeErr)
	}

	return loadedCfg, metadata, nil
}

func mergeConfig(base *Config, loaded Config, meta toml.MetaData, src string) {
	bv := reflect.ValueOf(base).Elem()
	lv := reflect.ValueOf(loaded)
	t := bv.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tomlKey := strings.SplitN(field.Tag.Get("toml"), ",", 2)[0]
		if tomlKey == "" || tomlKey == "-" {
			continue
		}
		if !meta.IsDefined(tomlKey) {
			continue
		}
		bv.Field(i).Set(lv.Field(i))
		base.sources[field.Name] = src
	}
}

func loadConfigFromEnv(cfg *Config) {
	for _, f := range envFields {
		if v := os.Getenv(f.envKey); v != "" {
			if f.apply(cfg, v) {
				cfg.sources[f.srcKey] = "env"
			}
		}
	}
}

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.APIKey != nil && *flags.APIKey != "" {
		cfg.GeminiAPIKey = *flags.APIKey
		cfg.sources["GeminiAPIKey"] = "flag"
	}
	if flags.AutoStage != nil {
		cfg.AutoStage = *flags.AutoStage
		cfg.sources["AutoStage"] = "flag"
	}
	if flags.AutoPush != nil {
		cfg.AutoPush = *flags.AutoPush
		cfg.sources["AutoPush"] = "flag"
	}
}

func LoadConfig(projectPath string, flags CLIFlags) (Config, error) {
	cfg := defaultConfig()
	cfg.sources = make(map[string]string)
	t := reflect.TypeOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tomlKey := strings.SplitN(field.Tag.Get("toml"), ",", 2)[0]
		if tomlKey == "" || tomlKey == "-" {
			continue
		}
		cfg.sources[field.Name] = "default"
	}

	userCfg, userMeta, err := loadUserConfig()
	if err != nil {
		return cfg, fmt.Errorf("failed to apply user configuration: %w", err)
	}
	if len(userMeta.Keys()) > 0 {
		mergeConfig(&cfg, userCfg, userMeta, "user home config")
	}

	projectCfg, projectMeta, err := loadProjectConfig(projectPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to apply project configuration: %w", err)
	}
	if len(projectMeta.Keys()) > 0 {
		mergeConfig(&cfg, projectCfg, projectMeta, "project")
	}

	loadConfigFromEnv(&cfg)
	applyFlags(&cfg, flags)

	return cfg, nil
}
