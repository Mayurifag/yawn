package app

import (
	"context"
	"errors"
	"testing"

	"github.com/Mayurifag/yawn/internal/ai"
	"github.com/Mayurifag/yawn/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAIStream struct {
	message string
	err     error
}

func (s fakeAIStream) Collect(onChunk func(string)) (string, error) {
	if s.message != "" {
		onChunk(s.message)
	}
	if s.err != nil {
		return "", s.err
	}
	return s.message, nil
}

type fakeAIClient struct {
	streams []ai.Stream
	calls   int
}

func (c *fakeAIClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (ai.Stream, error) {
	if c.calls >= len(c.streams) {
		return nil, errors.New("unexpected stream request")
	}
	stream := c.streams[c.calls]
	c.calls++
	return stream, nil
}

func TestGenerateCommitMessageRetriesStreamDeadline(t *testing.T) {
	client := &fakeAIClient{streams: []ai.Stream{
		fakeAIStream{err: context.DeadlineExceeded},
		fakeAIStream{message: "fix: retry deadline"},
	}}
	a := &App{Config: config.Config{RequestTimeoutSeconds: 30}}

	message, err := a.generateCommitMessageAndStream(context.Background(), client, "", "")

	require.NoError(t, err)
	assert.Equal(t, "fix: retry deadline", message)
	assert.Equal(t, 2, client.calls)
}
