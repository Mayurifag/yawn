package git

import "testing"

func TestIsHTTPSRemoteURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://github.com/owner/repo.git", true},
		{"http://github.com/owner/repo.git", true},
		{"HTTPS://github.com/owner/repo.git", true},
		{"git@github.com:owner/repo.git", false},
		{"ssh://git@github.com/owner/repo.git", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			if got := IsHTTPSRemoteURL(tc.url); got != tc.want {
				t.Errorf("IsHTTPSRemoteURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestConvertHTTPSToSSH(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"github https", "https://github.com/owner/repo.git", "git@github.com:owner/repo.git", false},
		{"github https no .git", "https://github.com/owner/repo", "git@github.com:owner/repo.git", false},
		{"gitlab https", "https://gitlab.com/group/project.git", "git@gitlab.com:group/project.git", false},
		{"custom domain", "https://git.example.org/o/r.git", "git@git.example.org:o/r.git", false},
		{"already ssh fails", "git@github.com:owner/repo.git", "", true},
		{"empty fails", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertHTTPSToSSH(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("ConvertHTTPSToSSH(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		out  string
		want bool
	}{
		{"git@github.com: Permission denied (publickey).", true},
		{"fatal: Authentication failed for 'https://...'", true},
		{"could not read Username for 'https://github.com'", true},
		{"remote: HTTP Basic: Access denied", false},
		{"fatal: unable to access ... 401 Unauthorized", true},
		{"fatal: unable to access ... 403 Forbidden", true},
		{"error: failed to push some refs to 'origin' (non-fast-forward)", false},
		{"fatal: could not read from remote repository", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.out, func(t *testing.T) {
			if got := IsAuthError(tc.out); got != tc.want {
				t.Errorf("IsAuthError(%q) = %v, want %v", tc.out, got, tc.want)
			}
		})
	}
}
