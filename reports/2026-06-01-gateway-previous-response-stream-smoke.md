# 2026-06-01 Gateway Previous Response Stream Smoke

## Scope

This report records a real-upstream smoke run for `/v1/responses` stream follow-up using `previous_response_id`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18091`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-prev-stream-20260601
```

## Enabled Scenario

- responses stream follow-up with `previous_response_id`

## Result

Status: `passed`

Summary:

```json
{
  "responses_previous_response_stream": "passed"
}
```

Observed:

- first non-stream bootstrap call succeeded and returned a response id
- second call used that id as `previous_response_id`
- streamed follow-up completed with `response.*` events and `[DONE]`

## Conclusion

This provides real-upstream evidence that the current gateway source tree can execute a `/v1/responses` stream follow-up call using `previous_response_id` against the configured upstream/model pair.
