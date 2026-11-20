package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"
)

func (c *ExecGitClient) diffCategories(entries []numstatEntry, base string) (map[string]fileCategory, error) {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.path
	}
	sources := [][]string{nil, {"--cached"}}
	if base != "" {
		sources = append(sources, []string{"--source=" + base})
	}
	head, err := c.runGitCommand("rev-parse", "--verify", "--quiet", "HEAD")
	var gitErr *GitError
	if err != nil && (!errors.As(err, &gitErr) || gitErr.ExitCode != 1) {
		return nil, err
	}
	if head != "" && head != base {
		sources = append(sources, []string{"--source=" + head})
	}
	categories := make(map[string]fileCategory)
	for _, source := range sources {
		args := append([]string{"check-attr", "-z", "--stdin"}, source...)
		args = append(args, "filter", "diff", "yawn")
		out, err := c.runGitCommandInput(strings.Join(paths, "\x00")+"\x00", args...)
		if err != nil {
			return nil, fmt.Errorf("cannot check redaction attributes (Git 2.43+ required): %w", err)
		}
		attrs := parseCheckAttrOutput(out)
		for _, entry := range entries {
			if category := classifyEntry(entry, attrs[entry.path]); category != catNormal {
				categories[entry.path] = category
			}
		}
	}
	return categories, nil
}

func (c *ExecGitClient) buildFilteredDiff(numstat string, baseArgs []string, base string) (string, error) {
	entries := parseNumstatEntries(numstat)
	if len(entries) == 0 {
		return "", nil
	}
	categories, err := c.diffCategories(entries, base)
	if err != nil {
		return "", err
	}
	var normal []numstatEntry
	var redacted []classifiedFile
	args := append([]string{"--no-literal-pathspecs"}, baseArgs...)
	args = append(args, "--no-ext-diff", "--no-textconv", "--no-renames", "--")
	for _, entry := range entries {
		category := categories[entry.path]
		// A redacted file can become a directory in the same diff. Keep the whole boundary private.
		for parent := path.Dir(entry.path); category == catNormal && parent != "."; parent = path.Dir(parent) {
			if categories[parent] != catNormal {
				category = catSkipped
			}
		}
		if category == catNormal {
			normal = append(normal, entry)
		} else {
			redacted = append(redacted, classifiedFile{entry: entry, category: category})
			args = append(args, ":(top,exclude,literal)"+entry.path)
		}
	}
	if len(normal) == 0 {
		return formatRedactedSummary(redacted), nil
	}
	out, truncated, err := c.readLimitedDiff(args)
	if err != nil {
		return "", err
	}
	summary := formatRedactedSummary(redacted)
	if truncated || len(out)+len(summary)+2 > MaxDiffBytes {
		for _, entry := range normal {
			redacted = append(redacted, classifiedFile{entry: entry, category: catLarge})
		}
		summary = formatRedactedSummary(redacted)
	}
	var result strings.Builder
	writeLimited(&result, out, MaxDiffBytes-len(summary)-2)
	if result.Len() > 0 && summary != "" {
		result.WriteString("\n\n")
	}
	result.WriteString(summary)
	return result.String(), nil
}

type limitedDiffBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedDiffBuffer) Write(p []byte) (int, error) {
	size := len(p)
	if remaining := MaxDiffBytes - b.buffer.Len(); size > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return size, nil
}

func (c *ExecGitClient) readLimitedDiff(args []string) (string, bool, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = c.RepoPath
	var out limitedDiffBuffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", false, fmt.Errorf("failed to read diff: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.buffer.String()), out.truncated, nil
}
