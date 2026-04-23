# /Pol/DebateCLI

Multi-agent AI debate orchestrator. Runs structured debates between AI agent teams through configurable round types with judge evaluation and comprehensive output.

## Install

```bash
go build -o debate .
```

## Quick Start

```bash
# Set API key
export OPENROUTER_API_KEY=your_key_here

# Run with required topic
debate run --topic "AI regulation is necessary"

# Custom topic
debate run --topic "AI regulation is necessary"

# Simulate without API calls
debate simulate

# Validate config
debate validate-config
```

## Commands

| Command | Description |
|---------|-------------|
| `debate run` | Execute a debate between AI agent teams |
| `debate resume` | Resume a saved debate session |
| `debate validate-config` | Validate a configuration file |
| `debate simulate` | Run with mock responses (no API calls) |
| `debate inspect` | Inspect past results or configuration |
| `debate docs` | Print documentation |

## Session Saving and Resumption

Long debates can be saved and resumed later. Use `--save-session` to enable auto-saving:

```bash
# Save session every 30 seconds to a file
debate run --save-session ./session.json

# Custom save interval
debate run --save-session ./session.json --save-interval 1m
```

When session saving is enabled, the engine also writes debounced checkpoint saves at lifecycle boundaries (team completion, judge completion, context update) in addition to interval and end-of-round saves.

If the process is interrupted (Ctrl+C, crash, timeout), resume from where it left off:

```bash
# Resume a saved session
debate resume ./session.json

# With custom timeout
debate resume ./session.json --timeout 30m
```

Session files capture:
- Full debate progress (completed rounds, cycle, round index)
- All messages and judge evaluations
- Configuration used
- Metadata (start time, models used, token counts)
- Checkpoint metadata (last checkpoint time and reason)
- Failed rounds for retry

Completed sessions cannot be resumed.

## Configuration

Config loaded from YAML with override precedence: **CLI flags > env vars > config file > defaults**

```yaml
topic: "Universal basic income should be adopted globally"

models:
  defaults:
    economist: "x-ai/grok-4.20"
    historian: "x-ai/grok-4.1-fast"
    strategist: "openai/gpt-5-mini"

teams:
  team_a:
    name: "Proponents"
    side: "for"
    agents:
      - id: "economist_a"
        role: "economist"
  team_b:
    name: "Opponents"
    side: "against"
    agents:
      - id: "economist_b"
        role: "economist"

rounds:
  count: 1
  sequence: [opening, steelman, rebuttal, cross_examination, fact_check, position_swap, closing]

judges:
  types: [logic, evidence, clarity, adversarial]

api_client:
  base_url: "https://openrouter.ai/api/v1"
  api_key_env_var: "OPENROUTER_API_KEY"
```

See `configs/default.yaml` for a full example with all options.

## Architecture

```
CLI (Cobra) → Engine → Rounds → Agents → OpenRouter API
                      → Judges → Scoring
                      → Context Manager → Summarization
                      → Output (JSON + Terminal + Transcript)
```

Four layers: CLI, Orchestration, Domain, Infrastructure. Agents within a team execute in parallel. Teams run sequentially per round. Judges evaluate after each round.

## Round Types

| Round | Description |
|-------|-------------|
| Opening | Present core thesis and key arguments |
| Steelman | Reconstruct strongest version of opponent's position |
| Rebuttal | Counter opposing arguments |
| Cross-Examination | Pose sharp questions exposing weaknesses |
| Fact-Check | Address injected factual claims |
| Position Swap | Argue opposing side (tests intellectual honesty) |
| Closing | Synthesize arguments, decisive conclusion |

## Judge Types

| Judge | Evaluates |
|-------|-----------|
| Logic | Logical consistency, fallacies, contradictions |
| Evidence | Evidence quality, unsupported claims, fabrication |
| Clarity | Communication clarity, evasion, obfuscation |
| Adversarial | Position drift, contradictions between rounds (harsher by design) |

## Environment Variables

| Variable | Effect |
|----------|--------|
| `OPENROUTER_API_KEY` | API key (required for `run`) |
| `DEBATE_TOPIC` | Override topic |
| `DEBATE_API_BASE_URL` | Override API base URL (must be HTTPS OpenRouter host unless unsafe override is enabled) |
| `DEBATE_ALLOW_UNSAFE_BASE_URL` | Allow non-HTTPS or non-OpenRouter base URL (`true`/`false`) |
| `DEBATE_LOG_LEVEL` | Override log level |
| `DEBATE_OUTPUT_PATH` | Override output directory |

## Output

Results written to `./debate_output/` by default:
- `result.json` - Full structured result with scores, messages, judge evaluations
- `transcript.txt` - Human-readable debate transcript

Terminal display shows formatted scores and round breakdown.

## Failure Handling

Automatic recovery for API errors:
- Rate limits (429): Exponential backoff with `Retry-After` respect
- Server errors (5xx): Retry with backoff and jitter
- Timeouts: Simplified prompt retry, then model substitution
- Model failures: Automatic fallback to configured backup model
- Degraded responses tracked in output with original/fallback model details

## Documentation

```bash
debate docs architecture
debate docs configuration
debate docs cli
debate docs prompts
debate docs failure-modes
```
