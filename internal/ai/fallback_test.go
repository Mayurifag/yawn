package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	stream Stream
	err    error
}

func (c testClient) GenerateCommitMessageStream(context.Context, string, string) (Stream, error) {
	return c.stream, c.err
}

type testStream struct {
	text string
	err  error
}

type contextClient struct{}

func (contextClient) GenerateCommitMessageStream(ctx context.Context, _, _ string) (Stream, error) {
	return contextStream{ctx: ctx}, nil
}

type contextStream struct {
	ctx context.Context
}

func (s contextStream) Collect(func(string)) (string, error) {
	<-s.ctx.Done()
	return "", s.ctx.Err()
}

func (s testStream) Collect(onChunk func(string)) (string, error) {
	if s.text != "" {
		onChunk(s.text)
	}
	return s.text, s.err
}

func TestFallbackReportsProviderErrors(t *testing.T) {
	primaryErr := errors.New("invalid OpenCode output")
	fallbackErr := statusError{StatusCode: 429, Body: "quota exceeded"}
	var notified error
	client := &fallbackClient{
		primary: testClient{stream: testStream{err: primaryErr}},
		fallback: func() (Client, error) {
			return testClient{err: fallbackErr}, nil
		},
		onFallback: func(err error) { notified = err },
	}

	stream, err := client.GenerateCommitMessageStream(context.Background(), "", "")
	require.NoError(t, err)
	_, err = stream.Collect(func(string) {})

	require.Error(t, err)
	assert.ErrorContains(t, err, primaryErr.Error())
	assert.ErrorContains(t, err, "fallback provider failed: provider returned HTTP 429: quota exceeded")
	assert.True(t, IsTransientError(err))
	assert.ErrorIs(t, notified, primaryErr)
}

func TestFallbackReportsInitializationFailure(t *testing.T) {
	primaryErr := errors.New("primary failed to start")
	fallbackErr := errors.New("fallback API key missing")
	client := &fallbackClient{
		primary:  testClient{err: primaryErr},
		fallback: func() (Client, error) { return nil, fallbackErr },
	}

	_, err := client.GenerateCommitMessageStream(context.Background(), "", "")

	require.ErrorContains(t, err, primaryErr.Error())
	assert.ErrorIs(t, err, fallbackErr)
}

func TestFallbackOnStreamFailure(t *testing.T) {
	primaryErr := errors.New("invalid OpenCode output")
	var notified error
	client := &fallbackClient{
		primary: testClient{stream: testStream{err: primaryErr}},
		fallback: func() (Client, error) {
			return testClient{stream: testStream{text: "fix: fallback"}}, nil
		},
		onFallback: func(err error) { notified = err },
	}

	stream, err := client.GenerateCommitMessageStream(context.Background(), "", "")
	require.NoError(t, err)
	message, err := stream.Collect(func(string) {})

	require.NoError(t, err)
	assert.Equal(t, "fix: fallback", message)
	assert.ErrorIs(t, notified, primaryErr)
}

func TestFallbackGetsOwnTimeout(t *testing.T) {
	var notified error
	client := &fallbackClient{
		primary: &timeoutClient{client: contextClient{}, timeout: 20 * time.Millisecond},
		fallback: func() (Client, error) {
			return &timeoutClient{client: testClient{stream: testStream{text: "fix: fallback"}}, timeout: time.Second}, nil
		},
		onFallback: func(err error) { notified = err },
	}

	stream, err := client.GenerateCommitMessageStream(context.Background(), "", "")
	require.NoError(t, err)
	message, err := stream.Collect(func(string) {})

	require.NoError(t, err)
	assert.Equal(t, "fix: fallback", message)
	assert.ErrorIs(t, notified, context.DeadlineExceeded)
}

func TestFallbackErrorDeterminesRetry(t *testing.T) {
	client := &fallbackClient{
		primary: testClient{stream: testStream{err: context.DeadlineExceeded}},
		fallback: func() (Client, error) {
			return testClient{err: statusError{StatusCode: 400}}, nil
		},
	}

	stream, err := client.GenerateCommitMessageStream(context.Background(), "", "")
	require.NoError(t, err)
	_, err = stream.Collect(func(string) {})

	require.ErrorContains(t, err, "context deadline exceeded")
	assert.ErrorContains(t, err, "fallback provider failed: provider returned HTTP 400")
	assert.False(t, IsTransientError(err))
}

func TestCancelledRequestDoesNotStartFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fallbackClient{
		primary: testClient{err: context.Canceled},
		fallback: func() (Client, error) {
			t.Fatal("fallback must not start after cancellation")
			return nil, nil
		},
	}

	_, err := client.GenerateCommitMessageStream(ctx, "", "")

	assert.ErrorIs(t, err, context.Canceled)
}

func TestNoFallbackAfterPartialOutput(t *testing.T) {
	primaryErr := errors.New("stream interrupted")
	client := &fallbackClient{
		primary: testClient{stream: testStream{text: "partial", err: primaryErr}},
		fallback: func() (Client, error) {
			t.Fatal("fallback must not append to partial output")
			return nil, nil
		},
	}

	stream, err := client.GenerateCommitMessageStream(context.Background(), "", "")
	require.NoError(t, err)
	_, err = stream.Collect(func(string) {})

	assert.ErrorIs(t, err, primaryErr)
}
