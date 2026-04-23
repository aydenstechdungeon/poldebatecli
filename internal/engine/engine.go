package engine

import (
	"context"
	"time"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/failure"
	"github.com/poldebatecli/internal/judge"
	"github.com/poldebatecli/internal/round"
	"github.com/poldebatecli/internal/session"
)

type Engine interface {
	Run(ctx context.Context, topic string, cfg *config.Config, liveCh chan<- round.LiveEvent, sessionCfg *SessionConfig) (*DebateResult, error)
	ValidateConfig(cfg *config.Config) error
	Simulate(ctx context.Context, topic string, cfg *config.Config) (*DebateResult, error)
}

type SessionConfig struct {
	SavePath              string
	SaveInterval          time.Duration
	CheckpointMinInterval time.Duration
	ResumeFrom            *session.SessionSnapshot
}

type DebateResult struct {
	Topic        string                  `json:"topic"`
	Winner       config.TeamID           `json:"winner"`
	Scores       judge.AggregatedScores  `json:"scores"`
	Rounds       []round.RoundResult     `json:"rounds"`
	Transcript   []round.Message         `json:"transcript"`
	Judges       []judge.JudgeResult     `json:"judges"`
	Metadata     ResultMetadata          `json:"metadata"`
	Degraded     []failure.DegradedEntry `json:"degraded,omitempty"`
	FailedRounds []FailedRound           `json:"failed_rounds,omitempty"`
}

type FailedRound struct {
	RoundType config.RoundType `json:"round_type"`
	Cycle     int              `json:"cycle"`
	Error     string           `json:"error"`
}

type ResultMetadata struct {
	Timestamp   string         `json:"timestamp"`
	Duration    string         `json:"duration"`
	ModelsUsed  map[string]int `json:"models_used"`
	TotalTokens int            `json:"total_tokens"`
}
