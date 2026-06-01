# 2026-06-01 Gateway Reasoning Stream Smoke Report

## Scope

This report records a focused real-upstream reasoning-stream validation of `gaiasec-llm-gateway` using the current source tree and the live upstream configuration.

Validated endpoints:

- `/v1/responses` stream
- `/v1/messages` stream

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-real-reasoning-20260601
```

## Result

Status: `passed`

### `/v1/responses` stream

Observed:

- `response.output_item.added` for a `reasoning` item
- repeated `response.reasoning.delta`
- terminal `response.output_item.done` / `response.completed`

This proves that the gateway can surface upstream reasoning into Responses-style SSE on a real upstream path.

Evidence file:

- `/tmp/gateway-real-reasoning-20260601/responses-reasoning-stream.txt`

### `/v1/messages` stream

Observed:

- `content_block_start` with `type="thinking"`
- repeated `thinking_delta`
- `content_block_stop`
- terminal `message_delta` / `message_stop`

This proves that the gateway can surface reasoning into Anthropic-style streamed `thinking` blocks on a real upstream path.

Evidence file:

- `/tmp/gateway-real-reasoning-20260601/anthropic-reasoning-stream.txt`

## Conclusion

Real-upstream reasoning stream evidence now exists for:

- Responses-compatible streaming output
- Anthropic-compatible streaming output

This does not yet prove every richer stream combination, but it removes “reasoning stream lacks real-upstream evidence” as a blanket gap.
