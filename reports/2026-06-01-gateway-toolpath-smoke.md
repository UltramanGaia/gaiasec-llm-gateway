# 2026-06-01 Gateway Tool-Path Smoke Report

## Scope

This report records focused real-upstream tool-path validation of `gaiasec-llm-gateway` on the current source tree.

Validated endpoints:

- `/v1/responses` non-stream with function tools
- `/v1/messages` non-stream with tool-use capable requests

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-real-tools-20260601
/tmp/gateway-e2e-smoke-20260601-tools
```

## Result

Status: `passed`

### `/v1/responses` tool path

Observed:

- gateway returned HTTP `200`
- response remained in Responses shape
- output contained:
  - a `reasoning` item
  - a `function_call` item

Evidence files:

- `/tmp/gateway-real-tools-20260601/responses-tool2.headers`
- `/tmp/gateway-real-tools-20260601/responses-tool2.body`

### `/v1/messages` tool path

Observed:

- gateway returned HTTP `200`
- response remained in Anthropic Messages shape
- content contained:
  - a `thinking` block
  - a `tool_use` block

Evidence files:

- `/tmp/gateway-real-tools-20260601/anthropic-tool2.headers`
- `/tmp/gateway-real-tools-20260601/anthropic-tool2.body`

## Notable Fix

During this validation, request-side tool normalization issues were identified and fixed for cross-protocol request paths, especially:

- Responses request `tool_choice` / `tools` normalization for Chat upstreams
- Anthropic request `tool_choice` normalization for Chat upstreams

Relevant implementation:

- [protocol/request_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/request_codec.go:125)

## Conclusion

This run removes a significant remaining real-upstream gap:

- real tool-path evidence now exists for `/v1/responses`
- real tool-path evidence now exists for `/v1/messages`

## Scripted Re-Run

The same two tool-path scenarios were also re-run through the optional smoke-script entrypoints.

Summary:

```json
{
  "responses_tool_nonstream": "passed",
  "anthropic_tool_nonstream": "passed"
}
```

The same scenarios were re-run after tightening the smoke-script assertions so that the tool-path checks require:

- `responses` output contains a `function_call` item
- `anthropic` output contains a `tool_use` block
