package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNumstatEntries(t *testing.T) {
	output := "10\t5\tfoo.go\x00-\t-\timg.png\x003\t0\tdir/bar.txt\x00"
	entries := parseNumstatEntries(output)
	assert.Len(t, entries, 3)

	assert.Equal(t, "foo.go", entries[0].path)
	assert.False(t, entries[0].binary)
	assert.Equal(t, "10", entries[0].additions)
	assert.Equal(t, "5", entries[0].deletions)

	assert.Equal(t, "img.png", entries[1].path)
	assert.True(t, entries[1].binary)

	assert.Equal(t, "dir/bar.txt", entries[2].path)
	assert.False(t, entries[2].binary)
}

func TestParseNumstatEntries_SkipsMalformed(t *testing.T) {
	entries := parseNumstatEntries("\x00broken record\x0010\t5\tok.go\x00")
	assert.Len(t, entries, 1)
	assert.Equal(t, "ok.go", entries[0].path)
}

func TestParseNumstatEntries_PathWithNewline(t *testing.T) {
	output := "1\t0\tweird\nname.txt\x002\t1\tfoo.go\x00"
	entries := parseNumstatEntries(output)
	assert.Len(t, entries, 2)
	assert.Equal(t, "weird\nname.txt", entries[0].path)
	assert.Equal(t, "foo.go", entries[1].path)
}

func TestClassifyEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry numstatEntry
		attrs map[string]string
		want  fileCategory
	}{
		{
			name:  "normal text file",
			entry: numstatEntry{path: "main.go"},
			want:  catNormal,
		},
		{
			name:  "binary by numstat",
			entry: numstatEntry{path: "logo.png", binary: true},
			want:  catBinary,
		},
		{
			name:  "git-crypt via filter attr",
			entry: numstatEntry{path: "secrets.yml"},
			attrs: map[string]string{"filter": "git-crypt", "diff": "git-crypt"},
			want:  catGitCrypt,
		},
		{
			name:  "git-crypt via diff attr only",
			entry: numstatEntry{path: "secrets.yml"},
			attrs: map[string]string{"diff": "git-crypt"},
			want:  catGitCrypt,
		},
		{
			name:  "ejson encrypted",
			entry: numstatEntry{path: "config/secrets.ejson"},
			want:  catEncrypted,
		},
		{
			name:  "age encrypted",
			entry: numstatEntry{path: "vault/key.age"},
			want:  catEncrypted,
		},
		{
			name:  "gpg encrypted",
			entry: numstatEntry{path: "secrets.gpg"},
			want:  catEncrypted,
		},
		{
			name:  "lockfile package-lock.json",
			entry: numstatEntry{path: "package-lock.json"},
			want:  catLockfile,
		},
		{
			name:  "lockfile go.sum nested",
			entry: numstatEntry{path: "module/go.sum"},
			want:  catLockfile,
		},
		{
			name:  "yawn=skip attr",
			entry: numstatEntry{path: "any.txt"},
			attrs: map[string]string{"yawn": "skip"},
			want:  catSkipped,
		},
		{
			name:  "yawn boolean set attr",
			entry: numstatEntry{path: "any.txt"},
			attrs: map[string]string{"yawn": "set"},
			want:  catSkipped,
		},
		{
			name:  "git-crypt beats encrypted suffix",
			entry: numstatEntry{path: "secrets.gpg"},
			attrs: map[string]string{"filter": "git-crypt"},
			want:  catGitCrypt,
		},
		{
			name:  "yawn skip beats binary",
			entry: numstatEntry{path: "img.png", binary: true},
			attrs: map[string]string{"yawn": "skip"},
			want:  catSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEntry(tt.entry, tt.attrs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatRedactedSummary(t *testing.T) {
	redacted := []classifiedFile{
		{entry: numstatEntry{path: "package-lock.json", additions: "120", deletions: "45"}, category: catLockfile},
		{entry: numstatEntry{path: "img.png", binary: true}, category: catBinary},
		{entry: numstatEntry{path: "secrets.ejson", additions: "2", deletions: "1"}, category: catEncrypted},
		{entry: numstatEntry{path: "vault/key.txt", additions: "3", deletions: "0"}, category: catGitCrypt},
	}
	got := formatRedactedSummary(redacted)

	assert.True(t, strings.HasPrefix(got, "### Files redacted from diff"))
	assert.Contains(t, got, "package-lock.json: lockfile, +120 -45")
	assert.Contains(t, got, "img.png: binary, binary")
	assert.Contains(t, got, "secrets.ejson: encrypted, +2 -1")
	assert.Contains(t, got, "vault/key.txt: git-crypt, +3 -0")
}

func TestFormatRedactedSummary_Empty(t *testing.T) {
	assert.Equal(t, "", formatRedactedSummary(nil))
}

func TestParseCheckAttrOutput(t *testing.T) {
	out := "foo.go\x00filter\x00unspecified\x00foo.go\x00diff\x00unspecified\x00secrets.yml\x00filter\x00git-crypt\x00"
	got := parseCheckAttrOutput(out)

	assert.Equal(t, "unspecified", got["foo.go"]["filter"])
	assert.Equal(t, "unspecified", got["foo.go"]["diff"])
	assert.Equal(t, "git-crypt", got["secrets.yml"]["filter"])
}

func TestParseCheckAttrOutput_Empty(t *testing.T) {
	got := parseCheckAttrOutput("")
	assert.Empty(t, got)
}
