package ctxmgr

import (
	"context"
	"fmt"
	"strings"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/round"
)

type Summarizer struct {
	client *client.OpenRouterClientImpl
	model  string
}

const DefaultContextModel = "openai/gpt-5.4-nano"

func NewSummarizer(c *client.OpenRouterClientImpl, model string) *Summarizer {
	if model == "" {
		model = DefaultContextModel
	}
	return &Summarizer{
		client: c,
		model:  model,
	}
}

func (s *Summarizer) Summarize(ctx context.Context, messages []round.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	if s.client == nil {
		return "", fmt.Errorf("summarizer client is nil")
	}

	var sb strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&sb, "[%s (%s)]: %s\n\n", msg.AgentID, msg.Role, msg.Content)
	}

	prompt := fmt.Sprintf(`Summarize the key arguments, claims, and evidence from this debate round in under 200 words. Focus on the strongest points from each side.

Debate round:
%s`, sb.String())

	resp, err := s.client.Complete(ctx, client.CompletionRequest{
		Model:       s.model,
		Messages:    []client.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   512,
	})
	if err != nil {
		return "", fmt.Errorf("summarization API call failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func (s *Summarizer) SetModel(model string) {
	s.model = model
}
