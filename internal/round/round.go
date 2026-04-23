package round

import (
	"context"
	"sort"
	"time"

	"github.com/poldebatecli/internal/agent"
	"github.com/poldebatecli/internal/config"
)

type TeamMeta struct {
	Name                string
	Side                string
	PositionDescription string
}

type DebateState struct {
	Topic          string
	TeamMeta       map[config.TeamID]TeamMeta
	TeamOrder      []config.TeamID
	ContextSummary string
	AllMessages    []Message
	KeyArguments   []KeyArg
	RoundResults   []RoundResult
	JudgeResults   []any

	TeamAgents       map[config.TeamID][]agent.Agent
	PromptBuilder    *agent.PromptBuilder
	OnAgentError     AgentErrorHandler
	OnTeamComplete   TeamCompleteHandler
	LiveEvents       chan<- LiveEvent
	StreamingEnabled bool
	StreamBlockSize  string
}

type AgentErrorHandler func(ctx context.Context, err error, ag agent.Agent, prompt string, opts agent.GenerateOpts) (*agent.AgentResponse, error)
type TeamCompleteHandler func(roundType config.RoundType, teamID config.TeamID)

func NewDebateState(topic string) *DebateState {
	return &DebateState{
		Topic: topic,
	}
}

func (s *DebateState) AddRound(r *RoundResult) {
	if r != nil {
		s.RoundResults = append(s.RoundResults, *r)
	}
}

func (s *DebateState) AddMessages(msgs []Message) {
	s.AllMessages = append(s.AllMessages, msgs...)
}

type RoundResult struct {
	Type     config.RoundType `json:"type"`
	Messages []Message        `json:"messages"`
	Duration time.Duration    `json:"duration"`
	Errors   []RoundError     `json:"errors,omitempty"`
}

type Message struct {
	AgentID    string           `json:"agent_id"`
	Team       config.TeamID    `json:"team"`
	Role       config.AgentRole `json:"role"`
	Round      config.RoundType `json:"round"`
	Content    string           `json:"content"`
	Model      string           `json:"model"`
	TokensUsed int              `json:"tokens_used"`
	Degraded   bool             `json:"degraded"`
}

type RoundError struct {
	Agent string `json:"agent"`
	Error string `json:"error"`
}

type LiveEventType string

const (
	LiveEventRoundStart LiveEventType = "round_start"
	LiveEventAgentStart LiveEventType = "agent_start"
	LiveEventAgentChunk LiveEventType = "agent_chunk"
	LiveEventAgentDone  LiveEventType = "agent_done"
	LiveEventTeamDone   LiveEventType = "team_done"
	LiveEventRoundDone  LiveEventType = "round_done"
)

type LiveEvent struct {
	Type      LiveEventType
	RoundType config.RoundType
	AgentID   string
	TeamID    config.TeamID
	TeamName  string
	Content   string
	Model     string
	Failed    bool
}

type KeyArg struct {
	AgentID  string
	Team     config.TeamID
	Round    config.RoundType
	Claim    string
	Evidence string
}

type Round interface {
	Name() config.RoundType
	Execute(ctx context.Context, state *DebateState) (*RoundResult, error)
	PromptTemplates() []string
	RequiresJudge() bool
}

type Registry struct {
	rounds map[config.RoundType]Round
}

func NewRegistry() *Registry {
	r := &Registry{
		rounds: make(map[config.RoundType]Round),
	}
	r.Register(&OpeningRound{BaseRound{RoundType: config.RoundOpening, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "opening.tmpl"}}})
	r.Register(&SteelmanRound{BaseRound{RoundType: config.RoundSteelman, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "steelman.tmpl"}}})
	r.Register(&RebuttalRound{BaseRound{RoundType: config.RoundRebuttal, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "rebuttal.tmpl"}}})
	r.Register(&CrossExamRound{BaseRound{RoundType: config.RoundCrossExam, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "cross_exam.tmpl"}}})
	r.Register(&FactCheckRound{BaseRound{RoundType: config.RoundFactCheck, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "fact_check.tmpl"}}})
	r.Register(&PositionSwapRound{BaseRound{RoundType: config.RoundPositionSwap, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "position_swap.tmpl"}}})
	r.Register(&ClosingRound{BaseRound{RoundType: config.RoundClosing, RequiresJudgeFlag: true, Templates: []string{"system.tmpl", "closing.tmpl"}}})
	return r
}

func (r *Registry) Register(round Round) {
	r.rounds[round.Name()] = round
}

func (r *Registry) Get(rt config.RoundType) (Round, bool) {
	round, ok := r.rounds[rt]
	return round, ok
}

func (r *Registry) All() []Round {
	roundTypes := make([]config.RoundType, 0, len(r.rounds))
	for rt := range r.rounds {
		roundTypes = append(roundTypes, rt)
	}
	sort.Slice(roundTypes, func(i, j int) bool {
		return roundTypes[i] < roundTypes[j]
	})

	result := make([]Round, 0, len(roundTypes))
	for _, rt := range roundTypes {
		result = append(result, r.rounds[rt])
	}
	return result
}
