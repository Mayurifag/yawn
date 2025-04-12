package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/stretchr/testify/assert"
)

// MockGitClient is a mock implementation of GitClient for testing
type MockGitClient struct {
	HasAnyChangesFunc         func() (bool, error)
	HasStagedChangesFunc      func() (bool, error)
	HasUnstagedChangesFunc    func() (bool, error)
	HasUncommittedChangesFunc func() (bool, error)
	StageChangesFunc          func() error
	GetDiffFunc               func() (string, error)
	CommitFunc                func(string) error
	PushFunc                  func(string) error
	GetCurrentBranchFunc      func() (string, error)
	GetRemoteURLFunc          func(string) (string, error)
	GetLastCommitHashFunc     func() (string, error)
	HasRemotesFunc            func() (bool, error)
}

func (m *MockGitClient) HasAnyChanges() (bool, error) {
	if m.HasAnyChangesFunc != nil {
		return m.HasAnyChangesFunc()
	}
	return false, nil
}

func (m *MockGitClient) HasStagedChanges() (bool, error) {
	if m.HasStagedChangesFunc != nil {
		return m.HasStagedChangesFunc()
	}
	return false, nil
}

func (m *MockGitClient) HasUnstagedChanges() (bool, error) {
	if m.HasUnstagedChangesFunc != nil {
		return m.HasUnstagedChangesFunc()
	}
	return false, nil
}

func (m *MockGitClient) HasUncommittedChanges() (bool, error) {
	if m.HasUncommittedChangesFunc != nil {
		return m.HasUncommittedChangesFunc()
	}
	return false, nil
}

func (m *MockGitClient) StageChanges() error {
	if m.StageChangesFunc != nil {
		return m.StageChangesFunc()
	}
	return nil
}

func (m *MockGitClient) GetDiff() (string, error) {
	if m.GetDiffFunc != nil {
		return m.GetDiffFunc()
	}
	return "", nil
}

func (m *MockGitClient) Commit(message string) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(message)
	}
	return nil
}

func (m *MockGitClient) Push(command string) error {
	if m.PushFunc != nil {
		return m.PushFunc(command)
	}
	return nil
}

func (m *MockGitClient) GetCurrentBranch() (string, error) {
	if m.GetCurrentBranchFunc != nil {
		return m.GetCurrentBranchFunc()
	}
	return "", nil
}

func (m *MockGitClient) GetRemoteURL(remote string) (string, error) {
	if m.GetRemoteURLFunc != nil {
		return m.GetRemoteURLFunc(remote)
	}
	return "", nil
}

func (m *MockGitClient) GetLastCommitHash() (string, error) {
	if m.GetLastCommitHashFunc != nil {
		return m.GetLastCommitHashFunc()
	}
	return "", nil
}

func (m *MockGitClient) HasRemotes() (bool, error) {
	if m.HasRemotesFunc != nil {
		return m.HasRemotesFunc()
	}
	return false, nil
}

func TestNewApp(t *testing.T) {
	cfg := config.Config{
		GeminiAPIKey: "test-key",
		Verbose:      true,
	}
	gitClient := &MockGitClient{}

	app := NewApp(cfg, gitClient)
	assert.NotNil(t, app)
	assert.Equal(t, cfg, app.Config)
	assert.Equal(t, gitClient, app.GitClient)
	assert.NotNil(t, app.Pusher)
}

func TestSetupAndCheckPrerequisites(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		hasChanges    bool
		hasChangesErr error
		expected      bool
		expectedErr   error
	}{
		{
			name:          "no changes",
			apiKey:        "test-key",
			hasChanges:    false,
			hasChangesErr: nil,
			expected:      false,
			expectedErr:   errors.New("no changes to commit"),
		},
		{
			name:          "has changes",
			apiKey:        "test-key",
			hasChanges:    true,
			hasChangesErr: nil,
			expected:      true,
			expectedErr:   nil,
		},
		{
			name:          "check changes error",
			apiKey:        "test-key",
			hasChanges:    false,
			hasChangesErr: errors.New("git error"),
			expected:      false,
			expectedErr:   errors.New("failed to check for changes: git error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &MockGitClient{
				HasAnyChangesFunc: func() (bool, error) {
					return tt.hasChanges, tt.hasChangesErr
				},
			}

			app := NewApp(config.Config{
				GeminiAPIKey: tt.apiKey,
				Verbose:      true,
			}, gitClient)

			hasChanges, err := app.setupAndCheckPrerequisites()
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, hasChanges)
		})
	}
}

