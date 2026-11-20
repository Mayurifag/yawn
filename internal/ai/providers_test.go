package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mayurifag/yawn/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiHTTPStreamCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		err  error
	}{
		{"done marker", "data: {\"choices\":[{\"delta\":{\"content\":\"fix: complete\"}}]}\n\ndata: [DONE]\n\n", "fix: complete", nil},
		{"final content with stop", "data: {\"choices\":[{\"delta\":{\"content\":\"fix: complete\"},\"finish_reason\":\"stop\"}]}\n\n", "fix: complete", nil},
		{"interrupted output", "data: {\"choices\":[{\"delta\":{\"content\":\"fix: incomplete\"}}]}\n\n", "", io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			stream, err := startJSONStream(context.Background(), server.URL, "test-key", struct{}{}, nil, parseGeminiChatEvent)
			require.NoError(t, err)

			message, err := stream.Collect(nil)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.want, message)
		})
	}
}

func TestParseGeminiChatEventRejectsTruncation(t *testing.T) {
	_, _, err := parseGeminiChatEvent([]byte(`{"choices":[{"delta":{},"finish_reason":"length"}]}`))
	assert.ErrorContains(t, err, "length")
}

func TestParseGeminiChatEvent(t *testing.T) {
	text, done, err := parseGeminiChatEvent([]byte(`{"choices":[{"delta":{"content":"fix"}}]}`))

	assert.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "fix", text)
}

func TestParseGeminiChatEventDone(t *testing.T) {
	text, done, err := parseGeminiChatEvent([]byte(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`))

	assert.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, "", text)
}

func TestParseOpenCodeCLIOutput(t *testing.T) {
	text, err := parseOpenCodeCLIOutput([]byte("{\"type\":\"step_start\"}\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"fix\"}}\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\": config\"}}\n"))

	assert.NoError(t, err)
	assert.Equal(t, "fix: config", text)
}

func TestParseOpenCodeCLIOutputError(t *testing.T) {
	_, err := parseOpenCodeCLIOutput([]byte("{\"type\":\"error\",\"error\":{\"message\":\"failed\"}}\n"))

	assert.ErrorContains(t, err, "failed")
}

func TestOpenCodeCLIArgs(t *testing.T) {
	args := openCodeCLIArgs("openai/gpt-5.3-codex-spark")
	joinedArgs := strings.Join(args, " ")

	assert.Equal(t, []string{
		"run",
		"--standalone",
		"--agent", "yawn",
		"--model", "openai/gpt-5.3-codex-spark#low",
		"--format", "json",
	}, args)
	assert.NotContains(t, joinedArgs, "token")
}

func TestOpenCodeCLIArgsPreserveExplicitVariant(t *testing.T) {
	assert.Contains(t, openCodeCLIArgs("openai/gpt-5.3-codex-spark#high"), "openai/gpt-5.3-codex-spark#high")
}

func TestOpenCodeCLIConfigDisablesTools(t *testing.T) {
	var cfg struct {
		Snapshots bool `json:"snapshots"`
		Agents    map[string]struct {
			Permissions []struct {
				Action   string `json:"action"`
				Resource string `json:"resource"`
				Effect   string `json:"effect"`
			} `json:"permissions"`
		} `json:"agents"`
	}
	assert.NoError(t, json.Unmarshal([]byte(openCodeCLIConfig), &cfg))
	assert.False(t, cfg.Snapshots)
	if assert.Len(t, cfg.Agents["yawn"].Permissions, 1) {
		rule := cfg.Agents["yawn"].Permissions[0]
		assert.Equal(t, "*", rule.Action)
		assert.Equal(t, "*", rule.Resource)
		assert.Equal(t, "deny", rule.Effect)
	}
}

func TestOpenCodeCLIPromptKeepsConfiguredPrompt(t *testing.T) {
	prompt := openCodeCLIPrompt("configured prompt", "diff content")

	assert.Contains(t, prompt, "configured prompt")
	assert.Contains(t, prompt, "diff content")
	assert.Contains(t, prompt, "Do not include reasoning")
}

func TestNewClientDoesNotRequireFallbackAPIKey(t *testing.T) {
	client, err := NewClient(config.Config{
		MainProvider:     config.ProviderOpenCodeCLI,
		FallbackProvider: config.ProviderGemini,
	}, nil)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"context deadline", context.DeadlineExceeded, true},
		{"http 429", statusError{StatusCode: 429}, true},
		{"http 503", statusError{StatusCode: 503}, true},
		{"http 400", statusError{StatusCode: 400}, false},
		{"plain", fmt.Errorf("failed"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTransientError(tt.err))
		})
	}
}
