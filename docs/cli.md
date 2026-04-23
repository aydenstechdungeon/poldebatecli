# CLI Reference

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | debate.yaml | Path to config file |
| `-v, --verbose` | bool | false | Verbose output (sets log level to debug) |
| `--version` | | | Print version |

## Commands

### `debate run`

Execute a structured multi-agent AI debate.

```bash
debate run [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--topic` | string | | Debate topic (required) |
| `--models` | string | | Comma-separated model overrides (role=model pairs, e.g. "economist=openai/gpt-4o") |
| `--rounds` | int | 1 | Number of full debate cycles |
| `--output` | string | | Output directory path (overrides config) |
| `--dry-run` | bool | false | Validate and show plan without executing API calls |
| `--no-judge` | bool | false | Skip judge evaluation rounds |
| `--no-stream` | bool | false | Disable streaming output (wait for full responses) |
| `--stream-block` | string | sentence | Streaming block size: sentence, paragraph, token |
| `--timeout` | duration | 10m | Global timeout for the entire debate |
| `-v, --verbose` | bool | false | Verbose logging |
| `--save-session` | string | | Path to save session state for later resumption |
| `--save-interval` | duration | 30s | Auto-save interval when --save-session is set |

When `--save-session` is enabled, checkpoint saves are also triggered at team/judge/context boundaries and debounced using `session.checkpoint_min_interval` from config (default `3s`).

Examples:
```bash
debate run --topic "AI regulation is necessary"
debate run --topic "AI regulation is necessary" --config ./my_debate.yaml
debate run --dry-run
debate run --models "economist=openai/gpt-4o,historian=anthropic/claude-3.5-sonnet"
debate run --rounds 3 --output ./results/
debate run --no-stream
debate run --stream-block paragraph
debate run --save-session ./session.json
debate run --save-session ./session.json --save-interval 1m
```

### `debate resume`

Resume a previously saved debate session.

```bash
debate resume <session-file> [flags]
```

Resumes from where the session was interrupted. The session file is created by `debate run --save-session`. Completed sessions cannot be resumed.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | duration | 10m | Global timeout for the resumed debate |
| `-v, --verbose` | bool | false | Verbose logging |

Examples:
```bash
debate resume ./session.json
debate resume ./session.json --timeout 30m
debate resume ./session.json -v
```

### `debate validate-config`

Validate a debate configuration file without executing a debate.

```bash
debate validate-config [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | debate.yaml | Path to config file |

Examples:
```bash
debate validate-config --config ./my_debate.yaml
```

### `debate simulate`

Run a debate using mock/stub responses instead of real API calls. Useful for testing pipeline logic and output formatting without consuming API credits.

```bash
debate simulate [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | debate.yaml | Path to config file |
| `--topic` | string | | Debate topic (required) |
| `--seed` | int | 42 | Random seed for reproducible mock responses |
| `--output` | string | | Output directory path |

Examples:
```bash
debate simulate --topic "AI regulation is necessary" --seed 123 --output ./test_output/
```

### `debate inspect`

Inspect a past debate result or current configuration.

```bash
debate inspect [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--result` | string | | Path to a past debate result JSON file |
| `--config` | string | debate.yaml | Path to config file |
| `--models` | bool | false | List available models from OpenRouter (requires API key) |
| `--format` | string | table | Output format: table, json, yaml |

Examples:
```bash
debate inspect --result ./debate_output/result.json
debate inspect --models
debate inspect --config ./my_debate.yaml --format json
```

### `debate docs`

Print project documentation to stdout.

```bash
debate docs [topic] [flags]
```

Topics: architecture, configuration, cli, prompts, failure-modes

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--open` | bool | false | Open documentation in default browser |

Examples:
```bash
debate docs architecture
debate docs configuration
debate docs all
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error |

## Output

When `output.terminal_display` is true, results are printed to the terminal in a formatted table. JSON output is always written to `output.path/result.json`. A human-readable transcript is written to `output.path/transcript.txt` when `output.transcript` is true.
