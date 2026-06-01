# 2026-06-01 Gateway Smoke Max Report

## Scope

This report records a full real-upstream smoke run of `gaiasec-llm-gateway` with all currently supported optional smoke scenarios enabled.

Gateway runtime:

- launched locally from current source with `go run .`
- bind address: `http://127.0.0.1:18090`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-20260601-max6
```

## Enabled Scenarios

- chat non-stream
- chat stream
- chat tool non-stream
- chat tool stream
- chat structured output
- responses structured output
- vision non-stream
- file non-stream
- responses previous_response_id non-stream
- responses previous_response_id stream
- responses previous_response_id tool non-stream
- responses previous_response_id tool stream
- responses non-stream
- anthropic non-stream
- responses stream
- anthropic stream
- responses tool non-stream
- anthropic tool non-stream
- responses tool stream
- anthropic tool stream

## Result

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
  "responses_structured_output": "passed",
  "vision_nonstream": "failed_upstream",
  "file_nonstream": "failed_upstream",
  "responses_previous_response_nonstream": "passed",
  "responses_previous_response_stream": "passed",
  "responses_previous_response_tool_nonstream": "passed",
  "responses_previous_response_tool_stream": "passed",
  "responses_nonstream": "passed",
  "anthropic_nonstream": "passed",
  "responses_stream": "passed",
  "anthropic_stream": "passed",
  "responses_tool_nonstream": "passed",
  "anthropic_tool_nonstream": "passed",
  "responses_tool_stream": "passed",
  "anthropic_tool_stream": "passed"
}
```

## Interpretation

### Real-upstream passes

- all three public endpoints succeed in basic non-stream mode
- all three public endpoints succeed in basic stream mode
- responses/messages tool-path non-stream succeed
- responses/messages tool-path stream succeed
- chat structured output succeeds
- responses structured output succeeds
- responses previous_response_id non-stream succeeds
- responses previous_response_id stream succeeds
- responses previous_response_id tool non-stream succeeds
- responses previous_response_id tool stream succeeds

### Real-upstream classified limitations

- vision non-stream is classified as an upstream/model capability limitation
- file non-stream is classified as an upstream/model compatibility limitation

## Conclusion

This is the strongest single real-upstream smoke artifact currently available in the repo.

It proves that the gateway, on the current source tree, supports the main chat/responses/messages paths plus structured output, responses previous_response_id follow-up, responses previous_response_id + tool follow-up, and tool-path/tool-stream scenarios against the configured real upstream, while also classifying the current upstream limitations for vision and file scenarios.
