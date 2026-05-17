package git

import (
	"fmt"
	"path"
	"strings"
)

type fileCategory int

const (
	catNormal fileCategory = iota
	catBinary
	catLockfile
	catGitCrypt
	catEncrypted
	catSkipped
	catLarge
)

func (c fileCategory) label() string {
	switch c {
	case catBinary:
		return "binary"
	case catLockfile:
		return "lockfile"
	case catGitCrypt:
		return "git-crypt"
	case catEncrypted:
		return "encrypted"
	case catSkipped:
		return "skipped"
	case catLarge:
		return "large diff omitted"
	}
	return "normal"
}

type numstatEntry struct {
	additions string
	deletions string
	binary    bool
	path      string
}

type classifiedFile struct {
	entry    numstatEntry
	category fileCategory
}

var lockfileBasenames = map[string]struct{}{
	"go.sum":            {},
	"package-lock.json": {},
	"yarn.lock":         {},
	"pnpm-lock.yaml":    {},
	"Cargo.lock":        {},
	"Gemfile.lock":      {},
	"uv.lock":           {},
	"composer.lock":     {},
	"Pipfile.lock":      {},
	"poetry.lock":       {},
	"mix.lock":          {},
	"bun.lockb":         {},
	"Podfile.lock":      {},
}

var encryptedSuffixes = []string{".ejson", ".age", ".gpg", ".enc"}

func parseNumstatEntries(output string) []numstatEntry {
	var entries []numstatEntry
	for _, record := range splitNumstatRecords(output) {
		parts := strings.SplitN(record, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		entries = append(entries, numstatEntry{
			additions: parts[0],
			deletions: parts[1],
			binary:    parts[0] == "-" && parts[1] == "-",
			path:      parts[2],
		})
	}
	return entries
}

func splitNumstatRecords(output string) []string {
	var records []string
	for _, r := range strings.Split(output, "\x00") {
		if r == "" {
			continue
		}
		records = append(records, r)
	}
	return records
}

func classifyEntry(e numstatEntry, attrs map[string]string) fileCategory {
	if v := attrs["yawn"]; v == "skip" || v == "set" || v == "true" {
		return catSkipped
	}
	if attrs["filter"] == "git-crypt" || attrs["diff"] == "git-crypt" {
		return catGitCrypt
	}
	base := path.Base(e.path)
	if _, ok := lockfileBasenames[base]; ok {
		return catLockfile
	}
	for _, suffix := range encryptedSuffixes {
		if strings.HasSuffix(base, suffix) {
			return catEncrypted
		}
	}
	if e.binary {
		return catBinary
	}
	return catNormal
}

func formatRedactedSummary(redacted []classifiedFile) string {
	if len(redacted) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("### Files redacted from diff (summary only):\n")
	for _, r := range redacted {
		var stat string
		if r.entry.binary {
			stat = "binary"
		} else {
			stat = fmt.Sprintf("+%s -%s", r.entry.additions, r.entry.deletions)
		}
		fmt.Fprintf(&b, "- %s: %s, %s\n", r.entry.path, r.category.label(), stat)
	}
	return b.String()
}

func parseCheckAttrOutput(output string) map[string]map[string]string {
	result := map[string]map[string]string{}
	parts := strings.Split(output, "\x00")
	for i := 0; i+2 < len(parts); i += 3 {
		path, attr, val := parts[i], parts[i+1], parts[i+2]
		if path == "" {
			continue
		}
		if result[path] == nil {
			result[path] = map[string]string{}
		}
		result[path][attr] = val
	}
	return result
}
