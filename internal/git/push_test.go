package git

import (
	"strings"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name           string
		remoteURL      string
		expectedInfo   *RemoteInfo
		expectedErrMsg string
	}{
		{
			name:      "GitHub SSH URL",
			remoteURL: "git@github.com:owner/repo.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitHub,
				Owner:    "owner",
				Repo:     "repo",
				URL:      "git@github.com:owner/repo.git",
			},
		},
		{
			name:      "GitHub HTTPS URL",
			remoteURL: "https://github.com/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitHub,
				Owner:    "owner",
				Repo:     "repo",
				URL:      "https://github.com/owner/repo.git",
			},
		},
		{
			name:      "GitHub URL without .git suffix",
			remoteURL: "https://github.com/owner/repo",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitHub,
				Owner:    "owner",
				Repo:     "repo",
				URL:      "https://github.com/owner/repo",
			},
		},
		{
			name:      "GitLab SSH URL",
			remoteURL: "git@gitlab.com:owner/repo.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitLab,
				Owner:    "owner",
				Repo:     "repo",
				URL:      "git@gitlab.com:owner/repo.git",
			},
		},
		{
			name:      "GitLab HTTPS URL",
			remoteURL: "https://gitlab.com/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitLab,
				Owner:    "owner",
				Repo:     "repo",
				URL:      "https://gitlab.com/owner/repo.git",
			},
		},
		{
			name:      "Repository with hyphens",
			remoteURL: "https://github.com/owner-name/repo-name.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitHub,
				Owner:    "owner-name",
				Repo:     "repo-name",
				URL:      "https://github.com/owner-name/repo-name.git",
			},
		},
		{
			name:      "Repository with numbers",
			remoteURL: "https://github.com/owner123/repo456.git",
			expectedInfo: &RemoteInfo{
				Provider: ProviderGitHub,
				Owner:    "owner123",
				Repo:     "repo456",
				URL:      "https://github.com/owner123/repo456.git",
			},
		},
		{
			name:           "Empty URL",
			remoteURL:      "",
			expectedErrMsg: "remote URL is empty",
		},
		{
			name:           "Unsupported provider",
			remoteURL:      "git@bitbucket.org:owner/repo.git",
			expectedErrMsg: "unsupported hosting provider",
		},
		{
			name:           "Invalid SSH URL format",
			remoteURL:      "git@github.comowner/repo.git",
			expectedErrMsg: "invalid SSH URL format",
		},
		{
			name:           "Invalid repository path format",
			remoteURL:      "git@github.com:owner/repo/extra.git",
			expectedErrMsg: "invalid repository path format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseRemoteURL(tt.remoteURL)

			// Check if we expect an error
			if tt.expectedErrMsg != "" {
				if err == nil {
					t.Errorf("ParseRemoteURL() expected error containing %q, got nil", tt.expectedErrMsg)
					return
				}
				if !contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("ParseRemoteURL() error = %v, expected to contain %q", err, tt.expectedErrMsg)
				}
				return
			}

			// We expect success
			if err != nil {
				t.Errorf("ParseRemoteURL() unexpected error: %v", err)
				return
			}

			// Check RemoteInfo fields
			if info.Provider != tt.expectedInfo.Provider {
				t.Errorf("Provider = %v, expected %v", info.Provider, tt.expectedInfo.Provider)
			}
			if info.Owner != tt.expectedInfo.Owner {
				t.Errorf("Owner = %v, expected %v", info.Owner, tt.expectedInfo.Owner)
			}
			if info.Repo != tt.expectedInfo.Repo {
				t.Errorf("Repo = %v, expected %v", info.Repo, tt.expectedInfo.Repo)
			}
			if info.URL != tt.expectedInfo.URL {
				t.Errorf("URL = %v, expected %v", info.URL, tt.expectedInfo.URL)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestGenerateRepoLink tests the GenerateRepoLink function.
func TestGenerateRepoLink(t *testing.T) {
	tests := []struct {
		name     string
		provider HostingProvider
		owner    string
		repo     string
		expected string
	}{
		{
			name:     "GitHub repo",
			provider: ProviderGitHub,
			owner:    "owner",
			repo:     "repo",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "GitHub repo with .git suffix",
			provider: ProviderGitHub,
			owner:    "owner",
			repo:     "repo.git",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "GitLab repo",
			provider: ProviderGitLab,
			owner:    "owner",
			repo:     "repo",
			expected: "https://gitlab.com/owner/repo",
		},
		{
			name:     "Unknown provider",
			provider: ProviderUnknown,
			owner:    "owner",
			repo:     "repo",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRepoLink(tt.provider, tt.owner, tt.repo)
			if result != tt.expected {
				t.Errorf("GenerateRepoLink() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
