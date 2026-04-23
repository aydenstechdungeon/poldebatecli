package session

import (
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/judge"
	"github.com/poldebatecli/internal/round"
)

type FailedRound struct {
	RoundType config.RoundType `json:"round_type"`
	Cycle     int              `json:"cycle"`
	Error     string           `json:"error"`
}

type SessionSnapshot struct {
	Version      int                 `json:"version"`
	Config       *config.Config      `json:"config"`
	Topic        string              `json:"topic"`
	Progress     Progress            `json:"progress"`
	Rounds       []round.RoundResult `json:"completed_rounds"`
	Messages     []round.Message     `json:"messages"`
	JudgeResults []judge.JudgeResult `json:"judge_results,omitempty"`
	FailedRounds []FailedRound       `json:"failed_rounds,omitempty"`
	Metadata     SessionMetadata     `json:"metadata"`
}

type Progress struct {
	CurrentCycle      int `json:"current_cycle"`
	CurrentRoundIndex int `json:"current_round_index"`
	TotalCycles       int `json:"total_cycles"`
	TotalRounds       int `json:"total_rounds_per_cycle"`
}

type SessionMetadata struct {
	StartedAt            string         `json:"started_at"`
	LastSavedAt          string         `json:"last_saved_at"`
	LastCheckpointAt     string         `json:"last_checkpoint_at,omitempty"`
	LastCheckpointReason string         `json:"last_checkpoint_reason,omitempty"`
	ModelsUsed           map[string]int `json:"models_used"`
	TotalTokens          int            `json:"total_tokens"`
	Status               SessionStatus  `json:"status"`
}

type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusCompleted  SessionStatus = "completed"
)

const CurrentVersion = 1
