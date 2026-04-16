package gemini

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

const FallbackModel = "gemini-flash-lite-latest"

type streamResult struct {
	resp *genai.GenerateContentResponse
	err  error
}

type StreamIterator struct {
	ch      <-chan streamResult
	pending *streamResult
}

func (s *StreamIterator) Next() (*genai.GenerateContentResponse, error) {
	if s.pending != nil {
		r := s.pending
		s.pending = nil
		if r.err != nil {
			return nil, r.err
		}
		return r.resp, nil
	}
	result, ok := <-s.ch
	if !ok {
		return nil, iterator.Done
	}
	if result.err != nil {
		return nil, result.err
	}
	return result.resp, nil
}

func (s *StreamIterator) Collect(onChunk func(string)) (string, error) {
	var sb strings.Builder
	for {
		resp, err := s.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}
		chunk := getTextFromResponse(resp)
		onChunk(chunk)
		sb.WriteString(chunk)
	}
	return sb.String(), nil
}

type Client interface {
	GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (*StreamIterator, error)
}

type GenaiClient struct {
	client *genai.Client
	model  string
}

func NewClient(apiKey, model string) (*GenaiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GenaiClient{client: client, model: model}, nil
}

func getTextFromResponse(resp *genai.GenerateContentResponse) string {
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

func (c *GenaiClient) generateStreamWithModel(ctx context.Context, modelName, systemPrompt, userContent string) (*StreamIterator, error) {
	temperature := float32(0)
	cfg := &genai.GenerateContentConfig{
		Temperature:    &temperature,
		ThinkingConfig: &genai.ThinkingConfig{ThinkingBudget: genai.Ptr[int32](0)},
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
	}

	stream := c.client.Models.GenerateContentStream(ctx, modelName, genai.Text(userContent), cfg)

	ch := make(chan streamResult, 1)
	go func() {
		defer close(ch)
		for resp, err := range stream {
			if err != nil {
				select {
				case ch <- streamResult{err: err}:
				case <-ctx.Done():
				}
				return
			}
			select {
			case ch <- streamResult{resp: resp}:
			case <-ctx.Done():
				return
			}
		}
	}()

	select {
	case first, ok := <-ch:
		if !ok {
			return &StreamIterator{ch: ch}, nil
		}
		if first.err != nil {
			return nil, first.err
		}
		return &StreamIterator{ch: ch, pending: &first}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *GenaiClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (*StreamIterator, error) {
	iter, err := c.generateStreamWithModel(ctx, c.model, systemPrompt, userContent)
	if err != nil {
		iter, err = c.generateStreamWithModel(ctx, FallbackModel, systemPrompt, userContent)
		if err != nil {
			return nil, err
		}
	}
	return iter, nil
}
