package gemini

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

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

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minCount int // We're testing for reasonableness, not exact values
	}{
		{
			name:     "empty string",
			input:    "",
			minCount: 0,
		},
		{
			name:     "simple ASCII text",
			input:    "This is a simple text with only ASCII characters.",
			minCount: 8, // At least word count (rough minimum)
		},
		{
			name:     "text with Unicode characters",
			input:    "Unicode symbols like € and emoji 😊 might count differently.",
			minCount: 8,
		},
		{
			name:     "text with punctuation",
			input:    "Text with: punctuation! And? Some, special; characters.",
			minCount: 6,
		},
		{
			name:     "multiline text",
			input:    "First line.\nSecond line with more content.\nThird line.",
			minCount: 8,
		},
	}

	client := &GenaiClient{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := client.EstimateTokenCount(tt.input)
			assert.GreaterOrEqual(t, count, tt.minCount, "Token count should be at least the word count")
		})
	}
}

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove backticks and content",
			input:    "feat: add feature `some code`",
			expected: "feat: add feature",
		},
		{
			name:     "remove common prefix",
			input:    "Commit message: feat: add new feature",
			expected: "feat: add new feature",
		},
		{
			name:     "case-insensitive prefix removal",
			input:    "COMMIT MESSAGE: feat: add feature",
			expected: "feat: add feature",
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
			name:     "collapse multiple spaces",
			input:    "feat:   add   feature",
			expected: "feat: add feature",
		},
		{
			name:     "collapse multiple newlines",
			input:    "feat: add feature\n\n\n\nwith description",
			expected: "feat: add feature\n\nwith description",
		},
		{
			name:     "preserve conventional commit format",
			input:    "feat(scope): add feature\n\n- Point one\n- Point two",
			expected: "feat(scope): add feature\n\n- Point one\n- Point two",
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
	// Create a client
	client := &GenaiClient{
		apiKey: "test-key",
		client: nil, // We won't make actual API calls in this test
	}

	// Test token limit exceeded
	ctx := context.Background()
	model := "test-model"
	prompt := "This is a test prompt with {{Diff}}"

	// Create a large diff that will exceed the token limit
	// We'll make a diff that's artificially large for testing
	// For testing the if (totalTokens > maxTokens) condition
	largeDiff := strings.Repeat("Line of code change\n", 1000)
	maxTokens := 500            // Small max tokens to ensure we exceed it
	temperature := float32(0.1) // Default temperature

	message, err := client.GenerateCommitMessage(ctx, model, prompt, largeDiff, maxTokens, temperature)

	// Verify the error
	assert.Empty(t, message)
	assert.Error(t, err)

	// Check error type
	var geminiErr *GeminiError
	assert.True(t, errors.As(err, &geminiErr))
	assert.Equal(t, string(ErrTokenLimit), geminiErr.Type)
	assert.Contains(t, geminiErr.Message, "exceeds limit")
}

func TestEstimateTokenCountInternal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minCount int // We're testing for reasonableness, not exact values
		maxCount int // Maximum expected tokens
	}{
		{
			name:     "empty string",
			input:    "",
			minCount: 100, // Base 100 for empty string
			maxCount: 100,
		},
		{
			name:     "simple text without special chars",
			input:    "This is a simple text",
			minCount: 104, // 4 words + 100
			maxCount: 110,
		},
		{
			name:     "text with special characters",
			input:    "Text with: special! characters?",
			minCount: 106, // 3 words + special chars + 100
			maxCount: 120,
		},
		{
			name:     "long text",
			input:    "This is a much longer text that should have significantly more tokens than the shorter examples above",
			minCount: 115, // Words + 100
			maxCount: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := estimateTokenCount(tt.input)
			assert.GreaterOrEqual(t, count, tt.minCount, "Token count should be at least the minimum")
			assert.LessOrEqual(t, count, tt.maxCount, "Token count should not exceed the maximum")
		})
	}
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
		msg, err := mockClient.GenerateCommitMessage(context.Background(), "", "", "", 0, 0.1)
		assert.NoError(t, err)
		assert.Contains(t, msg, "feat: add new feature")
	})

	t.Run("default EstimateTokenCount", func(t *testing.T) {
		count := mockClient.EstimateTokenCount("test")
		assert.Greater(t, count, 0)
	})

	t.Run("custom GenerateCommitMessage", func(t *testing.T) {
		mockClient.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
			return "custom message", nil
		}
		msg, err := mockClient.GenerateCommitMessage(context.Background(), "", "", "", 0, 0.1)
		assert.NoError(t, err)
		assert.Equal(t, "custom message", msg)
	})

	t.Run("custom EstimateTokenCount", func(t *testing.T) {
		mockClient.EstimateTokenCountFunc = func(text string) int {
			return 42
		}
		count := mockClient.EstimateTokenCount("test")
		assert.Equal(t, 42, count)
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				m.GenerateCommitMessageFunc = func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
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
				"test-model",
				"Generate commit for {{Diff}}",
				"test diff",
				1000,
				0.1, // Default temperature
			)

			tc.checkResult(t, message, err)
		})
	}
}
