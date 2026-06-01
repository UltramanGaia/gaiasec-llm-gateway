# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Prompt Cache Follow-up Smoke

## Scope

This report records a dedicated real-upstream smoke result for a `/v1/responses` follow-up using `previous_response_id` together with prompt cache fields on model `gpt-5.2`.

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

Status: `classified upstream limitations`

Summary slice:

```json
{
  "responses_previous_response_prompt_cache_nonstream": "failed_upstream"
}
```

Observed:

- bootstrap `/v1/responses` non-stream returned a response id
- follow-up `/v1/responses` non-stream used that id as `previous_response_id`
- follow-up also supplied:
  - `prompt_cache_key="cache-key"`
  - `prompt_cache_retention="24h"`
- follow-up was classified as `failed_upstream`

Representative provider error:

- `Unsupported parameter: prompt_cache_retention`

## Conclusion

This provides a richer multi-turn request-semantic limitation case on the supported `openai_responses` route.

It shows that `prompt_cache_*` can succeed as a standalone request semantic on `gpt-5.2`, while the combination `previous_response_id + prompt_cache_*` is currently limited by upstream/provider behavior.
