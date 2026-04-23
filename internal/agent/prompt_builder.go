package agent

import (
	"strings"

	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/prompt"
)

var RoleDescriptions = map[config.AgentRole]string{
	config.RoleEconomist:  "economic analysis, cost-benefit reasoning, market dynamics, fiscal impact, resource allocation",
	config.RoleHistorian:  "historical precedent, longitudinal patterns, institutional evolution, path dependency, civilizational outcomes",
	config.RoleStrategist: "strategic positioning, coalition dynamics, implementation feasibility, adversarial analysis, second-order effects",
}

type PromptBuilder struct {
	templateEngine *prompt.TemplateEngine
}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func NewPromptBuilderWithEngine(te *prompt.TemplateEngine) *PromptBuilder {
	return &PromptBuilder{templateEngine: te}
}

type PromptParams struct {
	AgentID             string
	Role                config.AgentRole
	TeamName            string
	Side                string
	PositionDescription string
	Topic               string
	RoundName           config.RoundType
	RoundDescription    string
	ContextSummary      string
	FactCheckData       []string
	OppositeSide        string
}

func roundDescription(rt config.RoundType) string {
	switch rt {
	case config.RoundOpening:
		return "Present your core thesis and key arguments"
	case config.RoundSteelman:
		return "Reconstruct the strongest version of your opponent's position"
	case config.RoundRebuttal:
		return "Counter opposing arguments with evidence and reasoning"
	case config.RoundCrossExam:
		return "Pose sharp questions exposing weaknesses in opposing reasoning"
	case config.RoundFactCheck:
		return "Verify claims from prior rounds and assess their implications"
	case config.RoundPositionSwap:
		return "Argue the opposing side to test intellectual honesty"
	case config.RoundClosing:
		return "Synthesize strongest arguments into a decisive conclusion"
	default:
		return ""
	}
}

func (pb *PromptBuilder) Build(params PromptParams) string {
	roleDesc := RoleDescriptions[params.Role]
	if roleDesc == "" {
		roleDesc = "general debate perspective"
	}
	if params.RoundDescription == "" {
		params.RoundDescription = roundDescription(params.RoundName)
	}
	if params.OppositeSide == "" {
		if params.Side == "for" {
			params.OppositeSide = "against"
		} else {
			params.OppositeSide = "for"
		}
	}

	if pb.templateEngine != nil {
		if result, err := pb.buildFromTemplate(params, roleDesc); err == nil {
			return result
		}
	}

	return buildFromStrings(params, roleDesc)
}

func (pb *PromptBuilder) buildFromTemplate(params PromptParams, roleDesc string) (string, error) {
	rpp := prompt.RoundPromptParams{
		AgentID:             params.AgentID,
		Role:                string(params.Role),
		RoleDescription:     roleDesc,
		TeamName:            params.TeamName,
		Side:                params.Side,
		PositionDescription: params.PositionDescription,
		Topic:               params.Topic,
		RoundName:           string(params.RoundName),
		RoundDescription:    params.RoundDescription,
		ContextSummary:      params.ContextSummary,
		OppositeSide:        params.OppositeSide,
		FactCheckData:       params.FactCheckData,
	}

	result, err := pb.templateEngine.BuildRoundPrompt(string(params.RoundName), rpp)
	if err != nil {
		return "", err
	}
	return result, nil
}

func buildFromStrings(params PromptParams, roleDesc string) string {
	var sb strings.Builder
	sb.WriteString(buildSystemPrompt(params, roleDesc))
	sb.WriteString(buildRoundPrompt(params))
	return sb.String()
}

// BuildFallbackPrompt creates a string-based prompt when template engine is unavailable
func BuildFallbackPrompt(params PromptParams) string {
	if params.RoundDescription == "" {
		params.RoundDescription = roundDescription(params.RoundName)
	}
	if params.OppositeSide == "" {
		if params.Side == "for" {
			params.OppositeSide = "against"
		} else {
			params.OppositeSide = "for"
		}
	}
	roleDesc := RoleDescriptions[params.Role]
	if roleDesc == "" {
		roleDesc = "general debate perspective"
	}
	var sb strings.Builder
	sb.WriteString(buildSystemPrompt(params, roleDesc))
	sb.WriteString(buildRoundPrompt(params))
	return sb.String()
}

