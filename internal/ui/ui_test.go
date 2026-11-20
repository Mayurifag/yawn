package ui

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestPromptsAtEOF(t *testing.T) {
	oldReader := reader
	t.Cleanup(func() { reader = oldReader })
	reader = bufio.NewReader(strings.NewReader(""))
	if AskYesNo("Stage and push?", true) {
		t.Fatal("EOF must not approve an action")
	}
	if input, err := AskForSecret("API key:"); input != "" || err == nil {
		t.Fatalf("expected an error at EOF, got %q, %v", input, err)
	}
	reader = bufio.NewReader(strings.NewReader("yes"))
	if !AskYesNo("Proceed?", false) {
		t.Fatal("explicit input without a final newline must still work")
	}
}

func TestPrintPreGenerationInfo(t *testing.T) {
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() {
		_ = os.Stdout.Close()
		os.Stdout = oldStdout
	}()

	testCases := []struct {
		name       string
		branchName string
		additions  int
		deletions  int
		model      string
	}{
		{
			name:       "known values",
			branchName: "main",
			additions:  42,
			deletions:  10,
			model:      "gemini-flash-latest",
		},
		{
			name:       "zero changes",
			branchName: "feature/test",
			additions:  0,
			deletions:  0,
			model:      "gemini-flash-latest",
		},
		{
			name:       "long branch name",
			branchName: "feature/very-long-branch-name-with-many-words",
			additions:  100,
			deletions:  100,
			model:      "gemini-pro",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			PrintPreGenerationInfo(tc.branchName, tc.additions, tc.deletions, tc.model)
		})
	}
}
