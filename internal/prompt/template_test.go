package prompt

import (
	"strings"
	"testing"
)

func TestNewTemplateEngine(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}
	if te == nil {
		t.Fatal("TemplateEngine is nil")
	}
	if te.templates == nil {
		t.Fatal("templates is nil")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildSystemPrompt(SystemPromptParams{
		AgentID:          "economist_a",
		Role:             "economist",
		RoleDescription:  "economic analysis, cost-benefit reasoning",
		TeamName:         "Proponents",
		Side:             "for",
		Topic:            "UBI should be adopted",
		RoundName:        "opening",
		RoundDescription: "Present your core thesis",
	})
	if err != nil {
		t.Fatalf("BuildSystemPrompt failed: %v", err)
	}

	if !strings.Contains(result, "economist_a") {
		t.Error("system prompt missing agent ID")
	}
	if !strings.Contains(result, "Proponents") {
		t.Error("system prompt missing team name")
	}
	if !strings.Contains(result, "UBI should be adopted") {
		t.Error("system prompt missing topic")
	}
	if !strings.Contains(result, "for") {
		t.Error("system prompt missing side")
	}
}

func TestBuildSystemPromptWithContextSummary(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildSystemPrompt(SystemPromptParams{
		AgentID:          "historian_a",
		Role:             "historian",
		RoleDescription:  "historical precedent",
		TeamName:         "Proponents",
		Side:             "for",
		Topic:            "Test topic",
		RoundName:        "rebuttal",
		RoundDescription: "Counter arguments",
		ContextSummary:   "Team A argued that UBI reduces poverty.",
	})
	if err != nil {
		t.Fatalf("BuildSystemPrompt failed: %v", err)
	}

	if !strings.Contains(result, "Prior debate context") {
		t.Error("system prompt missing context summary section")
	}
	if !strings.Contains(result, "UBI reduces poverty") {
		t.Error("system prompt missing context summary content")
	}
}

func TestBuildSystemPromptWithoutContextSummary(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildSystemPrompt(SystemPromptParams{
		AgentID:          "economist_a",
		Role:             "economist",
		RoleDescription:  "economic analysis",
		TeamName:         "Proponents",
		Side:             "for",
		Topic:            "Test topic",
		RoundName:        "opening",
		RoundDescription: "Present thesis",
	})
	if err != nil {
		t.Fatalf("BuildSystemPrompt failed: %v", err)
	}

	if strings.Contains(result, "Prior debate context") {
		t.Error("system prompt should not contain context summary when empty")
	}
}

func TestBuildOpeningPrompt(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildOpeningPrompt(RoundPromptParams{
		AgentID:          "economist_a",
		Role:             "economist",
		RoleDescription:  "economic analysis",
		TeamName:         "Proponents",
		Side:             "for",
		Topic:            "Test topic",
		RoundName:        "opening",
		RoundDescription: "Present thesis",
	})
	if err != nil {
		t.Fatalf("BuildOpeningPrompt failed: %v", err)
	}

	if !strings.Contains(result, "economist_a") {
		t.Error("opening prompt missing agent ID (system template not composed)")
	}
	if !strings.Contains(result, "opening statement") {
		t.Error("opening prompt missing round-specific instruction")
	}
}

func TestBuildRoundPromptByType(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	params := RoundPromptParams{
		AgentID:          "strategist_a",
		Role:             "strategist",
		RoleDescription:  "strategic positioning",
		TeamName:         "Proponents",
		Side:             "for",
		Topic:            "Test topic",
		RoundName:        "steelman",
		RoundDescription: "Reconstruct opponent's position",
	}

	result, err := te.BuildRoundPrompt("steelman", params)
	if err != nil {
		t.Fatalf("BuildRoundPrompt failed: %v", err)
	}

	if !strings.Contains(result, "strategist_a") {
		t.Error("steelman prompt missing agent ID")
	}
	if !strings.Contains(result, "charitably") {
		t.Error("steelman prompt missing steelman instruction")
	}
}

func TestBuildAllRoundPrompts(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	baseParams := RoundPromptParams{
		AgentID:          "test_agent",
		Role:             "economist",
		RoleDescription:  "economic analysis",
		TeamName:         "Team A",
		Side:             "for",
		Topic:            "Test topic",
		RoundName:        "opening",
		RoundDescription: "Test description",
		OppositeSide:     "against",
		FactCheckData:    []string{"Fact one", "Fact two"},
	}

	builders := []struct {
		name     string
		build    func(RoundPromptParams) (string, error)
		contains string
	}{
		{"opening", te.BuildOpeningPrompt, "opening statement"},
		{"steelman", te.BuildSteelmanPrompt, "charitably"},
		{"rebuttal", te.BuildRebuttalPrompt, "Rebut"},
		{"cross_exam", te.BuildCrossExamPrompt, "Cross-examine"},
		{"fact_check", te.BuildFactCheckPrompt, "CLAIMS RAISED"},
		{"position_swap", te.BuildPositionSwapPrompt, "POSITION SWAP"},
		{"closing", te.BuildClosingPrompt, "closing statement"},
	}

	for _, b := range builders {
		result, err := b.build(baseParams)
		if err != nil {
			t.Errorf("%s prompt failed: %v", b.name, err)
			continue
		}
		if !strings.Contains(result, "test_agent") {
			t.Errorf("%s prompt missing agent ID (system template not composed)", b.name)
		}
		if !strings.Contains(result, b.contains) {
			t.Errorf("%s prompt missing expected content %q", b.name, b.contains)
		}
	}
}

