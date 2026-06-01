# 2026-06-01 Gateway Previous Response Smoke

## Scope

This report records a real-upstream smoke run for `/v1/responses` multi-turn follow-up using `previous_response_id`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18091`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`

Artifacts:

```text
/tmp/gateway-e2e-smoke-prev-20260601
```

## Enabled Scenario

- responses non-stream follow-up with `previous_response_id`

## Result

Status: `passed`

Summary:

```json
{
  "responses_previous_response_nonstream": "passed"
}
```

First call:

- status: `completed`
- response id: `chatcmpl-8757b1acb4687f0b`

Second call:

- status: `completed`
- response id: `chatcmpl-a24c47db7a6d3b5d`
- request used `previous_response_id` from the first call

## Conclusion

This provides real-upstream evidence that the current gateway source tree can execute a `/v1/responses` non-stream follow-up call using `previous_response_id` against the configured upstream/model pair.
