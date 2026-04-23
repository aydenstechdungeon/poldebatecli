package judge

import (
	"testing"

	"github.com/poldebatecli/internal/config"
)

func TestParseJudgeScoresNewSchema(t *testing.T) {
	input := `{"teams":{"team_a":{"logical_consistency":7,"evidence_quality":8,"responsiveness":7,"strategic_strength":6},"team_b":{"logical_consistency":6,"evidence_quality":6,"responsiveness":7,"strategic_strength":7}}}`
	scores, err := parseJudgeScores(input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(scores.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(scores.Teams))
	}
}

func TestParseJudgeScoresLegacySchema(t *testing.T) {
	input := `{"team_a":{"logical_consistency":7,"evidence_quality":8,"responsiveness":7,"strategic_strength":6},"team_b":{"logical_consistency":6,"evidence_quality":6,"responsiveness":7,"strategic_strength":7}}`
	scores, err := parseJudgeScores(input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if _, ok := scores.Teams[config.TeamA]; !ok {
		t.Fatal("expected team_a in parsed scores")
	}
}
