# 2026-06-01 Gateway Responses-Upstream GPT-5.4-mini Smoke

## Scope

This report records a dedicated real-upstream smoke run for the gateway on `upstream_type=openai_responses` using model `gpt-5.4-mini`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.4-mini`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt54mini-20260601
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

## Conclusion

This provides a third real model sample on the supported `openai_responses` route.

It shows that model/provider behavior still varies across samples, but `gpt-5.4-mini` currently behaves closer to `MiniMax/MiniMax-M2.5` than to `gpt-5.2` for `previous_response_id` follow-up scenarios.
