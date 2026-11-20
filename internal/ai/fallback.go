package ai

import (
	"context"
	"fmt"
)

type fallbackClient struct {
	primary    Client
	fallback   func() (Client, error)
	onFallback func(error)
}

type fallbackStream struct {
	primary  Stream
	fallback func(error) (Stream, error)
}

func (c *fallbackClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error) {
	primaryStream, err := c.primary.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
	if err != nil {
		return c.startFallback(ctx, systemPrompt, userContent, err)
	}
	return &fallbackStream{
		primary: primaryStream,
		fallback: func(err error) (Stream, error) {
			return c.startFallback(ctx, systemPrompt, userContent, err)
		},
	}, nil
}

func (c *fallbackClient) startFallback(ctx context.Context, systemPrompt, userContent string, primaryErr error) (Stream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.onFallback != nil {
		c.onFallback(primaryErr)
	}
	fallback, err := c.fallback()
	if err == nil {
		var stream Stream
		stream, err = fallback.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
		if err == nil {
			return stream, nil
		}
	}
	return nil, fmt.Errorf("primary provider failed: %v; fallback provider failed: %w", primaryErr, err)
}

func (s *fallbackStream) Collect(onChunk func(string)) (string, error) {
	emitted := false
	message, err := s.primary.Collect(func(chunk string) {
		emitted = true
		onChunk(chunk)
	})
	if err == nil {
		return message, nil
	}
	if emitted {
		return "", err
	}
	stream, fallbackErr := s.fallback(err)
	if fallbackErr != nil {
		return "", fallbackErr
	}
	return stream.Collect(onChunk)
}
