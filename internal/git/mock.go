package git

type MockGitClient struct {
	MockHasStagedChanges      func() (bool, error)
	MockHasUnstagedChanges    func() (bool, error)
	MockHasAnyChanges         func() (bool, error)
	MockGetDiff               func() (string, error)
	MockStageChanges          func() error
	MockCommit                func(message string) error
	MockAmendCommit           func() error
	MockPush                  func(command string) (string, error)
	MockHasRemotes            func() (bool, error)
	MockGetCurrentBranch      func() (string, error)
	MockGetRemoteURL          func(remoteName string) (string, error)
	MockGetLastCommitHash     func() (string, error)
	MockGetDiffNumStatSummary func() (additions int, deletions int, err error)
	MockFindBranchBase        func(branch string) (string, error)
	MockGetCommitCountRange   func(base string) (int, error)
	MockGetDiffRange          func(base string) (string, error)
	MockGetDiffNumStatRange   func(base string) (additions int, deletions int, err error)
	MockResetSoft             func(commit string) error
	MockStash                 func() error
	MockStashPop              func() error
	MockPull                  func() error
	MockGetUnpushedCommits    func() ([]string, error)
	MockGetRemoteOnlyCommits  func() ([]string, error)
	MockGetDivergenceVsOrigin func(branch string) ([]string, []string, error)
	MockGetStatusShort        func() (string, error)
	MockGetDefaultBranch      func() (string, error)
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

func (m *MockGitClient) AmendCommit() error {
	if m.MockAmendCommit != nil {
		return m.MockAmendCommit()
	}
	return nil
}

func (m *MockGitClient) Push(command string) (string, error) {
	if m.MockPush != nil {
		return m.MockPush(command)
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

func (m *MockGitClient) GetRemoteURL(remoteName string) (string, error) {
	if m.MockGetRemoteURL != nil {
		return m.MockGetRemoteURL(remoteName)
	}
	return "", nil
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

func (m *MockGitClient) FindBranchBase(branch string) (string, error) {
	if m.MockFindBranchBase != nil {
		return m.MockFindBranchBase(branch)
	}
	return "abc123", nil
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

func (m *MockGitClient) GetDiffNumStatRange(base string) (int, int, error) {
	if m.MockGetDiffNumStatRange != nil {
		return m.MockGetDiffNumStatRange(base)
	}
	return 0, 0, nil
}

func (m *MockGitClient) ResetSoft(commit string) error {
	if m.MockResetSoft != nil {
		return m.MockResetSoft(commit)
	}
	return nil
}

func (m *MockGitClient) Stash() error {
	if m.MockStash != nil {
		return m.MockStash()
	}
	return nil
}

func (m *MockGitClient) StashPop() error {
	if m.MockStashPop != nil {
		return m.MockStashPop()
	}
	return nil
}

func (m *MockGitClient) Pull() error {
	if m.MockPull != nil {
		return m.MockPull()
	}
	return nil
}

func (m *MockGitClient) GetUnpushedCommits() ([]string, error) {
	if m.MockGetUnpushedCommits != nil {
		return m.MockGetUnpushedCommits()
	}
	return nil, nil
}

func (m *MockGitClient) GetRemoteOnlyCommits() ([]string, error) {
	if m.MockGetRemoteOnlyCommits != nil {
		return m.MockGetRemoteOnlyCommits()
	}
	return nil, nil
}

func (m *MockGitClient) GetDivergenceVsOrigin(branch string) ([]string, []string, error) {
	if m.MockGetDivergenceVsOrigin != nil {
		return m.MockGetDivergenceVsOrigin(branch)
	}
	return nil, nil, nil
}

func (m *MockGitClient) GetStatusShort() (string, error) {
	if m.MockGetStatusShort != nil {
		return m.MockGetStatusShort()
	}
	return "", nil
}

func (m *MockGitClient) GetDefaultBranch() (string, error) {
	if m.MockGetDefaultBranch != nil {
		return m.MockGetDefaultBranch()
	}
	return "main", nil
}
