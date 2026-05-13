package ai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/Mayurifag/yawn/internal/config"
)

type Stream interface {
	Collect(onChunk func(string)) (string, error)
}

type Client interface {
	GenerateCommitMessageStream(ctx context.Context, systemPrompt, userContent string) (Stream, error)
}

const (
	geminiChatCompletionsEndpoint = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
)

type statusError struct {
	StatusCode int
	Body       string
}

func (e statusError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("provider returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("provider returned HTTP %d: %s", e.StatusCode, body)
}

func NewClient(cfg config.Config) (Client, error) {
	mainProvider := cfg.GetMainProvider()
	mainClient, err := newProviderClient(mainProvider, cfg.GetProviderConfig(mainProvider))
	if err != nil {
		return nil, err
	}

	fallbackProvider := cfg.GetFallbackProvider()
	if fallbackProvider == "" {
		return mainClient, nil
	}
	fallbackCfg := cfg.GetProviderConfig(fallbackProvider)
	return &fallbackClient{
		primary: mainClient,
		fallback: func() (Client, error) {
			return newProviderClient(fallbackProvider, fallbackCfg)
		},
	}, nil
}

func newProviderClient(provider string, providerCfg config.ProviderConfig) (Client, error) {
	if config.ProviderRequiresAPIKey(provider) && providerCfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required for provider %q", provider)
	}
	switch provider {
	case config.ProviderGemini:
		return newGeminiClient(providerCfg.APIKey, providerCfg.Model), nil
	case config.ProviderOpenCodeCLI:
		return newOpenCodeCLIClient(providerCfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var statusErr statusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == 408 || statusErr.StatusCode == 429 || statusErr.StatusCode == 500 ||
			statusErr.StatusCode == 502 || statusErr.StatusCode == 503 || statusErr.StatusCode == 504
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
