package agent

import (
	"context"
	"fmt"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
)

type Agent interface {
	ID() string
	TeamID() config.TeamID
	Role() config.AgentRole
	Model() string
	Generate(ctx context.Context, prompt string, opts GenerateOpts) (*AgentResponse, error)
	StreamGenerate(ctx context.Context, prompt string, opts GenerateOpts) (<-chan AgentStreamChunk, error)
}

type AgentResponse struct {
	AgentID    string `json:"agent_id"`
	Content    string `json:"content"`
	Model      string `json:"model"`
	TokensUsed int    `json:"tokens_used"`
	Degraded   bool   `json:"degraded"`
	Error      error  `json:"error,omitempty"`
}

type GenerateOpts struct {
	Temperature float64
	TopP        float64
	MaxTokens   int
	StreamBlock string
}

type AgentStreamChunk struct {
	Content    string
	Done       bool
	Model      string
	TokensUsed int
	Err        error
}

type debateAgent struct {
	id          string
	teamID      config.TeamID
	role        config.AgentRole
	model       string
	client      *client.OpenRouterClientImpl
	temperature float64
	topP        float64
	maxTokens   int
}

func (a *debateAgent) ID() string             { return a.id }
func (a *debateAgent) TeamID() config.TeamID  { return a.teamID }
func (a *debateAgent) Role() config.AgentRole { return a.role }
func (a *debateAgent) Model() string          { return a.model }

func (a *debateAgent) Generate(ctx context.Context, prompt string, opts GenerateOpts) (*AgentResponse, error) {
	temp := opts.Temperature
	if temp == 0 {
		temp = a.temperature
	}
	topP := opts.TopP
	if topP == 0 {
		topP = a.topP
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = a.maxTokens
	}

	resp, err := a.client.Complete(ctx, client.CompletionRequest{
		Model:       a.model,
		Messages:    []client.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: temp,
		TopP:        topP,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return &AgentResponse{
			AgentID: a.id,
			Error:   err,
		}, err
	}

	return &AgentResponse{
		AgentID:    a.id,
		Content:    resp.Content,
		Model:      resp.Model,
		TokensUsed: resp.TokensUsed,
	}, nil
}

func (a *debateAgent) StreamGenerate(ctx context.Context, prompt string, opts GenerateOpts) (<-chan AgentStreamChunk, error) {
	temp := opts.Temperature
	if temp == 0 {
		temp = a.temperature
	}
	topP := opts.TopP
	if topP == 0 {
		topP = a.topP
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = a.maxTokens
	}

	blockSize := client.BlockSize(opts.StreamBlock)
	if blockSize == "" {
		blockSize = client.BlockSentence
	}

	orchestrator := client.NewStreamOrchestrator(a.client, blockSize)
	result, err := orchestrator.StreamAgent(ctx, client.CompletionRequest{
		Model:       a.model,
		Messages:    []client.ChatMessage{{Role: "user", Content: prompt}},
		Temperature: temp,
		TopP:        topP,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return nil, err
	}

	out := make(chan AgentStreamChunk, 16)
	go func() {
		defer close(out)
		for chunk := range result.Stream {
			out <- AgentStreamChunk{
				Content:    chunk.Content,
				Done:       chunk.Done,
				Model:      chunk.Model,
				TokensUsed: chunk.TokensUsed,
				Err:        chunk.Err,
			}
		}
	}()

	return out, nil
}

type Factory struct {
	cfg    *config.Config
	client *client.OpenRouterClientImpl
}

func NewFactory(cfg *config.Config, c *client.OpenRouterClientImpl) *Factory {
	return &Factory{cfg: cfg, client: c}
}

func (f *Factory) Create(ac config.AgentConfig, teamID config.TeamID) (Agent, error) {
	model := ac.Model
	if model == "" {
		var ok bool
		model, ok = f.cfg.Models.Defaults[ac.Role]
		if !ok {
			return nil, fmt.Errorf("no default model for role %s", ac.Role)
		}
	}

	temp := f.cfg.Models.Temperatures[ac.Role]
	topP := f.cfg.Models.TopP[ac.Role]
	maxTokens := f.cfg.Models.MaxTokens[ac.Role]

	return &debateAgent{
		id:          ac.ID,
		teamID:      teamID,
		role:        ac.Role,
		model:       model,
		client:      f.client,
		temperature: temp,
		topP:        topP,
		maxTokens:   maxTokens,
	}, nil
}

func (f *Factory) CreateWithModel(ac config.AgentConfig, teamID config.TeamID, model string, temp float64, topP float64, maxTokens int) Agent {
	return &debateAgent{
		id:          ac.ID,
		teamID:      teamID,
		role:        ac.Role,
		model:       model,
		client:      f.client,
		temperature: temp,
		topP:        topP,
		maxTokens:   maxTokens,
	}
}
