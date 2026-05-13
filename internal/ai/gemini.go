package ai

import (
	"context"
	"encoding/json"
	"fmt"
)

type geminiClient struct {
	apiKey string
	model  string
}

type geminiChatRequest struct {
	Model       string              `json:"model"`
	Messages    []geminiChatMessage `json:"messages"`
	Stream      bool                `json:"stream"`
	Temperature float32             `json:"temperature"`
}

type geminiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type geminiChatEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func newGeminiClient(apiKey, model string) *geminiClient {
	return &geminiClient{apiKey: apiKey, model: model}
}

func (c *geminiClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error) {
	return startJSONStream(ctx, geminiChatCompletionsEndpoint, c.apiKey, geminiChatRequest{
		Model: c.model,
		Messages: []geminiChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Stream:      true,
		Temperature: 0,
	}, nil, parseGeminiChatEvent)
}

func parseGeminiChatEvent(data []byte) (string, bool, error) {
	var event geminiChatEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return "", false, err
	}
	if event.Error != nil {
		return "", false, fmt.Errorf("provider stream error: %s", event.Error.Message)
	}
	for _, choice := range event.Choices {
		if choice.Delta.Content != "" {
			return choice.Delta.Content, false, nil
		}
		if choice.FinishReason != nil {
			return "", true, nil
		}
	}
	return "", false, nil
}
