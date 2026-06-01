# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Smoke

## Scope

This report records a dedicated real-upstream smoke run for the gateway on `upstream_type=openai_responses` using model `gpt-5.2`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.2`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt52-structured2-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed with mixed follow-up outcomes`

Summary:

```json
{
  "responses_baseline_nonstream": "passed",
  "responses_baseline_stream": "passed",
  "responses_previous_response_nonstream": "passed",
  "responses_previous_response_stream": "passed",
  "responses_previous_response_tool_nonstream": "passed",
  "responses_previous_response_tool_stream": "failed_upstream"
}
```

## Interpretation

### Real-upstream passes

- baseline `/v1/responses` non-stream succeeds
- baseline `/v1/responses` stream succeeds
- `previous_response_id` non-stream follow-up succeeds
- `previous_response_id` stream follow-up succeeds
- `previous_response_id + tool` non-stream follow-up succeeds

### Real-upstream classified limitation

- `previous_response_id + tool` stream follow-up is classified as `failed_upstream`

Observed upstream/provider error on the failed follow-up:

- `401 Unauthorized`
- provider message indicated `Invalid token`

## Conclusion

This provides a second real model sample on the supported `openai_responses` route.

It shows that current behavior is model/provider dependent: unlike `MiniMax/MiniMax-M2.5`, model `gpt-5.2` can complete plain `previous_response_id` non-stream and stream follow-up successfully, as well as tool follow-up in non-stream mode, while the tool-stream follow-up still fails due to an upstream/provider authorization issue.
