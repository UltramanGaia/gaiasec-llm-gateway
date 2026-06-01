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
/tmp/gateway-responses-upstream-smoke-gpt52-stable-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed with mixed follow-up outcomes`

Summary:

```json
{
  "responses_baseline_nonstream": "passed",
  "responses_baseline_stream": "failed_upstream",
  "responses_previous_response_nonstream": "passed",
  "responses_previous_response_stream": "passed",
  "responses_previous_response_tool_nonstream": "passed",
  "responses_previous_response_tool_stream": "passed",
  "responses_previous_response_structured_nonstream": "passed",
  "responses_previous_response_structured_stream": "passed",
  "responses_prompt_cache_nonstream": "passed",
  "responses_previous_response_prompt_cache_nonstream": "failed_upstream",
  "responses_previous_response_prompt_cache_stream": "failed_upstream"
}
```

## Interpretation

### Real-upstream passes

- baseline `/v1/responses` non-stream succeeds
- `previous_response_id` non-stream follow-up succeeds
- `previous_response_id` stream follow-up succeeds
- `previous_response_id + tool` non-stream follow-up succeeds
- `previous_response_id + tool` stream follow-up succeeds
- `previous_response_id + structured output` non-stream follow-up succeeds
- `previous_response_id + structured output` stream follow-up succeeds
- `prompt_cache_*` non-stream succeeds

### Real-upstream classified limitation

- baseline `/v1/responses` stream is classified as `failed_upstream`
- `previous_response_id + prompt_cache_*` non-stream follow-up is classified as `failed_upstream`
- `previous_response_id + prompt_cache_*` stream follow-up is classified as `failed_upstream`

Observed upstream/provider error on the failed follow-up:

- `401 Unauthorized` on one baseline stream request
- `Unsupported parameter: prompt_cache_retention` on prompt-cache follow-up

## Conclusion

This provides a second real model sample on the supported `openai_responses` route, with a broader richer follow-up matrix than the earlier single-scenario reports.

It shows that current behavior is model/provider dependent: unlike several other samples, model `gpt-5.2` can complete multiple richer `previous_response_id` follow-up paths successfully, including tool-stream and structured-output follow-up, while prompt-cache follow-up remains limited by upstream/provider behavior.
