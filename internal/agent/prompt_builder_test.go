package agent

import (
	"strings"
	"testing"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/prompt"
)

func TestPromptBuilderWithoutEngine(t *testing.T) {
	pb := NewPromptBuilder()

	result := pb.Build(PromptParams{
		AgentID:   "economist_a",
		Role:      config.RoleEconomist,
		TeamName:  "Proponents",
		Side:      "for",
		Topic:     "UBI should be adopted",
		RoundName: config.RoundOpening,
	})

	if !strings.Contains(result, "economist_a") {
		t.Error("prompt missing agent ID")
	}
	if !strings.Contains(result, "Proponents") {
		t.Error("prompt missing team name")
	}
	if !strings.Contains(result, "opening statement") {
		t.Error("prompt missing round instruction")
	}
}

func TestPromptBuilderWithEngine(t *testing.T) {
	te, err := prompt.NewTemplateEngine(prompt.GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	pb := NewPromptBuilderWithEngine(te)

	result := pb.Build(PromptParams{
		AgentID:   "historian_a",
		Role:      config.RoleHistorian,
		TeamName:  "Proponents",
		Side:      "for",
		Topic:     "Test debate topic",
		RoundName: config.RoundSteelman,
	})

	if !strings.Contains(result, "historian_a") {
		t.Error("prompt missing agent ID")
	}
	if !strings.Contains(result, "charitably") {
		t.Error("steelman prompt missing steelman instruction")
	}
}

func TestPromptBuilderAutoOppositeSide(t *testing.T) {
	pb := NewPromptBuilder()

	params := PromptParams{
		AgentID:   "strategist_a",
		Role:      config.RoleStrategist,
		TeamName:  "Team A",
		Side:      "for",
		Topic:     "Test topic",
		RoundName: config.RoundPositionSwap,
	}

	result := pb.Build(params)
	if !strings.Contains(result, "against") {
		t.Error("position swap prompt should reference opposite side 'against'")
	}

	params.Side = "against"
	result = pb.Build(params)
	if !strings.Contains(result, "for") {
		t.Error("position swap prompt should reference opposite side 'for'")
	}
}

func TestPromptBuilderFactCheck(t *testing.T) {
	pb := NewPromptBuilder()

	result := pb.Build(PromptParams{
		AgentID:       "economist_a",
		Role:          config.RoleEconomist,
		TeamName:      "Team A",
		Side:          "for",
		Topic:         "Test topic",
		RoundName:     config.RoundFactCheck,
		FactCheckData: []string{"[economist_a, team_a]: Claim A (Evidence: Study X)", "[historian_b, team_b]: Claim B"},
	})

	if !strings.Contains(result, "[economist_a, team_a]: Claim A") {
		t.Error("fact check prompt missing claim data")
	}
	if !strings.Contains(result, "[historian_b, team_b]: Claim B") {
		t.Error("fact check prompt missing claim data")
	}
	if !strings.Contains(result, "CLAIMS RAISED") {
		t.Error("fact check prompt missing CLAIMS RAISED header")
	}
}

func TestPromptBuilderContextSummary(t *testing.T) {
	pb := NewPromptBuilder()

	result := pb.Build(PromptParams{
		AgentID:        "economist_a",
		Role:           config.RoleEconomist,
		TeamName:       "Team A",
		Side:           "for",
		Topic:          "Test topic",
		RoundName:      config.RoundRebuttal,
		ContextSummary: "Team A argued that UBI reduces poverty.",
	})

	if !strings.Contains(result, "Prior debate context") {
		t.Error("prompt missing context summary section")
	}
	if !strings.Contains(result, "UBI reduces poverty") {
		t.Error("prompt missing context summary content")
	}
}

func TestPromptBuilderAutoRoundDescription(t *testing.T) {
	pb := NewPromptBuilder()

	params := PromptParams{
		AgentID:   "economist_a",
		Role:      config.RoleEconomist,
		TeamName:  "Team A",
		Side:      "for",
		Topic:     "Test topic",
		RoundName: config.RoundCrossExam,
	}

	result := pb.Build(params)
	if !strings.Contains(result, "Pose sharp questions") {
		t.Error("prompt should contain auto-generated round description")
	}
}
