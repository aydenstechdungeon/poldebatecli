package round

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/config"
)

type BaseRound struct {
	RoundType         config.RoundType
	RequiresJudgeFlag bool
	Templates         []string
}

func (b *BaseRound) Name() config.RoundType {
	return b.RoundType
}

func (b *BaseRound) RequiresJudge() bool {
	return b.RequiresJudgeFlag
}

func (b *BaseRound) PromptTemplates() []string {
	return b.Templates
}

type OpeningRound struct{ BaseRound }

func (r *OpeningRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundOpening,
		RoundDescription: "Present your core thesis and key arguments",
	})
}

type SteelmanRound struct{ BaseRound }

func (r *SteelmanRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundSteelman,
		RoundDescription: "Reconstruct the strongest version of your opponent's position",
	})
}

type RebuttalRound struct{ BaseRound }

func (r *RebuttalRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundRebuttal,
		RoundDescription: "Counter opposing arguments with evidence and reasoning",
	})
}

type CrossExamRound struct{ BaseRound }

func (r *CrossExamRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundCrossExam,
		RoundDescription: "Pose sharp questions exposing weaknesses in opposing reasoning",
	})
}

type FactCheckRound struct{ BaseRound }

func (r *FactCheckRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	var claims []string
	for _, arg := range state.KeyArguments {
		claim := fmt.Sprintf("[%s, %s]: %s", arg.AgentID, arg.Team, arg.Claim)
		if arg.Evidence != "" {
			claim += fmt.Sprintf(" (Evidence: %s)", arg.Evidence)
		}
		claims = append(claims, claim)
	}
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundFactCheck,
		RoundDescription: "Verify factual claims from prior rounds and assess their implications",
		FactCheckData:    claims,
	})
}

type PositionSwapRound struct{ BaseRound }

func (r *PositionSwapRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundPositionSwap,
		RoundDescription: "Argue the opposing side to test intellectual honesty",
	})
}

type ClosingRound struct{ BaseRound }

func (r *ClosingRound) Execute(ctx context.Context, state *DebateState) (*RoundResult, error) {
	return executeStandardRound(ctx, state, r, agent.PromptParams{
		RoundName:        config.RoundClosing,
		RoundDescription: "Synthesize strongest arguments into a decisive conclusion",
	})
}

func executeStandardRound(ctx context.Context, state *DebateState, round Round, params agent.PromptParams) (*RoundResult, error) {
	start := time.Now()
	var messages []Message
	var errs []RoundError
	var mu sync.Mutex

	params.Topic = state.Topic
	params.ContextSummary = state.ContextSummary

	teams := make([]struct {
		id   config.TeamID
		name string
		side string
	}, 0, len(state.TeamOrder))
	for _, teamID := range state.TeamOrder {
		meta := state.TeamMeta[teamID]
		teams = append(teams, struct {
			id   config.TeamID
			name string
			side string
		}{id: teamID, name: meta.Name, side: meta.Side})
	}

	streaming := state.StreamingEnabled && state.LiveEvents != nil

	for _, t := range teams {
		agents := state.TeamAgents[t.id]
		if len(agents) == 0 {
			slog.Warn("no agents for team", "team", t.id, "round", round.Name())
			continue
		}

		params.TeamName = t.name
		params.Side = t.side
		params.PositionDescription = state.TeamMeta[t.id].PositionDescription
		params.OppositeSide = oppositeSideForTeam(state, t.id, t.side)

		if streaming {
			var wg sync.WaitGroup
			for _, ag := range agents {
				wg.Add(1)
				go func(a agent.Agent) {
					defer wg.Done()
					msg := executeAgentStreaming(ctx, a, t.id, t.name, state, round, params)
					if msg != nil {
						mu.Lock()
						messages = append(messages, *msg)
						mu.Unlock()
					} else {
						mu.Lock()
						errs = append(errs, RoundError{Agent: a.ID(), Error: "streaming failed"})
						mu.Unlock()
						slog.Warn("agent streaming failed", "agent", a.ID(), "round", round.Name())
					}
				}(ag)
			}
			wg.Wait()
		} else {
			var wg sync.WaitGroup
			for _, ag := range agents {
				wg.Add(1)
				go func(a agent.Agent) {
					defer wg.Done()

					agentParams := params
					agentParams.AgentID = a.ID()
					agentParams.Role = a.Role()

					var prompt string
					if state.PromptBuilder != nil {
						prompt = state.PromptBuilder.Build(agentParams)
					} else {
						prompt = agent.BuildFallbackPrompt(agentParams)
					}

					opts := agent.GenerateOpts{}
					resp, err := a.Generate(ctx, prompt, opts)
					if err != nil {
						if state.OnAgentError != nil {
							resp, err = state.OnAgentError(ctx, err, a, prompt, opts)
						}
						if err != nil {
							mu.Lock()
							errs = append(errs, RoundError{Agent: a.ID(), Error: err.Error()})
							mu.Unlock()
							slog.Warn("agent failed", "agent", a.ID(), "round", round.Name(), "error", err)
							return
						}
					}

					mu.Lock()
					messages = append(messages, Message{
						AgentID:    a.ID(),
						Team:       a.TeamID(),
						Role:       a.Role(),
						Round:      round.Name(),
						Content:    resp.Content,
						Model:      resp.Model,
						TokensUsed: resp.TokensUsed,
						Degraded:   resp.Degraded,
					})
					mu.Unlock()
				}(ag)
			}
			wg.Wait()
		}

		TrySendLiveEvent(state.LiveEvents, LiveEvent{
			Type:      LiveEventTeamDone,
			RoundType: round.Name(),
			TeamID:    t.id,
			TeamName:  t.name,
		})
		if state.OnTeamComplete != nil {
			state.OnTeamComplete(round.Name(), t.id)
		}
	}

	return &RoundResult{
		Type:     round.Name(),
		Messages: messages,
		Duration: time.Since(start),
		Errors:   errs,
	}, nil
}

