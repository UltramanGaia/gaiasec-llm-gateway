# 2026-06-01 Gateway Responses-Upstream GPT-5.4-mini Structured Follow-up Smoke

## Scope

This report records a dedicated real-upstream smoke run for `/v1/responses` follow-up scenarios using `previous_response_id` together with structured output on model `gpt-5.4-mini`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.4-mini`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt54mini-structured-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `classified upstream limitations`

Summary:

```json
{
  "responses_previous_response_structured_nonstream": "failed_upstream",
  "responses_previous_response_structured_stream": "failed_upstream"
}
```

Observed:

- baseline `/v1/responses` non-stream and stream both succeeded
- `previous_response_id + structured output` non-stream follow-up was classified as `failed_upstream`
- `previous_response_id + structured output` stream follow-up was classified as `failed_upstream`

## Conclusion

This provides a contrasting richer multi-turn model sample against the successful `gpt-5.2` case.

It shows that `previous_response_id + structured output` follow-up behavior is model/provider dependent even within the supported `openai_responses` route.
