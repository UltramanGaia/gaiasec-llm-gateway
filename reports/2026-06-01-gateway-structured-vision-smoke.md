# 2026-06-01 Gateway Structured/Vision Smoke Report

## Scope

This report records a focused real-upstream smoke run for richer scenarios against the current `gaiasec-llm-gateway` worktree:

- structured output via `/v1/chat/completions`
- structured output via `/v1/responses`
- vision non-stream via `/v1/chat/completions`

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

## Result

### Structured output

Status: `passed`

Observed behavior:

- gateway returned HTTP `200`
- response remained in Chat shape
- assistant content was valid JSON text:

```json
{
  "pong": true
}
```

Evidence file:

- `/tmp/gateway-e2e-smoke-20260601-structured-vision/structured.json`

### Responses structured output

Status: `passed`

Observed behavior:

- gateway returned HTTP `200`
- response remained in Responses shape
- `output[0].content[0].text` contained valid JSON text:

```json
{
  "pong": true
}
```

Evidence files:

- `/tmp/responses-structured5.headers`
- `/tmp/responses-structured5.body`

### Vision non-stream

Status: `failed upstream capability`

Observed behavior:

- gateway returned HTTP `400`
- upstream error body explicitly stated:

```json
{
  "error": {
    "message": "... MiniMax/MiniMax-M2.5 is not a multimodal model ..."
  }
}
```

Interpretation:

- this is not evidence of a gateway transform failure
- it is evidence that the current upstream model rejects multimodal image input

Evidence files:

- `/tmp/gateway-e2e-smoke-20260601-structured-vision/vision-reprobe.headers`
- `/tmp/gateway-e2e-smoke-20260601-structured-vision/vision-reprobe.json`

## Conclusion

This run strengthens real-upstream evidence in three ways:

- structured output works through the gateway against the current upstream
- responses structured output also works through the gateway against the current upstream
- vision failure is now concretely classified as an upstream/model capability limit rather than an unverified gap

It does **not** prove vision support for the gateway as a whole; it only proves that this particular upstream/model combination does not support it.