func oppositeSideForTeam(state *DebateState, teamID config.TeamID, side string) string {
	if side == "for" {
		return "against"
	}
	if side == "against" {
		return "for"
	}
	for _, otherID := range state.TeamOrder {
		if otherID == teamID {
			continue
		}
		other := state.TeamMeta[otherID]
		if other.Side != side && (other.Side == "for" || other.Side == "against") {
			return other.Side
		}
	}
	return "against"
}

func TrySendLiveEvent(ch chan<- LiveEvent, event LiveEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- event:
	default:
		slog.Warn("live events channel full, dropping event", "type", event.Type, "agent", event.AgentID)
	}
}

func executeAgentStreaming(ctx context.Context, a agent.Agent, teamID config.TeamID, teamName string, state *DebateState, round Round, params agent.PromptParams) *Message {
	agentParams := params
	agentParams.AgentID = a.ID()
	agentParams.Role = a.Role()

	var prompt string
	if state.PromptBuilder != nil {
		prompt = state.PromptBuilder.Build(agentParams)
	} else {
		prompt = agent.BuildFallbackPrompt(agentParams)
	}

	TrySendLiveEvent(state.LiveEvents, LiveEvent{
		Type:      LiveEventAgentStart,
		RoundType: round.Name(),
		AgentID:   a.ID(),
		TeamID:    teamID,
		TeamName:  teamName,
	})

	opts := agent.GenerateOpts{
		StreamBlock: state.StreamBlockSize,
	}

	streamCh, err := a.StreamGenerate(ctx, prompt, opts)
	if err != nil {
		slog.Warn("streaming failed, falling back", "agent", a.ID(), "error", err)
		if state.OnAgentError != nil {
			resp, handlerErr := state.OnAgentError(ctx, err, a, prompt, opts)
			if handlerErr == nil {
				TrySendLiveEvent(state.LiveEvents, LiveEvent{
					Type:      LiveEventAgentChunk,
					RoundType: round.Name(),
					AgentID:   a.ID(),
					TeamID:    teamID,
					TeamName:  teamName,
					Content:   resp.Content,
				})
				TrySendLiveEvent(state.LiveEvents, LiveEvent{
					Type:      LiveEventAgentDone,
					RoundType: round.Name(),
					AgentID:   a.ID(),
					TeamID:    teamID,
					TeamName:  teamName,
					Model:     resp.Model,
				})
				return &Message{
					AgentID:    a.ID(),
					Team:       teamID,
					Role:       a.Role(),
					Round:      round.Name(),
					Content:    resp.Content,
					Model:      resp.Model,
					TokensUsed: resp.TokensUsed,
					Degraded:   resp.Degraded,
				}
			}
		}
		TrySendLiveEvent(state.LiveEvents, LiveEvent{
			Type:      LiveEventAgentDone,
			RoundType: round.Name(),
			AgentID:   a.ID(),
			TeamID:    teamID,
			TeamName:  teamName,
			Failed:    true,
		})
		return nil
	}

	var contentBuf strings.Builder
	var model string
	var tokensUsed int
	for chunk := range streamCh {
		if chunk.Err != nil {
			slog.Warn("streaming failed for agent", "agent", a.ID(), "round", round.Name(), "error", chunk.Err)
			TrySendLiveEvent(state.LiveEvents, LiveEvent{
				Type:      LiveEventAgentDone,
				RoundType: round.Name(),
				AgentID:   a.ID(),
				TeamID:    teamID,
				TeamName:  teamName,
				Failed:    true,
			})
			return nil
		}
		if chunk.Content != "" {
			contentBuf.WriteString(chunk.Content)
			TrySendLiveEvent(state.LiveEvents, LiveEvent{
				Type:      LiveEventAgentChunk,
				RoundType: round.Name(),
				AgentID:   a.ID(),
				TeamID:    teamID,
				TeamName:  teamName,
				Content:   chunk.Content,
			})
		}
		if chunk.Done {
			model = chunk.Model
			tokensUsed = chunk.TokensUsed
		}
	}

	if model == "" {
		model = a.Model()
	}

	TrySendLiveEvent(state.LiveEvents, LiveEvent{
		Type:      LiveEventAgentDone,
		RoundType: round.Name(),
		AgentID:   a.ID(),
		TeamID:    teamID,
		TeamName:  teamName,
		Model:     model,
	})

	return &Message{
		AgentID:    a.ID(),
		Team:       teamID,
		Role:       a.Role(),
		Round:      round.Name(),
		Content:    contentBuf.String(),
		Model:      model,
		TokensUsed: tokensUsed,
	}
}
