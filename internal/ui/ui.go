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
	Version       string
	promptPrefix  = color.New(color.FgYellow).Sprint("? ")
	infoPrefix    = color.New(color.FgBlue).Sprint("* ")
	errorPrefix   = color.New(color.FgRed).Sprint("! ")
	successPrefix = color.New(color.FgGreen).Sprint("✓ ")
	isTerminal    = term.IsTerminal(int(os.Stdout.Fd()))

	reader = bufio.NewReader(os.Stdin)

	colorRed    = color.New(color.FgRed)
	colorGreen  = color.New(color.FgGreen)
	colorBlue   = color.New(color.FgBlue)
	colorYellow = color.New(color.FgYellow)
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
	if isTerminal {
		link := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, color.BlueString(url))
		fmt.Printf("%s %s\n", message, link)
	} else {
		fmt.Printf("%s %s\n", message, color.BlueString(url))
	}
}

func readSingleKey() string {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		input, _ := reader.ReadString('\n')
		return strings.ToLower(strings.TrimSpace(input))
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()
	buf := make([]byte, 1)
	n, _ := os.Stdin.Read(buf)
	if n == 0 || buf[0] == '\r' || buf[0] == '\n' || buf[0] == 3 {
		fmt.Println()
		return ""
	}
	fmt.Printf("%c\n", buf[0])
	return strings.ToLower(string(buf[:1]))
}

func PrintDirtyChanges(status string) {
	lines := strings.Split(strings.TrimSpace(status), "\n")
	count := 0
	for _, l := range lines {
		if l != "" {
			count++
		}
	}
	fmt.Printf("%s %d file(s) with changes:\n", infoPrefix, count)
	for _, l := range lines {
		if l != "" {
			fmt.Printf("  %s\n", colorYellow.Sprint(l))
		}
	}
}

func askDirtyAction(options string) string {
	fmt.Printf("%sWorking tree is dirty. [Enter] cancel  %s: ", promptPrefix, options)
	key := readSingleKey()
	ClearLine()
	return key
}

func AskSquashDirtyAction() string {
	return askDirtyAction("[s] stash+restore  [a] add to squash")
}

func AskAmendDirtyAction() string {
	return askDirtyAction("[a] add and amend commit")
}

func PrintForcePushPreview(remoteCommits, localCommits []string) {
	if len(remoteCommits) == 0 && len(localCommits) == 0 {
		return
	}
	fmt.Printf("%s Force push preview:\n", infoPrefix)
	if len(remoteCommits) > 0 {
		fmt.Printf("  %s\n", colorRed.Sprintf("Remote (%d commit(s) will be overwritten):", len(remoteCommits)))
		for _, c := range remoteCommits {
			fmt.Printf("    %s\n", colorRed.Sprint("- "+c))
		}
	}
	if len(localCommits) > 0 {
		fmt.Printf("  %s\n", colorGreen.Sprintf("Local (%d commit(s) will be pushed):", len(localCommits)))
		for _, c := range localCommits {
			fmt.Printf("    %s\n", colorGreen.Sprint("+ "+c))
		}
	}
}

func PrintPreGenerationInfo(branchName string, additions int, deletions int, model string) {
	msg := colorBlue.Sprintf("Branch: %s | Changes: %s %s | Model: %s",
		colorYellow.Sprint(branchName),
		colorGreen.Sprintf("↑ %d", additions),
		colorRed.Sprintf("↓ %d", deletions),
		colorYellow.Sprint(model),
	)
	if Version != "" {
		msg += colorBlue.Sprintf(" | yawn %s", colorYellow.Sprint(Version))
	}
	fmt.Printf("%s %s\n", infoPrefix, msg)
}
