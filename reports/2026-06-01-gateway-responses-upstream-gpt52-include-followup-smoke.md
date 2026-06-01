# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Include Follow-up Smoke

## Scope

This report records a dedicated real-upstream smoke result for `/v1/responses` follow-up using `previous_response_id` together with `include` on model `gpt-5.2`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.2`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt52-include-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed`

Summary slice:

```json
{
  "responses_previous_response_include_nonstream": "passed"
}
```

Observed:

- bootstrap `/v1/responses` non-stream returned a response id
- follow-up `/v1/responses` non-stream used that id as `previous_response_id`
- follow-up also supplied:
  - `include=["reasoning.encrypted_content"]`
- follow-up completed successfully

## Conclusion

This provides another richer multi-turn request-semantic success case on the supported `openai_responses` route.
