# 2026-06-01 Gateway Responses-Upstream Smoke

## Scope

This report records a dedicated real-upstream smoke run for the gateway when the backend is configured with `upstream_type=openai_responses`.

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

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed with classified upstream limitations`

Summary:

```json
{
  "responses_baseline_nonstream": "passed",
  "responses_baseline_stream": "passed",
  "responses_previous_response_nonstream": "failed_upstream",
  "responses_previous_response_stream": "failed_upstream",
  "responses_previous_response_tool_nonstream": "failed_upstream",
  "responses_previous_response_tool_stream": "failed_upstream"
}
```

## Interpretation

### Real-upstream passes

- baseline `/v1/responses` non-stream succeeds
- baseline `/v1/responses` stream succeeds

### Real-upstream classified limitations

- `previous_response_id` non-stream follow-up is classified as `failed_upstream`
- `previous_response_id` stream follow-up is classified as `failed_upstream`
- `previous_response_id + tool` non-stream follow-up is classified as `failed_upstream`
- `previous_response_id + tool` stream follow-up is classified as `failed_upstream`

Observed provider behavior on follow-up:

- the current upstream/model pair returns `404 not found` for follow-up lookup scenarios

## Conclusion

This is the cleanest current-source evidence for `/v1/responses` behavior on the supported `openai_responses` route.

It shows that the gateway can serve baseline responses traffic on this route, while `previous_response_id` follow-up scenarios are currently limited by upstream/provider behavior rather than by silent gateway field loss.
