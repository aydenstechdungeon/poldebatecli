package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Models    ModelsConfig      `yaml:"models" json:"models"`
	Teams     TeamsConfig       `yaml:"teams" json:"teams"`
	Rounds    RoundsConfig      `yaml:"rounds" json:"rounds"`
	Judges    JudgesConfig      `yaml:"judges" json:"judges"`
	Output    OutputConfig      `yaml:"output" json:"output"`
	APIClient APIConfig         `yaml:"api_client" json:"api_client"`
	Logging   LoggingConfig     `yaml:"logging" json:"logging"`
	Session   SessionYAMLConfig `yaml:"session" json:"session"`
}

type SessionYAMLConfig struct {
	SaveInterval          time.Duration `yaml:"save_interval" json:"save_interval"`
	CheckpointMinInterval time.Duration `yaml:"checkpoint_min_interval" json:"checkpoint_min_interval"`
}

type ModelsConfig struct {
	Defaults     map[AgentRole]string  `yaml:"defaults" json:"defaults"`
	Fallbacks    map[string]string     `yaml:"fallbacks" json:"fallbacks"`
	Temperatures map[AgentRole]float64 `yaml:"temperatures" json:"temperatures"`
	MaxTokens    map[AgentRole]int     `yaml:"max_tokens" json:"max_tokens"`
	TopP         map[AgentRole]float64 `yaml:"top_p" json:"top_p"`
	ContextModel string                `yaml:"context_model,omitempty" json:"context_model,omitempty"`
}

type TeamsConfig struct {
	List  []TeamConfigWithID `yaml:"list,omitempty" json:"list,omitempty"`
	Count int                `yaml:"count,omitempty" json:"count,omitempty"`
	TeamA TeamConfig         `yaml:"team_a,omitempty" json:"team_a,omitempty"`
	TeamB TeamConfig         `yaml:"team_b,omitempty" json:"team_b,omitempty"`
}

type TeamConfig struct {
	Name                string        `yaml:"name" json:"name"`
	Agents              []AgentConfig `yaml:"agents" json:"agents"`
	Side                string        `yaml:"side" json:"side"`
	PositionDescription string        `yaml:"position_description,omitempty" json:"position_description,omitempty"`
}

type TeamConfigWithID struct {
	ID         TeamID `yaml:"id" json:"id"`
	TeamConfig `yaml:",inline" json:",inline"`
}

type AgentConfig struct {
	ID    string    `yaml:"id" json:"id"`
	Role  AgentRole `yaml:"role" json:"role"`
	Model string    `yaml:"model,omitempty" json:"model,omitempty"`
}

type RoundsConfig struct {
	Count    int                         `yaml:"count" json:"count"`
	Sequence []RoundType                 `yaml:"sequence" json:"sequence"`
	Timeouts map[RoundType]time.Duration `yaml:"timeouts" json:"timeouts"`
}

type APIConfig struct {
	BaseURL            string        `yaml:"base_url" json:"base_url"`
	APIKeyEnvVar       string        `yaml:"api_key_env_var" json:"api_key_env_var"`
	Timeout            time.Duration `yaml:"timeout" json:"timeout"`
	MaxRetries         int           `yaml:"max_retries" json:"max_retries"`
	RetryBaseDelay     time.Duration `yaml:"retry_base_delay" json:"retry_base_delay"`
	RateLimitRPS       float64       `yaml:"rate_limit_rps" json:"rate_limit_rps"`
	AllowUnsafeBaseURL bool          `yaml:"allow_unsafe_base_url,omitempty" json:"allow_unsafe_base_url,omitempty"`
}

type JudgesConfig struct {
	Types             []JudgeType           `yaml:"types" json:"types"`
	Models            map[JudgeType]string  `yaml:"models" json:"models"`
	Fallbacks         map[string]string     `yaml:"fallbacks" json:"fallbacks"`
	Temperatures      map[JudgeType]float64 `yaml:"temperatures" json:"temperatures"`
	AdversarialConfig AdversarialConfig     `yaml:"adversarial_config" json:"adversarial_config"`
}

type AdversarialConfig struct {
	BiasThreshold        float64 `yaml:"bias_threshold" json:"bias_threshold"`
	ContradictionPenalty float64 `yaml:"contradiction_penalty" json:"contradiction_penalty"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

type OutputConfig struct {
	Format          string `yaml:"format" json:"format"`
	Path            string `yaml:"path" json:"path"`
	Transcript      bool   `yaml:"transcript" json:"transcript"`
	TerminalDisplay bool   `yaml:"terminal_display" json:"terminal_display"`
	Streaming       bool   `yaml:"streaming" json:"streaming"`
	StreamBlockSize string `yaml:"stream_block_size" json:"stream_block_size"`
	SkipJudges      bool   `yaml:"-" json:"-"`
}

type TeamID string

const (
	TeamA TeamID = "team_a"
	TeamB TeamID = "team_b"
	Tie   TeamID = "tie"
)

func TeamIDForIndex(idx int) TeamID {
	if idx < 0 {
		return TeamA
	}
	return TeamID(fmt.Sprintf("team_%s", teamIndexLabel(idx)))
}

func TeamLabel(id TeamID) string {
	s := string(id)
	if strings.HasPrefix(s, "team_") {
		suffix := s[len("team_"):]
		if isLowerAlpha(suffix) {
			return "Team " + strings.ToUpper(suffix)
		}
	}
	return string(id)
}

func teamIndexLabel(idx int) string {
	n := idx
	out := ""
	for {
		out = string(rune('a'+(n%26))) + out
		n = n/26 - 1
		if n < 0 {
			return out
		}
	}
}

func isLowerAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func (t *TeamsConfig) EffectiveTeams() []TeamConfigWithID {
	if len(t.List) > 0 {
		out := make([]TeamConfigWithID, 0, len(t.List))
		for i, team := range t.List {
			if team.ID == "" {
				team.ID = TeamIDForIndex(i)
			}
			out = append(out, team)
		}
		return out
	}
	return []TeamConfigWithID{
		{ID: TeamA, TeamConfig: t.TeamA},
		{ID: TeamB, TeamConfig: t.TeamB},
	}
}

func (t *TeamsConfig) SetTeams(teams []TeamConfigWithID) {
	t.List = make([]TeamConfigWithID, len(teams))
	copy(t.List, teams)
	if len(teams) > 0 {
		t.TeamA = teams[0].TeamConfig
	}
	if len(teams) > 1 {
		t.TeamB = teams[1].TeamConfig
	}
}

type AgentRole string

const (
	RoleEconomist  AgentRole = "economist"
	RoleHistorian  AgentRole = "historian"
	RoleStrategist AgentRole = "strategist"
)

type RoundType string

const (
	RoundOpening      RoundType = "opening"
	RoundSteelman     RoundType = "steelman"
	RoundRebuttal     RoundType = "rebuttal"
	RoundCrossExam    RoundType = "cross_examination"
	RoundFactCheck    RoundType = "fact_check"
	RoundPositionSwap RoundType = "position_swap"
	RoundClosing      RoundType = "closing"
)

type JudgeType string

const (
	JudgeLogic       JudgeType = "logic"
	JudgeEvidence    JudgeType = "evidence"
	JudgeClarity     JudgeType = "clarity"
	JudgeAdversarial JudgeType = "adversarial"
)
