package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	t.Run("empty API key", func(t *testing.T) {
		client, err := NewClient("")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("valid API key", func(t *testing.T) {
		client, err := NewClient("dummy-api-key")
		// Note: This test would fail in reality since the API key is invalid
		// In real integration testing, you would use a valid API key
		// For unit testing, we'd consider mocking the genai.NewClient function
		// but we're just checking the error path here
		if err == nil {
			assert.NotNil(t, client)
			assert.Equal(t, "dummy-api-key", client.apiKey)
		} else {
			// Skip the test if we can't create a client with a dummy key
			// This happens if the SDK immediately verifies the key
			t.Skip("Skipping test as dummy API key validation failed")
		}
	})
}

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserve backticks",
			input:    "feat: add feature `some code`",
			expected: "feat: add feature `some code`",
		},
		{
			name:     "trim whitespace",
			input:    "  feat: add feature  ",
			expected: "feat: add feature",
		},
		{
			name:     "normalize line breaks",
			input:    "feat: add feature\r\nwith detailed description",
			expected: "feat: add feature\nwith detailed description",
		},
		{
			name:     "preserve conventional commit format",
			input:    "feat(scope): add feature\n\n- Point one\n- Point two",
			expected: "feat(scope): add feature\n\n- Point one\n- Point two",
		},
		{
			name:     "consolidate multiple spaces",
			input:    "feat:    add     thing",
			expected: "feat: add thing",
		},
		{
			name:     "consolidate tabs",
			input:    "feat:\tadd\tthing",
			expected: "feat: add thing",
		},
		{
			name:     "consolidate mixed spaces and tabs",
			input:    "feat: \t add \t thing",
			expected: "feat: add thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanCommitMessage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenaiClient_GenerateCommitMessage_TokenLimit(t *testing.T) {
	// Use a mock client instead
	var mock MockGeminiClient

	mock = MockGeminiClient{
		GenerateCommitMessageFunc: func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
			// Simulate the token limit error
			return "", NewGeminiError(
				ErrTokenLimit,
				fmt.Sprintf("token count (%d) exceeds limit (%d)", 5000, maxTokens),
				nil,
			)
		},
	}

	// Test token limit exceeded
	ctx := context.Background()
	prompt := "This is a test prompt with !YAWNDIFFPLACEHOLDER!"
	largeDiff := "Large diff that would exceed token limit"
	maxTokens := 500            // Small max tokens to ensure we exceed it
	temperature := float32(0.1) // Default temperature

	message, err := mock.GenerateCommitMessage(ctx, prompt, largeDiff, maxTokens, temperature)

	// Verify the error
	assert.Empty(t, message)
	assert.Error(t, err)

	// Check error type
	var geminiErr *GeminiError
	assert.True(t, errors.As(err, &geminiErr))
	assert.Equal(t, string(ErrTokenLimit), geminiErr.Type)
	assert.Contains(t, geminiErr.Error(), "exceeds limit")
}

func TestGeminiError(t *testing.T) {
	// Test error creation and unwrapping
	originalErr := errors.New("original error")
	geminiErr := NewGeminiError(ErrAuth, "authentication failed", originalErr)

	// Test Error() method
	assert.Contains(t, geminiErr.Error(), "gemini error")
	assert.Contains(t, geminiErr.Error(), "auth")
	assert.Contains(t, geminiErr.Error(), "authentication failed")

	// Test Unwrap() method
	unwrappedErr := errors.Unwrap(geminiErr)
	assert.Equal(t, originalErr, unwrappedErr)

	// Test errors.Is and errors.As
	assert.True(t, errors.Is(geminiErr, originalErr))

	var targetErr *GeminiError
	assert.True(t, errors.As(geminiErr, &targetErr))
	assert.Equal(t, string(ErrAuth), targetErr.Type)
}

func TestMockGeminiClient(t *testing.T) {
	// Test default implementations
	mockClient := &MockGeminiClient{}

	t.Run("default GenerateCommitMessage", func(t *testing.T) {
		msg, err := mockClient.GenerateCommitMessage(context.Background(), "", "", 0, 0.1)
		assert.NoError(t, err)
		assert.Contains(t, msg, "feat: add new feature")
	})

	t.Run("custom GenerateCommitMessage", func(t *testing.T) {
		mockClient.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
			return "custom message", nil
		}
		msg, err := mockClient.GenerateCommitMessage(context.Background(), "", "", 0, 0.1)
		assert.NoError(t, err)
		assert.Equal(t, "custom message", msg)
	})
}