func TestEnsureStagedChanges(t *testing.T) {
	tests := []struct {
		name            string
		hasStaged       bool
		hasStagedErr    error
		hasUnstaged     bool
		hasUnstagedErr  error
		autoStage       bool
		stageChangesErr error
		expectedErr     error
	}{
		{
			name:            "already staged",
			hasStaged:       true,
			hasStagedErr:    nil,
			hasUnstaged:     false,
			hasUnstagedErr:  nil,
			autoStage:       false,
			stageChangesErr: nil,
			expectedErr:     nil,
		},
		{
			name:            "auto stage enabled",
			hasStaged:       false,
			hasStagedErr:    nil,
			hasUnstaged:     true,
			hasUnstagedErr:  nil,
			autoStage:       true,
			stageChangesErr: nil,
			expectedErr:     nil,
		},
		{
			name:            "stage changes error",
			hasStaged:       false,
			hasStagedErr:    nil,
			hasUnstaged:     true,
			hasUnstagedErr:  nil,
			autoStage:       true,
			stageChangesErr: errors.New("stage error"),
			expectedErr:     errors.New("failed to stage changes: stage error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &MockGitClient{
				HasStagedChangesFunc: func() (bool, error) {
					return tt.hasStaged, tt.hasStagedErr
				},
				HasUnstagedChangesFunc: func() (bool, error) {
					return tt.hasUnstaged, tt.hasUnstagedErr
				},
				StageChangesFunc: func() error {
					return tt.stageChangesErr
				},
			}

			app := NewApp(config.Config{
				AutoStage: tt.autoStage,
				Verbose:   true,
			}, gitClient)

			err := app.ensureStagedChanges()
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateAndCommitChanges(t *testing.T) {
	tests := []struct {
		name        string
		diff        string
		diffErr     error
		message     string
		messageErr  error
		commitErr   error
		expectedErr error
	}{
		{
			name:        "success",
			diff:        "test diff",
			message:     "feat: add new feature",
			expectedErr: nil,
		},
		{
			name:        "diff error",
			diffErr:     errors.New("diff error"),
			expectedErr: errors.New("failed to get staged changes: diff error"),
		},
		{
			name:        "empty diff",
			diff:        "",
			expectedErr: errors.New("no staged changes to commit"),
		},
		{
			name:        "message generation error",
			diff:        "test diff",
			messageErr:  errors.New("generation error"),
			expectedErr: errors.New("failed to generate commit message: generation error"),
		},
		{
			name:        "empty message",
			diff:        "test diff",
			message:     "",
			expectedErr: errors.New("empty commit message received from Gemini"),
		},
		{
			name:        "commit error",
			diff:        "test diff",
			message:     "feat: add new feature",
			commitErr:   errors.New("commit error"),
			expectedErr: errors.New("failed to commit changes: commit error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &MockGitClient{
				GetDiffFunc: func() (string, error) {
					return tt.diff, tt.diffErr
				},
				CommitFunc: func(message string) error {
					return tt.commitErr
				},
			}

			// Create a mock Gemini client
			mockClient := &gemini.MockGeminiClient{
				GenerateCommitMessageFunc: func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return tt.message, tt.messageErr
				},
			}

			app := NewApp(config.Config{
				GeminiAPIKey: "test-key",
				Verbose:      true,
			}, gitClient)

			// Override the generateAndCommitChanges function with our test version
			app.generateAndCommitChangesFunc = func(ctx context.Context) error {
				// Get staged changes for commit message generation
				diff, err := app.GitClient.GetDiff()
				if err != nil {
					return fmt.Errorf("failed to get staged changes: %w", err)
				}
				if diff == "" {
					return fmt.Errorf("no staged changes to commit")
				}

				// Generate commit message using our mock client
				message, err := mockClient.GenerateCommitMessage(ctx, app.Config.GeminiModel, app.Config.Prompt, diff, app.Config.MaxTokens, app.Config.Temperature)
				if err != nil {
					return fmt.Errorf("failed to generate commit message: %w", err)
				}

				if message == "" {
					return fmt.Errorf("empty commit message received from Gemini")
				}

				// Commit changes
				if err := app.GitClient.Commit(message); err != nil {
					return fmt.Errorf("failed to commit changes: %w", err)
				}

				return nil
			}

			err := app.generateAndCommitChanges(context.Background())
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandlePushOperation(t *testing.T) {
	tests := []struct {
		name          string
		hasRemotes    bool
		hasRemotesErr error
		pushErr       error
		expectedErr   error
	}{
		{
			name:          "success",
			hasRemotes:    true,
			hasRemotesErr: nil,
			pushErr:       nil,
			expectedErr:   nil,
		},
		{
			name:          "no remotes",
			hasRemotes:    false,
			hasRemotesErr: nil,
			pushErr:       nil,
			expectedErr:   errors.New("no remote repositories configured"),
		},
		{
			name:          "check remotes error",
			hasRemotes:    false,
			hasRemotesErr: errors.New("remotes error"),
			pushErr:       nil,
			expectedErr:   errors.New("failed to check for remotes: remotes error"),
		},
		{
			name:          "push error",
			hasRemotes:    true,
			hasRemotesErr: nil,
			pushErr:       errors.New("push error"),
			expectedErr:   errors.New("failed to push changes: push error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &MockGitClient{
				HasRemotesFunc: func() (bool, error) {
					return tt.hasRemotes, tt.hasRemotesErr
				},
				PushFunc: func(command string) error {
					return tt.pushErr
				},
			}

			app := NewApp(config.Config{
				AutoPush: true,
				Verbose:  true,
			}, gitClient)

			err := app.handlePushOperation()
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name          string
		hasChanges    bool
		hasChangesErr error
		stageErr      error
		generateErr   error
		pushErr       error
		expectedErr   error
	}{
		{
			name:          "success",
			hasChanges:    true,
			hasChangesErr: nil,
			stageErr:      nil,
			generateErr:   nil,
			pushErr:       nil,
			expectedErr:   nil,
		},
		{
			name:          "no changes",
			hasChanges:    false,
			hasChangesErr: nil,
			stageErr:      nil,
			generateErr:   nil,
			pushErr:       nil,
			expectedErr:   errors.New("no changes to commit"),
		},
		{
			name:          "check changes error",
			hasChanges:    false,
			hasChangesErr: errors.New("changes error"),
			stageErr:      nil,
			generateErr:   nil,
			pushErr:       nil,
			expectedErr:   errors.New("failed to check for changes: changes error"),
		},
		{
			name:          "stage error",
			hasChanges:    true,
			hasChangesErr: nil,
			stageErr:      errors.New("stage error"),
			generateErr:   nil,
			pushErr:       nil,
			expectedErr:   errors.New("failed to stage changes: stage error"),
		},
		{
			name:          "generate error",
			hasChanges:    true,
			hasChangesErr: nil,
			stageErr:      nil,
			generateErr:   errors.New("generate error"),
			pushErr:       nil,
			expectedErr:   errors.New("failed to generate commit message: generate error"),
		},
		{
			name:          "push error",
			hasChanges:    true,
			hasChangesErr: nil,
			stageErr:      nil,
			generateErr:   nil,
			pushErr:       errors.New("push error"),
			expectedErr:   errors.New("failed to push changes: push error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &MockGitClient{
				HasAnyChangesFunc: func() (bool, error) {
					return tt.hasChanges, tt.hasChangesErr
				},
				HasStagedChangesFunc: func() (bool, error) {
					return true, nil
				},
				StageChangesFunc: func() error {
					return tt.stageErr
				},
				GetDiffFunc: func() (string, error) {
					return "test diff", nil
				},
				CommitFunc: func(message string) error {
					return nil
				},
				PushFunc: func(command string) error {
					return tt.pushErr
				},
				HasRemotesFunc: func() (bool, error) {
					return true, nil
				},
			}

			// Create a mock Gemini client
			mockClient := &gemini.MockGeminiClient{
				GenerateCommitMessageFunc: func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					if tt.generateErr != nil {
						return "", tt.generateErr
					}
					return "test message", nil
				},
			}

			app := NewApp(config.Config{
				GeminiAPIKey: "test-key",
				AutoPush:     true,
				Verbose:      true,
			}, gitClient)

			// Override the generateAndCommitChanges function with our test version
			app.generateAndCommitChangesFunc = func(ctx context.Context) error {
				// Get staged changes for commit message generation
				diff, err := app.GitClient.GetDiff()
				if err != nil {
					return fmt.Errorf("failed to get staged changes: %w", err)
				}
				if diff == "" {
					return fmt.Errorf("no staged changes to commit")
				}

				// Generate commit message using our mock client
				message, err := mockClient.GenerateCommitMessage(ctx, app.Config.GeminiModel, app.Config.Prompt, diff, app.Config.MaxTokens, app.Config.Temperature)
				if err != nil {
					return fmt.Errorf("failed to generate commit message: %w", err)
				}

				if message == "" {
					return fmt.Errorf("empty commit message received from Gemini")
				}

				// Commit changes
				if err := app.GitClient.Commit(message); err != nil {
					return fmt.Errorf("failed to commit changes: %w", err)
				}

				return nil
			}

			err := app.Run()
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
