# Architecture

## System Overview

/Pol/DebateCLI is a Go-based CLI that orchestrates multi-agent AI debates via the OpenRouter API. It coordinates teams of agents through structured debate rounds, evaluates arguments with multiple judge types, and produces comprehensive output.

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLI Layer (Cobra)                          │
│  debate run | validate-config | simulate | inspect | docs           │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────────┐
│                    Orchestration Engine                              │
│  ┌─────────────┐ ┌──────────────┐ ┌────────────┐ ┌──────────────┐  │
│  │ Debate Loop  │ │ Round Mgr    │ │ Team Coord  │ │ Event Emitter│  │
│  └──────┬──────┘ └──────┬───────┘ └──────┬─────┘ └──────┬───────┘  │
└─────────┼───────────────┼────────────────┼──────────────┼──────────┘
          │               │                │              │
┌─────────▼───────────────▼────────────────▼──────────────▼──────────┐
│                      Domain Layer                                   │
│  Agent Factory │ Team Builder │ Round Executor │ Judge Evaluator    │
│  Prompt Engine │ Context Mgr  │ Transcript     │ Failure Handler    │
└────────────────────────────┬───────────────────────────────────────┘
                             │
┌────────────────────────────▼───────────────────────────────────────┐
│                    Infrastructure Layer                              │
│  OpenRouter Client │ Config Loader │ Logger (slog)                  │
│  Rate Limiter      │ Retry/Backoff │ Output Writer                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Layer Descriptions

- **CLI Layer**: Cobra commands with flag binding, help customization, and config loading. Each command loads config, validates it, and delegates to the engine.
- **Orchestration Engine**: Coordinates the debate loop. Iterates through round sequences for each cycle, manages state, and triggers judge evaluation after each round.
- **Domain Layer**: Core business logic. Agents generate arguments, rounds define execution strategies, judges evaluate, the prompt engine builds per-agent prompts from templates, and the failure handler provides resilience.
- **Infrastructure Layer**: OpenRouter HTTP client with rate limiting and retry logic, YAML config loading with env/flag overrides, structured logging, and output writers.

## Data Flow

1. Config loaded from YAML file, merged with env vars and CLI flags
2. Config validated for required fields, valid roles, round types, judge types
3. Engine deps initialized: API client, agent factory, team builder, round/judge registries, template engine
4. For each cycle (configurable count):
   - For each round in the sequence:
     - Each team's agents execute in parallel (goroutines + sync.WaitGroup)
     - Prompts built per-agent from template engine (system + round template)
     - On agent failure: failure handler retries, substitutes model, or marks degraded
     - After round: judges evaluate in parallel, context manager summarizes
5. Scores aggregated across all rounds and judges
6. Winner determined by total score comparison
7. Results written: JSON file, transcript file, terminal display

## Package Structure

```
cmd/                - Cobra commands (root, run, validate-config, simulate, inspect, docs)
internal/
  agent/            - Agent interface, factory, prompt builder
  client/           - OpenRouter client, rate limiter, retry, streaming
  config/           - Config structs, YAML loader, env binding, validation
  context/          - Context manager, summarizer, argument tracker
  docs/             - Embedded documentation sections
  engine/           - Engine interface, debate loop, deps wiring
  failure/          - Failure handler, retry strategies, degraded tracking
  judge/            - Judge interface, 4 judge types, registry, scoring
  output/           - JSON writer, terminal writer, transcript writer
  prompt/           - Template engine, embedded template files
  round/            - Round interface, 7 round types, registry
  team/             - Team struct, builder
```

## Concurrency Model

- Agents within a team run in parallel using goroutines with `sync.WaitGroup` and `sync.Mutex` for result collection
- Teams execute sequentially within a round (Team A completes, then Team B)
- Judges evaluate in parallel after each round
- Context manager runs after each round to summarize/compress prior debate context
- Each agent's API call is rate-limited independently

## State Management

`DebateState` is an accumulator pattern: rounds and messages are appended, never mutated. The context manager compresses raw messages into summaries after each round, keeping the token budget bounded. Key arguments are extracted and tracked separately for cross-round reference.

## Template System

The prompt engine uses Go's `text/template` with embedded template files. All templates are parsed from the embedded FS at initialization. Round templates compose with the system template via `{{template "system.tmpl" .}}`. Judge templates are standalone. If template execution fails, the prompt builder falls back to string-based construction.

## Dependency Injection

Engine deps are constructed manually in `engine.NewEngineDeps()` rather than using Wire code generation. This keeps the build simple while maintaining clear dependency ordering. The template engine is created first, then passed to both the prompt builder and judge registry.
