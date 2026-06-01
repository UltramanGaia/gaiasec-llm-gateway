# 2026-06-01 Gateway Smoke Expanded Report

## Scope

This report records an expanded real-upstream smoke run of `gaiasec-llm-gateway` using the current worktree and the live upstream configuration provided during the Rosetta gap-closure work.

Validated endpoint groups:

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/messages`

Validated modes:

- non-stream
- stream

## Execution

Gateway runtime:

- local gateway started from current source via `go run .`
- bind address: `http://127.0.0.1:18090`

Smoke command:

```bash
UPSTREAM_BASE_URL=...
UPSTREAM_API_KEY=...
UPSTREAM_MODEL=...
GATEWAY_URL=http://127.0.0.1:18090
CONFIG_NAME=minimax-m25
ENABLE_RESPONSES_SMOKE=1
ENABLE_ANTHROPIC_SMOKE=1
ENABLE_RESPONSES_STREAM_SMOKE=1
ENABLE_ANTHROPIC_STREAM_SMOKE=1
./scripts/e2e_gateway_smoke.sh
```

Artifacts:

```text
/tmp/gateway-e2e-smoke-20260601-rerun
```

## Result

Status: `passed`

Summary:

```json
{
  "config_test": "passed",
  "nonstream_no_tools": {
    "content": "pong"
  },
  "nonstream_with_tools": {
    "tool_name": "get_weather"
  },
  "stream_no_tools": "passed",
  "stream_with_tools": "passed",
  "responses_nonstream": "passed",
  "anthropic_nonstream": "passed",
  "responses_stream": "passed",
  "anthropic_stream": "passed"
}
```

## Notable Finding

During this run, a real bug was identified and fixed:

- `/v1/responses` routed to `openai_chat` upstream could incorrectly passthrough `chat.completion` JSON instead of re-encoding to `response` JSON when the upstream body was plain JSON and not gzip-encoded.

The fix was applied in:

- [handlers/chat_response.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/chat_response.go:535)

## Remaining Gaps

This expanded smoke still does not prove:

- structured output against the real upstream
- vision/file real-upstream behavior
- richer built-in tool semantics against the real upstream
- broader multi-turn richer parity across all three public endpoints
