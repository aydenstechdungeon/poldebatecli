# Prompt Templates

## Template System

The prompt engine uses Go's `text/template` with embedded template files in `internal/prompt/templates/`. All templates are parsed from the embedded filesystem at initialization and composed at runtime.

Round templates include the system prompt via `{{template "system.tmpl" .}}`, producing a single prompt containing the agent's identity, role, rules, and round-specific instructions. If template execution fails, the prompt builder falls back to string-based construction.

## System Prompt

Every agent receives a system prompt containing:

- Agent ID and role
- Team name, side (for/against), and position description
- Topic
- Role-specific expertise description
- Debate rules
- Current round name and description
- Prior debate context summary (if available)

## Role Descriptions

| Role | Expertise |
|------|-----------|
| economist | economic analysis, cost-benefit reasoning, market dynamics, fiscal impact, resource allocation |
| historian | historical precedent, longitudinal patterns, institutional evolution, path dependency, civilizational outcomes |
| strategist | strategic positioning, coalition dynamics, implementation feasibility, adversarial analysis, second-order effects |

## Round-Specific Prompts

### Opening
Present core thesis from your role perspective. 2-3 key arguments with evidence. Set the strategic frame.

### Steelman
Reconstruct the strongest version of your opponent's arguments. Present their position charitably before identifying where it breaks down.

### Rebuttal
Counter opposing arguments. Address specific claims. Identify logical flaws, unsupported assertions, or evidence gaps.

### Cross-Examination
Pose sharp questions exposing weaknesses. Challenge evidence, assumptions, and conclusions. 3-5 questions with anticipated responses.

### Fact-Check
Verify factual claims raised in prior rounds (extracted dynamically from `KeyArguments`). Acknowledge contradictory evidence. Update arguments if necessary. Honest engagement is scored positively.

### Position Swap
Argue the OPPOSING side. Tests intellectual honesty and argument depth. References to your earlier arguments as points to overcome are encouraged.

### Closing
Synthesize strongest arguments. Address counter-arguments. Decisive conclusion. No new evidence.

## Judge Prompts

Each judge type has a standalone template (no system prompt composition):

- `judge_logic.tmpl` - Evaluates logical consistency, identifies fallacies and contradictions
- `judge_evidence.tmpl` - Evaluates evidence quality, flags unsupported claims and fabrication
- `judge_clarity.tmpl` - Evaluates clarity and responsiveness, penalizes evasion
- `judge_adversarial.tmpl` - Harsher evaluation, applies contradiction penalty for position drift

All judge templates produce JSON output with team scores, reasoning, and argument references.

## Customization

To modify prompts, edit the template files in `internal/prompt/templates/`. Templates are embedded at compile time, so changes require a rebuild.

If template execution fails, the prompt builder falls back to string-based construction in `internal/agent/prompt_builder.go`, ensuring the debate can continue even with a broken template.
