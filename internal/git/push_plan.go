package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type PushTarget struct {
	Remote        string
	URL           string
	Branch        string
	LocalBranch   string
	DefaultBranch string
	Force         bool
	setUpstream   bool
	options       []string
}

type PushPlan struct {
	Target        PushTarget
	Head          string
	RemoteHead    string
	LocalCommits  []string
	RemoteCommits []string
}

func (c *ExecGitClient) ResolvePushTarget(command string) (PushTarget, error) {
	branch, err := c.GetCurrentBranch()
	if err != nil || branch == "HEAD" || branch == "" {
		return PushTarget{}, fmt.Errorf("push requires a checked-out branch")
	}
	target := PushTarget{Branch: branch, LocalBranch: branch}
	positional, err := parsePushOptions(command, &target)
	if err != nil {
		return PushTarget{}, err
	}
	if len(positional) > 2 {
		return PushTarget{}, fmt.Errorf("push_command must target exactly one remote and branch")
	}
	if len(positional) > 0 {
		target.Remote = positional[0]
	} else {
		target.Remote, err = c.defaultPushRemote(branch)
		if err != nil {
			return PushTarget{}, err
		}
	}
	if len(positional) == 2 {
		if err := target.setRefspec(positional[1]); err != nil {
			return PushTarget{}, err
		}
	} else if configured, _ := c.runGitCommand("config", "--get-all", "remote."+target.Remote+".push"); configured != "" {
		return PushTarget{}, fmt.Errorf("remote.%s.push is configured; specify the single branch explicitly in push_command", target.Remote)
	}
	if _, err := c.runGitCommand("check-ref-format", "refs/heads/"+target.Branch); err != nil {
		return PushTarget{}, fmt.Errorf("invalid destination branch %q", target.Branch)
	}
	target.URL, err = c.pushURL(target.Remote)
	return target, err
}

func (c *ExecGitClient) pushURL(remote string) (string, error) {
	value, err := c.runGitCommand("remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return "", err
	}
	if value == "" || strings.Contains(value, "\n") {
		return "", fmt.Errorf("remote %q must have exactly one push URL", remote)
	}
	if parsed, err := url.Parse(value); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User != nil {
		return "", fmt.Errorf("push URLs must not embed credentials; use SSH or a Git credential helper")
	}
	return value, nil
}

func (t *PushTarget) setRefspec(spec string) error {
	if strings.HasPrefix(spec, "+") {
		t.Force = true
		spec = strings.TrimPrefix(spec, "+")
	}
	source, destination, hasDestination := strings.Cut(spec, ":")
	if source != "HEAD" && source != t.LocalBranch && source != "refs/heads/"+t.LocalBranch {
		return fmt.Errorf("yawn can only push the current branch %q", t.LocalBranch)
	}
	if hasDestination {
		if strings.HasPrefix(destination, "refs/") && !strings.HasPrefix(destination, "refs/heads/") {
			return fmt.Errorf("push destination must be a branch, not %q", destination)
		}
		t.Branch = strings.TrimPrefix(destination, "refs/heads/")
	}
	return nil
}

func parsePushOptions(command string, target *PushTarget) ([]string, error) {
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "git" || parts[1] != "push" || strings.ContainsAny(command, "\"'\\") {
		return nil, fmt.Errorf("push_command must be 'git push [options] [remote [branch[:destination]]]' without shell quoting")
	}
	var positional []string
	for _, arg := range parts[2:] {
		switch arg {
		case "--force", "-f", "--force-with-lease":
			target.Force = true
		case "--set-upstream", "-u":
			target.setUpstream = true
		case "--verbose", "-v", "--quiet", "-q", "--porcelain", "--no-verify", "--verify", "--force-if-includes":
			target.options = append(target.options, arg)
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unsupported push option %q; yawn requires one branch and manages its own explicit force-with-lease", arg)
			}
			positional = append(positional, arg)
		}
	}
	return positional, nil
}

