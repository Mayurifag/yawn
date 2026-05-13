package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxErrorBodyBytes = 4096

var jsonStreamHTTPClient = &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}

type streamResult struct {
	text string
	err  error
}

type httpStream struct {
	ch  <-chan streamResult
	ctx context.Context
}

type streamParser func([]byte) (string, bool, error)

func (s *httpStream) Collect(onChunk func(string)) (string, error) {
	var sb strings.Builder
	for result := range s.ch {
		if result.err != nil {
			return "", result.err
		}
		if result.text == "" {
			continue
		}
		onChunk(result.text)
		sb.WriteString(result.text)
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		return "", s.ctx.Err()
	}
	return sb.String(), nil
}

func startJSONStream(ctx context.Context, endpoint, apiKey string, payload any, headers map[string]string, parse streamParser) (Stream, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := jsonStreamHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, statusError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	ch := make(chan streamResult, 16)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()
		scanSSE(ctx, resp.Body, ch, parse)
	}()

	return &httpStream{ch: ch, ctx: ctx}, nil
}

func scanSSE(ctx context.Context, body io.Reader, ch chan<- streamResult, parse streamParser) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return
		}
		text, done, err := parse([]byte(data))
		if err != nil {
			sendStreamResult(ctx, ch, streamResult{err: err})
			return
		}
		if text != "" {
			sendStreamResult(ctx, ch, streamResult{text: text})
		}
		if done {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		sendStreamResult(ctx, ch, streamResult{err: err})
	}
}

func sendStreamResult(ctx context.Context, ch chan<- streamResult, result streamResult) {
	select {
	case ch <- result:
	case <-ctx.Done():
	}
}
