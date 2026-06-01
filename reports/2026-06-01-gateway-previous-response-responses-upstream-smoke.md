# 2026-06-01 Gateway Previous Response Responses-Upstream Smoke

## Scope

This report records real-upstream smoke results for `/v1/responses` follow-up scenarios using `previous_response_id` when the gateway backend is configured with `upstream_type=openai_responses`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `MiniMax/MiniMax-M2.5`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke
```

## Result

Status: `classified upstream limitations`

Baseline:

- responses non-stream: `200 OK`
- responses stream: `200 OK`

Observed HTTP results on the follow-up call:

- `previous_response_id` non-stream: `404 Not Found`
- `previous_response_id` stream: `404 Not Found`
- `previous_response_id + tool` non-stream: `404 Not Found`
- `previous_response_id + tool` stream: `404 Not Found`

Representative upstream/provider error bodies:

- follow-up lookup failure:
  - `Response with id '...' not found`
- tool follow-up failure:
  - same `Response with id '...' not found` class on the follow-up request

Responses baseline requests still succeeded in both non-stream and stream mode, but the follow-up calls did not succeed on the current upstream/model pair when routed through `openai_responses`.

## Conclusion

This is the strongest current evidence for `previous_response_id` behavior on the supported `openai_responses` upstream route.

It shows that, on the current source tree and current upstream/model pair, `previous_response_id` follow-up scenarios are currently limited by upstream/provider behavior rather than by silent gateway field loss.
