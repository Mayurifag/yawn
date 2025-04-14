package gemini

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Client defines the interface for interacting with the Gemini API.
type Client interface {
	GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error)
	EstimateTokenCount(text string) int
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

	client, err := genai.NewClient(context.Background(), option.WithAPIKey(c.apiKey))
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

// estimateTokenCount estimates the number of tokens in a string.
// This is a rough estimation based on word count and special characters.
func estimateTokenCount(s string) int {
	// Count words (split by whitespace)
	words := strings.Fields(s)
	count := len(words)

	// Add extra tokens for special characters and formatting
	for _, word := range words {
		// Count special characters that might be tokenized separately
		for _, c := range word {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && !unicode.IsSpace(c) {
				count++
			}
		}
	}

	// Add buffer for system prompt and formatting
	return count + 100
}

// cleanCommitMessage cleans and formats the AI-generated commit message.
func cleanCommitMessage(message string) string {
	message = strings.TrimSpace(message)
	message = regexp.MustCompile(`[ \t]+`).ReplaceAllString(message, " ")
	message = strings.ReplaceAll(message, "\r\n", "\n")
	return message
}

func (c *GenaiClient) checkTokenLimit(promptTemplate, diff string, maxTokens int) error {
	promptTokens := estimateTokenCount(promptTemplate)
	diffTokens := estimateTokenCount(diff)
	totalTokens := promptTokens + diffTokens

	if totalTokens > maxTokens {
		return NewGeminiError(
			ErrTokenLimit,
			fmt.Sprintf("estimated token count (%d) exceeds limit (%d). Consider reducing the diff size or increasing max_tokens", totalTokens, maxTokens),
			nil,
		)
	}
	return nil
}

func (c *GenaiClient) handleGenerateContentError(err error) error {
	var blockedErr *genai.BlockedError
	if errors.As(err, &blockedErr) {
		if blockedErr.PromptFeedback != nil && blockedErr.PromptFeedback.BlockReason != genai.BlockReasonUnspecified {
			return NewGeminiError(
				ErrSafety,
				fmt.Sprintf("prompt blocked: %s", blockedErr.PromptFeedback.BlockReason),
				err,
			)
		}

		if blockedErr.Candidate != nil && blockedErr.Candidate.FinishReason == genai.FinishReasonSafety {
			return NewGeminiError(
				ErrSafety,
				"response blocked by safety settings",
				err,
			)
		}

		return NewGeminiError(
			ErrSafety,
			"content blocked for safety reasons",
			err,
		)
	}

	errMsg := err.Error()
	switch {
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
	text, ok := part.(genai.Text)
	if !ok {
		return "", NewGeminiError(
			ErrInvalidFormat,
			"received non-text response from Gemini API",
			nil,
		)
	}

	message := string(text)
	if message == "" {
		return "", NewGeminiError(
			ErrEmptyMessage,
			"received empty message from Gemini API",
			nil,
		)
	}

	return cleanCommitMessage(message), nil
}

// GenerateCommitMessage generates a commit message using the Gemini API.
// It takes the model name, prompt template, diff content, max tokens, and temperature as parameters.
// Returns the generated message and any error encountered.
func (c *GenaiClient) GenerateCommitMessage(ctx context.Context, modelName string, promptTemplate string, diff string, maxTokens int, temperature float32) (string, error) {
	if err := c.checkTokenLimit(promptTemplate, diff, maxTokens); err != nil {
		return "", err
	}

	model := c.client.GenerativeModel(modelName)
	temp := temperature
	model.SetTemperature(temp)

	finalPrompt := strings.Replace(promptTemplate, "{{Diff}}", diff, 1)

	resp, err := model.GenerateContent(ctx, genai.Text(finalPrompt))
	if err != nil {
		return "", c.handleGenerateContentError(err)
	}

	return c.processGenaiResponse(resp)
}

// EstimateTokenCount provides a very rough estimate of token count.
// A common approximation is 1 token ~ 4 characters in English.
// This doesn't account for specific model tokenization rules.
func (c *GenaiClient) EstimateTokenCount(text string) int {
	charCount := utf8.RuneCountInString(text)
	return (charCount / 4) + 5
}

// MockGeminiClient is a mock implementation of Client.
type MockGeminiClient struct {
	GenerateCommitMessageFunc func(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error)
	EstimateTokenCountFunc    func(text string) int
}

func (m *MockGeminiClient) GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int, temperature float32) (string, error) {
	if m.GenerateCommitMessageFunc != nil {
		return m.GenerateCommitMessageFunc(ctx, model, promptTemplate, diff, maxTokens, temperature)
	}
	return "feat: add new feature\n\nImplement the feature based on the diff.", nil
}

func (m *MockGeminiClient) EstimateTokenCount(text string) int {
	if m.EstimateTokenCountFunc != nil {
		return m.EstimateTokenCountFunc(text)
	}
	charCount := utf8.RuneCountInString(text)
	return (charCount / 4) + 5
}
