package git

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var prURLRe = regexp.MustCompile(`https://\S+`)

func extractPRLink(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if match := prURLRe.FindString(line); match != "" {
			lower := strings.ToLower(match)
			if strings.Contains(lower, "/pull/") || strings.Contains(lower, "merge_request") || strings.Contains(lower, "/compare/") {
				return strings.TrimRight(match, ".,;)")
			}
		}
	}
	return ""
}

type RemoteInfo struct {
	Host  string
	Owner string
	Repo  string
	URL   string
}

func GenerateRepoLink(host, owner, repo string) string {
	if host == "" || owner == "" || repo == "" {
		return ""
	}
	repo = strings.TrimSuffix(repo, ".git")
	return fmt.Sprintf("https://%s/%s/%s", host, owner, repo)
}

func newRemoteInfo(host, path, rawURL string) (*RemoteInfo, error) {
	owner, repo, ok := strings.Cut(path, "/")
	if !ok || strings.Contains(repo, "/") {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}
	return &RemoteInfo{
		Host:  host,
		Owner: owner,
		Repo:  strings.TrimSuffix(repo, ".git"),
		URL:   rawURL,
	}, nil
}

func parseGitAtSSHURL(remoteURL string) (*RemoteInfo, error) {
	u := strings.TrimPrefix(remoteURL, "git@")
	host, path, ok := strings.Cut(u, ":")
	if !ok {
		return nil, fmt.Errorf("invalid SSH URL format: %s", remoteURL)
	}
	return newRemoteInfo(host, path, remoteURL)
}

func parseParsedURL(remoteURL string) (*RemoteInfo, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	return newRemoteInfo(parsedURL.Hostname(), strings.TrimPrefix(parsedURL.Path, "/"), remoteURL)
}

func ParseRemoteURL(remoteURL string) (*RemoteInfo, error) {
	if remoteURL == "" {
		return nil, fmt.Errorf("remote URL is empty")
	}
	if strings.HasPrefix(remoteURL, "git@") {
		return parseGitAtSSHURL(remoteURL)
	}
	return parseParsedURL(remoteURL)
}

var defaultBranches = map[string]bool{
	"main":   true,
	"master": true,
	"dev":    true,
}

func GeneratePRURL(host, owner, repo, branch string) string {
	if host == "" || owner == "" || repo == "" || branch == "" || defaultBranches[branch] {
		return ""
	}
	if strings.Contains(host, "gitlab") {
		return fmt.Sprintf("https://%s/%s/%s/-/merge_requests/new?merge_request%%5Bsource_branch%%5D=%s",
			host, owner, repo, url.QueryEscape(branch))
	}
	return fmt.Sprintf("https://%s/%s/%s/compare/%s?expand=1", host, owner, repo, branch)
}

type PushResult struct {
	Success       bool
	PRLink        string
	SuggestPRLink string
	RepoLink      string
}

type PushProvider interface {
	ExecutePush(command string) (*PushResult, error)
	HasRemotes() (bool, error)
}

type Pusher struct {
	gitClient GitClient
}

func NewPusher(gitClient GitClient) *Pusher {
	return &Pusher{gitClient: gitClient}
}

func (p *Pusher) ExecutePush(command string) (*PushResult, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("push command is empty")
	}
	if len(parts) < 2 || parts[0] != "git" || parts[1] != "push" {
		return nil, fmt.Errorf("invalid push command format: expected 'git push ...', got '%s'", command)
	}

	output, err := p.gitClient.Push(command)
	if err != nil {
		return &PushResult{Success: false}, fmt.Errorf("failed to push changes using command '%s': %w", command, err)
	}

	result := &PushResult{
		Success: true,
		PRLink:  extractPRLink(output),
	}

	if remoteURL, err := p.gitClient.GetRemoteURL(""); err == nil {
		if remoteInfo, err := ParseRemoteURL(remoteURL); err == nil {
			result.RepoLink = GenerateRepoLink(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo)
			if result.PRLink == "" {
				if branch, err := p.gitClient.GetCurrentBranch(); err == nil {
					defaultBranch, _ := p.gitClient.GetDefaultBranch()
					if branch != defaultBranch {
						result.SuggestPRLink = GeneratePRURL(remoteInfo.Host, remoteInfo.Owner, remoteInfo.Repo, branch)
					}
				}
			}
		}
	}

	return result, nil
}

func (p *Pusher) HasRemotes() (bool, error) {
	return p.gitClient.HasRemotes()
}
