# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Structured Follow-up Smoke

## Scope

This report records a dedicated real-upstream smoke result for a `/v1/responses` follow-up using `previous_response_id` together with structured output on model `gpt-5.2`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.2`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt52-structured-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed`

Summary slice:

```json
{
  "responses_previous_response_structured_nonstream": "passed"
}
```

Observed:

- bootstrap `/v1/responses` non-stream returned a response id
- follow-up `/v1/responses` non-stream used that id as `previous_response_id`
- follow-up also supplied `text.format.type=json_schema`
- follow-up response completed successfully and returned a structured JSON payload:
  - `{"pong":true}`

## Conclusion

This provides a concrete richer multi-turn real-upstream success case on the supported `openai_responses` route.

It shows that at least one current model/provider combination (`gpt-5.2`) can complete `previous_response_id + structured output` non-stream follow-up successfully on the current gateway source tree.
