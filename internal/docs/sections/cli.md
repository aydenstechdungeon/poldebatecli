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

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--topic` | string | | Debate topic (overrides config file) |
| `--models` | string | | Comma-separated model overrides (role=model pairs) |
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

### `debate resume`

Resume a previously saved debate session.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | duration | 10m | Global timeout for the resumed debate |
| `-v, --verbose` | bool | false | Verbose logging |

### `debate validate-config`

Validate a debate configuration file without executing a debate.

### `debate simulate`

Run a debate using mock/stub responses instead of real API calls.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--seed` | int | 42 | Random seed for reproducible mock responses |
| `--output` | string | | Output directory path |
| `--topic` | string | | Debate topic (overrides config) |

### `debate inspect`

Inspect a past debate result or current configuration.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--result` | string | | Path to a past debate result JSON file |
| `--models` | bool | false | List available models from OpenRouter (requires API key) |
| `--format` | string | table | Output format: table, json, yaml |

### `debate docs`

Print project documentation to stdout.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--open` | bool | false | Open documentation in default browser |

Topics: architecture, configuration, cli, prompts, failure-modes

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error |
