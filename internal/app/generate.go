package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Mayurifag/yawn/internal/ai"
	"github.com/Mayurifag/yawn/internal/ui"
)

const maxCommitGenRetries = 3

var errRequestTimeout = errors.New("request timeout")

func (a *App) newAIClient() (ai.Client, error) {
	return ai.NewClient(a.Config, func(err error) {
		provider := a.Config.GetFallbackProvider()
		ui.PrintInfo(fmt.Sprintf("Primary provider failed: %v", err))
		ui.PrintInfo(fmt.Sprintf("Trying fallback provider: %s/%s", provider, a.Config.GetProviderConfig(provider).Model))
	})
}

func (a *App) doGenerateStream(ctx context.Context, aiClient ai.Client, systemPrompt, userContent string) (string, error) {
	spinner := ui.StartSpinner("Generating commit message...")
	stream, err := aiClient.GenerateCommitMessageStream(ctx, systemPrompt, userContent)
	ui.StopSpinner(spinner)

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("provider request timed out after %s: %w: %w", a.Config.GetRequestTimeout(), errRequestTimeout, err)
		}
		return "", fmt.Errorf("failed to start commit message generation: %w", err)
	}

	ui.PrintInfo("Generated commit message:")
	message, err := stream.Collect(func(chunk string) { fmt.Print(chunk) })
	fmt.Println()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("provider request timed out after %s: %w: %w", a.Config.GetRequestTimeout(), errRequestTimeout, err)
		}
		return "", fmt.Errorf("error receiving commit message stream: %w", err)
	}
	if message == "" {
		return "", fmt.Errorf("empty commit message received from AI provider")
	}
	return message, nil
}

func (a *App) generateCommitMessageAndStream(ctx context.Context, aiClient ai.Client, systemPrompt, userContent string) (string, error) {
	var lastErr error
	for attempt := range maxCommitGenRetries {
		msg, err := a.doGenerateStream(ctx, aiClient, systemPrompt, userContent)
		if err == nil {
			return msg, nil
		}
		lastErr = err
		isRetryable := ai.IsTransientError(err) || errors.Is(err, errRequestTimeout)
		if !isRetryable || attempt == maxCommitGenRetries-1 || ctx.Err() != nil {
			return "", err
		}
		pause := time.Duration(attempt+1) * time.Second
		ui.PrintInfo(fmt.Sprintf("Retrying in %s... (attempt %d/%d)", pause, attempt+1, maxCommitGenRetries))
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", lastErr
}
