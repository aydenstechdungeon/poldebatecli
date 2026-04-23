package ctxmgr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/round"
)

type ContextManager struct {
	maxTokens  int
	summarizer *Summarizer
	tracker    *ArgumentTracker
	model      string
}

func NewContextManager(c *client.OpenRouterClientImpl, maxTokens int, model string) *ContextManager {
	if model == "" {
		model = DefaultContextModel
	}
	return &ContextManager{
		maxTokens:  maxTokens,
		summarizer: NewSummarizer(c, model),
		tracker:    NewArgumentTracker(c, model),
		model:      model,
	}
}

func (m *ContextManager) ProcessRound(ctx context.Context, state *round.DebateState, rr *round.RoundResult) {
	if rr == nil || len(rr.Messages) == 0 {
		return
	}

	summary, err := m.summarizer.Summarize(ctx, rr.Messages)
	if err != nil {
		slog.Warn("summarization failed, using truncation", "error", err)
		summary = truncateMessages(rr.Messages, 500)
	}

	if state.ContextSummary != "" {
		m.enforceTokenBudget(state)
		state.ContextSummary = state.ContextSummary + "\n\n" + summary
	} else {
		state.ContextSummary = summary
	}

	keyArgs := m.tracker.Extract(ctx, rr.Messages)
	state.KeyArguments = append(state.KeyArguments, keyArgs...)

	m.enforceTokenBudget(state)
}

func (m *ContextManager) ReprocessForResume(ctx context.Context, state *round.DebateState) {
	if len(state.RoundResults) == 0 {
		return
	}
	for i := range state.RoundResults {
		rr := &state.RoundResults[i]
		if len(rr.Messages) == 0 {
			continue
		}
		summary, err := m.summarizer.Summarize(ctx, rr.Messages)
		if err != nil {
			slog.Warn("summarization failed during resume, using truncation", "error", err)
			summary = truncateMessages(rr.Messages, 500)
		}
		if state.ContextSummary != "" {
			state.ContextSummary = state.ContextSummary + "\n\n" + summary
		} else {
			state.ContextSummary = summary
		}
		keyArgs := m.tracker.Extract(ctx, rr.Messages)
		state.KeyArguments = append(state.KeyArguments, keyArgs...)
		m.enforceTokenBudget(state)
	}
}

func (m *ContextManager) Summarizer() *Summarizer {
	return m.summarizer
}

func (m *ContextManager) Tracker() *ArgumentTracker {
	return m.tracker
}

func (m *ContextManager) BuildContextMessages(state *round.DebateState, teamID config.TeamID) string {
	var parts []string

	if state.ContextSummary != "" {
		parts = append(parts, "Prior debate context (summary):\n"+state.ContextSummary)
	}

	keyArgs := filterArgsByTeam(state.KeyArguments, teamID)
	if len(keyArgs) > 0 {
		var sb strings.Builder
		sb.WriteString("Key arguments from your team's prior rounds:\n")
		for _, arg := range keyArgs {
			fmt.Fprintf(&sb, "- [%s]: %s\n", arg.AgentID, arg.Claim)
			if arg.Evidence != "" {
				fmt.Fprintf(&sb, "  Evidence: %s\n", arg.Evidence)
			}
		}
		parts = append(parts, sb.String())
	}

	opposingTeamIDs := opposingTeams(state, teamID)
	opposingArgs := filterArgsByTeams(state.KeyArguments, opposingTeamIDs)
	if len(opposingArgs) > 0 {
		var sb strings.Builder
		sb.WriteString("Key arguments from the opposing team:\n")
		for _, arg := range opposingArgs {
			fmt.Fprintf(&sb, "- [%s]: %s\n", arg.AgentID, arg.Claim)
		}
		parts = append(parts, sb.String())
	}

	return strings.Join(parts, "\n")
}

func (m *ContextManager) enforceTokenBudget(state *round.DebateState) {
	estimatedTokens := estimateTokens(state.ContextSummary)
	if estimatedTokens <= m.maxTokens {
		return
	}

	// Keep recent key arguments but cap at a reasonable number
	maxKeyArgs := 8
	if len(state.KeyArguments) > maxKeyArgs {
		// Keep first 3 (early context) and last 5 (recent context)
		head := state.KeyArguments[:3]
		tail := state.KeyArguments[len(state.KeyArguments)-5:]
		state.KeyArguments = make([]round.KeyArg, 0, len(head)+len(tail))
		state.KeyArguments = append(state.KeyArguments, head...)
		state.KeyArguments = append(state.KeyArguments, tail...)
	}

	estimatedTokens = estimateTokens(state.ContextSummary)
	if estimatedTokens > m.maxTokens {
		// Truncate context summary keeping both beginning and end
		words := strings.Fields(state.ContextSummary)
		targetWords := m.maxTokens * 3 / 4
		if targetWords < len(words) {
			headWords := targetWords / 3
			tailWords := targetWords - headWords
			truncated := make([]string, 0, targetWords+1)
			truncated = append(truncated, words[:headWords]...)
			truncated = append(truncated, "...")
			truncated = append(truncated, words[len(words)-tailWords:]...)
			state.ContextSummary = strings.Join(truncated, " ")
		}
	}
}

func estimateTokens(text string) int {
	// Approximate token count: ~4 chars per token for English text,
	// but account for whitespace and special characters.
	// This is a conservative estimate; actual tokenization varies by model.
	if len(text) == 0 {
		return 0
	}
	// Count words and use ~1.3 tokens per word as a rough heuristic
	// that accounts for subword tokenization
	wordCount := len(strings.Fields(text))
	charCount := len(text)
	// Blend both estimates for better accuracy
	charEstimate := charCount / 4
	wordEstimate := int(float64(wordCount) * 1.3)
	// Use the higher estimate to be conservative
	if charEstimate > wordEstimate {
		return charEstimate
	}
	return wordEstimate
}

func truncateMessages(msgs []round.Message, maxWords int) string {
	var sb strings.Builder
	wordCount := 0
	for _, msg := range msgs {
		words := strings.Fields(msg.Content)
		for _, w := range words {
			if wordCount >= maxWords {
				return sb.String()
			}
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(w)
			wordCount++
		}
	}
	return sb.String()
}

func filterArgsByTeam(args []round.KeyArg, teamID config.TeamID) []round.KeyArg {
	var filtered []round.KeyArg
	for _, arg := range args {
		if arg.Team == teamID {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

func opposingTeams(state *round.DebateState, teamID config.TeamID) []config.TeamID {
	teamMeta, ok := state.TeamMeta[teamID]
	if !ok {
		return nil
	}
	var ids []config.TeamID
	for _, otherID := range state.TeamOrder {
		if otherID == teamID {
			continue
		}
		otherMeta := state.TeamMeta[otherID]
		if otherMeta.Side != teamMeta.Side {
			ids = append(ids, otherID)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	for _, otherID := range state.TeamOrder {
		if otherID != teamID {
			ids = append(ids, otherID)
		}
	}
	return ids
}

func filterArgsByTeams(args []round.KeyArg, teamIDs []config.TeamID) []round.KeyArg {
	if len(teamIDs) == 0 {
		return nil
	}
	allow := make(map[config.TeamID]struct{}, len(teamIDs))
	for _, id := range teamIDs {
		allow[id] = struct{}{}
	}
	var filtered []round.KeyArg
	for _, arg := range args {
		if _, ok := allow[arg.Team]; ok {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}
