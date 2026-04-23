package session

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/poldebatecli/internal/config"
)

func Load(path string) (*SessionSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	snap := &SessionSnapshot{}
	if err := json.Unmarshal(data, snap); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}

	if err := ValidateSnapshot(snap); err != nil {
		return nil, err
	}

	return snap, nil
}

func ValidateSnapshot(snap *SessionSnapshot) error {
	if snap.Version != CurrentVersion {
		return fmt.Errorf("unsupported session version %d (expected %d)", snap.Version, CurrentVersion)
	}

	if snap.Metadata.Status == StatusCompleted {
		return fmt.Errorf("session already completed, cannot resume")
	}

	if snap.Metadata.Status != StatusInProgress {
		return fmt.Errorf("invalid session status: %s", snap.Metadata.Status)
	}

	if snap.Config == nil {
		return fmt.Errorf("session missing config")
	}

	if err := config.Validate(snap.Config); err != nil {
		return fmt.Errorf("session config invalid: %w", err)
	}

	if snap.Progress.CurrentCycle < 0 || snap.Progress.CurrentRoundIndex < 0 {
		return fmt.Errorf("invalid session progress: cycle=%d round_index=%d", snap.Progress.CurrentCycle, snap.Progress.CurrentRoundIndex)
	}

	return nil
}
