# 2026-06-01 Gateway Tool-Stream Smoke Report

## Scope

This report records focused real-upstream tool-stream validation of `gaiasec-llm-gateway` on the current source tree.

Validated endpoints:

- `/v1/responses` stream with function tools
- `/v1/messages` stream with tool-use capable requests

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-20260601-tool-streams
```

## Result

Status: `passed`

Summary:

```json
{
  "responses_tool_stream": "passed",
  "anthropic_tool_stream": "passed"
}
```

## Interpretation

This adds real-upstream evidence for richer tool-path streaming on two non-chat public endpoints:

- Responses-compatible tool streaming
- Anthropic-compatible tool streaming

It materially reduces the remaining gap for real-upstream richer tool evidence.
