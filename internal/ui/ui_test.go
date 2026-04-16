package ui

import (
	"os"
	"testing"
)

func TestPrintPreGenerationInfo(t *testing.T) {
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

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
