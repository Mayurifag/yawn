package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckSSHKeysAvailable checks if ssh-add is available and if it reports any SSH keys
// Return values:
// - true, nil: SSH keys are available
// - false, nil: SSH agent has no identities (keys)
// - false, err: Error checking keys (including if ssh-add is not found)
func CheckSSHKeysAvailable() (bool, error) {
	// Check if ssh-add exists
	_, err := exec.LookPath("ssh-add")
	if err != nil {
		// ssh-add not found, return error
		return false, fmt.Errorf("ssh-add command not found: %w", err)
	}

	// Run ssh-add -l to check for keys
	cmd := exec.Command("ssh-add", "-l")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Command succeeded - keys are available
	if err == nil {
		return true, nil
	}

	// Command failed with exit code, check if it's the "no identities" error
	_, ok := err.(*exec.ExitError)
	if ok && strings.Contains(outputStr, "The agent has no identities") {
		// Agent is running but has no keys
		return false, nil
	}

	// Some other error occurred
	return false, err
}
