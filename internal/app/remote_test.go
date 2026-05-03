package app

import (
	"errors"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/git"
)

func TestEnsureSSHRemote_NoRemotes(t *testing.T) {
	a := &App{
		Config: config.Config{},
		GitClient: &git.MockGitClient{
			MockHasRemotes: func() (bool, error) { return false, nil },
		},
	}
	if err := a.ensureSSHRemote(); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
}

func TestEnsureSSHRemote_AlreadySSH(t *testing.T) {
	called := false
	a := &App{
		Config: config.Config{},
		GitClient: &git.MockGitClient{
			MockHasRemotes:   func() (bool, error) { return true, nil },
			MockGetRemoteURL: func(string) (string, error) { return "git@github.com:o/r.git", nil },
			MockSetRemoteURL: func(string, string) error { called = true; return nil },
		},
	}
	if err := a.ensureSSHRemote(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if called {
		t.Errorf("SetRemoteURL must not be called for SSH remote")
	}
}

func TestIsRetryableNetworkErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"timeout", git.ErrNetworkTimeout, true},
		{"wrapped timeout", errors.New("x: " + git.ErrNetworkTimeout.Error()), false},
		{"auth gitErr", &git.GitError{Output: "Permission denied (publickey)"}, false},
		{"non-fast-forward", &git.GitError{Output: "rejected (non-fast-forward)"}, false},
		{"transient gitErr", &git.GitError{Output: "fatal: unable to access remote: Could not resolve host"}, true},
		{"nil", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableNetworkErr(tc.err); got != tc.want {
				t.Errorf("isRetryableNetworkErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
