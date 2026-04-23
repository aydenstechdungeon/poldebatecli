package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
)

type TranscriptWriter struct{}

func NewTranscriptWriter() *TranscriptWriter {
	return &TranscriptWriter{}
}

func (w *TranscriptWriter) Write(ctx context.Context, result *engine.DebateResult, cfg *config.Config) error {
	if !cfg.Output.Transcript {
		return nil
	}

	outputPath := cfg.Output.Path
	if outputPath == "" {
		outputPath = "./debate_output"
	}

	if err := os.MkdirAll(outputPath, 0700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	transcriptPath := filepath.Join(outputPath, "transcript.txt")

	f, err := os.OpenFile(transcriptPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create transcript file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var sb strings.Builder

	sb.WriteString("DEBATE TRANSCRIPT\n")
	sb.WriteString("==================\n")
	fmt.Fprintf(&sb, "Topic: %s\n", result.Topic)
	sb.WriteString("Date: " + result.Metadata.Timestamp + "\n")
	sb.WriteString("Duration: " + result.Metadata.Duration + "\n")
	sb.WriteString("Winner: " + string(result.Winner) + "\n\n")

	for _, rd := range result.Rounds {
		fmt.Fprintf(&sb, "─── %s ───\n\n", w.roundDisplayName(rd.Type))

		for _, msg := range rd.Messages {
			degradedTag := ""
			if msg.Degraded {
				degradedTag = " [DEGRADED]"
			}
			fmt.Fprintf(&sb, "[%s] %s (%s)%s:\n", msg.Team, msg.AgentID, msg.Role, degradedTag)
			fmt.Fprintf(&sb, "%s\n\n", msg.Content)
		}

		sb.WriteString(fmt.Sprintf("Duration: %s\n\n", rd.Duration))
	}

	sb.WriteString("─── FINAL SCORES ───\n\n")
	for _, tc := range cfg.Teams.EffectiveTeams() {
		score, ok := result.Scores.Teams[tc.ID]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s Total: %.1f\n", config.TeamLabel(tc.ID), score.Total))
	}
	sb.WriteString(fmt.Sprintf("Winner: %s\n", result.Winner))

	if len(result.Degraded) > 0 {
		sb.WriteString("\n─── DEGRADED RESPONSES ───\n\n")
		for _, d := range result.Degraded {
			sb.WriteString(fmt.Sprintf("- %s in %s: %s → %s (%s)\n",
				d.AgentID, d.Round, d.OriginalModel, d.UsedModel, d.Reason))
		}
	}

	_, err = f.WriteString(sb.String())
	if err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}

	return nil
}

func (w *TranscriptWriter) roundDisplayName(rt config.RoundType) string {
	displayNames := map[config.RoundType]string{
		config.RoundOpening:      "OPENING STATEMENTS",
		config.RoundSteelman:     "STEELMAN OPPONENT",
		config.RoundRebuttal:     "REBUTTAL",
		config.RoundCrossExam:    "CROSS-EXAMINATION",
		config.RoundFactCheck:    "FACT-CHECK INJECTION",
		config.RoundPositionSwap: "POSITION SWAP",
		config.RoundClosing:      "CLOSING STATEMENTS",
	}
	if name, ok := displayNames[rt]; ok {
		return name
	}
	return strings.ToUpper(string(rt))
}
