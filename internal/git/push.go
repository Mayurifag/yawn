package git

import (
	"fmt"
	"net/url"
	"strings"
)

// HostingProvider represents a Git hosting service.
type HostingProvider string

const (
	// ProviderGitHub represents GitHub hosting service
	ProviderGitHub HostingProvider = "github"
	// ProviderGitLab represents GitLab hosting service
	ProviderGitLab HostingProvider = "gitlab"
	// ProviderUnknown represents an unsupported or unknown hosting service
	ProviderUnknown HostingProvider = "unknown"
)

// RemoteInfo contains parsed information about a Git remote URL.
type RemoteInfo struct {
	// Provider is the identified hosting provider
	Provider HostingProvider
	// Owner is the repository owner/namespace
	Owner string
	// Repo is the repository name (without .git extension)
	Repo string
	// URL is the original remote URL
	URL string
}

// GenerateRepoLink creates a web URL for the repository based on the provider and repository path.
func GenerateRepoLink(provider HostingProvider, owner, repo string) string {
	// Remove .git suffix if present
	repo = strings.TrimSuffix(repo, ".git")

	switch provider {
	case ProviderGitHub:
		return fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	case ProviderGitLab:
		return fmt.Sprintf("https://gitlab.com/%s/%s", owner, repo)
	default:
		return ""
	}
}

// ParseRemoteURL parses a Git remote URL and returns information about the hosting provider and repository.
// It supports both HTTPS and SSH URL formats:
// - HTTPS: https://github.com/owner/repo.git
// - SSH: git@github.com:owner/repo.git
func ParseRemoteURL(remoteURL string) (*RemoteInfo, error) {
	if remoteURL == "" {
		return nil, fmt.Errorf("remote URL is empty")
	}

	// Handle SSH URLs
	if strings.HasPrefix(remoteURL, "git@") {
		return parseSSHURL(remoteURL)
	}

	// Handle HTTPS URLs
	return parseHTTPSURL(remoteURL)
}

// parseSSHURL parses a Git SSH URL (git@host:owner/repo.git).
func parseSSHURL(remoteURL string) (*RemoteInfo, error) {
	// Remove git@ prefix
	url := strings.TrimPrefix(remoteURL, "git@")

	// Split into host and path
	parts := strings.SplitN(url, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid SSH URL format: %s", remoteURL)
	}

	host := parts[0]
	path := parts[1]

	// Remove .git suffix if present
	path = strings.TrimSuffix(path, ".git")

	// Split path into owner and repo
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}

	provider := identifyProvider(host)
	if provider == ProviderUnknown {
		return nil, fmt.Errorf("unsupported hosting provider: %s", host)
	}

	return &RemoteInfo{
		Provider: provider,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		URL:      remoteURL,
	}, nil
}

// parseHTTPSURL parses a Git HTTPS URL (https://host.com/owner/repo.git).
func parseHTTPSURL(remoteURL string) (*RemoteInfo, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTPS URL: %w", err)
	}

	// Remove .git suffix if present
	path := strings.TrimSuffix(parsedURL.Path, ".git")

	// Split path into owner and repo
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}

	provider := identifyProvider(parsedURL.Host)
	if provider == ProviderUnknown {
		return nil, fmt.Errorf("unsupported hosting provider: %s", parsedURL.Host)
	}

	return &RemoteInfo{
		Provider: provider,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		URL:      remoteURL,
	}, nil
}

// identifyProvider determines the hosting provider from a hostname.
func identifyProvider(host string) HostingProvider {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "github.com"):
		return ProviderGitHub
	case strings.Contains(host, "gitlab.com"):
		return ProviderGitLab
	default:
		return ProviderUnknown
	}
}

// PushResult contains information about the push operation.
type PushResult struct {
	// Success indicates whether the push was successful
	Success bool
	// RemoteURL is the URL of the remote repository
	RemoteURL string
	// Branch is the current branch name
	Branch string
	// CommitHash is the hash of the last commit
	CommitHash string
	// RemoteInfo contains parsed information about the remote URL
	RemoteInfo *RemoteInfo
	// RepoLink is the web URL for the repository
	RepoLink string
}

// PushProvider defines the interface for handling Git push operations.
type PushProvider interface {
	// ExecutePush performs the Git push operation using the provided command.
	// It returns a PushResult containing information about the push operation.
	ExecutePush(command string) (*PushResult, error)
	// HasRemotes checks if the repository has any remote repositories configured.
	HasRemotes() (bool, error)
}

// Pusher implements the PushProvider interface and handles Git push operations.
type Pusher struct {
	gitClient GitClient
}

// NewPusher creates a new Pusher instance with the given GitClient.
func NewPusher(gitClient GitClient) *Pusher {
	return &Pusher{
		gitClient: gitClient,
	}
}

// ExecutePush performs the Git push operation using the provided command.
func (p *Pusher) ExecutePush(command string) (*PushResult, error) {
	// Split the command string into parts for exec.Command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("push command is empty")
	}
	if len(parts) < 2 || parts[0] != "git" {
		return nil, fmt.Errorf("invalid push command format: expected 'git push ...', got '%s'", command)
	}

	// Execute the push command
	err := p.gitClient.Push(command)
	if err != nil {
		return &PushResult{Success: false}, fmt.Errorf("failed to push changes using command '%s': %w", command, err)
	}

	// Get additional information about the push
	result := &PushResult{Success: true}

	// Get the current branch
	branch, err := p.gitClient.GetCurrentBranch()
	if err == nil {
		result.Branch = branch
	}

	// Get the remote URL (defaulting to "origin")
	remoteURL, err := p.gitClient.GetRemoteURL("")
	if err == nil {
		result.RemoteURL = remoteURL
		// Parse the remote URL to get provider and repository information
		if remoteInfo, err := ParseRemoteURL(remoteURL); err == nil {
			result.RemoteInfo = remoteInfo
			// Generate the repository link
			result.RepoLink = GenerateRepoLink(remoteInfo.Provider, remoteInfo.Owner, remoteInfo.Repo)
		}
	}

	// Get the last commit hash
	commitHash, err := p.gitClient.GetLastCommitHash()
	if err == nil {
		result.CommitHash = commitHash
	}

	return result, nil
}

// HasRemotes checks if the repository has any remote repositories configured.
func (p *Pusher) HasRemotes() (bool, error) {
	return p.gitClient.HasRemotes()
}
