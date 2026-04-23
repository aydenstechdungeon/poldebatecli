package planner

import (
	"fmt"
	"strings"

	"github.com/poldebatecli/internal/config"
)

type TeamPlanner interface {
	Plan(topic string, cfg *config.Config) (*TeamPlan, error)
}

type TeamPlan struct {
	Topic             string
	Mode              string
	ExplicitMultiTeam bool
	Teams             []config.TeamConfigWithID
}

type RuleBasedPlanner struct{}

func NewRuleBasedPlanner() *RuleBasedPlanner {
	return &RuleBasedPlanner{}
}

func (p *RuleBasedPlanner) Plan(topic string, cfg *config.Config) (*TeamPlan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	base := cfg.Teams.EffectiveTeams()
	if len(base) == 0 {
		base = config.DefaultConfig().Teams.EffectiveTeams()
	}

	targetCount := 2
	explicitMulti := false
	if cfg.Teams.Count > 0 {
		targetCount = cfg.Teams.Count
		explicitMulti = targetCount > 2
	}
	if len(cfg.Teams.List) > 0 {
		if cfg.Teams.Count == 0 {
			targetCount = len(cfg.Teams.List)
		}
		explicitMulti = explicitMulti || len(cfg.Teams.List) > 2
	}
	if targetCount < 2 {
		targetCount = 2
	}
	if targetCount > config.MaxTeamsCount {
		return nil, fmt.Errorf("teams.count must be <= %d", config.MaxTeamsCount)
	}

	planned := make([]config.TeamConfigWithID, 0, targetCount)
	for i := 0; i < targetCount; i++ {
		var team config.TeamConfigWithID
		if i < len(base) {
			team = base[i]
		} else {
			seed := base[i%len(base)]
			team = seed
		}

		team.ID = config.TeamIDForIndex(i)
		if strings.TrimSpace(team.Name) == "" || i >= len(base) {
			team.Name = config.TeamLabel(team.ID)
		}
		if strings.TrimSpace(team.Side) == "" || i >= len(base) {
			if i%2 == 0 {
				team.Side = "for"
			} else {
				team.Side = "against"
			}
		}
		if strings.TrimSpace(team.PositionDescription) == "" {
			if team.Side == "for" {
				team.PositionDescription = "Defend the proposition with concrete mechanisms, evidence, and practical tradeoff analysis."
			} else {
				team.PositionDescription = "Challenge the proposition by stress-testing assumptions, costs, and second-order effects."
			}
		}

		for aIdx := range team.Agents {
			if i < len(base) && strings.TrimSpace(team.Agents[aIdx].ID) != "" {
				continue
			}
			suffix := rune('a' + i)
			if suffix > 'z' {
				suffix = 'z'
			}
			team.Agents[aIdx].ID = fmt.Sprintf("%s_%c", team.Agents[aIdx].Role, suffix)
		}
		planned = append(planned, team)
	}

	ensureSideCoverage(planned)

	mode := "head_to_head"
	if len(planned) > 2 {
		mode = "free_for_all"
	}

	cfg.Teams.Count = len(planned)
	cfg.Teams.SetTeams(planned)

	return &TeamPlan{
		Topic:             topic,
		Mode:              mode,
		ExplicitMultiTeam: explicitMulti,
		Teams:             planned,
	}, nil
}

func ensureSideCoverage(teams []config.TeamConfigWithID) {
	if len(teams) < 2 {
		return
	}
	var forIdx, againstIdx = -1, -1
	for i, t := range teams {
		if t.Side == "for" && forIdx == -1 {
			forIdx = i
		}
		if t.Side == "against" && againstIdx == -1 {
			againstIdx = i
		}
	}
	if forIdx == -1 {
		teams[0].Side = "for"
	}
	if againstIdx == -1 {
		teams[1].Side = "against"
	}
}
