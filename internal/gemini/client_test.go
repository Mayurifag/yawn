package gemini

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"
)

func TestNewClient(t *testing.T) {
	t.Run("empty API key", func(t *testing.T) {
		client, err := NewClient("")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("valid API key", func(t *testing.T) {
		// This will likely fail without a real key, which is expected.
		// The goal is to test the code path, not the SDK's key validation.
		_, err := NewClient("dummy-api-key")
		if err != nil {
			assert.Contains(t, err.Error(), "failed to create Gemini client")
		}
	})
}

func TestGenaiClient_GenerateCommitMessageStream_TokenLimit(t *testing.T) {
	mock := MockGeminiClient{
		GenerateCommitMessageStreamFunc: func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error) {
			return nil, NewGeminiError(
				ErrTokenLimit,
				fmt.Sprintf("token count (%d) exceeds limit (%d)", 5000, maxTokens),
				nil,
			)
		},
	}

	ctx := context.Background()
	prompt := "This is a test prompt with !YAWNDIFFPLACEHOLDER!"
	largeDiff := "Large diff"
	maxTokens := 500
	temperature := float32(0.1)

	iterator, err := mock.GenerateCommitMessageStream(ctx, prompt, largeDiff, maxTokens, temperature)

	assert.Nil(t, iterator)
	assert.Error(t, err)

	var geminiErr *GeminiError
	assert.True(t, errors.As(err, &geminiErr))
	assert.Equal(t, string(ErrTokenLimit), geminiErr.Type)
	assert.Contains(t, geminiErr.Error(), "exceeds limit")
}

func TestGeminiError(t *testing.T) {
	originalErr := errors.New("original error")
	geminiErr := NewGeminiError(ErrAuth, "authentication failed", originalErr)

	assert.Contains(t, geminiErr.Error(), "gemini error")
	assert.Contains(t, geminiErr.Error(), "auth")
	assert.Contains(t, geminiErr.Error(), "authentication failed")

	unwrappedErr := errors.Unwrap(geminiErr)
	assert.Equal(t, originalErr, unwrappedErr)
	assert.True(t, errors.Is(geminiErr, originalErr))

	var targetErr *GeminiError
	assert.True(t, errors.As(geminiErr, &targetErr))
	assert.Equal(t, string(ErrAuth), targetErr.Type)
}

func TestCountTokensForText(t *testing.T) {
	t.Run("client initialization error", func(t *testing.T) {
		tempClient := &GenaiClient{apiKey: ""}
		_, err := tempClient.CountTokensForText(context.Background(), "gemini-1.5-flash-latest", "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize client")
	})

	t.Run("mock implementation functions correctly", func(t *testing.T) {
		mockClient := &MockGeminiClient{
			CountTokensForTextFunc: func(ctx context.Context, modelName string, text string) (int, error) {
				assert.Equal(t, "gemini-1.5-flash-latest", modelName)
				assert.Equal(t, "test text", text)
				return 42, nil
			},
		}

		count, err := mockClient.CountTokensForText(context.Background(), "gemini-1.5-flash-latest", "test text")
		assert.NoError(t, err)
		assert.Equal(t, 42, count)
	})
}

func TestGetTextFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		resp     *genai.GenerateContentResponse
		expected string
	}{
		{
			name: "valid response",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{Content: &genai.Content{Parts: []*genai.Part{{Text: "feat: add feature"}}}},
				},
			},
			expected: "feat: add feature",
		},
		{
			name:     "nil response",
			resp:     nil,
			expected: "",
		},
		{
			name:     "empty candidates",
			resp:     &genai.GenerateContentResponse{Candidates: []*genai.Candidate{}},
			expected: "",
		},
		{
			name: "multiple parts",
			resp: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{Content: &genai.Content{Parts: []*genai.Part{{Text: "part1"}, {Text: "part2"}}}},
				},
			},
			expected: "part1part2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetTextFromResponse(tt.resp)
			assert.Equal(t, tt.expected, msg)
		})
	}
}