func TestBuildFactCheckPromptWithClaims(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildFactCheckPrompt(RoundPromptParams{
		AgentID:         "economist_a",
		Role:            "economist",
		RoleDescription: "economic analysis",
		TeamName:        "Proponents",
		Side:            "for",
		Topic:           "Test topic",
		RoundName:       "fact_check",
		FactCheckData:   []string{"[economist_a, team_a]: UBI reduces poverty (Evidence: World Bank study)", "[historian_b, team_b]: UBI causes inflation"},
	})
	if err != nil {
		t.Fatalf("BuildFactCheckPrompt failed: %v", err)
	}

	if !strings.Contains(result, "[economist_a, team_a]: UBI reduces poverty") {
		t.Error("fact check prompt missing claim data")
	}
	if !strings.Contains(result, "[historian_b, team_b]: UBI causes inflation") {
		t.Error("fact check prompt missing claim data")
	}
	if !strings.Contains(result, "CLAIMS RAISED") {
		t.Error("fact check prompt missing CLAIMS RAISED header")
	}
}

func TestBuildPositionSwapPrompt(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	result, err := te.BuildPositionSwapPrompt(RoundPromptParams{
		AgentID:         "economist_a",
		Role:            "economist",
		RoleDescription: "economic analysis",
		TeamName:        "Proponents",
		Side:            "for",
		Topic:           "Test topic",
		RoundName:       "position_swap",
		OppositeSide:    "against",
	})
	if err != nil {
		t.Fatalf("BuildPositionSwapPrompt failed: %v", err)
	}

	if !strings.Contains(result, "against") {
		t.Error("position swap prompt missing opposite side")
	}
}

func TestBuildJudgePrompts(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	params := JudgePromptParams{
		RoundName:            "opening",
		Topic:                "Test topic",
		TeamSections:         "Team A argued X\n\nTeam B argued Y",
		ContradictionPenalty: 2.0,
	}

	builders := []struct {
		name     string
		build    func(JudgePromptParams) (string, error)
		contains string
	}{
		{"logic", te.BuildJudgeLogicPrompt, "Logic Judge"},
		{"evidence", te.BuildJudgeEvidencePrompt, "Evidence Judge"},
		{"clarity", te.BuildJudgeClarityPrompt, "Clarity Judge"},
		{"adversarial", te.BuildJudgeAdversarialPrompt, "ADVERSARIAL Judge"},
	}

	for _, b := range builders {
		result, err := b.build(params)
		if err != nil {
			t.Errorf("%s judge prompt failed: %v", b.name, err)
			continue
		}
		if !strings.Contains(result, b.contains) {
			t.Errorf("%s judge prompt missing %q", b.name, b.contains)
		}
		if !strings.Contains(result, "Team A argued X") {
			t.Errorf("%s judge prompt missing team A messages", b.name)
		}
	}
}

func TestBuildJudgePromptByType(t *testing.T) {
	te, err := NewTemplateEngine(GetTemplatesFS())
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	params := JudgePromptParams{
		RoundName:    "rebuttal",
		Topic:        "Test topic",
		TeamSections: "A argued\n\nB argued",
	}

	result, err := te.BuildJudgePrompt("logic", params)
	if err != nil {
		t.Fatalf("BuildJudgePrompt failed: %v", err)
	}
	if !strings.Contains(result, "Logic Judge") {
		t.Error("logic judge prompt missing judge type identifier")
	}

	_, err = te.BuildJudgePrompt("nonexistent", params)
	if err == nil {
		t.Fatal("BuildJudgePrompt for unknown type should return an error")
	}
}

func TestRoundTmplName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"opening", "opening.tmpl"},
		{"steelman", "steelman.tmpl"},
		{"rebuttal", "rebuttal.tmpl"},
		{"cross_examination", "cross_exam.tmpl"},
		{"fact_check", "fact_check.tmpl"},
		{"position_swap", "position_swap.tmpl"},
		{"closing", "closing.tmpl"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := roundTmplName(tt.input)
		if result != tt.expected {
			t.Errorf("roundTmplName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestJudgeTmplName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"logic", "judge_logic.tmpl"},
		{"evidence", "judge_evidence.tmpl"},
		{"clarity", "judge_clarity.tmpl"},
		{"adversarial", "judge_adversarial.tmpl"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := judgeTmplName(tt.input)
		if result != tt.expected {
			t.Errorf("judgeTmplName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
