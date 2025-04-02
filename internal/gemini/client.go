package gemini

import (
	"context"
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
	GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error)
	EstimateTokenCount(text string) int
	SetAPIKey(apiKey string) error
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

	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GenaiClient{
		apiKey: apiKey,
		client: client,
	}, nil
}

// SetAPIKey updates the API key used by the client.
func (c *GenaiClient) SetAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	c.apiKey = apiKey
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
// It takes the model name, prompt template, diff content, and max tokens as parameters.
// Returns the generated message and any error encountered.
func (c *GenaiClient) GenerateCommitMessage(ctx context.Context, modelName string, promptTemplate string, diff string, maxTokens int) (string, error) {
	// Estimate total token count
	promptTokens := estimateTokenCount(promptTemplate)
	diffTokens := estimateTokenCount(diff)
	totalTokens := promptTokens + diffTokens

	// Check if we're likely to exceed the token limit
	if totalTokens > maxTokens {
		return "", &GeminiError{
			Type:    "token_limit",
			Message: fmt.Sprintf("estimated token count (%d) exceeds limit (%d). Consider reducing the diff size or increasing max_tokens", totalTokens, maxTokens),
		}
	}

	// Create the model
	model := c.client.GenerativeModel(modelName)

	// Build the final prompt
	finalPrompt := strings.Replace(promptTemplate, "{{Diff}}", diff, 1)

	// Generate content
	resp, err := model.GenerateContent(ctx, genai.Text(finalPrompt))
	if err != nil {
		// Check for specific error conditions
		if strings.Contains(err.Error(), "authentication") {
			return "", &GeminiError{
				Type:    "auth",
				Message: "invalid API key or authentication failed",
				Err:     err,
			}
		}
		if strings.Contains(err.Error(), "rate limit") {
			return "", &GeminiError{
				Type:    "rate_limit",
				Message: "API rate limit exceeded. Please try again later",
				Err:     err,
			}
		}
		if strings.Contains(err.Error(), "safety") {
			return "", &GeminiError{
				Type:    "safety",
				Message: "content blocked by safety settings",
				Err:     err,
			}
		}
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	// Process the response
	if resp == nil || len(resp.Candidates) == 0 {
		return "", &GeminiError{
			Type:    "empty_response",
			Message: "received empty response from Gemini API",
		}
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", &GeminiError{
			Type:    "empty_content",
			Message: "response content is empty",
		}
	}

	// Extract and clean the message
	part := candidate.Content.Parts[0]
	message, ok := part.(genai.Text)
	if !ok {
		return "", &GeminiError{
			Type:    "invalid_format",
			Message: "unexpected response format: expected text content",
		}
	}
	if message == "" {
		return "", &GeminiError{
			Type:    "empty_message",
			Message: "generated message is empty",
		}
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
	GenerateCommitMessageFunc func(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error)
	EstimateTokenCountFunc    func(text string) int
	SetAPIKeyFunc             func(apiKey string) error
}

func (m *MockGeminiClient) GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error) {
	if m.GenerateCommitMessageFunc != nil {
		return m.GenerateCommitMessageFunc(ctx, model, promptTemplate, diff, maxTokens)
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

func (m *MockGeminiClient) SetAPIKey(apiKey string) error {
	if m.SetAPIKeyFunc != nil {
		return m.SetAPIKeyFunc(apiKey)
	}
	return nil
}
