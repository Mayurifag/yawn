package ui

import (
	"os"
	"testing"
)

func TestPrintPreGenerationInfo(t *testing.T) {
	// This test just ensures the function runs without errors
	// It's difficult to test the exact output format with colors in a unit test

	// Redirect stdout to avoid polluting test output
	// In a more comprehensive test, we could capture and analyze the output
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	// Call the function with various inputs
	testCases := []struct {
		name       string
		tokenCount string
		tokenLimit int
		branchName string
		additions  int
		deletions  int
	}{
		{
			name:       "known values",
			tokenCount: "500",
			tokenLimit: 1000,
			branchName: "main",
			additions:  42,
			deletions:  10,
		},
		{
			name:       "unknown token count",
			tokenCount: "?",
			tokenLimit: 1000,
			branchName: "feature/test",
			additions:  0,
			deletions:  0,
		},
		{
			name:       "long branch name",
			tokenCount: "500",
			tokenLimit: 1000,
			branchName: "feature/very-long-branch-name-with-many-words",
			additions:  100,
			deletions:  100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Just verify it doesn't panic
			PrintPreGenerationInfo(tc.tokenCount, tc.tokenLimit, tc.branchName, tc.additions, tc.deletions)
		})
	}
}
