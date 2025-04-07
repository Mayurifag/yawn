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
// It removes common prefixes, backticks, and extra whitespace while preserving newlines.
func cleanCommitMessage(message string) string {
	// Remove backticks and their content
	message = regexp.MustCompile("`[^`]*`").ReplaceAllString(message, "")

	// Remove common prefixes
	prefixes := []string{
		"commit:",
		"commit message:",
		"commit message is:",
		"here's the commit message:",
		"the commit message should be:",
		"the commit message is:",
		"commit message:",
		"commit:",
		"message:",
		"text:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(message), prefix) {
			message = message[len(prefix):]
		}
	}

	// Trim space from start and end
	message = strings.TrimSpace(message)

	// Normalize line breaks (convert \r\n to \n)
	message = strings.ReplaceAll(message, "\r\n", "\n")

	// Clean up excessive whitespace while preserving newlines
	// Replace multiple spaces with a single space
	message = regexp.MustCompile(`[ \t]+`).ReplaceAllString(message, " ")

	// Replace multiple newlines with double newlines (preserve paragraph structure)
	message = regexp.MustCompile(`\n{3,}`).ReplaceAllString(message, "\n\n")

	return message
}

// GenerateCommitMessage generates a commit message using the Gemini API.
// It takes the model name, prompt template, diff content, max tokens, and temperature as parameters.
// Returns the generated message and any error encountered.
func (c *GenaiClient) GenerateCommitMessage(ctx context.Context, modelName string, promptTemplate string, diff string, maxTokens int, temperature float32) (string, error) {
	// Estimate total token count
	promptTokens := estimateTokenCount(promptTemplate)
	diffTokens := estimateTokenCount(diff)
	totalTokens := promptTokens + diffTokens

	// Check if we're likely to exceed the token limit
	if totalTokens > maxTokens {
		return "", NewGeminiError(
			ErrTokenLimit,
			fmt.Sprintf("estimated token count (%d) exceeds limit (%d). Consider reducing the diff size or increasing max_tokens", totalTokens, maxTokens),
			nil,
		)
	}

	// Create the model
	model := c.client.GenerativeModel(modelName)

	// Set temperature for generation
	temp := temperature // Create a copy for the pointer
	model.SetTemperature(temp)

	// Build the final prompt
	finalPrompt := strings.Replace(promptTemplate, "{{Diff}}", diff, 1)

	// Generate content
	resp, err := model.GenerateContent(ctx, genai.Text(finalPrompt))
	if err != nil {
		// Check if this is a BlockedError from the genai SDK
		var blockedErr *genai.BlockedError
		if errors.As(err, &blockedErr) {
			// Check if we have prompt feedback
			if blockedErr.PromptFeedback != nil && blockedErr.PromptFeedback.BlockReason != genai.BlockReasonUnspecified {
				return "", NewGeminiError(
					ErrSafety,
					fmt.Sprintf("prompt blocked: %s", blockedErr.PromptFeedback.BlockReason),
					err,
				)
			}

			// Check if we have candidate feedback (response blocked)
			if blockedErr.Candidate != nil && blockedErr.Candidate.FinishReason == genai.FinishReasonSafety {
				return "", NewGeminiError(
					ErrSafety,
					"response blocked by safety settings",
					err,
				)
			}

			// Fallback for other blocked errors
			return "", NewGeminiError(
				ErrSafety,
				"content blocked for safety reasons",
				err,
			)
		}

		// Fallback to string-based error detection for other error types
		// that may not be explicitly modeled in the SDK
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "authentication"), strings.Contains(errMsg, "invalid token"),
			strings.Contains(errMsg, "auth"), strings.Contains(errMsg, "credential"):
			return "", NewGeminiError(ErrAuth, "invalid API key or authentication failed", err)

		case strings.Contains(errMsg, "rate limit"), strings.Contains(errMsg, "quota"):
			return "", NewGeminiError(ErrRateLimit, "API rate limit exceeded. Please try again later", err)

		default:
			// Generic error with proper wrapping
			return "", fmt.Errorf("failed to generate commit message: %w", err)
		}
	}

	// Process the response
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
			"response content is empty",
			nil,
		)
	}

	// Extract and clean the message
	part := candidate.Content.Parts[0]
	message, ok := part.(genai.Text)
	if !ok {
		return "", NewGeminiError(
			ErrInvalidFormat,
			"unexpected response format: expected text content",
			nil,
		)
	}
	if message == "" {
		return "", NewGeminiError(
			ErrEmptyMessage,
			"generated message is empty",
			nil,
		)
	}

	return cleanCommitMessage(string(message)), nil
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
