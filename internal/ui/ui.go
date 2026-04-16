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
	promptPrefix  = color.New(color.FgYellow).Sprint("? ")
	infoPrefix    = color.New(color.FgBlue).Sprint("* ")
	errorPrefix   = color.New(color.FgRed).Sprint("! ")
	successPrefix = color.New(color.FgGreen).Sprint("✓ ")
	isTerminal    = term.IsTerminal(int(os.Stdout.Fd()))

	reader = bufio.NewReader(os.Stdin)
)

func AskYesNo(prompt string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}

	fmt.Printf("%s%s %s ", promptPrefix, prompt, hint)

	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	result := defaultYes
	if input != "" {
		result = input == "y" || input == "yes"
	}
	ClearLine()

	return result
}

func AskForInput(prompt string, required bool) string {
	for {
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

func PrintInfo(message string) {
	fmt.Printf("%s %s\n", infoPrefix, color.BlueString(message))
}

func PrintSuccess(message string) {
	fmt.Printf("%s %s\n", successPrefix, color.GreenString(message))
}

func PrintError(message string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", errorPrefix, color.RedString(message))
}

func StartSpinner(message string) *spinner.Spinner {
	if !isTerminal {
		PrintInfo(message + "...")
		return nil
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + message
	if err := s.Color("cyan"); err != nil && s.Writer != nil {
		_, _ = fmt.Fprintf(s.Writer, "Warning: Failed to set spinner color: %v\n", err)
	}
	s.Start()
	return s
}

func StopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

func ClearLine() {
	if isTerminal {
		fmt.Print("\033[1A\r\033[K")
	}
}

func PrintRepoLink(message string, url string) {
	fmt.Printf("%s %s\n", message, color.BlueString(url))
}

func AskSquashDirtyAction() string {
	fmt.Printf("%sWorking tree is dirty. [Enter] cancel  [s] stash+restore  [a] add to squash: ", promptPrefix)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	ClearLine()
	return input
}

func PrintPreGenerationInfo(branchName string, additions int, deletions int, model string) {
	blue := color.New(color.FgBlue)
	yellow := color.New(color.FgYellow)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	message := blue.Sprintf("Branch: %s | Changes: %s %s | Model: %s",
		yellow.Sprint(branchName),
		green.Sprintf("↑ %d", additions),
		red.Sprintf("↓ %d", deletions),
		yellow.Sprint(model),
	)

	fmt.Printf("%s %s\n", infoPrefix, message)
}
