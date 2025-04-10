package git

import (
	"fmt"
	"net/url"
	"strings"
)

// RemoteInfo contains parsed information about a Git remote URL.
type RemoteInfo struct {
	// Host is the hostname of the remote (e.g., github.com)
	Host string
	// Owner is the repository owner/namespace
	Owner string
	// Repo is the repository name (without .git extension)
	Repo string
	// URL is the original remote URL
	URL string
}

// GenerateRepoLink creates a web URL for the repository based on the host, owner and repo.
func GenerateRepoLink(host, owner, repo string) string {
	if host == "" || owner == "" || repo == "" {
		return ""
	}

	// Remove .git suffix if present
	repo = strings.TrimSuffix(repo, ".git")

	return fmt.Sprintf("https://%s/%s/%s", host, owner, repo)
}

// ParseRemoteURL parses a Git remote URL and returns information about the host and repository.
// It supports both HTTPS and SSH URL formats:
// - HTTPS: https://host.com/owner/repo.git
// - SSH (git@): git@host.com:owner/repo.git
// - SSH (ssh://): ssh://user@host.com:port/owner/repo.git
func ParseRemoteURL(remoteURL string) (*RemoteInfo, error) {
	if remoteURL == "" {
		return nil, fmt.Errorf("remote URL is empty")
	}

	// Handle SSH URLs with git@ prefix
	if strings.HasPrefix(remoteURL, "git@") {
		return parseGitAtSSHURL(remoteURL)
	}

	// Handle SSH URLs with ssh:// prefix
	if strings.HasPrefix(remoteURL, "ssh://") {
		return parseSSHProtocolURL(remoteURL)
	}

	// Handle HTTPS URLs
	return parseHTTPSURL(remoteURL)
}

// parseGitAtSSHURL parses a Git SSH URL (git@host:owner/repo.git).
func parseGitAtSSHURL(remoteURL string) (*RemoteInfo, error) {
	// Remove git@ prefix
	url := strings.TrimPrefix(remoteURL, "git@")

	// Split into host and path
	parts := strings.SplitN(url, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid SSH URL format: %s", remoteURL)
	}

	host := parts[0]
	path := parts[1]

	// Split path into owner and repo
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}

	return &RemoteInfo{
		Host:  host,
		Owner: pathParts[0],
		Repo:  strings.TrimSuffix(pathParts[1], ".git"),
		URL:   remoteURL,
	}, nil
}

// parseSSHProtocolURL parses a Git SSH URL with protocol (ssh://user@host:port/owner/repo.git).
func parseSSHProtocolURL(remoteURL string) (*RemoteInfo, error) {
	// Parse the URL
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH URL: %w", err)
	}

	// Extract host (remove port if present)
	host := parsedURL.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	// Clean the path (remove leading slash and .git suffix)
	path := strings.TrimPrefix(parsedURL.Path, "/")

	// Split path into owner and repo
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}

	return &RemoteInfo{
		Host:  host,
		Owner: pathParts[0],
		Repo:  strings.TrimSuffix(pathParts[1], ".git"),
		URL:   remoteURL,
	}, nil
}

// parseHTTPSURL parses a Git HTTPS URL (https://host.com/owner/repo.git).
func parseHTTPSURL(remoteURL string) (*RemoteInfo, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTTPS URL: %w", err)
	}

	// Clean the path (remove leading slash)
	path := strings.TrimPrefix(parsedURL.Path, "/")

	// Split path into owner and repo
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}

	return &RemoteInfo{
		Host:  parsedURL.Host,
		Owner: pathParts[0],
		Repo:  strings.TrimSuffix(pathParts[1], ".git"),
		URL:   remoteURL,
	}, nil
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
		// Parse the remote URL to get host and repository information
		if remoteInfo, err := ParseRemoteURL(remoteURL); err == nil {
			result.RemoteInfo = remoteInfo
			// Generate the repository link
			result.RepoLink = GenerateRepoLink(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo)
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
