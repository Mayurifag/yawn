package ai

import (
	"context"
	"time"
)

type timeoutClient struct {
	client  Client
	timeout time.Duration
}

type timeoutStream struct {
	stream Stream
	cancel context.CancelFunc
}

func (c *timeoutClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	stream, err := c.client.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
	if err != nil {
		cancel()
		return nil, err
	}
	return &timeoutStream{stream: stream, cancel: cancel}, nil
}

func (s *timeoutStream) Collect(onChunk func(string)) (string, error) {
	defer s.cancel()
	return s.stream.Collect(onChunk)
}