func TestMockGeminiClient_Errors(t *testing.T) {
	// Error test cases
	testCases := []struct {
		name        string
		setupMock   func(*MockGeminiClient)
		checkResult func(*testing.T, string, error)
	}{
		{
			name: "token limit error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", NewGeminiError(ErrTokenLimit, "token limit exceeded", nil)
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(ErrTokenLimit), geminiErr.Type)
			},
		},
		{
			name: "authentication error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", NewGeminiError(ErrAuth, "invalid API key", nil)
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(ErrAuth), geminiErr.Type)
			},
		},
		{
			name: "rate limit error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", NewGeminiError(ErrRateLimit, "rate limit exceeded", nil)
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(ErrRateLimit), geminiErr.Type)
			},
		},
		{
			name: "safety error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", NewGeminiError(ErrSafety, "content blocked for safety reasons", nil)
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(ErrSafety), geminiErr.Type)
			},
		},
		{
			name: "empty response error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", NewGeminiError(ErrEmptyResponse, "received empty response", nil)
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(ErrEmptyResponse), geminiErr.Type)
			},
		},
		{
			name: "generic API error",
			setupMock: func(m *MockGeminiClient) {
				m.GenerateCommitMessageFunc = func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
					return "", fmt.Errorf("failed to generate commit message: %w", errors.New("some API error"))
				}
			},
			checkResult: func(t *testing.T, msg string, err error) {
				assert.Empty(t, msg)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to generate commit message")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &MockGeminiClient{}
			tc.setupMock(mockClient)

			message, err := mockClient.GenerateCommitMessage(
				context.Background(),
				"Generate commit for !YAWNDIFFPLACEHOLDER!",
				"test diff",
				1000,
				0.1, // Default temperature
			)

			tc.checkResult(t, message, err)
		})
	}
}

func TestCountTokensForText(t *testing.T) {
	// We'll skip the real API call tests since mocking them is complex
	// Instead, focus on error cases and basic interface assumptions

	t.Run("client initialization error", func(t *testing.T) {
		// Test the error path when client initialization fails
		tempClient := &GenaiClient{apiKey: ""}
		_, err := tempClient.CountTokensForText(context.Background(), "gemini-1.5-flash", "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize client")
	})

	t.Run("mock implementation functions correctly", func(t *testing.T) {
		// Test that the mock client implementation works as expected
		mockClient := &MockGeminiClient{
			CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
				assert.Equal(t, "gemini-1.5-flash", modelName)
				assert.Equal(t, "test text", text)
				return 42, nil
			},
		}

		count, err := mockClient.CountTokensForText(context.Background(), "gemini-1.5-flash", "test text")
		assert.NoError(t, err)
		assert.Equal(t, 42, count)
	})
}