func (c *ExecGitClient) defaultPushRemote(branch string) (string, error) {
	for _, key := range []string{"branch." + branch + ".pushRemote", "remote.pushDefault", "branch." + branch + ".remote"} {
		remote, err := c.runGitCommand("config", "--get", key)
		var gitErr *GitError
		if err != nil && (!errors.As(err, &gitErr) || gitErr.ExitCode != 1) {
			return "", err
		}
		if remote != "" && remote != "." {
			return remote, nil
		}
	}
	return "origin", nil
}

func (c *ExecGitClient) PreparePush(command string) (*PushPlan, error) {
	target, err := c.ResolvePushTarget(command)
	if err != nil {
		return nil, err
	}
	head, err := c.GetLastCommitHash()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), NetworkPushTimeout)
	defer cancel()
	ref := "refs/heads/" + target.Branch
	out, err := c.runGitCommandContext(ctx, "", "ls-remote", "--exit-code", "--symref", target.URL, "HEAD", ref)
	plan := &PushPlan{Target: target, Head: head}
	var gitErr *GitError
	if err != nil && (!errors.As(err, &gitErr) || gitErr.ExitCode != 2) {
		return nil, fmt.Errorf("cannot inspect push destination: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			plan.Target.DefaultBranch = strings.TrimPrefix(fields[1], "refs/heads/")
		}
		if len(fields) == 2 && fields[1] == ref {
			plan.RemoteHead = fields[0]
		}
	}
	if plan.RemoteHead == "" {
		plan.LocalCommits, err = c.getCommitList(head, "--not", "--remotes="+target.Remote)
		return plan, err
	}
	if _, err := c.runGitCommandContext(ctx, "", "fetch", "--no-tags", "--no-write-fetch-head", target.URL, plan.RemoteHead); err != nil {
		return nil, fmt.Errorf("cannot fetch the destination commit for preview: %w", err)
	}
	plan.LocalCommits, err = c.getCommitList(plan.RemoteHead + ".." + head)
	if err != nil {
		return nil, err
	}
	plan.RemoteCommits, err = c.getCommitList(head + ".." + plan.RemoteHead)
	return plan, err
}

func (c *ExecGitClient) IsAncestor(ancestor, descendant string) (bool, error) {
	_, err := c.runGitCommand("merge-base", "--is-ancestor", ancestor, descendant)
	var gitErr *GitError
	if errors.As(err, &gitErr) && gitErr.ExitCode == 1 {
		return false, nil
	}
	return err == nil, err
}

func (c *ExecGitClient) Push(plan *PushPlan, force bool) (string, error) {
	currentURL, err := c.pushURL(plan.Target.Remote)
	if err != nil {
		return "", err
	}
	if currentURL != plan.Target.URL {
		return "", fmt.Errorf("push URL changed since preview; inspect the destination again before pushing")
	}
	args := append([]string{"push"}, plan.Target.options...)
	ref := "refs/heads/" + plan.Target.Branch
	if force || plan.Target.Force {
		args = append(args, "--force-with-lease="+ref+":"+plan.RemoteHead)
	}
	args = append(args, "--", plan.Target.Remote, plan.Head+":"+ref)
	ctx, cancel := context.WithTimeout(context.Background(), NetworkPushTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.RepoPath
	cmd.Stdin = os.Stdin
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", ErrNetworkTimeout
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", &GitError{Command: "git push", Output: string(output), ExitCode: exitErr.ExitCode()}
		}
		return "", err
	}
	if plan.Target.setUpstream {
		if _, err := c.runGitCommand("config", "branch."+plan.Target.LocalBranch+".remote", plan.Target.Remote); err != nil {
			return "", fmt.Errorf("push succeeded but saving upstream remote failed: %w", err)
		}
		if _, err := c.runGitCommand("config", "branch."+plan.Target.LocalBranch+".merge", ref); err != nil {
			return "", fmt.Errorf("push succeeded but saving upstream branch failed: %w", err)
		}
	}
	return strings.TrimSpace(string(output)), nil
}
