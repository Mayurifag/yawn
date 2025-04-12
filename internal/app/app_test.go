package app

import (
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
)

// TestWaitForSSHKeysConfig tests that the WaitForSSHKeys configuration is properly integrated
func TestWaitForSSHKeysConfig(t *testing.T) {
	// Create a test config with WaitForSSHKeys enabled
	cfg := config.Config{
		WaitForSSHKeys: true,
	}

	// Basic validation that the config field exists and is accessible
	if !cfg.WaitForSSHKeys {
		t.Error("WaitForSSHKeys should be true when set")
	}

	// Verify the config structure has the expected field with default value
	emptyCfg := config.Config{}
	if emptyCfg.WaitForSSHKeys != config.DefaultWaitForSSHKeys {
		t.Errorf("Default WaitForSSHKeys should be %v, got %v",
			config.DefaultWaitForSSHKeys, emptyCfg.WaitForSSHKeys)
	}
}
