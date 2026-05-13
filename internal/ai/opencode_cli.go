package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const openCodeCLILowReasoningVariant = "low"

type openCodeCLIClient struct {
	model string
}

type openCodeCLIStream struct {
	ch  <-chan streamResult
	ctx context.Context
}

type openCodeCLIEvent struct {
	Type  string `json:"type"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Part struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"part"`
}

func newOpenCodeCLIClient(model string) *openCodeCLIClient {
	return &openCodeCLIClient{model: model}
}

func (c *openCodeCLIClient) GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error) {
	cmd := exec.CommandContext(ctx, "opencode", openCodeCLIArgs(c.model)...)
	prompt := openCodeCLIPrompt(systemPrompt, userContent)
	cmd.Stdin = strings.NewReader(prompt)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to capture opencode output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, openCodeCLICommandError(err, stderr.String())
	}

	ch := make(chan streamResult, 16)
	go func() {
		defer close(ch)
		scanErr := scanOpenCodeCLIOutput(ctx, stdout, ch)
		waitErr := cmd.Wait()
		if ctx.Err() != nil {
			return
		}
		if scanErr != nil {
			sendStreamResult(ctx, ch, streamResult{err: scanErr})
			return
		}
		if waitErr != nil {
			sendStreamResult(ctx, ch, streamResult{err: openCodeCLICommandError(waitErr, stderr.String())})
		}
	}()

	return &openCodeCLIStream{ch: ch, ctx: ctx}, nil
}

func (s *openCodeCLIStream) Collect(onChunk func(string)) (string, error) {
	var sb strings.Builder
	for result := range s.ch {
		if result.err != nil {
			return "", result.err
		}
		if result.text == "" {
			continue
		}
		if onChunk != nil {
			onChunk(result.text)
		}
		sb.WriteString(result.text)
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		return "", s.ctx.Err()
	}
	return strings.TrimSpace(sb.String()), nil
}

func openCodeCLIArgs(model string) []string {
	return []string{
		"run",
		"--pure",
		"--model", model,
		"--variant", openCodeCLILowReasoningVariant,
		"--no-thinking",
		"--format", "json",
	}
}

func openCodeCLIPrompt(systemPrompt, userContent string) string {
	parts := []string{
		strings.TrimSpace(systemPrompt),
		"Do not use tools. Do not include reasoning, analysis, or commentary. Return only the requested commit message.",
		strings.TrimSpace(userContent),
	}
	return strings.Join(parts, "\n\n")
}

func parseOpenCodeCLIOutput(output []byte) (string, error) {
	var sb strings.Builder
	for _, line := range bytes.Split(output, []byte("\n")) {
		text, err := parseOpenCodeCLIEvent(line)
		if err != nil {
			return "", err
		}
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String()), nil
}

func scanOpenCodeCLIOutput(ctx context.Context, output io.Reader, ch chan<- streamResult) error {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var firstErr error
	for scanner.Scan() {
		text, err := parseOpenCodeCLIEvent(scanner.Bytes())
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if firstErr != nil || text == "" {
			continue
		}
		sendStreamResult(ctx, ch, streamResult{text: text})
	}
	if err := scanner.Err(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func parseOpenCodeCLIEvent(line []byte) (string, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return "", nil
	}

	var event openCodeCLIEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return "", fmt.Errorf("failed to parse opencode JSON output: %w", err)
	}
	if event.Error != nil {
		return "", fmt.Errorf("opencode error: %s", event.Error.Message)
	}
	if event.Part.Error != nil {
		return "", fmt.Errorf("opencode error: %s", event.Part.Error.Message)
	}
	if event.Type == "text" && event.Part.Type == "text" {
		return event.Part.Text, nil
	}
	return "", nil
}

func openCodeCLICommandError(err error, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		return err
	}
	return fmt.Errorf("opencode run failed: %w: %s", err, detail)
}
