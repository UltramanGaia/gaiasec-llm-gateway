# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Tool Stream Follow-up Smoke

## Scope

This report records a dedicated real-upstream smoke result for a `/v1/responses` follow-up using `previous_response_id` together with a function-tool path in stream mode on model `gpt-5.2`.

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

Status: `passed`

Summary slice:

```json
{
  "responses_previous_response_tool_stream": "passed"
}
```

Observed:

- bootstrap `/v1/responses` non-stream returned a response id
- follow-up `/v1/responses` stream used that id as `previous_response_id`
- follow-up also supplied:
  - one function tool definition
  - required function tool choice
- streamed follow-up completed successfully with:
  - `response.function_call_arguments.delta`
  - completed `function_call` output item

## Conclusion

This provides another richer multi-turn real-upstream success case on the supported `openai_responses` route.

It shows that at least one current model/provider combination (`gpt-5.2`) can complete `previous_response_id + tool` follow-up successfully in stream mode.
