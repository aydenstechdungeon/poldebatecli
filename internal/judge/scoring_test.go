package judge

import (
	"testing"

	"github.com/poldebatecli/internal/config"
)

func TestAggregateScoresMultiTeam(t *testing.T) {
	results := []JudgeResult{
		{
			Scores: TeamScores{Teams: map[config.TeamID]ScoreBreakdown{
				config.TeamA:            {LogicalConsistency: 8, EvidenceQuality: 8, Responsiveness: 8, StrategicStrength: 8},
				config.TeamB:            {LogicalConsistency: 7, EvidenceQuality: 7, Responsiveness: 7, StrategicStrength: 7},
				config.TeamID("team_c"): {LogicalConsistency: 9, EvidenceQuality: 9, Responsiveness: 9, StrategicStrength: 9},
			}},
		},
	}
	agg := AggregateScores(results)
	if len(agg.Teams) != 3 {
		t.Fatalf("expected 3 teams, got %d", len(agg.Teams))
	}
	if winner := DetermineWinner(agg); winner != config.TeamID("team_c") {
		t.Fatalf("expected team_c winner, got %s", winner)
	}
}

func TestDetermineWinnerTie(t *testing.T) {
	agg := AggregatedScores{Teams: map[config.TeamID]TeamAggregated{
		config.TeamA: {Total: 7.0},
		config.TeamB: {Total: 7.0},
	}}
	if winner := DetermineWinner(agg); winner != config.Tie {
		t.Fatalf("expected tie winner, got %s", winner)
	}
}
