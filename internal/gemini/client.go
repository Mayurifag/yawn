package gemini

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Client defines the interface for interacting with the Gemini API.
type Client interface {
	GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error)
	EstimateTokenCount(text string) int
	SetAPIKey(apiKey string)
}

// GenaiClient implements the Client interface using the official Google GenAI SDK.
type GenaiClient struct {
	apiKey string
}

// NewClient creates a new Gemini client.
func NewClient(apiKey string) *GenaiClient {
	return &GenaiClient{apiKey: apiKey}
}

// SetAPIKey updates the API key used by the client.
func (c *GenaiClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// GenerateCommitMessage sends the diff and prompt to the Gemini API and returns the generated message.
func (c *GenaiClient) GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("gemini API key is not set")
	}

	// Construct the final prompt
	prompt := strings.Replace(promptTemplate, "{{Diff}}", diff, 1)

	// Estimate combined token count (prompt + diff + potential output)
	// This is a rough estimate; actual tokenization depends on the model.
	// We focus on the input size primarily.
	estimatedInputTokens := c.EstimateTokenCount(prompt)
	// Leave some room for the output, maybe 10-20% of maxTokens? Or a fixed amount?
	// Let's reserve ~200 tokens for the output message for safety.
	maxInputTokens := maxTokens - 200
	if maxInputTokens <= 0 {
		maxInputTokens = maxTokens / 2 // Fallback if maxTokens is very small
	}

	if estimatedInputTokens > maxInputTokens {
		return "", fmt.Errorf("git diff is too large (%d estimated input tokens, limit %d for input). Please reduce changes or increase max_tokens in config", estimatedInputTokens, maxInputTokens)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(c.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	gemini := client.GenerativeModel(model)
	// Configure the model - potentially set max output tokens if API supports it well.
	// gemini.SetMaxOutputTokens(int32(maxTokens - estimatedInputTokens)) // Example if supported
	// Setting total max tokens might be handled by the underlying API based on model limits.
	// Let's rely on the input check for now.

	resp, err := gemini.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		// TODO: Check for specific error types (rate limit, API key invalid, etc.) if the SDK provides them.
		return "", fmt.Errorf("failed to generate content from gemini: %w", err)
	}

	// Extract the text content from the response
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		// Check finish reason if available
		finishReason := "unknown"
		if len(resp.Candidates) > 0 && resp.Candidates[0].FinishReason != genai.FinishReasonUnspecified {
			finishReason = string(resp.Candidates[0].FinishReason)
		}
		// Check safety ratings if available
		safetyInfo := ""
		if len(resp.Candidates) > 0 && len(resp.Candidates[0].SafetyRatings) > 0 {
			var ratings []string
			for _, sr := range resp.Candidates[0].SafetyRatings {
				ratings = append(ratings, fmt.Sprintf("%s: %s", sr.Category, sr.Probability))
			}
			safetyInfo = fmt.Sprintf(" (Safety Ratings: %s)", strings.Join(ratings, ", "))
		}

		return "", fmt.Errorf("received empty or invalid response from gemini. Finish Reason: %s%s", finishReason, safetyInfo)
	}

	// Assuming the first part of the first candidate is the text response
	part := resp.Candidates[0].Content.Parts[0]
	message, ok := part.(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response format from gemini: expected text, got %T", part)
	}

	// Clean up the message (remove potential markdown backticks, leading/trailing whitespace)
	cleanedMessage := strings.TrimSpace(string(message))
	cleanedMessage = strings.TrimPrefix(cleanedMessage, "```")
	cleanedMessage = strings.TrimSuffix(cleanedMessage, "```")
	cleanedMessage = strings.TrimPrefix(cleanedMessage, "commit") // Remove potential leading "commit" if model adds it
	cleanedMessage = strings.TrimSpace(cleanedMessage)

	return cleanedMessage, nil
}

// EstimateTokenCount provides a very rough estimate of token count.
// A common approximation is 1 token ~ 4 characters in English.
// This doesn't account for specific model tokenization rules.
func (c *GenaiClient) EstimateTokenCount(text string) int {
	// Use rune count for better UTF-8 handling than len()
	charCount := utf8.RuneCountInString(text)
	// Add a small buffer?
	return (charCount / 4) + 5 // Very rough estimate
}

// --- Mock Client for Testing ---

// MockGeminiClient is a mock implementation of Client.
type MockGeminiClient struct {
	GenerateCommitMessageFunc func(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error)
	EstimateTokenCountFunc    func(text string) int
	SetAPIKeyFunc             func(apiKey string)
}

func (m *MockGeminiClient) GenerateCommitMessage(ctx context.Context, model, promptTemplate, diff string, maxTokens int) (string, error) {
	if m.GenerateCommitMessageFunc != nil {
		return m.GenerateCommitMessageFunc(ctx, model, promptTemplate, diff, maxTokens)
	}
	// Default mock response
	return "feat: add new feature\n\nImplement the feature based on the diff.", nil
}

func (m *MockGeminiClient) EstimateTokenCount(text string) int {
	if m.EstimateTokenCountFunc != nil {
		return m.EstimateTokenCountFunc(text)
	}
	// Default simple estimation for mock
	charCount := utf8.RuneCountInString(text)
	return (charCount / 4) + 5
}

func (m *MockGeminiClient) SetAPIKey(apiKey string) {
	if m.SetAPIKeyFunc != nil {
		m.SetAPIKeyFunc(apiKey)
	}
	// Default mock implementation does nothing
}
