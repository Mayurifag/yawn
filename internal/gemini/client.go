package gemini

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

const (
	PrimaryModel  = "gemini-2.5-flash"
	FallbackModel = "gemini-2.5-flash-lite"
)

// StreamIterator wraps the Go 1.23+ iterator to provide a Next() method
type StreamIterator struct {
	stream func() (*genai.GenerateContentResponse, error)
	done   bool
}

// Next returns the next response from the stream
func (s *StreamIterator) Next() (*genai.GenerateContentResponse, error) {
	if s.done {
		return nil, iterator.Done
	}

	resp, err := s.stream()
	if err != nil {
		if err.Error() == "iterator done" {
			s.done = true
			return nil, iterator.Done
		}
		return nil, err
	}
	return resp, nil
}

// Client defines the interface for interacting with the Gemini API.
type Client interface {
	GenerateCommitMessageStream(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error)
	CountTokensForText(ctx context.Context, modelName string, text string) (int, error)
}

// GenaiClient implements the Client interface using the official Google GenAI SDK.
type GenaiClient struct {
	apiKey string
	client *genai.Client
}

// NewClient creates a new Gemini client.
func NewClient(apiKey string) (*GenaiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	c := &GenaiClient{
		apiKey: apiKey,
	}

	if err := c.initClient(); err != nil {
		return nil, err
	}

	return c, nil
}

// initClient initializes the underlying genai.Client.
func (c *GenaiClient) initClient() error {
	if c.apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  c.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	c.client = client
	return nil
}

// GeminiError represents specific error conditions from the Gemini API.
type GeminiError struct {
	Type    string
	Message string
	Err     error
}

func (e *GeminiError) Error() string {
	return fmt.Sprintf("gemini error (%s): %s", e.Type, e.Message)
}

func (e *GeminiError) Unwrap() error {
	return e.Err
}

// GeminiErrorType defines the possible error types.
type GeminiErrorType string

const (
	ErrTokenLimit    GeminiErrorType = "token_limit"
	ErrAuth          GeminiErrorType = "auth"
	ErrRateLimit     GeminiErrorType = "rate_limit"
	ErrSafety        GeminiErrorType = "safety"
	ErrEmptyResponse GeminiErrorType = "empty_response"
	ErrEmptyContent  GeminiErrorType = "empty_content"
	ErrInvalidFormat GeminiErrorType = "invalid_format"
	ErrEmptyMessage  GeminiErrorType = "empty_message"
)

func NewGeminiError(errType GeminiErrorType, message string, err error) *GeminiError {
	return &GeminiError{
		Type:    string(errType),
		Message: message,
		Err:     err,
	}
}

// GetTextFromResponse extracts the text content from a streaming response chunk.
func GetTextFromResponse(resp *genai.GenerateContentResponse) string {
	var textBuilder strings.Builder
	if resp != nil {
		for _, cand := range resp.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					textBuilder.WriteString(part.Text)
				}
			}
		}
	}
	return textBuilder.String()
}

// CountTokensForText counts the number of tokens in a text.
func (c *GenaiClient) CountTokensForText(ctx context.Context, modelName string, text string) (int, error) {
	if c.client == nil {
		if err := c.initClient(); err != nil {
			return 0, fmt.Errorf("failed to initialize client for token counting: %w", err)
		}
	}

	resp, err := c.client.Models.CountTokens(ctx, modelName, genai.Text(text), nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	return int(resp.TotalTokens), nil
}

func (c *GenaiClient) checkTokenLimit(promptTemplate, diff string, modelName string, maxTokens int) error {
	ctx := context.Background()
	finalPrompt := strings.Replace(promptTemplate, "!YAWNDIFFPLACEHOLDER!", diff, 1)

	tokenCount, err := c.CountTokensForText(ctx, modelName, finalPrompt)
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

func (c *GenaiClient) generateStreamWithModel(ctx context.Context, modelName, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error) {
	if err := c.checkTokenLimit(promptTemplate, diff, modelName, maxTokens); err != nil {
		return nil, err
	}

	finalPrompt := strings.Replace(promptTemplate, "!YAWNDIFFPLACEHOLDER!", diff, 1)

	config := &genai.GenerateContentConfig{
		Temperature:     &temperature,
		MaxOutputTokens: int32(maxTokens),
	}

	stream := c.client.Models.GenerateContentStream(ctx, modelName, genai.Text(finalPrompt), config)

	// Create a channel to convert the Go 1.23+ iterator to a function
	respChan := make(chan *genai.GenerateContentResponse)
	errChan := make(chan error)
	done := make(chan bool)

	go func() {
		defer close(respChan)
		defer close(errChan)
		defer close(done)

		for resp, err := range stream {
			if err != nil {
				errChan <- err
				return
			}
			respChan <- resp
		}
		done <- true
	}()

	streamFunc := func() (*genai.GenerateContentResponse, error) {
		select {
		case resp := <-respChan:
			return resp, nil
		case err := <-errChan:
			return nil, err
		case <-done:
			return nil, fmt.Errorf("iterator done")
		}
	}

	return &StreamIterator{stream: streamFunc}, nil
}

// GenerateCommitMessageStream generates a commit message using the Gemini API and streams the response.
func (c *GenaiClient) GenerateCommitMessageStream(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error) {
	iter, err := c.generateStreamWithModel(ctx, PrimaryModel, promptTemplate, diff, maxTokens, temperature)
	if err != nil {
		// Attempt fallback
		iter, fallbackErr := c.generateStreamWithModel(ctx, FallbackModel, promptTemplate, diff, maxTokens, temperature)
		if fallbackErr != nil {
			return nil, err
		}
		return iter, nil
	}
	return iter, nil
}

// MockGeminiClient is a mock implementation of Client.
type MockGeminiClient struct {
	GenerateCommitMessageStreamFunc func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error)
	CountTokensForTextFunc          func(ctx context.Context, modelName string, text string) (int, error)
}

func (m *MockGeminiClient) GenerateCommitMessageStream(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (*StreamIterator, error) {
	if m.GenerateCommitMessageStreamFunc != nil {
		return m.GenerateCommitMessageStreamFunc(ctx, promptTemplate, diff, maxTokens, temperature)
	}
	// This is difficult to mock properly without a real iterator.
	// Returning an error is a safe default for tests.
	return nil, fmt.Errorf("mock GenerateCommitMessageStream not implemented")
}

func (m *MockGeminiClient) CountTokensForText(ctx context.Context, modelName string, text string) (int, error) {
	if m.CountTokensForTextFunc != nil {
		return m.CountTokensForTextFunc(ctx, modelName, text)
	}
	return len(strings.Fields(text)), nil
}
