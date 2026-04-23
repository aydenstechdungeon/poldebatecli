package failure

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
)

type DegradedEntry struct {
	AgentID       string           `json:"agent_id"`
	Round         config.RoundType `json:"round"`
	OriginalModel string           `json:"original_model"`
	UsedModel     string           `json:"used_model"`
	Reason        string           `json:"reason"`
	RetryCount    int              `json:"retry_count"`
}

type FailureHandler interface {
	Handle(ctx context.Context, err error, ag agent.Agent, prompt string, opts agent.GenerateOpts) (*agent.AgentResponse, error)
	DegradedEntries() []DegradedEntry
	SetRound(rt config.RoundType)
	Reset()
}

type failureHandler struct {
	client       *client.OpenRouterClientImpl
	fallbacks    map[string]string
	degraded     []DegradedEntry
	maxRetries   int
	currentRound config.RoundType
	cfg          *config.Config
	mu           sync.Mutex
}

func NewHandler(c *client.OpenRouterClientImpl, fallbacks map[string]string, cfg *config.Config) FailureHandler {
	return &failureHandler{
		client:     c,
		fallbacks:  fallbacks,
		maxRetries: 2,
		cfg:        cfg,
	}
}

func (h *failureHandler) SetRound(rt config.RoundType) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentRound = rt
}

func (h *failureHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.degraded = nil
	h.currentRound = ""
}

func (h *failureHandler) Handle(ctx context.Context, err error, ag agent.Agent, prompt string, opts agent.GenerateOpts) (*agent.AgentResponse, error) {
	originalPrompt := prompt
	var softened string
	retryCount := 0

	if isRefusal(err) {
		softened = softenPrompt(prompt)
		for attempt := 0; attempt < h.maxRetries; attempt++ {
			resp, retryErr := ag.Generate(ctx, softened, opts)
			retryCount++
			if retryErr == nil && isValidResponse(resp) {
				return resp, nil
			}
		}
	}

	if isEmptyResponse(err) {
		augmented := originalPrompt + "\n\nYou must respond with substantive content."
		for attempt := 0; attempt < h.maxRetries; attempt++ {
			resp, retryErr := ag.Generate(ctx, augmented, opts)
			retryCount++
			if retryErr == nil && isValidResponse(resp) {
				return resp, nil
			}
		}
	}

	if fallback, ok := client.FindFallback(ag.Model(), h.fallbacks); ok {
		slog.Warn("substituting fallback model", "agent", ag.ID(), "original", ag.Model(), "fallback", fallback)
		resp, fallbackErr := h.callWithFallbackModel(ctx, ag, fallback, originalPrompt, opts)
		if fallbackErr == nil {
			h.mu.Lock()
			h.degraded = append(h.degraded, DegradedEntry{
				AgentID:       ag.ID(),
				Round:         h.currentRound,
				OriginalModel: ag.Model(),
				UsedModel:     fallback,
				Reason:        err.Error(),
				RetryCount:    retryCount,
			})
			h.mu.Unlock()
			resp.Degraded = true
			return resp, nil
		}
		slog.Warn("fallback model also failed", "agent", ag.ID(), "fallback", fallback, "error", fallbackErr)
	}

	return nil, fmt.Errorf("all failure strategies exhausted for agent %s: %w", ag.ID(), err)
}

func (h *failureHandler) DegradedEntries() []DegradedEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]DegradedEntry(nil), h.degraded...)
}

func (h *failureHandler) callWithFallbackModel(ctx context.Context, ag agent.Agent, fallbackModel string, prompt string, opts agent.GenerateOpts) (*agent.AgentResponse, error) {
	temp := opts.Temperature
	if temp == 0 {
		if h.cfg != nil {
			if t, ok := h.cfg.Models.Temperatures[ag.Role()]; ok {
				temp = t
			}
		}
		if temp == 0 {
			temp = 0.7
		}
	}
	topP := opts.TopP
	if topP == 0 {
		if h.cfg != nil {
			if t, ok := h.cfg.Models.TopP[ag.Role()]; ok {
				topP = t
			}
		}
		if topP == 0 {
			topP = 0.95
		}
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		if h.cfg != nil {
			if t, ok := h.cfg.Models.MaxTokens[ag.Role()]; ok {
				maxTokens = t
			}
		}
		if maxTokens == 0 {
			maxTokens = 2048
		}
	}

	resp, err := h.client.Complete(ctx, client.CompletionRequest{
		Model:       fallbackModel,
		Messages:    []client.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: temp,
		TopP:        topP,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, err
	}

	return &agent.AgentResponse{
		AgentID:    ag.ID(),
		Content:    resp.Content,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
	}, nil
}

func isRefusal(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	refusalIndicators := []string{
		"refusal",
		"content_filter",
		"content policy",
		"safety system",
		"content_violation",
		"inappropriate content",
		"i can't help with that",
		"i cannot help with that",
		"i'm not able to",
		"i am not able to",
		"cannot comply",
		"can't comply",
		"blocked",
		"not something i can",
		"unable to fulfill",
		"against my guidelines",
	}
	// Check for specific refusal patterns to reduce false positives
	for _, indicator := range refusalIndicators {
		if strings.Contains(msg, indicator) {
			return true
		}
	}
	// Only match "safety" if it appears in a context that suggests a filter/refusal
	if strings.Contains(msg, "safety") &&
		(strings.Contains(msg, "filter") || strings.Contains(msg, "violation") || strings.Contains(msg, "policy")) {
		return true
	}
	return false
}

func isEmptyResponse(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "empty_response") || strings.Contains(msg, "empty")
}

func isValidResponse(resp *agent.AgentResponse) bool {
	return resp != nil && resp.Error == nil && len(strings.TrimSpace(resp.Content)) >= 10
}

func softenPrompt(prompt string) string {
	replacements := map[string]string{
		"You must":      "Please consider",
		"you must":      "please consider",
		"Never concede": "Try to avoid conceding",
		"Argue ONLY":    "Focus primarily on arguing",
		"argue ONLY":    "focus primarily on arguing",
		"Do not repeat": "Try to avoid repeating",
		"Never":         "Avoid",
	}
	for old, new := range replacements {
		prompt = strings.ReplaceAll(prompt, old, new)
	}
	return prompt
}
