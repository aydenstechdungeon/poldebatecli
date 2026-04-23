package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/poldebatecli/internal/config"
)

type OpenRouterClientImpl struct {
	HTTPClient  *http.Client
	BaseURL     string
	APIKey      string
	RateLimiter *RateLimiter
	RetryPolicy RetryPolicy
	Logger      *slog.Logger
}

type RetryPolicy struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	RetryableCodes []int
}

func (p RetryPolicy) IsRetryable(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		for _, code := range p.RetryableCodes {
			if apiErr.StatusCode == code {
				return true
			}
		}
		return apiErr.IsRetryable()
	}
	return false
}

func NewOpenRouterClient(cfg config.APIConfig) *OpenRouterClientImpl {
	apiKey := os.Getenv(cfg.APIKeyEnvVar)
	if apiKey == "" {
		slog.Warn("API key environment variable is empty", "env_var", cfg.APIKeyEnvVar)
	}
	transport := &http.Transport{
		MaxIdleConns:        20,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	return &OpenRouterClientImpl{
		HTTPClient:  &http.Client{Timeout: cfg.Timeout, Transport: transport},
		BaseURL:     cfg.BaseURL,
		APIKey:      apiKey,
		RateLimiter: NewRateLimiter(cfg.RateLimitRPS),
		RetryPolicy: RetryPolicy{
			MaxRetries:     cfg.MaxRetries,
			BaseDelay:      cfg.RetryBaseDelay,
			MaxDelay:       30 * time.Second,
			RetryableCodes: []int{429, 500, 502, 503, 504},
		},
		Logger: slog.Default(),
	}
}

func (c *OpenRouterClientImpl) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if err := c.RateLimiter.Wait(ctx, req.Model); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.RetryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Cap the bit shift to prevent overflow for large attempt values
			shift := uint(attempt)
			if shift > 30 {
				shift = 30
			}
			delay := c.RetryPolicy.BaseDelay * time.Duration(1<<shift)
			if delay > c.RetryPolicy.MaxDelay {
				delay = c.RetryPolicy.MaxDelay
			}
			jitter := time.Duration(rand.IntN(500)) * time.Millisecond
			delay += jitter

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.DoRequest(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !c.RetryPolicy.IsRetryable(err) {
			break
		}

		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 429 && apiErr.RetryAfter > 0 {
			c.Logger.Warn("rate limited, waiting", "retry_after", apiErr.RetryAfter)
			// When server specifies exact RetryAfter, use it without jitter
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(apiErr.RetryAfter) * time.Second):
			}
			continue
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", c.RetryPolicy.MaxRetries, lastErr)
}

type openRouterRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *OpenRouterClientImpl) DoRequest(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	body, err := json.Marshal(openRouterRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return nil, &APIError{Type: "marshal", Err: err}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, &APIError{Type: "request", Err: err}
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://github.com/poldebatecli")
	httpReq.Header.Set("X-Title", "PolDebateCLI")

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &APIError{Type: "network", Err: err}
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == 429 {
		retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
		return nil, &APIError{Type: "rate_limit", StatusCode: 429, RetryAfter: retryAfter}
	}

	if httpResp.StatusCode >= 500 {
		return nil, &APIError{Type: "server", StatusCode: httpResp.StatusCode}
	}

	if httpResp.StatusCode >= 400 {
		respBody, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, &APIError{Type: "client", StatusCode: httpResp.StatusCode, Body: fmt.Sprintf("(failed to read response body: %v)", readErr)}
		}
		body := string(respBody)
		if isAuthErrorStatus(httpResp.StatusCode) {
			return nil, &APIError{Type: "auth", StatusCode: httpResp.StatusCode, Body: formatAuthErrorBody(body)}
		}
		return nil, &APIError{Type: "client", StatusCode: httpResp.StatusCode, Body: body}
	}

	var orResp openRouterResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&orResp); err != nil {
		return nil, &APIError{Type: "parse", Err: err}
	}

	if len(orResp.Choices) == 0 {
		return nil, &APIError{Type: "empty_response"}
	}

	return &CompletionResponse{
		Content:      orResp.Choices[0].Message.Content,
		Model:        orResp.Model,
		TokensUsed:   orResp.Usage.TotalTokens,
		FinishReason: orResp.Choices[0].FinishReason,
	}, nil
}

type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContextLength int    `json:"context_length"`
}

func (c *OpenRouterClientImpl) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/models", nil)
	if err != nil {
		return nil, &APIError{Type: "request", Err: err}
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, &APIError{Type: "network", Err: err}
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != 200 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("list models failed (%d): %s", httpResp.StatusCode, string(respBody))
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, &APIError{Type: "parse", Err: err}
	}

	return result.Data, nil
}

func parseRetryAfter(header string) int {
	if header == "" {
		return 0
	}
	var seconds int
	if _, err := fmt.Sscanf(header, "%d", &seconds); err == nil {
		return seconds
	}
	if t, err := http.ParseTime(header); err == nil {
		secondsUntil := int(time.Until(t).Seconds())
		if secondsUntil > 0 {
			return secondsUntil
		}
	}
	return 0
}

func isAuthErrorStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func formatAuthErrorBody(body string) string {
	msg := extractAPIErrorMessage(body)
	if msg == "" {
		return "authentication failed: API key missing or invalid"
	}

	lower := strings.ToLower(msg)
	if strings.Contains(lower, "invalid") && strings.Contains(lower, "key") {
		return "invalid API key"
	}
	if strings.Contains(lower, "missing") && strings.Contains(lower, "key") {
		return "missing API key"
	}
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") {
		return "authentication failed: API key missing or invalid"
	}
	return "authentication failed: " + msg
}

func extractAPIErrorMessage(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return trimmed
	}

	if errObj, ok := decoded["error"].(map[string]any); ok {
		if msg, ok := errObj["message"].(string); ok {
			return strings.TrimSpace(msg)
		}
	}
	if msg, ok := decoded["message"].(string); ok {
		return strings.TrimSpace(msg)
	}

	return trimmed
}
