package git

import (
	"os/exec"
	"testing"
)

// TestCheckSSHKeysAvailable does basic validation of the ssh key check logic
// Note: This is not a comprehensive test as we can't easily mock exec.Command,
// so it relies on the local environment's ssh-agent state.
func TestCheckSSHKeysAvailable(t *testing.T) {
	// We'll just check that the function runs without panicking
	// and returns some boolean value and possibly an error
	available, err := CheckSSHKeysAvailable()

	// We're just testing that the function returns without panicking
	// The actual result will depend on the local environment
	t.Logf("CheckSSHKeysAvailable() = %v, err = %v", available, err)

	// If ssh-add doesn't exist, we should now get (false, error)
	_, lookPathErr := exec.LookPath("ssh-add")
	if lookPathErr != nil {
		if available || err == nil {
			t.Errorf("When ssh-add is not available, expected (false, error), got (%v, %v)", available, err)
		}
	}
}
