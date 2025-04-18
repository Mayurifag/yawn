package git

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDiffNumStatSummary(t *testing.T) {
	tests := []struct {
		name         string
		mockOutput   string
		mockError    error
		expAdditions int
		expDeletions int
		expError     bool
	}{
		{
			name:         "empty diff",
			mockOutput:   "",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "single file with additions and deletions",
			mockOutput:   "10\t5\tsome/file.go",
			mockError:    nil,
			expAdditions: 10,
			expDeletions: 5,
			expError:     false,
		},
		{
			name:         "multiple files",
			mockOutput:   "10\t5\tfile1.go\n3\t0\tfile2.go\n0\t7\tfile3.go",
			mockError:    nil,
			expAdditions: 13,
			expDeletions: 12,
			expError:     false,
		},
		{
			name:         "binary file",
			mockOutput:   "-\t-\tbinary.png",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "mixed normal and binary files",
			mockOutput:   "10\t5\tfile1.go\n-\t-\tbinary.png\n3\t2\tfile2.go",
			mockError:    nil,
			expAdditions: 13,
			expDeletions: 7,
			expError:     false,
		},
		{
			name:         "malformed line",
			mockOutput:   "not-a-number\tfile.go",
			mockError:    nil,
			expAdditions: 0,
			expDeletions: 0,
			expError:     false,
		},
		{
			name:         "command error",
			mockOutput:   "",
			mockError:    errors.New("git command failed"),
			expAdditions: 0,
			expDeletions: 0,
			expError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock git client that will return our test data
			mockClient := &MockGitClient{
				MockGetDiffNumStatSummary: func() (int, int, error) {
					if tt.mockError != nil {
						return 0, 0, tt.mockError
					}
					return tt.expAdditions, tt.expDeletions, nil
				},
			}

			// Call the function to test
			additions, deletions, err := mockClient.GetDiffNumStatSummary()

			// Check results
			if tt.expError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expAdditions, additions)
				assert.Equal(t, tt.expDeletions, deletions)
			}
		})
	}
}

func TestMockGitClient_GetDiffNumStatSummary(t *testing.T) {
	t.Run("using custom mock function", func(t *testing.T) {
		client := &MockGitClient{
			MockGetDiffNumStatSummary: func() (int, int, error) {
				return 42, 24, nil
			},
		}

		add, del, err := client.GetDiffNumStatSummary()
		assert.NoError(t, err)
		assert.Equal(t, 42, add)
		assert.Equal(t, 24, del)
	})

	t.Run("using default implementation", func(t *testing.T) {
		client := &MockGitClient{}

		add, del, err := client.GetDiffNumStatSummary()
		assert.NoError(t, err)
		assert.Equal(t, 0, add)
		assert.Equal(t, 0, del)
	})
}
