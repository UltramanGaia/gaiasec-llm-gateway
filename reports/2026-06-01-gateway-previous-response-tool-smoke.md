# 2026-06-01 Gateway Previous Response Tool Smoke

## Scope

This report records a real-upstream smoke run for `/v1/responses` follow-up using `previous_response_id` together with a function-tool path.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18091`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-prev-tool-20260601
```

## Enabled Scenario

- responses non-stream follow-up with `previous_response_id` and function tools

## Result

Status: `passed`

Summary:

```json
{
  "responses_previous_response_tool_nonstream": "passed"
}
```

Observed:

- first non-stream bootstrap call succeeded and returned a response id
- second call used that id as `previous_response_id`
- second call also supplied a function tool definition and required tool choice
- follow-up output contained a `function_call` item

## Conclusion

This provides real-upstream evidence that the current gateway source tree can execute a `/v1/responses` non-stream follow-up call using `previous_response_id` together with a function-tool path against the configured upstream/model pair.
