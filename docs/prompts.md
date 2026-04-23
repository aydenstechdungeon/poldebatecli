# Prompt Templates

## Template System

The prompt engine uses Go's `text/template` with embedded template files in `internal/prompt/templates/`. All templates are parsed from the embedded filesystem at initialization and composed at runtime.

Round templates include the system prompt via `{{template "system.tmpl" .}}`, producing a single prompt containing the agent's identity, role, rules, and round-specific instructions.

## System Prompt

Every agent receives a system prompt containing:

- Agent ID and role
- Team name and position (for/against)
- Topic
- Role-specific expertise description
- Debate rules
- Current round name and description
- Prior debate context summary (if available)

Template: `internal/prompt/templates/system.tmpl`

```
You are {{.AgentID}}, a {{.Role}} on the {{.TeamName}} team arguing {{.Side}} the proposition: "{{.Topic}}".

Your expertise: {{.RoleDescription}}
Your team: {{.TeamName}}
Your position: {{.Side}}

Rules:
- Argue ONLY from your {{.Role}} perspective.
- Never concede your team's core position.
- Reference specific evidence, data, or historical precedent.
- Structure arguments with clear claims, evidence, and reasoning.
- Do not repeat arguments already made by your teammates.
- Current round: {{.RoundName}} ({{.RoundDescription}})
{{- if .ContextSummary}}

Prior debate context (summary):
{{.ContextSummary}}
{{- end}}
```

## Role Descriptions

| Role | Expertise |
|------|-----------|
| economist | economic analysis, cost-benefit reasoning, market dynamics, fiscal impact, resource allocation |
| historian | historical precedent, longitudinal patterns, institutional evolution, path dependency, civilizational outcomes |
| strategist | strategic positioning, coalition dynamics, implementation feasibility, adversarial analysis, second-order effects |

## Round-Specific Prompts

### Opening (`opening.tmpl`)
Present core thesis from your role perspective. 2-3 key arguments with evidence. Set the strategic frame.

### Steelman (`steelman.tmpl`)
Reconstruct the strongest version of your opponent's arguments. Present their position charitably before identifying where it breaks down.

### Rebuttal (`rebuttal.tmpl`)
Counter opposing arguments. Address specific claims. Identify logical flaws, unsupported assertions, or evidence gaps.

### Cross-Examination (`cross_exam.tmpl`)
Pose sharp questions exposing weaknesses. Challenge evidence, assumptions, and conclusions. 3-5 questions with anticipated responses.

### Fact-Check (`fact_check.tmpl`)
Verify factual claims raised in prior rounds (extracted dynamically from `KeyArguments`). Acknowledge contradictory evidence. Update arguments if necessary. Honest engagement is scored positively.

### Position Swap (`position_swap.tmpl`)
Argue the OPPOSING side (uses `OppositeSide` template variable). Tests intellectual honesty and argument depth. References to your earlier arguments as points to overcome are encouraged.

### Closing (`closing.tmpl`)
Synthesize strongest arguments. Address counter-arguments. Decisive conclusion. No new evidence.

## Judge Templates

Each judge type has a standalone template (no system prompt composition):

- `judge_logic.tmpl` - Evaluates logical consistency, identifies fallacies and contradictions
- `judge_evidence.tmpl` - Evaluates evidence quality, flags unsupported claims and fabrication
- `judge_clarity.tmpl` - Evaluates clarity and responsiveness, penalizes evasion
- `judge_adversarial.tmpl` - Harsher evaluation, applies contradiction penalty for position drift

All judge templates produce JSON output with team scores, reasoning, and argument references.

## Template Variables

### Round Template Variables

| Variable | Type | Description |
|----------|------|-------------|
| `AgentID` | string | Unique agent identifier |
| `Role` | string | Agent role (economist, historian, strategist) |
| `RoleDescription` | string | Role expertise description |
| `TeamName` | string | Team display name |
| `Side` | string | Debate position (for/against) |
| `Topic` | string | Debate topic |
| `RoundName` | string | Current round type |
| `RoundDescription` | string | Human-readable round description |
| `ContextSummary` | string | Prior debate summary (empty for first round) |
| `OppositeSide` | string | Opposing side (used in position_swap) |
| `FactCheckData` | []string | Claims extracted from prior rounds (used in fact_check) |

### Judge Template Variables

| Variable | Type | Description |
|----------|------|-------------|
| `RoundName` | string | Current round type |
| `Topic` | string | Debate topic |
| `TeamAMessages` | string | Formatted Team A arguments |
| `TeamBMessages` | string | Formatted Team B arguments |
| `ContradictionPenalty` | float64 | Penalty per contradiction (adversarial judge only) |

## Customization

To modify prompts, edit the template files in `internal/prompt/templates/`. Templates are embedded at compile time, so changes require a rebuild.

If template execution fails, the prompt builder falls back to string-based construction in `internal/agent/prompt_builder.go`, ensuring the debate can continue even with a broken template.

## Anti-Repetition Strategies

- Role differentiation: Each role has distinct expertise, preventing agents from making identical arguments
- Round-specific constraints: Each round type demands a different rhetorical approach
- Context summary: Agents receive summaries of prior arguments to avoid repetition
- The "Do not repeat arguments already made by your teammates" rule is built into the system prompt
