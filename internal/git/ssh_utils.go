package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const noIdentitiesMsg = "The agent has no identities"

var ErrSSHAddNotFound = errors.New("ssh-add not found in PATH")

func CheckSSHKeysAvailable() (bool, error) {
	_, err := exec.LookPath("ssh-add")
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrSSHAddNotFound, err)
	}

	cmd := exec.Command("ssh-add", "-l")
	output, err := cmd.CombinedOutput()

	if err == nil {
		return true, nil
	}

	if _, ok := err.(*exec.ExitError); ok && strings.Contains(string(output), noIdentitiesMsg) {
		return false, nil
	}

	return false, err
}
