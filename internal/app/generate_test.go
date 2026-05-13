package app

import (
	"context"
	"errors"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/Mayurifag/yawn/internal/gemini"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

type fakeGeminiStream struct {
	message string
	err     error
}

func (s fakeGeminiStream) Collect(onChunk func(string)) (string, error) {
	if s.message != "" {
		onChunk(s.message)
	}
	if s.err != nil {
		return "", s.err
	}
	return s.message, nil
}

type fakeGeminiClient struct {
	streams []gemini.Stream
	calls   int
}

func (c *fakeGeminiClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (gemini.Stream, error) {
	if c.calls >= len(c.streams) {
		return nil, errors.New("unexpected stream request")
	}
	stream := c.streams[c.calls]
	c.calls++
	return stream, nil
}

func TestGenerateCommitMessageRetriesStreamDeadline(t *testing.T) {
	client := &fakeGeminiClient{streams: []gemini.Stream{
		fakeGeminiStream{err: context.DeadlineExceeded},
		fakeGeminiStream{message: "fix: retry deadline"},
	}}
	a := &App{Config: config.Config{RequestTimeoutSeconds: 30}}

	message, err := a.generateCommitMessageAndStream(context.Background(), client, "", "")

	require.NoError(t, err)
	assert.Equal(t, "fix: retry deadline", message)
	assert.Equal(t, 2, client.calls)
}

func TestGenerateCommitMessageRetriesGemini503StreamError(t *testing.T) {
	client := &fakeGeminiClient{streams: []gemini.Stream{
		fakeGeminiStream{err: genai.APIError{Code: 503}},
		fakeGeminiStream{message: "fix: retry unavailable"},
	}}
	a := &App{Config: config.Config{RequestTimeoutSeconds: 30}}

	message, err := a.generateCommitMessageAndStream(context.Background(), client, "", "")

	require.NoError(t, err)
	assert.Equal(t, "fix: retry unavailable", message)
	assert.Equal(t, 2, client.calls)
}
