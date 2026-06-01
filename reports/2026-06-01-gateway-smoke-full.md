# 2026-06-01 Gateway Full Smoke Report

## Scope

This report records a full optional smoke run of `gaiasec-llm-gateway` against the real upstream using the current source tree.

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-20260601-full
```

## Enabled Smoke Flags

- `ENABLE_STRUCTURED_OUTPUT_SMOKE=1`
- `ENABLE_VISION_SMOKE=1`
- `ENABLE_FILE_SMOKE=1`
- `ENABLE_RESPONSES_SMOKE=1`
- `ENABLE_ANTHROPIC_SMOKE=1`
- `ENABLE_RESPONSES_STREAM_SMOKE=1`
- `ENABLE_ANTHROPIC_STREAM_SMOKE=1`

## Result Summary

Status: `passed with classified upstream limitations`

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
  "structured_output": "passed",
  "vision_nonstream": "failed_upstream",
  "file_nonstream": "failed_upstream",
  "responses_nonstream": "passed",
  "anthropic_nonstream": "passed",
  "responses_stream": "passed",
  "anthropic_stream": "passed"
}
```

## Interpretation

### Passed

- chat non-stream
- chat stream
- tool non-stream
- tool stream
- structured output
- responses non-stream
- responses stream
- anthropic non-stream
- anthropic stream

### Classified Upstream Limitations

- vision non-stream
- file non-stream

These two are no longer unverified gaps for the current upstream/model combination.

## Conclusion

This run materially improves real-upstream evidence for Phase 7:

- all three public endpoints were exercised against the live upstream
- both non-stream and stream behavior were exercised
- optional richer smoke scenarios now produce either passing evidence or explicit upstream-capability classifications
