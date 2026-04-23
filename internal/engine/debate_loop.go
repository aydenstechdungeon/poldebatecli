package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/config"
	ctxmgr "github.com/poldebatecli/internal/context"
	"github.com/poldebatecli/internal/failure"
	"github.com/poldebatecli/internal/judge"
	"github.com/poldebatecli/internal/round"
	"github.com/poldebatecli/internal/session"
)

type debateEngine struct {
	deps *EngineDeps
}

func NewEngine(deps *EngineDeps) Engine {
	return &debateEngine{deps: deps}
}

func (e *debateEngine) Run(ctx context.Context, topic string, cfg *config.Config, liveCh chan<- round.LiveEvent, sessionCfg *SessionConfig) (*DebateResult, error) {
	if liveCh != nil {
		defer close(liveCh)
	}

	state := round.NewDebateState(topic)
	state.TeamMeta = make(map[config.TeamID]round.TeamMeta)
	for _, tc := range cfg.Teams.EffectiveTeams() {
		state.TeamOrder = append(state.TeamOrder, tc.ID)
		state.TeamMeta[tc.ID] = round.TeamMeta{Name: tc.Name, Side: tc.Side, PositionDescription: tc.PositionDescription}
	}
	state.PromptBuilder = e.deps.PromptBuilder
	state.LiveEvents = liveCh
	state.StreamingEnabled = cfg.Output.Streaming && liveCh != nil
	state.StreamBlockSize = cfg.Output.StreamBlockSize

	builtTeams, err := e.deps.TeamBuilder.BuildTeams()
	if err != nil {
		return nil, fmt.Errorf("build teams: %w", err)
	}

	state.TeamAgents = make(map[config.TeamID][]agent.Agent, len(builtTeams))
	for _, tm := range builtTeams {
		state.TeamAgents[tm.ID] = tm.Agents
	}

	fh := failure.NewHandler(e.deps.Client, cfg.Models.Fallbacks, cfg)
	state.OnAgentError = func(agentCtx context.Context, agentErr error, ag agent.Agent, prompt string, opts agent.GenerateOpts) (*agent.AgentResponse, error) {
		return fh.Handle(agentCtx, agentErr, ag, prompt, opts)
	}

	cm := ctxmgr.NewContextManager(e.deps.Client, 6000, cfg.Models.ContextModel)

	var start time.Time
	var allRoundResults []round.RoundResult
	var allJudgeResults []judge.JudgeResult
	var allMessages []round.Message
	var failedRounds []FailedRound
	modelsUsed := make(map[string]int)
	totalTokens := 0

	if sessionCfg != nil && sessionCfg.ResumeFrom != nil {
		snap := sessionCfg.ResumeFrom
		parsedStart, parseErr := time.Parse(time.RFC3339, snap.Metadata.StartedAt)
		if parseErr != nil {
			slog.Warn("invalid session started_at on resume, using current time", "started_at", snap.Metadata.StartedAt, "error", parseErr)
			start = time.Now()
		} else {
			start = parsedStart
		}

		for _, rr := range snap.Rounds {
			allRoundResults = append(allRoundResults, rr)
			state.AddRound(&rr)
		}
		allMessages = append(allMessages, snap.Messages...)
		state.AddMessages(snap.Messages)
		allJudgeResults = append(allJudgeResults, snap.JudgeResults...)
		for i := range snap.JudgeResults {
			state.JudgeResults = append(state.JudgeResults, &snap.JudgeResults[i])
		}
		for _, fr := range snap.FailedRounds {
			failedRounds = append(failedRounds, FailedRound(fr))
		}
		for m, c := range snap.Metadata.ModelsUsed {
			modelsUsed[m] = c
		}
		totalTokens = snap.Metadata.TotalTokens

		cm.ReprocessForResume(ctx, state)

		slog.Info("resuming session", "cycle", snap.Progress.CurrentCycle, "round_index", snap.Progress.CurrentRoundIndex,
			"completed_rounds", len(snap.Rounds), "completed_messages", len(snap.Messages))
	} else {
		start = time.Now()
	}

	var saver *session.Saver
	if sessionCfg != nil && sessionCfg.SavePath != "" {
		if sessionCfg.ResumeFrom != nil {
			saver = session.NewSaverFromSnapshot(sessionCfg.SavePath, topic, sessionCfg.SaveInterval, sessionCfg.CheckpointMinInterval, sessionCfg.ResumeFrom)
		} else {
			saver = session.NewSaver(sessionCfg.SavePath, topic, sessionCfg.SaveInterval, sessionCfg.CheckpointMinInterval, cfg)
		}
		saver.Start(ctx)
		defer saver.Stop()
	}

	for cycle := 0; cycle < cfg.Rounds.Count; cycle++ {
		for roundIdx, roundType := range cfg.Rounds.Sequence {
			if sessionCfg != nil && sessionCfg.ResumeFrom != nil {
				resumeCycle := sessionCfg.ResumeFrom.Progress.CurrentCycle
				resumeRoundIdx := sessionCfg.ResumeFrom.Progress.CurrentRoundIndex
				numRounds := len(cfg.Rounds.Sequence)

				if resumeRoundIdx >= numRounds && numRounds > 0 {
					resumeCycle++
					resumeRoundIdx = 0
				}

				if cycle < resumeCycle {
					continue
				}
				if cycle == resumeCycle && roundIdx < resumeRoundIdx {
					continue
				}
			}

			roundTimeout := cfg.Rounds.Timeouts[roundType]
			roundCtx, cancel := context.WithTimeout(ctx, roundTimeout)

			rd, ok := e.deps.RoundRegistry.Get(roundType)
			if !ok {
				cancel()
				return nil, fmt.Errorf("unknown round type: %s", roundType)
			}

			fh.SetRound(roundType)
			slog.Info("starting round", "round", roundType, "cycle", cycle)

			if saver != nil {
				state.OnTeamComplete = func(roundType config.RoundType, teamID config.TeamID) {
					reason := fmt.Sprintf("team_complete:%s:%s:c%d:r%d", roundType, teamID, cycle, roundIdx)
					if err := saver.SaveCheckpoint(reason); err != nil {
						slog.Warn("checkpoint save failed", "reason", reason, "error", err)
					}
				}
			} else {
				state.OnTeamComplete = nil
			}

			round.TrySendLiveEvent(state.LiveEvents, round.LiveEvent{
				Type:      round.LiveEventRoundStart,
				RoundType: roundType,
			})

			roundResult, err := rd.Execute(roundCtx, state)
			cancel()

			round.TrySendLiveEvent(state.LiveEvents, round.LiveEvent{
				Type:      round.LiveEventRoundDone,
				RoundType: roundType,
			})

			if err != nil {
				slog.Warn("round failed", "round", roundType, "cycle", cycle, "error", err)
				failedRounds = append(failedRounds, FailedRound{
					RoundType: roundType,
					Cycle:     cycle,
					Error:     err.Error(),
				})
				if saver != nil {
					saver.UpdateFailedRounds(convertToSessionFailedRounds(failedRounds))
					if err := saver.Save(); err != nil {
						slog.Error("failed to save session", "error", err)
					}
				}
				continue
			}

			for _, msg := range roundResult.Messages {
				modelsUsed[msg.Model]++
				totalTokens += msg.TokensUsed
			}

			var judgeResults []judge.JudgeResult
			if !cfg.Output.SkipJudges && rd.RequiresJudge() && len(roundResult.Messages) > 0 {
				judgeResults = evaluateJudgesParallel(ctx, e.deps.JudgeRegistry.All(), *roundResult, state)
				if saver != nil {
					reason := fmt.Sprintf("judges_complete:%s:c%d:r%d", roundType, cycle, roundIdx)
					if err := saver.SaveCheckpoint(reason); err != nil {
						slog.Warn("checkpoint save failed", "reason", reason, "error", err)
					}
				}
			}

			cm.ProcessRound(ctx, state, roundResult)
			if saver != nil {
				reason := fmt.Sprintf("context_complete:%s:c%d:r%d", roundType, cycle, roundIdx)
				if err := saver.SaveCheckpoint(reason); err != nil {
					slog.Warn("checkpoint save failed", "reason", reason, "error", err)
				}
			}

			state.AddRound(roundResult)
			state.AddMessages(roundResult.Messages)
			for i := range judgeResults {
				state.JudgeResults = append(state.JudgeResults, &judgeResults[i])
			}

			allRoundResults = append(allRoundResults, *roundResult)
			allJudgeResults = append(allJudgeResults, judgeResults...)
			allMessages = append(allMessages, roundResult.Messages...)

			if saver != nil {
				saver.UpdateProgress(cycle, roundIdx+1, allRoundResults, allMessages, allJudgeResults, modelsUsed, totalTokens)
				saver.UpdateFailedRounds(convertToSessionFailedRounds(failedRounds))
				if err := saver.Save(); err != nil {
					slog.Error("failed to save session", "error", err)
				}
			}

			slog.Info("round complete", "round", roundType, "cycle", cycle,
				"messages", len(roundResult.Messages), "judges", len(judgeResults))
		}
	}

	aggregated := judge.AggregateScores(allJudgeResults)
	winner := judge.DetermineWinner(aggregated)

	degraded := fh.DegradedEntries()

	if saver != nil {
		if markErr := saver.MarkCompleted(); markErr != nil {
			slog.Warn("failed to mark session completed", "error", markErr)
		}
	}

	return &DebateResult{
		Topic:      topic,
		Winner:     winner,
		Scores:     aggregated,
		Rounds:     allRoundResults,
		Transcript: allMessages,
		Judges:     allJudgeResults,
		Metadata: ResultMetadata{
			Timestamp:   start.Format(time.RFC3339),
			Duration:    time.Since(start).String(),
			ModelsUsed:  modelsUsed,
			TotalTokens: totalTokens,
		},
		Degraded:     degraded,
		FailedRounds: failedRounds,
	}, nil
}

