package gemini

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
	"google.golang.org/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MockGeminiClient struct {
	GenerateCommitMessageStreamFunc func(ctx context.Context, systemPrompt, userContent string) (*StreamIterator, error)
}

func (m *MockGeminiClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (*StreamIterator, error) {
	if m.GenerateCommitMessageStreamFunc != nil {
		return m.GenerateCommitMessageStreamFunc(ctx, systemPrompt, userContent)
	}
	return nil, fmt.Errorf("mock GenerateCommitMessageStream not implemented")
}

func TestNewClient(t *testing.T) {
	t.Run("empty API key", func(t *testing.T) {
		client, err := NewClient("", "")
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("valid API key", func(t *testing.T) {
		_, err := NewClient("dummy-api-key", "gemini-flash-latest")
		if err != nil {
			assert.Contains(t, err.Error(), "failed to create Gemini client")
		}
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
			msg := getTextFromResponse(tt.resp)
			assert.Equal(t, tt.expected, msg)
		})
	}
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"plain error", fmt.Errorf("something failed"), false},
		{"context deadline", context.DeadlineExceeded, false},
		{"grpc unavailable", status.Error(codes.Unavailable, "high demand"), true},
		{"grpc deadline exceeded", status.Error(codes.DeadlineExceeded, "deadline expired"), true},
		{"grpc wrapped unavailable", fmt.Errorf("stream: %w", status.Error(codes.Unavailable, "high demand")), true},
		{"grpc wrapped deadline exceeded", fmt.Errorf("stream: %w", status.Error(codes.DeadlineExceeded, "deadline expired")), true},
		{"googleapi 503", &googleapi.Error{Code: 503}, true},
		{"googleapi 504", &googleapi.Error{Code: 504}, true},
		{"googleapi 500", &googleapi.Error{Code: 500}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTransientError(tt.err))
		})
	}
}
