package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/poldebatecli/internal/client"
	"github.com/poldebatecli/internal/config"
	"github.com/poldebatecli/internal/prompt"
	"github.com/poldebatecli/internal/round"
)

type Judge interface {
	Type() config.JudgeType
	Evaluate(ctx context.Context, roundResult round.RoundResult, state *round.DebateState) (*JudgeResult, error)
}

type JudgeResult struct {
	JudgeType  config.JudgeType `json:"judge_type"`
	RoundType  config.RoundType `json:"round_type"`
	Scores     TeamScores       `json:"scores"`
	Reasoning  string           `json:"reasoning"`
	References []ArgReference   `json:"references"`
}

type TeamScores struct {
	Teams map[config.TeamID]ScoreBreakdown `json:"teams"`
}

type ScoreBreakdown struct {
	LogicalConsistency float64 `json:"logical_consistency"`
	EvidenceQuality    float64 `json:"evidence_quality"`
	Responsiveness     float64 `json:"responsiveness"`
	StrategicStrength  float64 `json:"strategic_strength"`
}

func (sb *ScoreBreakdown) GetLogicalConsistency() float64 { return sb.LogicalConsistency }
func (sb *ScoreBreakdown) GetEvidenceQuality() float64    { return sb.EvidenceQuality }
func (sb *ScoreBreakdown) GetResponsiveness() float64     { return sb.Responsiveness }
func (sb *ScoreBreakdown) GetStrategicStrength() float64  { return sb.StrategicStrength }

type ArgReference struct {
	Agent string `json:"agent"`
	Claim string `json:"claim"`
	Issue string `json:"issue,omitempty"`
}

type baseJudge struct {
	judgeType      config.JudgeType
	model          string
	temperature    float64
	client         *client.OpenRouterClientImpl
	templateEngine *prompt.TemplateEngine
}

func (j *baseJudge) Type() config.JudgeType {
	return j.judgeType
}

type LogicJudge struct{ baseJudge }

func (j *LogicJudge) Evaluate(ctx context.Context, rr round.RoundResult, state *round.DebateState) (*JudgeResult, error) {
	return evaluateJudge(ctx, j.baseJudge, rr, state, "Logic", 0, 0)
}

type EvidenceJudge struct{ baseJudge }

func (j *EvidenceJudge) Evaluate(ctx context.Context, rr round.RoundResult, state *round.DebateState) (*JudgeResult, error) {
	return evaluateJudge(ctx, j.baseJudge, rr, state, "Evidence", 0, 0)
}

type ClarityJudge struct{ baseJudge }

func (j *ClarityJudge) Evaluate(ctx context.Context, rr round.RoundResult, state *round.DebateState) (*JudgeResult, error) {
	return evaluateJudge(ctx, j.baseJudge, rr, state, "Clarity", 0, 0)
}

type AdversarialJudge struct {
	baseJudge
	biasThreshold        float64
	contradictionPenalty float64
}

func (j *AdversarialJudge) Evaluate(ctx context.Context, rr round.RoundResult, state *round.DebateState) (*JudgeResult, error) {
	return evaluateJudge(ctx, j.baseJudge, rr, state, "Adversarial", j.biasThreshold, j.contradictionPenalty)
}

