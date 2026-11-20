package git

type MockGitClient struct {
	MockHasStagedChanges          func() (bool, error)
	MockHasUnstagedChanges        func() (bool, error)
	MockHasAnyChanges             func() (bool, error)
	MockGetDiff                   func() (string, error)
	MockStageChanges              func() error
	MockCommit                    func(message string) error
	MockAmendCommit               func(message string) error
	MockResolvePushTarget         func(command string) (PushTarget, error)
	MockPreparePush               func(command string) (*PushPlan, error)
	MockPush                      func(plan *PushPlan, force bool) (string, error)
	MockHasRemotes                func() (bool, error)
	MockGetCurrentBranch          func() (string, error)
	MockSetRemoteURL              func(remote, newURL string) error
	MockGetLastCommitHash         func() (string, error)
	MockGetDiffNumStatSummary     func() (additions int, deletions int, err error)
	MockFindBranchBase            func(branch, baseRef string) (string, error)
	MockFindBranchBaseRef         func(branch string) (string, error)
	MockGetCommitCountRange       func(base string) (int, error)
	MockGetDiffRange              func(base string) (string, error)
	MockGetDiffCachedRange        func(base string) (string, error)
	MockGetDiffNumStatCachedRange func(base string) (additions int, deletions int, err error)
	MockResetSoft                 func(commit string) error
	MockStash                     func() (string, error)
	MockStashPop                  func(stash string) error
	MockGetStatusShort            func() (string, error)
	MockGetDefaultBranch          func(remote string) (string, error)
	MockGetPullRequestURL         func(remoteURL, branch string) (string, error)
}

func (m *MockGitClient) HasStagedChanges() (bool, error) {
	if m.MockHasStagedChanges != nil {
		return m.MockHasStagedChanges()
	}
	return false, nil
}

func (m *MockGitClient) HasUnstagedChanges() (bool, error) {
	if m.MockHasUnstagedChanges != nil {
		return m.MockHasUnstagedChanges()
	}
	return false, nil
}

func (m *MockGitClient) HasAnyChanges() (bool, error) {
	if m.MockHasAnyChanges != nil {
		return m.MockHasAnyChanges()
	}
	return false, nil
}

func (m *MockGitClient) GetDiff() (string, error) {
	if m.MockGetDiff != nil {
		return m.MockGetDiff()
	}
	return "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new", nil
}

func (m *MockGitClient) StageChanges() error {
	if m.MockStageChanges != nil {
		return m.MockStageChanges()
	}
	return nil
}

func (m *MockGitClient) Commit(message string) error {
	if m.MockCommit != nil {
		return m.MockCommit(message)
	}
	return nil
}

func (m *MockGitClient) AmendCommit(message string) error {
	if m.MockAmendCommit != nil {
		return m.MockAmendCommit(message)
	}
	return nil
}

func (m *MockGitClient) ResolvePushTarget(command string) (PushTarget, error) {
	if m.MockResolvePushTarget != nil {
		return m.MockResolvePushTarget(command)
	}
	return PushTarget{Remote: "origin"}, nil
}

func (m *MockGitClient) PreparePush(command string) (*PushPlan, error) {
	if m.MockPreparePush != nil {
		return m.MockPreparePush(command)
	}
	target, err := m.ResolvePushTarget(command)
	return &PushPlan{Target: target}, err
}

func (m *MockGitClient) Push(plan *PushPlan, force bool) (string, error) {
	if m.MockPush != nil {
		return m.MockPush(plan, force)
	}
	return "", nil
}

func (m *MockGitClient) HasRemotes() (bool, error) {
	if m.MockHasRemotes != nil {
		return m.MockHasRemotes()
	}
	return true, nil
}

func (m *MockGitClient) GetCurrentBranch() (string, error) {
	if m.MockGetCurrentBranch != nil {
		return m.MockGetCurrentBranch()
	}
	return "", nil
}

func (m *MockGitClient) SetRemoteURL(remote, newURL string) error {
	if m.MockSetRemoteURL != nil {
		return m.MockSetRemoteURL(remote, newURL)
	}
	return nil
}

func (m *MockGitClient) GetLastCommitHash() (string, error) {
	if m.MockGetLastCommitHash != nil {
		return m.MockGetLastCommitHash()
	}
	return "", nil
}

func (m *MockGitClient) GetDiffNumStatSummary() (additions int, deletions int, err error) {
	if m.MockGetDiffNumStatSummary != nil {
		return m.MockGetDiffNumStatSummary()
	}
	return 0, 0, nil
}

func (m *MockGitClient) FindBranchBase(branch, baseRef string) (string, error) {
	if m.MockFindBranchBase != nil {
		return m.MockFindBranchBase(branch, baseRef)
	}
	return "abc123", nil
}

func (m *MockGitClient) FindBranchBaseRef(branch string) (string, error) {
	if m.MockFindBranchBaseRef != nil {
		return m.MockFindBranchBaseRef(branch)
	}
	return "", nil
}

func (m *MockGitClient) GetCommitCountRange(base string) (int, error) {
	if m.MockGetCommitCountRange != nil {
		return m.MockGetCommitCountRange(base)
	}
	return 3, nil
}

func (m *MockGitClient) GetDiffRange(base string) (string, error) {
	if m.MockGetDiffRange != nil {
		return m.MockGetDiffRange(base)
	}
	return "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new", nil
}

func (m *MockGitClient) GetDiffCachedRange(base string) (string, error) {
	if m.MockGetDiffCachedRange != nil {
		return m.MockGetDiffCachedRange(base)
	}
	return "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new", nil
}

func (m *MockGitClient) GetDiffNumStatCachedRange(base string) (int, int, error) {
	if m.MockGetDiffNumStatCachedRange != nil {
		return m.MockGetDiffNumStatCachedRange(base)
	}
	return 0, 0, nil
}

func (m *MockGitClient) ResetSoft(commit string) error {
	if m.MockResetSoft != nil {
		return m.MockResetSoft(commit)
	}
	return nil
}

func (m *MockGitClient) Stash() (string, error) {
	if m.MockStash != nil {
		return m.MockStash()
	}
	return "", nil
}

func (m *MockGitClient) StashPop(stash string) error {
	if m.MockStashPop != nil {
		return m.MockStashPop(stash)
	}
	return nil
}

func (m *MockGitClient) GetStatusShort() (string, error) {
	if m.MockGetStatusShort != nil {
		return m.MockGetStatusShort()
	}
	return "", nil
}

func (m *MockGitClient) GetDefaultBranch(remote string) (string, error) {
	if m.MockGetDefaultBranch != nil {
		return m.MockGetDefaultBranch(remote)
	}
	return "main", nil
}

func (m *MockGitClient) GetPullRequestURL(remoteURL, branch string) (string, error) {
	if m.MockGetPullRequestURL != nil {
		return m.MockGetPullRequestURL(remoteURL, branch)
	}
	return "", nil
}
