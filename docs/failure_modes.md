# Failure Modes

## Failure Mode Table

| Failure | Detection | Strategy | Recovery |
|---------|-----------|----------|----------|
| Model refusal (content filter) | Response contains refusal pattern or empty content | Retry with softened prompt (remove contentious phrasing) | After 2 retries: substitute fallback model. Mark degraded. |
| Empty response | Content is whitespace or <10 chars | Retry with explicit instruction "You must respond with substantive content" | After 2 retries: substitute model. Mark degraded. |
| Hallucinated structure | Judge response missing required JSON fields | Regex extraction of partial JSON. Fill missing scores with neutral 5.0. | Log warning. Use partial scores. Mark degraded. |
| API 429 (rate limit) | HTTP status 429 | Respect Retry-After header. Exponential backoff. | After 3 retries with backoff: fail round, continue debate. |
| API 5xx (server error) | HTTP status 500-504 | Exponential backoff with jitter | After 3 retries: substitute model. |
| Timeout (model or round) | Context deadline exceeded | Cancel pending requests. Retry with simplified prompt. | After 1 retry: substitute model. After 2: skip agent, mark degraded. |
| Network failure | TCP/connection error | Exponential backoff | After 3 retries: fail round. |
| Malformed JSON in judge response | JSON parse error | Regex extraction. Fallback to neutral scores. | If extraction fails: use neutral scores (5.0), log error. |
| All agents in a team fail | No messages from team | Skip round for that team. | Continue debate. Flag in output as incomplete. |
| Config validation error | Missing required fields, invalid values | Fail fast with detailed error messages. | No recovery. User must fix config. |
| SSE stream disconnect | Connection drops mid-stream | Flush buffered partial content. Retry stream. | After 1 retry: fall back to non-streaming Complete(). |
| Stream timeout | No tokens received for N seconds | Cancel stream. Retry with non-streaming request. | After 1 retry: substitute model. Mark degraded. |

## Degraded Response Tracking

When a fallback model is used or a response is incomplete, a `DegradedEntry` is recorded:

```json
{
  "agent_id": "economist_a",
  "round": "opening",
  "original_model": "x-ai/grok-4.20",
  "used_model": "anthropic/claude-haiku-4.5",
  "reason": "rate_limit: all 3 attempts failed",
  "retry_count": 2
}
```

Degraded entries appear in:
- The JSON output (`degraded` field)
- The terminal display (DEGRADED RESPONSES section)
- The transcript file

## Retry Strategy

The OpenRouter client uses exponential backoff with jitter:
- Base delay: configurable (default 2s)
- Max delay: 30s
- Jitter: random 0-500ms added to each delay
- Retryable errors: 429, 500, 502, 503, 504, network errors
- 429 responses: respect Retry-After header

## Fallback Model Chain

When a model fails, the failure handler looks up the fallback mapping from `models.fallbacks` config:

```yaml
models:
  fallbacks:
    "x-ai/grok-4.20": "anthropic/claude-haiku-4.5"
    "x-ai/grok-4.1-fast": "openai/gpt-5.4-nano"
    "openai/gpt-5-mini": "anthropic/claude-haiku-4.5"
```

The fallback model is used for all subsequent requests from that agent in the current round.

## Debugging

- Set `--verbose` or `logging.level: "debug"` for detailed logs
- Use `debate inspect --result` to examine past results
- Use `debate validate-config --strict` to catch config issues early
- Check `degraded` array in output for any model substitutions
- Use `debate simulate` to test the pipeline without API calls

## Known Limitations

- Judge model hallucination: Judges may produce invalid JSON despite instructions. Regex extraction and neutral score fallbacks mitigate this.
- Adversarial judge harshness: By design, the adversarial judge scores lower. This is intentional to counteract model sycophancy.
- Context compression: Summarization may lose nuanced arguments. The argument tracker preserves key claims separately.
- SSE stream reliability: Streaming depends on stable connections. The fallback to non-streaming requests handles most disconnect scenarios.
