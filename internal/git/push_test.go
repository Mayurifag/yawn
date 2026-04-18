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
				Host:  "github.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "git@github.com:owner/repo.git",
			},
		},
		{
			name:      "GitHub HTTPS URL",
			remoteURL: "https://github.com/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "github.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "https://github.com/owner/repo.git",
			},
		},
		{
			name:      "GitHub URL without .git suffix",
			remoteURL: "https://github.com/owner/repo",
			expectedInfo: &RemoteInfo{
				Host:  "github.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "https://github.com/owner/repo",
			},
		},
		{
			name:      "GitLab SSH URL",
			remoteURL: "git@gitlab.com:owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "gitlab.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "git@gitlab.com:owner/repo.git",
			},
		},
		{
			name:      "GitLab HTTPS URL",
			remoteURL: "https://gitlab.com/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "gitlab.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "https://gitlab.com/owner/repo.git",
			},
		},
		{
			name:      "Repository with hyphens",
			remoteURL: "https://github.com/owner-name/repo-name.git",
			expectedInfo: &RemoteInfo{
				Host:  "github.com",
				Owner: "owner-name",
				Repo:  "repo-name",
				URL:   "https://github.com/owner-name/repo-name.git",
			},
		},
		{
			name:      "Repository with numbers",
			remoteURL: "https://github.com/owner123/repo456.git",
			expectedInfo: &RemoteInfo{
				Host:  "github.com",
				Owner: "owner123",
				Repo:  "repo456",
				URL:   "https://github.com/owner123/repo456.git",
			},
		},
		{
			name:      "SSH URL with ssh:// protocol",
			remoteURL: "ssh://git@example.com/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "example.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "ssh://git@example.com/owner/repo.git",
			},
		},
		{
			name:      "SSH URL with port",
			remoteURL: "ssh://git@example.com:22/owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "example.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "ssh://git@example.com:22/owner/repo.git",
			},
		},
		{
			name:      "Gitea SSH URL with port",
			remoteURL: "ssh://git.lajsdhf.ru:222/gitea_admin/kapsod.git",
			expectedInfo: &RemoteInfo{
				Host:  "git.lajsdhf.ru",
				Owner: "gitea_admin",
				Repo:  "kapsod",
				URL:   "ssh://git.lajsdhf.ru:222/gitea_admin/kapsod.git",
			},
		},
		{
			name:      "SSH URL without git user",
			remoteURL: "ssh://user@host.com/owner/repo",
			expectedInfo: &RemoteInfo{
				Host:  "host.com",
				Owner: "owner",
				Repo:  "repo",
				URL:   "ssh://user@host.com/owner/repo",
			},
		},
		{
			name:      "Custom domain Git@ SSH URL",
			remoteURL: "git@git.example.org:owner/repo.git",
			expectedInfo: &RemoteInfo{
				Host:  "git.example.org",
				Owner: "owner",
				Repo:  "repo",
				URL:   "git@git.example.org:owner/repo.git",
			},
		},
		{
			name:           "Empty URL",
			remoteURL:      "",
			expectedErrMsg: "remote URL is empty",
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

			if tt.expectedErrMsg != "" {
				if err == nil {
					t.Errorf("ParseRemoteURL() expected error containing %q, got nil", tt.expectedErrMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("ParseRemoteURL() error = %v, expected to contain %q", err, tt.expectedErrMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseRemoteURL() unexpected error: %v", err)
				return
			}

			if info.Host != tt.expectedInfo.Host {
				t.Errorf("Host = %v, expected %v", info.Host, tt.expectedInfo.Host)
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

func TestGeneratePRURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		owner    string
		repo     string
		branch   string
		expected string
	}{
		{
			name:     "GitHub feature branch",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "feature/my-feature",
			expected: "https://github.com/owner/repo/compare/feature/my-feature?expand=1",
		},
		{
			name:     "GitLab feature branch",
			host:     "gitlab.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "feature/my-feature",
			expected: "https://gitlab.com/owner/repo/-/merge_requests/new?merge_request%5Bsource_branch%5D=feature%2Fmy-feature",
		},
		{
			name:     "Gitea custom host",
			host:     "git.example.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "my-branch",
			expected: "https://git.example.com/owner/repo/compare/my-branch?expand=1",
		},
		{
			name:     "self-hosted GitLab",
			host:     "gitlab.company.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "my-branch",
			expected: "https://gitlab.company.com/owner/repo/-/merge_requests/new?merge_request%5Bsource_branch%5D=my-branch",
		},
		{
			name:     "default branch main returns empty",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "main",
			expected: "",
		},
		{
			name:     "default branch master returns empty",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "master",
			expected: "",
		},
		{
			name:     "default branch dev returns empty",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "dev",
			expected: "",
		},
		{
			name:     "empty host returns empty",
			host:     "",
			owner:    "owner",
			repo:     "repo",
			branch:   "feature",
			expected: "",
		},
		{
			name:     "empty branch returns empty",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			branch:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratePRURL(tt.host, tt.owner, tt.repo, tt.branch)
			if result != tt.expected {
				t.Errorf("GeneratePRURL() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateRepoLink(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		owner    string
		repo     string
		expected string
	}{
		{
			name:     "GitHub repo",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "GitHub repo with .git suffix",
			host:     "github.com",
			owner:    "owner",
			repo:     "repo.git",
			expected: "https://github.com/owner/repo",
		},
		{
			name:     "GitLab repo",
			host:     "gitlab.com",
			owner:    "owner",
			repo:     "repo",
			expected: "https://gitlab.com/owner/repo",
		},
		{
			name:     "Custom domain repo",
			host:     "git.example.org",
			owner:    "owner",
			repo:     "repo",
			expected: "https://git.example.org/owner/repo",
		},
		{
			name:     "Gitea repo",
			host:     "git.lajsdhf.ru",
			owner:    "gitea_admin",
			repo:     "kapsod",
			expected: "https://git.lajsdhf.ru/gitea_admin/kapsod",
		},
		{
			name:     "Empty host",
			host:     "",
			owner:    "owner",
			repo:     "repo",
			expected: "",
		},
		{
			name:     "Empty owner",
			host:     "github.com",
			owner:    "",
			repo:     "repo",
			expected: "",
		},
		{
			name:     "Empty repo",
			host:     "github.com",
			owner:    "owner",
			repo:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRepoLink(tt.host, tt.owner, tt.repo)
			if result != tt.expected {
				t.Errorf("GenerateRepoLink() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
