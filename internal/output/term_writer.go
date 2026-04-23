package output

import (
	"context"
	"fmt"
	"strings"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/engine"
	"github.com/poldebatecli/internal/judge"
)

type TermWriter struct{}

func NewTermWriter() *TermWriter {
	return &TermWriter{}
}

func (w *TermWriter) Write(ctx context.Context, result *engine.DebateResult, cfg *config.Config) error {
	if !cfg.Output.TerminalDisplay {
		return nil
	}

	var sb strings.Builder

	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("  DEBATE RESULTS\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "  Topic:    %s\n", result.Topic)

	winnerLabel := w.winnerLabel(result.Winner, cfg)
	fmt.Fprintf(&sb, "  Winner:   %s\n", winnerLabel)
	fmt.Fprintf(&sb, "  Duration: %s\n", result.Metadata.Duration)
	sb.WriteString("\n")

	sb.WriteString("  SCORES\n")
	sb.WriteString("  ─────────────────────────────────────────────────────────\n")
	for _, tc := range cfg.Teams.EffectiveTeams() {
		score, ok := result.Scores.Teams[tc.ID]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "  %s (%s)\n", config.TeamLabel(tc.ID), tc.Name)
		fmt.Fprintf(&sb, "    Logical Cons.:  %.1f\n", score.LogicalConsistency)
		fmt.Fprintf(&sb, "    Evidence:       %.1f\n", score.EvidenceQuality)
		fmt.Fprintf(&sb, "    Responsiveness: %.1f\n", score.Responsiveness)
		fmt.Fprintf(&sb, "    Strategic:      %.1f\n", score.StrategicStrength)
		fmt.Fprintf(&sb, "    TOTAL:          %.1f\n", score.Total)
	}
	sb.WriteString("\n")

	sb.WriteString("  ROUND BREAKDOWN\n")
	sb.WriteString("  ─────────────────────────────────────────────────────────\n")

	for _, rd := range result.Rounds {
		roundScores := w.findRoundScores(result.Judges, rd.Type)
		fmt.Fprintf(&sb, "  %s\n", w.roundDisplayName(rd.Type))
		for _, tc := range cfg.Teams.EffectiveTeams() {
			fmt.Fprintf(&sb, "    %-8s %.1f\n", config.TeamLabel(tc.ID), roundScores[tc.ID])
		}
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	if len(result.Degraded) > 0 {
		sb.WriteString("\n  DEGRADED RESPONSES\n")
		sb.WriteString("  ─────────────────────────────────────────────────────────\n")
		for _, d := range result.Degraded {
			fmt.Fprintf(&sb, "  Agent: %s | Round: %s | Model: %s → %s\n",
				d.AgentID, d.Round, d.OriginalModel, d.UsedModel)
			fmt.Fprintf(&sb, "  Reason: %s (retries: %d)\n", d.Reason, d.RetryCount)
		}
		sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	}

	fmt.Print(sb.String())
	return nil
}

func (w *TermWriter) winnerLabel(winner config.TeamID, cfg *config.Config) string {
	switch winner {
	case config.TeamA:
		return fmt.Sprintf("%s (Team A)", cfg.Teams.TeamA.Name)
	case config.TeamB:
		return fmt.Sprintf("%s (Team B)", cfg.Teams.TeamB.Name)
	case config.Tie:
		return "Tie"
	default:
		for _, tc := range cfg.Teams.EffectiveTeams() {
			if tc.ID == winner {
				return fmt.Sprintf("%s (%s)", tc.Name, config.TeamLabel(tc.ID))
			}
		}
		return string(winner)
	}
}

func (w *TermWriter) findRoundScores(judges []judge.JudgeResult, roundType config.RoundType) map[config.TeamID]float64 {
	totals := map[config.TeamID]float64{}
	counts := map[config.TeamID]int{}

	for _, j := range judges {
		if j.RoundType != roundType {
			continue
		}
		for teamID, score := range j.Scores.Teams {
			totals[teamID] += score.LogicalConsistency + score.EvidenceQuality + score.Responsiveness + score.StrategicStrength
			counts[teamID]++
		}
	}

	result := map[config.TeamID]float64{}
	for teamID, total := range totals {
		count := counts[teamID]
		if count > 0 {
			result[teamID] = total / float64(count) / 4
		}
	}
	return result
}

func (w *TermWriter) roundDisplayName(rt config.RoundType) string {
	displayNames := map[config.RoundType]string{
		config.RoundOpening:      "Opening",
		config.RoundSteelman:     "Steelman",
		config.RoundRebuttal:     "Rebuttal",
		config.RoundCrossExam:    "Cross-Exam",
		config.RoundFactCheck:    "Fact-Check",
		config.RoundPositionSwap: "Position Swap",
		config.RoundClosing:      "Closing",
	}
	if name, ok := displayNames[rt]; ok {
		return name
	}
	return string(rt)
}
