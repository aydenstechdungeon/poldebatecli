# Configuration

## Config Precedence

CLI flags > environment variables > config file > defaults

## Full Schema

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `models.defaults` | map[role]string | economist/historian/strategist | Default model per role |
| `models.fallbacks` | map[string]string | claude→haiku, etc. | Fallback model mapping |
| `models.temperatures` | map[role]float | 0.5-0.8 | Temperature per role |
| `models.max_tokens` | map[role]int | 2048 | Max output tokens per role |
| `models.top_p` | map[role]float | 0.90-0.95 | Top-p per role |
| `teams.list` | list | 2 teams | Primary team configuration list |
| `teams.count` | int | 2 | Explicit requested team count |
| `teams.list[].position_description` | string | auto-generated | Verbose team stance description |
| `teams.team_a/team_b.*` | object | supported | Backward-compatible legacy shape |
| `rounds.count` | int | 1 | Number of full debate cycles |
| `rounds.sequence` | list | 7 round types | Round execution order |
| `rounds.timeouts` | map[round]duration | 60-90s | Per-round timeout |
| `judges.types` | list | logic,evidence,clarity,adversarial | Active judge types |
| `judges.models` | map[type]string | per-type | Model per judge type |
| `judges.temperatures` | map[type]float | 0.2-0.4 | Temperature per judge |
| `judges.adversarial_config.bias_threshold` | float | 0.3 | Bias detection threshold (0-1) |
| `judges.adversarial_config.contradiction_penalty` | float | 2.0 | Score penalty per contradiction |
| `output.format` | string | json | Output format: json or text |
| `output.path` | string | ./debate_output | Output directory |
| `output.transcript` | bool | true | Write transcript file |
| `output.terminal_display` | bool | true | Print results to terminal |
| `output.streaming` | bool | true | Enable streaming output |
| `output.stream_block_size` | string | sentence | Block size: sentence, paragraph, token |
| `api_client.base_url` | string | https://openrouter.ai/api/v1 | API base URL |
| `api_client.api_key_env_var` | string | OPENROUTER_API_KEY | Env var for API key |
| `api_client.timeout` | duration | 120s | HTTP client timeout |
| `api_client.max_retries` | int | 3 | Max retry attempts |
| `api_client.retry_base_delay` | duration | 2s | Initial retry delay |
| `api_client.rate_limit_rps` | float | 10.0 | Requests per second |
| `logging.level` | string | info | debug, info, warn, error |
| `logging.format` | string | json | json or text |

## Environment Variables

| Variable | Overrides |
|----------|-----------|
| `OPENROUTER_API_KEY` | API key (via api_key_env_var) |
| `DEBATE_API_BASE_URL` | api_client.base_url |
| `DEBATE_API_KEY_ENV_VAR` | api_client.api_key_env_var |
| `DEBATE_LOG_LEVEL` | logging.level |
| `DEBATE_OUTPUT_FORMAT` | output.format |
| `DEBATE_OUTPUT_PATH` | output.path |

## Validation Rules

- At least 2 teams are required
- Teams must include at least one `for` and one `against` side
- Each team must have at least one agent
- Team sides must be "for" or "against"
- Agent IDs must be unique across all teams
- Agent roles must be: economist, historian, or strategist
- Round sequence must have at least one entry
- Round types must be valid (opening, steelman, rebuttal, etc.)
- Judge types must be valid (logic, evidence, clarity, adversarial)
- `api_client.base_url` is required
- `api_client.max_retries` must be >= 0
- `api_client.rate_limit_rps` must be > 0
- `output.format` must be "json" or "text"
- `output.stream_block_size` must be "sentence", "paragraph", or "token"
- `judges.adversarial_config.bias_threshold` must be 0-1
- `judges.adversarial_config.contradiction_penalty` must be >= 0
