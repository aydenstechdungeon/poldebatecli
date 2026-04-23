# Architecture

## System Overview

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
│  Prompt Builder│ Context Mgr  │ Transcript     │ Failure Handler    │
└────────────────────────────┬───────────────────────────────────────┘
                             │
┌────────────────────────────▼───────────────────────────────────────┐
│                    Infrastructure Layer                              │
│  OpenRouter Client │ Config Loader │ Logger (slog)                  │
│  Rate Limiter      │ Retry/Backoff │ Output Writer                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Layer Descriptions

- **CLI Layer**: Cobra commands with flag binding, help customization, and config loading
- **Orchestration Engine**: Debate loop, round management, team coordination
- **Domain Layer**: Core business logic - agents, rounds, judges, prompts, failure handling
- **Infrastructure Layer**: API client, config, logging, output

## Data Flow

1. Config loaded (YAML + env + flags) → validated
2. Engine initialized with deps (client, factory, registry)
3. For each cycle/round: agents generate → judges evaluate → state updates
4. Results aggregated → output written (JSON + terminal + transcript)

## Concurrency Model

- Agents within a team run in parallel (goroutines + sync.WaitGroup)
- Teams execute sequentially within a round
- Judges evaluate in parallel after each round
- Context manager runs after each round to summarize/compress

## State Management

DebateState is an accumulator: rounds and messages are appended, never mutated. Context summaries are compressed via the context manager to stay within token budgets.
