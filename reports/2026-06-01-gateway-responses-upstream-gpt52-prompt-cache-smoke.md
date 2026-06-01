# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Prompt Cache Smoke

## Scope

This report records a dedicated real-upstream smoke result for `/v1/responses` prompt cache fields on model `gpt-5.2`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.2`
- upstream type: `openai_responses`

Artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt52-promptcache3-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Result

Status: `passed`

Summary slice:

```json
{
  "responses_prompt_cache_nonstream": "passed"
}
```

Observed:

- `/v1/responses` non-stream completed successfully
- request used:
  - `prompt_cache_key="cache-key"`
  - `prompt_cache_retention="24h"`
- response preserved:
  - `prompt_cache_key`
  - `prompt_cache_retention`

## Conclusion

This provides real-upstream evidence that the current gateway source tree can send prompt cache fields successfully on at least one supported `openai_responses` model sample (`gpt-5.2`).
