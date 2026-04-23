package ctxmgr

import (
	"context"
	"fmt"
	"strings"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/round"
)

type ArgumentTracker struct {
	client *client.OpenRouterClientImpl
	model  string
}

func NewArgumentTracker(c *client.OpenRouterClientImpl, model string) *ArgumentTracker {
	if model == "" {
		model = DefaultContextModel
	}
	return &ArgumentTracker{
		client: c,
		model:  model,
	}
}

func (t *ArgumentTracker) Extract(ctx context.Context, messages []round.Message) []round.KeyArg {
	if len(messages) == 0 {
		return nil
	}

	if t.client == nil {
		return t.extractSimple(messages)
	}

	var sb strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&sb, "[%s (%s, %s)]: %s\n\n", msg.AgentID, msg.Role, msg.Team, msg.Content)
	}

	prompt := fmt.Sprintf(`Extract the key claims and evidence from this debate round. For each claim, identify:
1. The agent who made it
2. Their team (team_a, team_b, team_c, ...)
3. The claim itself (one sentence)
4. The supporting evidence (if any)

Output one claim per line in format:
AGENT_ID|TEAM|CLAIM|EVIDENCE

Debate round:
%s`, sb.String())

	resp, err := t.client.Complete(ctx, client.CompletionRequest{
		Model:       t.model,
		Messages:    []client.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   1024,
	})
	if err != nil {
		return t.extractSimple(messages)
	}

	args := t.parseExtractedClaims(resp.Content, messages)
	if len(args) == 0 {
		return t.extractSimple(messages)
	}
	return args
}

func (t *ArgumentTracker) extractSimple(messages []round.Message) []round.KeyArg {
	var args []round.KeyArg
	for _, msg := range messages {
		sentences := strings.Split(msg.Content, ". ")
		if len(sentences) > 0 {
			claim := sentences[0]
			if len(claim) > 20 {
				if len(claim) > 200 {
					claim = claim[:200]
				}
				args = append(args, round.KeyArg{
					AgentID:  msg.AgentID,
					Team:     msg.Team,
					Round:    msg.Round,
					Claim:    claim,
					Evidence: "",
				})
			}
		}
		if len(args) >= 6 {
			break
		}
	}
	return args
}

func (t *ArgumentTracker) parseExtractedClaims(content string, messages []round.Message) []round.KeyArg {
	var args []round.KeyArg
	lines := strings.Split(strings.TrimSpace(content), "\n")

	var roundType config.RoundType
	if len(messages) > 0 {
		roundType = messages[0].Round
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 3 {
			teamStr := strings.TrimSpace(parts[1])
			if !strings.HasPrefix(teamStr, "team_") {
				continue
			}
			arg := round.KeyArg{
				AgentID: strings.TrimSpace(parts[0]),
				Team:    config.TeamID(teamStr),
				Round:   roundType,
				Claim:   strings.TrimSpace(parts[2]),
			}
			if len(parts) >= 4 {
				arg.Evidence = strings.TrimSpace(parts[3])
			}
			args = append(args, arg)
		}
	}
	return args
}
