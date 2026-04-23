package planner

import (
	"testing"

	"github.com/poldebatecli/internal/config"
)

func TestRuleBasedPlannerDefaultTwoTeams(t *testing.T) {
	cfg := config.DefaultConfig()
	pl, err := NewRuleBasedPlanner().Plan("test", cfg)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(pl.Teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(pl.Teams))
	}
	if pl.Mode != "head_to_head" {
		t.Fatalf("expected head_to_head mode, got %s", pl.Mode)
	}
}

func TestRuleBasedPlannerExplicitThreeTeams(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Teams.Count = 3
	pl, err := NewRuleBasedPlanner().Plan("test", cfg)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if len(pl.Teams) != 3 {
		t.Fatalf("expected 3 teams, got %d", len(pl.Teams))
	}
	if pl.Mode != "free_for_all" {
		t.Fatalf("expected free_for_all mode, got %s", pl.Mode)
	}
}

func TestRuleBasedPlannerAddsPositionDescription(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Teams.TeamA.PositionDescription = ""
	cfg.Teams.TeamB.PositionDescription = ""
	pl, err := NewRuleBasedPlanner().Plan("test", cfg)
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	for _, tm := range pl.Teams {
		if tm.PositionDescription == "" {
			t.Fatalf("team %s missing position description", tm.ID)
		}
	}
}
