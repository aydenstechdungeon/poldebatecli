package engine

import (
	"context"
	"fmt"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/judge"
	"github.com/poldebatecli/internal/prompt"
	"github.com/poldebatecli/internal/round"
	"github.com/poldebatecli/internal/team"
)

type EngineDeps struct {
	Client        *client.OpenRouterClientImpl
	AgentFactory  *agent.Factory
	TeamBuilder   *team.Builder
	RoundRegistry *round.Registry
	JudgeRegistry *judge.Registry
	PromptBuilder *agent.PromptBuilder
}

func NewEngineDeps(cfg *config.Config) (*EngineDeps, error) {
	orClient := client.NewOpenRouterClient(cfg.APIClient)

	templateEngine, err := prompt.NewTemplateEngine(prompt.GetTemplatesFS())
	if err != nil {
		return nil, fmt.Errorf("init template engine: %w", err)
	}

	agentFactory := agent.NewFactory(cfg, orClient)
	teamBuilder := team.NewBuilder(cfg, agentFactory)
	roundRegistry := round.NewRegistry()
	judgeRegistry := judge.NewRegistry(cfg, orClient, templateEngine)
	promptBuilder := agent.NewPromptBuilderWithEngine(templateEngine)

	return &EngineDeps{
		Client:        orClient,
		AgentFactory:  agentFactory,
		TeamBuilder:   teamBuilder,
		RoundRegistry: roundRegistry,
		JudgeRegistry: judgeRegistry,
		PromptBuilder: promptBuilder,
	}, nil
}

type mockAgent struct {
	id     string
	teamID config.TeamID
	role   config.AgentRole
	model  string
}

func (m *mockAgent) ID() string             { return m.id }
func (m *mockAgent) TeamID() config.TeamID  { return m.teamID }
func (m *mockAgent) Role() config.AgentRole { return m.role }
func (m *mockAgent) Model() string          { return m.model }
func (m *mockAgent) Generate(_ context.Context, _ string, _ agent.GenerateOpts) (*agent.AgentResponse, error) {
	return &agent.AgentResponse{
		AgentID:    m.id,
		Content:    fmt.Sprintf("[Simulated response from %s (%s)] This is a mock argument from the %s perspective on the debate topic.", m.id, m.role, m.role),
		Model:      m.model,
		TokensUsed: 150,
	}, nil
}

func (m *mockAgent) StreamGenerate(_ context.Context, _ string, _ agent.GenerateOpts) (<-chan agent.AgentStreamChunk, error) {
	ch := make(chan agent.AgentStreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- agent.AgentStreamChunk{
			Content: fmt.Sprintf("[Simulated response from %s (%s)] This is a mock argument from the %s perspective on the debate topic.", m.id, m.role, m.role),
		}
		ch <- agent.AgentStreamChunk{Done: true, Model: m.model}
	}()
	return ch, nil
}

func buildMockAgents(tc config.TeamConfig, teamID config.TeamID) []agent.Agent {
	agents := make([]agent.Agent, 0, len(tc.Agents))
	for _, ac := range tc.Agents {
		agents = append(agents, &mockAgent{
			id:     ac.ID,
			teamID: teamID,
			role:   ac.Role,
			model:  "mock/model",
		})
	}
	return agents
}

func generateMockMessages(agentsMap map[config.TeamID][]agent.Agent, teamOrder []config.TeamID, roundType config.RoundType) []round.Message {
	var messages []round.Message
	for _, teamID := range teamOrder {
		for _, ag := range agentsMap[teamID] {
			messages = append(messages, round.Message{
				AgentID:    ag.ID(),
				Team:       ag.TeamID(),
				Role:       ag.Role(),
				Round:      roundType,
				Content:    fmt.Sprintf("[Simulated] %s argues from their %s expertise on this round.", ag.ID(), ag.Role()),
				Model:      ag.Model(),
				TokensUsed: 150,
			})
		}
	}
	return messages
}
