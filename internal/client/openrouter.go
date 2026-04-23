package client

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

type OpenRouterClient interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type CompletionResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	TokensUsed   int    `json:"tokens_used"`
	FinishReason string `json:"finish_reason"`
}

type StreamChunk struct {
	Content    string `json:"content"`
	Done       bool   `json:"done"`
	Model      string `json:"model,omitempty"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	Err        error  `json:"-"`
}

type APIError struct {
	Type       string `json:"type"`
	StatusCode int    `json:"status_code,omitempty"`
	RetryAfter int    `json:"retry_after,omitempty"`
	Body       string `json:"body,omitempty"`
	Err        error  `json:"-"`
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Type + ": " + sanitizePath(e.Err.Error())
	}
	if e.Body != "" {
		return e.Type + ": " + sanitizePath(e.Body)
	}
	return e.Type
}

var pathLikeRegex = regexp.MustCompile(`(?:/[\w.-]+){2,}`)

func sanitizePath(msg string) string {
	msg = pathLikeRegex.ReplaceAllStringFunc(msg, func(match string) string {
		if strings.Contains(match, "/") && (strings.Contains(match, ".go") ||
			strings.Contains(match, ".yaml") || strings.Contains(match, ".json") ||
			strings.Contains(match, ".toml") || strings.Contains(match, "/home/") ||
			strings.Contains(match, "/tmp/") || strings.Contains(match, "/etc/") ||
			strings.Contains(match, "/usr/") || strings.Contains(match, "/var/")) {
			return filepath.Base(match)
		}
		return match
	})
	return msg
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func (e *APIError) IsRetryable() bool {
	switch e.Type {
	case "rate_limit", "server", "network":
		return true
	default:
		return false
	}
}
