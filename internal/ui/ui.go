package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"golang.org/x/term"
)

var (
	promptPrefix  = color.New(color.FgYellow).Sprint("? ") // Yellow question mark
	infoPrefix    = color.New(color.FgBlue).Sprint("* ")   // Blue asterisk
	errorPrefix   = color.New(color.FgRed).Sprint("! ")    // Red exclamation mark
	successPrefix = color.New(color.FgGreen).Sprint("✓ ")  // Green checkmark

	// For reading input
	reader = bufio.NewReader(os.Stdin)
)

// AskYesNo prompts the user with a yes/no question and returns the boolean result.
// It suggests a default answer (Y/n or y/N). Enter accepts the default.
func AskYesNo(prompt string, defaultYes bool) bool {
	var hint string
	if defaultYes {
		hint = "[Y/n]"
	} else {
		hint = "[y/N]"
	}

	// Ensure the prompt is correctly colored with the promptPrefix
	fmt.Printf("%s%s %s ", promptPrefix, prompt, hint)

	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	result := defaultYes
	if input != "" {
		result = input == "y" || input == "yes"

		ClearLine()
	}

	return result
}

// AskForInput prompts the user for text input.
// If required is true, it will loop until non-empty input is received.
func AskForInput(prompt string, required bool) string {
	for {
		// Ensure the prompt is correctly colored with the promptPrefix
		fmt.Printf("%s%s ", promptPrefix, prompt)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "" || !required {
			ClearLine()
			return input
		}
		PrintError("Input cannot be empty.")
	}
}

// PrintInfo displays an informational message.
func PrintInfo(message string) {
	fmt.Printf("%s %s\n", infoPrefix, message)
}

// PrintSuccess displays a success message.
func PrintSuccess(message string) {
	fmt.Printf("%s %s\n", successPrefix, color.GreenString(message))
}

// PrintError displays an error message.
func PrintError(message string) {
	// Use Fprintf to stderr for errors
	fmt.Fprintf(os.Stderr, "%s %s\n", errorPrefix, color.RedString(message))
}

// StartSpinner starts a CLI spinner with the given message.
func StartSpinner(message string) *spinner.Spinner {
	// Check if running in a TTY, disable spinner if not
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		PrintInfo(message + "...") // Print static message if not a TTY
		return nil                 // Return nil to indicate no active spinner
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Use a nice spinner character set
	s.Suffix = " " + message
	if err := s.Color("cyan"); err != nil {
		// Color is not critical, continue with default color
		if s != nil && s.Writer != nil {
			fmt.Fprintf(s.Writer, "Warning: Failed to set spinner color: %v\n", err)
		}
	}
	s.Start()
	return s
}

// StopSpinner stops the given spinner. If the spinner is nil (e.g., not a TTY), it does nothing.
func StopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

// ClearLine clears the current line in the terminal (useful after spinner).
// This might not be needed if spinner cleans up properly, but can be useful.
func ClearLine() {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Print("\033[1A\r\033[K") // Move up one line, carriage return, clear line
	}
}
