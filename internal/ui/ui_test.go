package ui

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskYesNo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		expected   bool
	}{
		{"default yes - empty input", "", true, true},
		{"default yes - y", "y", true, true},
		{"default yes - n", "n", true, false},
		{"default no - empty input", "", false, false},
		{"default no - y", "y", false, true},
		{"default no - n", "n", false, false},
		{"case insensitive", "Y", true, true},
		{"full word", "yes", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to simulate user input
			r, w, err := os.Pipe()
			require.NoError(t, err)
			oldStdin := os.Stdin
			os.Stdin = r

			// Write the test input
			go func() {
				w.WriteString(tt.input + "\n")
				w.Close()
			}()

			// Run the test
			result := AskYesNo("Test prompt", tt.defaultYes)

			// Restore stdin
			os.Stdin = oldStdin

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAskForInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		required bool
		expected string
	}{
		{"required - valid input", "test input", true, "test input"},
		{"required - empty input then valid", "\ntest input", true, "test input"},
		{"not required - empty input", "", false, ""},
		{"not required - valid input", "test input", false, "test input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to simulate user input
			r, w, err := os.Pipe()
			require.NoError(t, err)
			oldStdin := os.Stdin
			os.Stdin = r

			// Write the test input
			go func() {
				w.WriteString(tt.input + "\n")
				w.Close()
			}()

			// Run the test
			result := AskForInput("Test prompt", tt.required)

			// Restore stdin
			os.Stdin = oldStdin

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintMessages(t *testing.T) {
	tests := []struct {
		name     string
		function func(string)
		message  string
		expected string
	}{
		{"PrintInfo", PrintInfo, "test info", "* \x1b[34mtest info\x1b[0m\n"},
		{"PrintSuccess", PrintSuccess, "test success", "✓ \x1b[32mtest success\x1b[0m\n"},
		{"PrintError", PrintError, "test error", "! \x1b[31mtest error\x1b[0m\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to capture output
			r, w, err := os.Pipe()
			require.NoError(t, err)

			// Replace stdout/stderr with our pipe
			if tt.name == "PrintError" {
				oldStderr := os.Stderr
				os.Stderr = w
				defer func() { os.Stderr = oldStderr }()
			} else {
				oldStdout := os.Stdout
				os.Stdout = w
				defer func() { os.Stdout = oldStdout }()
			}

			// Run the test
			tt.function(tt.message)

			// Close the write end of the pipe
			w.Close()

			// Read the output
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(buf[:n]))
		})
	}
}

func TestSpinner(t *testing.T) {
	// Test spinner creation and cleanup
	s := StartSpinner("test spinner")
	require.NotNil(t, s)

	// Give the spinner a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the spinner
	StopSpinner(s)

	// Clear the line
	ClearLine()
}

func TestPrintRepoLink(t *testing.T) {
	// Create a pipe to capture output
	r, w, err := os.Pipe()
	require.NoError(t, err)

	// Replace stdout with our pipe
	oldStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	// Run the test
	PrintRepoLink("View repository:", "https://github.com/test/repo")

	// Close the write end of the pipe
	w.Close()

	// Read the output
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	require.NoError(t, err)

	expected := "View repository: \x1b[34mhttps://github.com/test/repo\x1b[0m\n"
	assert.Equal(t, expected, string(buf[:n]))
}