func convertToSessionFailedRounds(frs []FailedRound) []session.FailedRound {
	result := make([]session.FailedRound, len(frs))
	for i, fr := range frs {
		result[i] = session.FailedRound(fr)
	}
	return result
}

func (e *debateEngine) ValidateConfig(cfg *config.Config) error {
	return config.Validate(cfg)
}

func evaluateJudgesParallel(ctx context.Context, judges []judge.Judge, rr round.RoundResult, state *round.DebateState) []judge.JudgeResult {
	type judgeOutcome struct {
		result *judge.JudgeResult
		err    error
	}

	outcomes := make([]judgeOutcome, len(judges))
	var wg sync.WaitGroup

	for i, j := range judges {
		wg.Add(1)
		go func(idx int, jud judge.Judge) {
			defer wg.Done()
			jr, err := jud.Evaluate(ctx, rr, state)
			outcomes[idx] = judgeOutcome{result: jr, err: err}
		}(i, j)
	}

	wg.Wait()

	var results []judge.JudgeResult
	for i, outcome := range outcomes {
		if outcome.err != nil {
			slog.Warn("judge evaluation failed", "judge", judges[i].Type(), "error", outcome.err)
			continue
		}
		if outcome.result != nil {
			results = append(results, *outcome.result)
		}
	}
	return results
}

