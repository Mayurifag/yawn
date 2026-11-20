package git

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var prURLRe = regexp.MustCompile(`https://\S+`)

func extractPRLink(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
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
	separator := strings.LastIndex(path, "/")
	if host == "" || separator <= 0 || separator == len(path)-1 {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}
	owner, repo := path[:separator], strings.TrimSuffix(path[separator+1:], ".git")
	if repo == "" || strings.Contains(owner, "//") || (isGitHubHost(host) && strings.Contains(owner, "/")) {
		return nil, fmt.Errorf("invalid repository path format: %s", path)
	}
	return &RemoteInfo{
		Host:  host,
		Owner: owner,
		Repo:  repo,
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

func normalizeBranchName(branch string) string {
	branch = strings.TrimPrefix(branch, "refs/heads/")
	return strings.TrimPrefix(branch, "heads/")
}

func GeneratePRURL(host, owner, repo, branch, baseBranch string) string {
	branch = normalizeBranchName(branch)
	if host == "" || owner == "" || repo == "" || branch == "" || defaultBranches[branch] {
		return ""
	}
	baseBranch = normalizeBranchName(strings.TrimSpace(baseBranch))
	if baseBranch == branch {
		return ""
	}
	if strings.Contains(host, "gitlab") {
		values := url.Values{}
		values.Set("merge_request[source_branch]", branch)
		if baseBranch != "" {
			values.Set("merge_request[target_branch]", baseBranch)
		}
		return fmt.Sprintf("https://%s/%s/%s/-/merge_requests/new?%s", host, owner, repo, values.Encode())
	}
	comparison := branch
	if baseBranch != "" {
		comparison = baseBranch + "..." + branch
	}
	return (&url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     fmt.Sprintf("/%s/%s/compare/%s", owner, repo, comparison),
		RawQuery: "expand=1",
	}).String()
}

func FindPullRequestURL(gitClient GitClient, remoteURL, branch string) string {
	remote, err := ParseRemoteURL(remoteURL)
	if err != nil || !isGitHubHost(remote.Host) {
		return ""
	}
	branch = normalizeBranchName(branch)
	prURL, err := gitClient.GetPullRequestURL(remoteURL, branch)
	if err != nil {
		return ""
	}
	return prURL
}

type PushResult struct {
	Success       bool
	PRLink        string
	SuggestPRLink string
	RepoLink      string
}

type PushProvider interface {
	ExecutePush(plan *PushPlan, force bool) (*PushResult, error)
	HasRemotes() (bool, error)
}

type Pusher struct {
	gitClient GitClient
}

func NewPusher(gitClient GitClient) *Pusher {
	return &Pusher{gitClient: gitClient}
}

func (p *Pusher) ExecutePush(plan *PushPlan, force bool) (*PushResult, error) {
	output, err := p.gitClient.Push(plan, force)
	if err != nil {
		return nil, err
	}
	result := PushLinks(p.gitClient, plan.Target)
	result.Success = true
	if link := extractPRLink(output); link != "" {
		result.PRLink = link
		result.SuggestPRLink = ""
	}
	return &result, nil
}

func PushLinks(client GitClient, target PushTarget) PushResult {
	result := PushResult{}
	remote, err := ParseRemoteURL(target.URL)
	if err != nil {
		return result
	}
	result.RepoLink = GenerateRepoLink(remote.Host, remote.Owner, remote.Repo)
	defaultBranch := target.DefaultBranch
	if defaultBranch == "" {
		defaultBranch, _ = client.GetDefaultBranch(target.Remote)
	}
	if target.Branch != defaultBranch {
		result.PRLink = FindPullRequestURL(client, target.URL, target.Branch)
		if result.PRLink == "" {
			baseBranch, _ := client.FindBranchBaseRef(target.LocalBranch)
			result.SuggestPRLink = GeneratePRURL(remote.Host, remote.Owner, remote.Repo, target.Branch, baseBranch)
		}
	}
	return result
}

func (p *Pusher) HasRemotes() (bool, error) {
	return p.gitClient.HasRemotes()
}
