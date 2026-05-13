package ai

import "context"

type fallbackClient struct {
	primary  Client
	fallback func() (Client, error)
}

type fallbackStream struct {
	primary  Stream
	fallback func() (Stream, error)
}

func (c *fallbackClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error) {
	primaryStream, err := c.primary.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
	if err != nil {
		fallback, fallbackErr := c.fallback()
		if fallbackErr != nil {
			return nil, err
		}
		return fallback.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
	}
	return &fallbackStream{
		primary: primaryStream,
		fallback: func() (Stream, error) {
			fallback, err := c.fallback()
			if err != nil {
				return nil, err
			}
			return fallback.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
		},
	}, nil
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
	stream, fallbackErr := s.fallback()
	if fallbackErr != nil {
		return "", err
	}
	return stream.Collect(onChunk)
}