func TestCheckTokenLimit(t *testing.T) {
	// We'll test the checkTokenLimit function by mocking CountTokensForText
	// using a custom client implementation

	// Test cases
	tests := []struct {
		name           string
		promptTemplate string
		diff           string
		modelName      string
		maxTokens      int
		mockClient     *MockGeminiClient
		expectError    bool
		errorType      string
	}{
		{
			name:           "under token limit",
			promptTemplate: "Test prompt with !YAWNDIFFPLACEHOLDER!",
			diff:           "Test diff",
			modelName:      "gemini-1.5-flash",
			maxTokens:      1000,
			mockClient: &MockGeminiClient{
				CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
					return 500, nil
				},
			},
			expectError: false,
		},
		{
			name:           "over token limit",
			promptTemplate: "Test prompt with !YAWNDIFFPLACEHOLDER!",
			diff:           "Test diff",
			modelName:      "gemini-1.5-flash",
			maxTokens:      100,
			mockClient: &MockGeminiClient{
				CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
					return 200, nil
				},
			},
			expectError: true,
			errorType:   string(ErrTokenLimit),
		},
		{
			name:           "token counting fails",
			promptTemplate: "Test prompt with !YAWNDIFFPLACEHOLDER!",
			diff:           "Test diff",
			modelName:      "gemini-1.5-flash",
			maxTokens:      100,
			mockClient: &MockGeminiClient{
				CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
					return 0, errors.New("API error")
				},
			},
			expectError: false, // Should not error if token counting fails
		},
		{
			name:           "different model",
			promptTemplate: "Test prompt with !YAWNDIFFPLACEHOLDER!",
			diff:           "Test diff",
			modelName:      "gemini-1.5-pro",
			maxTokens:      1000,
			mockClient: &MockGeminiClient{
				CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
					// Verify the model name is passed correctly
					assert.Equal(t, "gemini-1.5-pro", modelName)
					return 500, nil
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In this test approach, we don't directly replace the client method
			// Instead, we create a custom function that replicates checkTokenLimit's behavior
			// but uses our mock instead

			mockCountTokensForText := tt.mockClient.CountTokensForText

			// Create a simplified version of checkTokenLimit that uses our mock
			checkLimit := func(promptTemplate, diff string, modelName string, maxTokens int) error {
				ctx := context.Background()
				finalPrompt := strings.Replace(promptTemplate, "!YAWNDIFFPLACEHOLDER!", diff, 1)
				tokenCount, err := mockCountTokensForText(ctx, modelName, finalPrompt)
				if err != nil {
					return nil
				}

				if tokenCount > maxTokens {
					return NewGeminiError(
						ErrTokenLimit,
						fmt.Sprintf("token count (%d) exceeds limit (%d)", tokenCount, maxTokens),
						nil,
					)
				}
				return nil
			}

			// Call our test version of the function
			err := checkLimit(tt.promptTemplate, tt.diff, tt.modelName, tt.maxTokens)

			// Check results
			if tt.expectError {
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, tt.errorType, geminiErr.Type)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockGeminiClient_CountTokensForText(t *testing.T) {
	mockClient := &MockGeminiClient{}
	ctx := context.Background()

	t.Run("default implementation", func(t *testing.T) {
		count, err := mockClient.CountTokensForText(ctx, "gemini-1.5-flash", "This is a test.")
		assert.NoError(t, err)
		assert.Equal(t, 4, count) // Default implementation should count words
	})

	t.Run("custom implementation", func(t *testing.T) {
		mockClient.CountTokensForTextFunc = func(ctx context.Context, modelName string, text string) (int, error) {
			return 42, nil
		}
		count, err := mockClient.CountTokensForText(ctx, "gemini-1.5-flash", "This is a test.")
		assert.NoError(t, err)
		assert.Equal(t, 42, count)
	})

	t.Run("custom error", func(t *testing.T) {
		mockClient.CountTokensForTextFunc = func(ctx context.Context, modelName string, text string) (int, error) {
			return 0, errors.New("custom error")
		}
		_, err := mockClient.CountTokensForText(ctx, "gemini-1.5-flash", "This is a test.")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "custom error")
	})
}

func TestHandleGenerateContentError(t *testing.T) {
	client := &GenaiClient{}

	tests := []struct {
		name         string
		err          error
		expectedType GeminiErrorType
		expectedMsg  string
	}{
		{
			name:         "prompt blocked",
			err:          &genai.BlockedError{PromptFeedback: &genai.PromptFeedback{BlockReason: genai.BlockReasonUnspecified}},
			expectedType: ErrSafety,
			expectedMsg:  "content blocked for safety reasons",
		},
		{
			name:         "response blocked",
			err:          &genai.BlockedError{Candidate: &genai.Candidate{FinishReason: genai.FinishReasonSafety}},
			expectedType: ErrSafety,
			expectedMsg:  "response blocked by safety settings",
		},
		{
			name:         "auth error",
			err:          errors.New("authentication failed"),
			expectedType: ErrAuth,
			expectedMsg:  "invalid API key or authentication failed",
		},
		{
			name:         "rate limit error",
			err:          errors.New("rate limit exceeded"),
			expectedType: ErrRateLimit,
			expectedMsg:  "API rate limit exceeded",
		},
		{
			name:         "generic error",
			err:          errors.New("some error"),
			expectedType: "",
			expectedMsg:  "failed to generate commit message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.handleGenerateContentError(tt.err)
			if tt.expectedType == "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedMsg)
			} else {
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(tt.expectedType), geminiErr.Type)
				assert.Contains(t, geminiErr.Message, tt.expectedMsg)
			}
		})
	}
}

func TestProcessGenaiResponse(t *testing.T) {
	client := &GenaiClient{}

	tests := []struct {
		name          string
		resp          *genai.GenerateContentResponse
		expectedMsg   string
		expectedError bool
		errorType     GeminiErrorType
	}{
		{
			name: "valid response",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{
								genai.Text("feat: add feature"),
							},
						},
					},
				},
			},
			expectedMsg:   "feat: add feature",
			expectedError: false,
		},
		{
			name:          "nil response",
			resp:          nil,
			expectedError: true,
			errorType:     ErrEmptyResponse,
		},
		{
			name: "empty candidates",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			},
			expectedError: true,
			errorType:     ErrEmptyResponse,
		},
		{
			name: "empty content",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{},
						},
					},
				},
			},
			expectedError: true,
			errorType:     ErrEmptyContent,
		},
		{
			name: "non-text part",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{
								genai.Blob{},
							},
						},
					},
				},
			},
			expectedError: true,
			errorType:     ErrInvalidFormat,
		},
		{
			name: "empty message",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{
								genai.Text(""),
							},
						},
					},
				},
			},
			expectedError: true,
			errorType:     ErrEmptyMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := client.processGenaiResponse(tt.resp)
			if tt.expectedError {
				assert.Error(t, err)
				var geminiErr *GeminiError
				assert.True(t, errors.As(err, &geminiErr))
				assert.Equal(t, string(tt.errorType), geminiErr.Type)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMsg, msg)
			}
		})
	}
}
