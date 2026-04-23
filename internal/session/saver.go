package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/judge"
	"github.com/poldebatecli/internal/round"
)

type Saver struct {
	path                  string
	interval              time.Duration
	checkpointMinInterval time.Duration
	lastCheckpointSave    time.Time
	roundsSeen            int
	messagesSeen          int
	judgesSeen            int
	snapshot              *SessionSnapshot
	mu                    sync.Mutex
	cancel                context.CancelFunc
}

func NewSaver(path string, topic string, interval, checkpointMinInterval time.Duration, cfg *config.Config) *Saver {
	return &Saver{
		path:                  path,
		interval:              interval,
		checkpointMinInterval: checkpointMinInterval,
		snapshot: &SessionSnapshot{
			Version: CurrentVersion,
			Config:  cfg,
			Topic:   topic,
			Progress: Progress{
				TotalCycles: cfg.Rounds.Count,
				TotalRounds: len(cfg.Rounds.Sequence),
			},
			Metadata: SessionMetadata{
				StartedAt:  time.Now().Format(time.RFC3339),
				ModelsUsed: make(map[string]int),
				Status:     StatusInProgress,
			},
		},
	}
}

func NewSaverFromSnapshot(path string, topic string, interval, checkpointMinInterval time.Duration, snap *SessionSnapshot) *Saver {
	snap.Metadata.Status = StatusInProgress
	snap.Topic = topic
	return &Saver{
		path:                  path,
		interval:              interval,
		checkpointMinInterval: checkpointMinInterval,
		roundsSeen:            len(snap.Rounds),
		messagesSeen:          len(snap.Messages),
		judgesSeen:            len(snap.JudgeResults),
		snapshot:              snap,
	}
}

func (s *Saver) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Save(); err != nil {
					slog.Warn("auto-save failed", "error", err)
				}
			}
		}
	}()
}

func (s *Saver) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Metadata.LastSavedAt = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(s.snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write session temp: %w", err)
	}

	tmpFile, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open session temp for sync: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("fsync session temp: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename session file: %w", err)
	}

	dirFile, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open session directory for sync: %w", err)
	}
	if err := dirFile.Sync(); err != nil {
		dirFile.Close()
		return fmt.Errorf("fsync session directory: %w", err)
	}
	dirFile.Close()

	return nil
}

func (s *Saver) SaveCheckpoint(reason string) error {
	now := time.Now()

	s.mu.Lock()
	if s.checkpointMinInterval > 0 && !s.lastCheckpointSave.IsZero() && now.Sub(s.lastCheckpointSave) < s.checkpointMinInterval {
		s.mu.Unlock()
		return nil
	}
	s.lastCheckpointSave = now
	s.snapshot.Metadata.LastCheckpointAt = now.Format(time.RFC3339)
	s.snapshot.Metadata.LastCheckpointReason = reason
	s.mu.Unlock()

	return s.Save()
}

func (s *Saver) UpdateProgress(cycle, roundIdx int, rounds []round.RoundResult, messages []round.Message, judgeResults []judge.JudgeResult, modelsUsed map[string]int, totalTokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot.Progress.CurrentCycle = cycle
	s.snapshot.Progress.CurrentRoundIndex = roundIdx

	if s.roundsSeen > len(rounds) {
		s.snapshot.Rounds = deepCopyRoundResults(rounds)
		s.roundsSeen = len(rounds)
	} else if s.roundsSeen < len(rounds) {
		s.snapshot.Rounds = append(s.snapshot.Rounds, deepCopyRoundResults(rounds[s.roundsSeen:])...)
		s.roundsSeen = len(rounds)
	}

	if s.messagesSeen > len(messages) {
		s.snapshot.Messages = append([]round.Message(nil), messages...)
		s.messagesSeen = len(messages)
	} else if s.messagesSeen < len(messages) {
		s.snapshot.Messages = append(s.snapshot.Messages, messages[s.messagesSeen:]...)
		s.messagesSeen = len(messages)
	}

	if s.judgesSeen > len(judgeResults) {
		s.snapshot.JudgeResults = deepCopyJudgeResults(judgeResults)
		s.judgesSeen = len(judgeResults)
	} else if s.judgesSeen < len(judgeResults) {
		s.snapshot.JudgeResults = append(s.snapshot.JudgeResults, deepCopyJudgeResults(judgeResults[s.judgesSeen:])...)
		s.judgesSeen = len(judgeResults)
	}

	s.snapshot.Metadata.ModelsUsed = copyStringIntMap(modelsUsed)
	s.snapshot.Metadata.TotalTokens = totalTokens
}

func (s *Saver) UpdateFailedRounds(failedRounds []FailedRound) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.FailedRounds = failedRounds
}

func (s *Saver) MarkCompleted() error {
	s.mu.Lock()
	s.snapshot.Metadata.Status = StatusCompleted
	s.mu.Unlock()
	return s.Save()
}

func (s *Saver) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if err := s.Save(); err != nil {
		slog.Warn("final save on stop failed", "error", err)
	}
}

func (s *Saver) Snapshot() *SessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

func deepCopyRoundResults(src []round.RoundResult) []round.RoundResult {
	if src == nil {
		return nil
	}
	result := make([]round.RoundResult, len(src))
	for i, rr := range src {
		result[i] = rr
		result[i].Messages = append([]round.Message(nil), rr.Messages...)
		result[i].Errors = append([]round.RoundError(nil), rr.Errors...)
	}
	return result
}

func deepCopyJudgeResults(src []judge.JudgeResult) []judge.JudgeResult {
	if src == nil {
		return nil
	}
	result := make([]judge.JudgeResult, len(src))
	for i, jr := range src {
		result[i] = jr
		result[i].References = append([]judge.ArgReference(nil), jr.References...)
	}
	return result
}

func copyStringIntMap(src map[string]int) map[string]int {
	if src == nil {
		return nil
	}
	result := make(map[string]int, len(src))
	for k, v := range src {
		result[k] = v
	}
	return result
}