func evaluateJudge(ctx context.Context, j baseJudge, rr round.RoundResult, state *round.DebateState, judgeCategory string, biasThreshold float64, contradictionPenalty float64) (*JudgeResult, error) {
	var judgePrompt string
	if j.templateEngine != nil {
		jpp := prompt.JudgePromptParams{
			RoundName:            string(rr.Type),
			Topic:                state.Topic,
			TeamSections:         buildJudgeTeamSections(rr, state.TeamOrder),
			TeamIDs:              state.TeamOrder,
			ContradictionPenalty: contradictionPenalty,
			BiasThreshold:        biasThreshold,
		}
		if result, err := j.templateEngine.BuildJudgePrompt(string(j.judgeType), jpp); err == nil && result != "" {
			judgePrompt = result
		} else {
			judgePrompt = buildJudgePrompt(judgeCategory, rr, state, biasThreshold, contradictionPenalty)
		}
	} else {
		judgePrompt = buildJudgePrompt(judgeCategory, rr, state, biasThreshold, contradictionPenalty)
	}

	resp, err := j.client.Complete(ctx, client.CompletionRequest{
		Model:       j.model,
		Messages:    []client.ChatMessage{{Role: "user", Content: judgePrompt}},
		Temperature: j.temperature,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, fmt.Errorf("judge model call failed: %w", err)
	}

	scores, err := parseJudgeScores(resp.Content)
	if err != nil {
		slog.Warn("failed to parse judge response, using neutral scores", "judge", j.judgeType, "error", err, "response", sanitizeForLog(resp.Content))
		fallbackScores := TeamScores{Teams: make(map[config.TeamID]ScoreBreakdown, len(state.TeamOrder))}
		for _, teamID := range state.TeamOrder {
			fallbackScores.Teams[teamID] = ScoreBreakdown{LogicalConsistency: 5.0, EvidenceQuality: 5.0, Responsiveness: 5.0, StrategicStrength: 5.0}
		}
		for _, s := range fallbackScores.Teams {
			_ = validateScoreRanges(s)
		}
		return &JudgeResult{
			JudgeType: j.judgeType,
			RoundType: rr.Type,
			Scores:    fallbackScores,
			Reasoning: fmt.Sprintf("Failed to parse judge response: %v", err),
		}, nil
	}

	return &JudgeResult{
		JudgeType: j.judgeType,
		RoundType: rr.Type,
		Scores:    *scores,
		Reasoning: extractReasoning(resp.Content),
	}, nil
}

func buildJudgePrompt(category string, rr round.RoundResult, state *round.DebateState, biasThreshold float64, contradictionPenalty float64) string {
	teamSections := buildJudgeTeamSections(rr, state.TeamOrder)

	adversarialInstructions := ""
	if category == "Adversarial" {
		adversarialInstructions = fmt.Sprintf(`

ADVERSARIAL JUDGE INSTRUCTIONS:
- Bias threshold: %.2f (flag arguments that show bias above this level)
- Contradiction penalty: %.1f (subtract from score when teams contradict their own prior arguments)
- Specifically look for: internal contradictions, biased framing, selective evidence presentation
- Cross-reference arguments between rounds to detect contradictions
`, biasThreshold, contradictionPenalty)
	}

	return fmt.Sprintf(`You are a %s Judge evaluating a debate round.

Round: %s
Topic: %s

Evaluate all teams on %s quality. Score each 0-10.
%s
%s

Score each dimension (0-10) for each team. Identify specific arguments
that contain issues. Reference arguments by agent role and content.

Output JSON:
{
  "teams": {
%s
  }
}`, category, rr.Type, state.Topic, category, adversarialInstructions, teamSections, judgeOutputSchema(state.TeamOrder))
}

func buildJudgeTeamSections(rr round.RoundResult, teamOrder []config.TeamID) string {
	var sections strings.Builder
	for _, teamID := range teamOrder {
		sections.WriteString(fmt.Sprintf("%s arguments:\n", config.TeamLabel(teamID)))
		sections.WriteString(formatMessagesForTeam(rr, teamID))
		sections.WriteString("\n\n")
	}
	return strings.TrimSpace(sections.String())
}

func judgeOutputSchema(teamOrder []config.TeamID) string {
	var sb strings.Builder
	for _, teamID := range teamOrder {
		sb.WriteString(fmt.Sprintf("    \"%s\": {\"logical_consistency\": N, \"evidence_quality\": N, \"responsiveness\": N, \"strategic_strength\": N, \"reasoning\": \"...\", \"references\": [{\"agent\": \"...\", \"claim\": \"...\", \"issue\": \"...\"}]},\n", teamID))
	}
	return strings.TrimSuffix(sb.String(), ",\n")
}

func formatMessagesForTeam(rr round.RoundResult, team config.TeamID) string {
	var b strings.Builder
	for _, msg := range rr.Messages {
		if msg.Team == team {
			fmt.Fprintf(&b, "[%s (%s)]: %s\n\n", msg.AgentID, msg.Role, msg.Content)
		}
	}
	if b.Len() == 0 {
		return "(No messages)"
	}
	return b.String()
}

func sanitizeForLog(input string) string {
	const maxLen = 2048
	var b strings.Builder
	b.Grow(len(input))

	for _, r := range input {
		if r == '\n' || r == '\t' || (r >= 32 && r != 127) {
			b.WriteRune(r)
		}
	}

	out := b.String()
	if len(out) > maxLen {
		return out[:maxLen] + "...(truncated)"
	}
	return out
}

// findJSONBlock extracts the outermost balanced JSON object from text.
// It handles nested braces correctly, unlike a simple regex.
func findJSONBlock(content string) string {
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(content); i++ {
		ch := content[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}
	// If we never balanced, return what we found (let JSON parser handle the error)
	return content[start:]
}

func parseJudgeScores(content string) (*TeamScores, error) {
	var result struct {
		Teams map[config.TeamID]ScoreBreakdown `json:"teams"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		match := findJSONBlock(content)
		if match == "" {
			return nil, fmt.Errorf("no JSON found in judge response")
		}
		if err := json.Unmarshal([]byte(match), &result); err != nil {
			return nil, fmt.Errorf("failed to parse judge JSON: %w", err)
		}
	}

	if len(result.Teams) == 0 {
		legacy, legacyErr := parseLegacyJudgeScores(content)
		if legacyErr != nil {
			return nil, legacyErr
		}
		return legacy, nil
	}
	for _, team := range result.Teams {
		if err := validateScoreRanges(team); err != nil {
			return nil, err
		}
	}

	return &TeamScores{Teams: result.Teams}, nil
}

func parseLegacyJudgeScores(content string) (*TeamScores, error) {
	var result struct {
		TeamA ScoreBreakdown `json:"team_a"`
		TeamB ScoreBreakdown `json:"team_b"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		match := findJSONBlock(content)
		if match == "" {
			return nil, fmt.Errorf("no JSON found in judge response")
		}
		if err := json.Unmarshal([]byte(match), &result); err != nil {
			return nil, fmt.Errorf("failed to parse judge JSON: %w", err)
		}
	}
	if err := validateScoreRanges(result.TeamA); err != nil {
		return nil, err
	}
	if err := validateScoreRanges(result.TeamB); err != nil {
		return nil, err
	}
	return &TeamScores{Teams: map[config.TeamID]ScoreBreakdown{config.TeamA: result.TeamA, config.TeamB: result.TeamB}}, nil
}

func validateScoreRanges(sb ScoreBreakdown) error {
	fields := []float64{sb.LogicalConsistency, sb.EvidenceQuality, sb.Responsiveness, sb.StrategicStrength}
	for _, f := range fields {
		if f < 0 || f > 10 {
			return fmt.Errorf("score out of range [0,10]: %v", f)
		}
	}
	return nil
}

func extractReasoning(content string) string {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return content
	}
	if teams, ok := result["teams"].(map[string]interface{}); ok {
		for _, teamData := range teams {
			if m, ok := teamData.(map[string]interface{}); ok {
				if r, ok := m["reasoning"].(string); ok {
					return r
				}
			}
		}
	}
	if teamA, ok := result["team_a"].(map[string]interface{}); ok {
		if r, ok := teamA["reasoning"].(string); ok {
			return r
		}
	}
	return ""
}
