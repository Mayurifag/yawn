package gemini

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

const (
	PrimaryModel  = "gemini-2.5-flash"
	FallbackModel = "gemini-2.5-flash-lite"
)

// Client defines the interface for interacting with the Gemini API.
type Client interface {
	GenerateCommitMessage(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error)
	CountTokensForText(ctx context.Context, modelName string, text string) (int, error)
}

// GenaiClient implements the Client interface using the official Google GenAI SDK.
type GenaiClient struct {
	apiKey string
	client *genai.Client
}

// NewClient creates a new Gemini client.
func NewClient(apiKey string) (*GenaiClient, error) {
	// API key is now required
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	c := &GenaiClient{
		apiKey: apiKey,
	}

	// Initialize the client immediately
	if err := c.initClient(); err != nil {
		return nil, err
	}

	return c, nil
}

// initClient initializes the underlying genai.Client.
// It returns an error if the API key is empty or if client creation fails.
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

// Error implements the error interface for GeminiError.
func (e *GeminiError) Error() string {
	return fmt.Sprintf("gemini error (%s): %s", e.Type, e.Message)
}

// Unwrap implements the errors.Unwrap interface for GeminiError.
func (e *GeminiError) Unwrap() error {
	return e.Err
}

// GeminiErrorType defines the possible error types that can occur when using the Gemini API.
type GeminiErrorType string

// Predefined error types for Gemini API operations
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

// NewGeminiError creates a new GeminiError with the specified type, message, and wrapped error.
func NewGeminiError(errType GeminiErrorType, message string, err error) *GeminiError {
	return &GeminiError{
		Type:    string(errType),
		Message: message,
		Err:     err,
	}
}

// cleanCommitMessage cleans and formats the AI-generated commit message.
func cleanCommitMessage(message string) string {
	message = strings.TrimSpace(message)
	message = regexp.MustCompile(`[ \t]+`).ReplaceAllString(message, " ")
	message = strings.ReplaceAll(message, "\r\n", "\n")
	return message
}

// CountTokensForText counts the number of tokens in a text using the SDK method.
// This provides an accurate token count directly from the model.
func (c *GenaiClient) CountTokensForText(ctx context.Context, modelName string, text string) (int, error) {
	if c.client == nil {
		if err := c.initClient(); err != nil {
			return 0, fmt.Errorf("failed to initialize client for token counting: %w", err)
		}
	}

	resp, err := c.client.Models.CountTokens(ctx, modelName, []*genai.Content{{Parts: []*genai.Part{{Text: text}}}}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	return int(resp.TotalTokens), nil
}

func (c *GenaiClient) checkTokenLimit(promptTemplate, diff string, modelName string, maxTokens int) error {
	// Use the context.Background() since we expect token counting to be fast
	ctx := context.Background()

	// Prepare the text content as we would for the actual request
	finalPrompt := strings.Replace(promptTemplate, "!YAWNDIFFPLACEHOLDER!", diff, 1)

	// Use the CountTokensForText method for accurate count
	tokenCount, err := c.CountTokensForText(ctx, modelName, finalPrompt)
	if err != nil {
		// If we can't count tokens, log the error but don't fail (this is not critical)
		return nil
	}

	if tokenCount > maxTokens {
		return NewGeminiError(
			ErrTokenLimit,
			fmt.Sprintf("token count (%d) exceeds limit (%d). Consider reducing the diff size or increasing max_tokens", tokenCount, maxTokens),
			nil,
		)
	}
	return nil
}

func (c *GenaiClient) handleGenerateContentError(err error) error {
	// For now, we'll handle errors based on error message content
	// The new SDK may have different error types that we'll need to discover
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "safety"), strings.Contains(errMsg, "blocked"):
		return NewGeminiError(ErrSafety, "content blocked for safety reasons", err)

	case strings.Contains(errMsg, "authentication"), strings.Contains(errMsg, "invalid token"),
		strings.Contains(errMsg, "auth"), strings.Contains(errMsg, "credential"):
		return NewGeminiError(ErrAuth, "invalid API key or authentication failed", err)

	case strings.Contains(errMsg, "rate limit"), strings.Contains(errMsg, "quota"):
		return NewGeminiError(ErrRateLimit, "API rate limit exceeded. Please try again later", err)

	default:
		return fmt.Errorf("failed to generate commit message: %w", err)
	}
}

func (c *GenaiClient) processGenaiResponse(resp *genai.GenerateContentResponse) (string, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", NewGeminiError(
			ErrEmptyResponse,
			"received empty response from Gemini API",
			nil,
		)
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", NewGeminiError(
			ErrEmptyContent,
			"received empty content from Gemini API",
			nil,
		)
	}

	part := candidate.Content.Parts[0]
	// Check if this is a text part (not an image, blob, etc.)
	if part.Text == "" && part.InlineData != nil {
		return "", NewGeminiError(
			ErrInvalidFormat,
			"received non-text response from Gemini API",
			nil,
		)
	}

	message := part.Text
	if message == "" {
		return "", NewGeminiError(
			ErrEmptyMessage,
			"received empty message from Gemini API",
			nil,
		)
	}

	return cleanCommitMessage(message), nil
}

// generateWithModel is a helper to generate a commit message with a specific model.
func (c *GenaiClient) generateWithModel(ctx context.Context, modelName string, promptTemplate string, diff string, maxTokens int, temperature float32) (string, error) {
	if err := c.checkTokenLimit(promptTemplate, diff, modelName, maxTokens); err != nil {
		return "", err
	}

	finalPrompt := strings.Replace(promptTemplate, "!YAWNDIFFPLACEHOLDER!", diff, 1)

	resp, err := c.client.Models.GenerateContent(ctx, modelName, []*genai.Content{{
		Parts: []*genai.Part{{Text: finalPrompt}},
	}}, &genai.GenerateContentConfig{
		Temperature: &temperature,
	})
	if err != nil {
		return "", c.handleGenerateContentError(err)
	}

	return c.processGenaiResponse(resp)
}

// GenerateCommitMessage generates a commit message using the Gemini API.
// It tries the primary model first, and falls back to a secondary model on error.
func (c *GenaiClient) GenerateCommitMessage(ctx context.Context, promptTemplate string, diff string, maxTokens int, temperature float32) (string, error) {
	message, err := c.generateWithModel(ctx, PrimaryModel, promptTemplate, diff, maxTokens, temperature)
	if err != nil {
		// Attempt fallback
		message, fallbackErr := c.generateWithModel(ctx, FallbackModel, promptTemplate, diff, maxTokens, temperature)
		if fallbackErr != nil {
			// Return the original error because it's probably more relevant
			return "", err
		}
		return message, nil
	}

	return message, nil
}

// MockGeminiClient is a mock implementation of Client.
type MockGeminiClient struct {
	GenerateCommitMessageFunc func(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error)
	CountTokensForTextFunc    func(ctx context.Context, modelName string, text string) (int, error)
}

func (m *MockGeminiClient) GenerateCommitMessage(ctx context.Context, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
	if m.GenerateCommitMessageFunc != nil {
		return m.GenerateCommitMessageFunc(ctx, promptTemplate, diff, maxTokens, temperature)
	}
	return "feat: add new feature\n\nImplement the feature based on the diff.", nil
}

func (m *MockGeminiClient) CountTokensForText(ctx context.Context, modelName string, text string) (int, error) {
	if m.CountTokensForTextFunc != nil {
		return m.CountTokensForTextFunc(ctx, modelName, text)
	}
	// Default implementation returns a conservative estimate
	return len(strings.Fields(text)), nil
}
