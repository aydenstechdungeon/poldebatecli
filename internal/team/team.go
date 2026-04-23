package team

import (
	"fmt"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/config"
)

type Team struct {
	Name   string
	Side   string
	ID     config.TeamID
	Agents []agent.Agent
}

type Builder struct {
	cfg     *config.Config
	factory *agent.Factory
}

func NewBuilder(cfg *config.Config, factory *agent.Factory) *Builder {
	return &Builder{cfg: cfg, factory: factory}
}

func (b *Builder) BuildTeams() ([]*Team, error) {
	teamCfgs := b.cfg.Teams.EffectiveTeams()
	teams := make([]*Team, 0, len(teamCfgs))
	for _, tc := range teamCfgs {
		tm, err := b.buildTeam(tc.TeamConfig, tc.ID)
		if err != nil {
			return nil, fmt.Errorf("build %s: %w", tc.ID, err)
		}
		teams = append(teams, tm)
	}
	return teams, nil
}

func (b *Builder) buildTeam(tc config.TeamConfig, id config.TeamID) (*Team, error) {
	agents := make([]agent.Agent, 0, len(tc.Agents))
	for _, ac := range tc.Agents {
		a, err := b.factory.Create(ac, id)
		if err != nil {
			return nil, fmt.Errorf("create agent %s: %w", ac.ID, err)
		}
		agents = append(agents, a)
	}
	return &Team{
		Name:   tc.Name,
		Side:   tc.Side,
		ID:     id,
		Agents: agents,
	}, nil
}