func (e *debateEngine) Simulate(ctx context.Context, topic string, cfg *config.Config) (*DebateResult, error) {
	state := round.NewDebateState(topic)
	state.TeamMeta = make(map[config.TeamID]round.TeamMeta)
	teamCfgs := cfg.Teams.EffectiveTeams()
	mockAgents := make(map[config.TeamID][]agent.Agent, len(teamCfgs))
	for _, tc := range teamCfgs {
		state.TeamOrder = append(state.TeamOrder, tc.ID)
		state.TeamMeta[tc.ID] = round.TeamMeta{Name: tc.Name, Side: tc.Side, PositionDescription: tc.PositionDescription}
		mockAgents[tc.ID] = buildMockAgents(tc.TeamConfig, tc.ID)
	}
	state.TeamAgents = mockAgents
	state.PromptBuilder = e.deps.PromptBuilder

	start := time.Now()

	var allRoundResults []round.RoundResult
	var allMessages []round.Message
	modelsUsed := make(map[string]int)
	totalTokens := 0

	for cycle := 0; cycle < cfg.Rounds.Count; cycle++ {
		for _, roundType := range cfg.Rounds.Sequence {
			if _, ok := e.deps.RoundRegistry.Get(roundType); !ok {
				continue
			}

			roundResult := &round.RoundResult{
				Type:     roundType,
				Messages: generateMockMessages(mockAgents, state.TeamOrder, roundType),
				Duration: 100 * time.Millisecond,
			}

			for _, msg := range roundResult.Messages {
				modelsUsed[msg.Model]++
				totalTokens += msg.TokensUsed
			}

			state.AddRound(roundResult)
			state.AddMessages(roundResult.Messages)

			allRoundResults = append(allRoundResults, *roundResult)
			allMessages = append(allMessages, roundResult.Messages...)
		}
	}

	aggregated := judge.AggregatedScores{Teams: map[config.TeamID]judge.TeamAggregated{}}
	for _, teamID := range state.TeamOrder {
		aggregated.Teams[teamID] = judge.TeamAggregated{
			LogicalConsistency: 7.5,
			EvidenceQuality:    7.0,
			Responsiveness:     7.4,
			StrategicStrength:  7.2,
			Total:              7.3,
		}
	}

	return &DebateResult{
		Topic:      topic,
		Scores:     aggregated,
		Rounds:     allRoundResults,
		Transcript: allMessages,
		Metadata: ResultMetadata{
			Timestamp:   start.Format(time.RFC3339),
			Duration:    time.Since(start).String(),
			ModelsUsed:  modelsUsed,
			TotalTokens: totalTokens,
		},
	}, nil
}