func buildSystemPrompt(params PromptParams, roleDesc string) string {
	p := "You are " + sanitizePromptField(params.AgentID) + ", a " + sanitizePromptField(string(params.Role)) + " on the " + sanitizePromptField(params.TeamName) + " team arguing " + sanitizePromptField(params.Side) + " the proposition: \"" + sanitizePromptField(params.Topic) + "\".\n\n"
	p += "Your expertise: " + roleDesc + "\n"
	p += "Your team: " + sanitizePromptField(params.TeamName) + "\n"
	p += "Your position: " + sanitizePromptField(params.Side) + "\n\n"
	if params.PositionDescription != "" {
		p += "Position detail: " + sanitizePromptField(params.PositionDescription) + "\n\n"
	}
	p += "Rules:\n"
	p += "- Argue ONLY from your " + string(params.Role) + " perspective.\n"
	p += "- Never concede your team's core position.\n"
	p += "- Reference specific evidence, data, or historical precedent.\n"
	p += "- Structure arguments with clear claims, evidence, and reasoning.\n"
	p += "- Do not repeat arguments already made by your teammates.\n"
	p += "- Current round: " + string(params.RoundName) + " (" + params.RoundDescription + ")\n"
	if params.ContextSummary != "" {
		p += "\nPrior debate context (summary):\n" + params.ContextSummary + "\n"
	}
	return p
}

// sanitizePromptField removes potentially problematic characters from user-controlled
// fields that are interpolated into LLM prompts. This prevents prompt injection
// attacks where crafted topic names or agent IDs could manipulate judge scoring.
func sanitizePromptField(s string) string {
	// Remove null bytes and control characters except newlines and tabs
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' {
			sb.WriteRune(r)
			continue
		}
		if r < 32 {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func buildRoundPrompt(params PromptParams) string {
	switch params.RoundName {
	case config.RoundOpening:
		return "\n\nDeliver your opening statement. Establish your core thesis from your " + string(params.Role) + " perspective.\nPresent 2-3 key arguments with evidence. Set the strategic frame for the debate.\n"
	case config.RoundSteelman:
		return "\n\nReconstruct the strongest possible version of your opponent's arguments.\nYou must present their position as charitably as possible before identifying\nwhere it breaks down. Show you understand before you counter.\n"
	case config.RoundRebuttal:
		return "\n\nRebut the opposing team's arguments from your " + string(params.Role) + " perspective.\nAddress specific claims made. Identify logical flaws, unsupported assertions,\nor evidence gaps. Strengthen your team's position.\n"
	case config.RoundCrossExam:
		return "\n\nCross-examine the opposing team. Pose sharp questions that expose weaknesses\nin their reasoning. Challenge their evidence, assumptions, and conclusions.\nFormat: Present 3-5 questions, then anticipate and counter likely responses.\n"
	case config.RoundFactCheck:
		p := "\n\nCLAIMS RAISED IN PRIOR ROUNDS:\n"
		for _, fact := range params.FactCheckData {
			p += "- " + fact + "\n"
		}
		p += "\nVerify these claims. Do they support or undermine your position?\nYou must: (1) acknowledge facts that contradict your stance, (2) explain how\nthey fit or don't fit your framework, (3) update your argument if necessary.\nHonest engagement with contradictory evidence is scored positively.\n"
		return p
	case config.RoundPositionSwap:
		return "\n\nPOSITION SWAP: You must now argue the OPPOSING side — " + params.OppositeSide + " the proposition.\n\nArgue against your original position as convincingly as possible.\nThis tests intellectual honesty and argument depth. Use your knowledge\nof the opposing side's strongest points. References to your earlier\narguments as points to overcome are encouraged.\n"
	case config.RoundClosing:
		return "\n\nDeliver your closing statement. Synthesize your team's strongest arguments.\nAddress the strongest counter-arguments raised. End with a decisive conclusion.\nDo not introduce new evidence — only reference what has been established.\n"
	default:
		return ""
	}
}
