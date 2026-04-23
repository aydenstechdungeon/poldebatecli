package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Rounds.Count != 1 {
		t.Error("default rounds count should be 1")
	}
	if len(cfg.Rounds.Sequence) != 7 {
		t.Errorf("expected 7 rounds in sequence, got %d", len(cfg.Rounds.Sequence))
	}
	if len(cfg.Judges.Types) != 4 {
		t.Errorf("expected 4 judge types, got %d", len(cfg.Judges.Types))
	}
	if cfg.APIClient.BaseURL == "" {
		t.Error("default config missing API base URL")
	}
	if cfg.APIClient.Timeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", cfg.APIClient.Timeout)
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	cfg := DefaultConfig()

	ApplyFlagOverrides(cfg, "/custom/path", 3, true, "paragraph", "")

	if cfg.Output.Path != "/custom/path" {
		t.Errorf("output path not overridden, got %q", cfg.Output.Path)
	}
	if cfg.Rounds.Count != 3 {
		t.Errorf("rounds count not overridden, got %d", cfg.Rounds.Count)
	}
	if cfg.Output.Streaming {
		t.Error("streaming should be disabled with noStream=true")
	}
	if cfg.Output.StreamBlockSize != "paragraph" {
		t.Errorf("stream block size not overridden, got %q", cfg.Output.StreamBlockSize)
	}
}

func TestApplyFlagOverridesNoOverride(t *testing.T) {
	cfg := DefaultConfig()
	originalPath := cfg.Output.Path

	ApplyFlagOverrides(cfg, "", 0, false, "", "")

	if cfg.Output.Path != originalPath {
		t.Error("empty path should not override")
	}
	if cfg.Rounds.Count != 1 {
		t.Error("zero rounds should not override")
	}
}

func TestValidate(t *testing.T) {
	cfg := DefaultConfig()
	if err := Validate(cfg); err != nil {
		t.Errorf("default config should validate: %v", err)
	}
}

func TestLoadLegacyTeamsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.yaml")
	content := `teams:
  team_a:
    name: "Alpha"
    side: "for"
    position_description: "Supports proposition"
    agents:
      - id: "economist_a"
        role: "economist"
  team_b:
    name: "Beta"
    side: "against"
    position_description: "Opposes proposition"
    agents:
      - id: "economist_b"
        role: "economist"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	teams := cfg.Teams.EffectiveTeams()
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if teams[0].Name != "Alpha" || teams[1].Name != "Beta" {
		t.Fatalf("legacy team names not loaded: %+v", teams)
	}
}

func TestLoadTeamsListConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teams_list.yaml")
	content := `teams:
  list:
    - id: team_a
      name: "Alpha"
      side: "for"
      position_description: "For plan"
      agents:
        - id: "economist_a"
          role: "economist"
    - id: team_b
      name: "Beta"
      side: "against"
      position_description: "Against plan"
      agents:
        - id: "economist_b"
          role: "economist"
    - id: team_c
      name: "Gamma"
      side: "for"
      position_description: "Third stance"
      agents:
        - id: "economist_c"
          role: "economist"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	teams := cfg.Teams.EffectiveTeams()
	if len(teams) != 3 {
		t.Fatalf("expected 3 teams, got %d", len(teams))
	}
	if teams[2].ID != TeamID("team_c") {
		t.Fatalf("expected team_c, got %s", teams[2].ID)
	}
}

func TestValidateRequiresBothSides(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Teams.SetTeams([]TeamConfigWithID{
		{ID: TeamA, TeamConfig: TeamConfig{Name: "A", Side: "for", Agents: []AgentConfig{{ID: "economist_a", Role: RoleEconomist}}}},
		{ID: TeamB, TeamConfig: TeamConfig{Name: "B", Side: "for", Agents: []AgentConfig{{ID: "economist_b", Role: RoleEconomist}}}},
	})
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least one 'for' and one 'against'") {
		t.Fatalf("expected side validation error, got %v", err)
	}
}

func TestValidateRejectsUnsafeBaseURLByDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIClient.BaseURL = "http://localhost:8080"

	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("expected https validation error, got %v", err)
	}
}

func TestValidateAllowsUnsafeBaseURLWhenExplicitlyEnabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIClient.BaseURL = "http://localhost:8080"
	cfg.APIClient.AllowUnsafeBaseURL = true

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected unsafe base URL to pass when explicitly enabled: %v", err)
	}
}

func TestValidateTeamsAndRoundsUpperBounds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Teams.Count = MaxTeamsCount + 1
	cfg.Rounds.Count = MaxRoundsCount + 1

	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected bound validation errors")
	}
	if !strings.Contains(err.Error(), "teams.count must be <=") {
		t.Fatalf("expected teams.count bound error, got %v", err)
	}
	if !strings.Contains(err.Error(), "rounds.count must be <=") {
		t.Fatalf("expected rounds.count bound error, got %v", err)
	}
}

func TestTeamIDForIndexBeyondZ(t *testing.T) {
	if got := TeamIDForIndex(0); got != "team_a" {
		t.Fatalf("expected team_a, got %s", got)
	}
	if got := TeamIDForIndex(25); got != "team_z" {
		t.Fatalf("expected team_z, got %s", got)
	}
	if got := TeamIDForIndex(26); got != "team_aa" {
		t.Fatalf("expected team_aa, got %s", got)
	}
	if got := TeamIDForIndex(27); got != "team_ab" {
		t.Fatalf("expected team_ab, got %s", got)
	}
}

func TestGetAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	key := GetAPIKey(cfg)
	if key != "" {
		t.Error("API key should be empty when env var not set")
	}
}
