# 2026-06-01 Gateway Responses-Upstream GPT-5.2 Follow-up Matrix

## Scope

This report consolidates the currently collected real-upstream `/v1/responses` follow-up evidence for model `gpt-5.2` on `upstream_type=openai_responses`.

Gateway runtime:

- launched locally from current source
- bind address: `http://127.0.0.1:18092`

Upstream:

- base URL: `http://172.31.29.10/v1`
- model: `gpt-5.2`
- upstream type: `openai_responses`

Primary artifacts:

```text
/tmp/gateway-responses-upstream-smoke-gpt52-stable-20260601
/tmp/gateway-responses-upstream-smoke-gpt52-structured2-20260601
/tmp/gateway-responses-upstream-smoke-gpt52-promptcache3-20260601
```

Driver:

- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)

## Consolidated Result

| Scenario | Status |
| --- | --- |
| baseline `/v1/responses` non-stream | `passed` |
| baseline `/v1/responses` stream | `passed` or `failed_upstream` depending on routed provider sample |
| `previous_response_id` non-stream | `passed` |
| `previous_response_id` stream | `passed` |
| `previous_response_id + tool` non-stream | `passed` |
| `previous_response_id + tool` stream | `passed` |
| `previous_response_id + structured output` non-stream | `passed` |
| `previous_response_id + structured output` stream | `passed` |
| `prompt_cache_*` non-stream | `passed` |
| `previous_response_id + prompt_cache_*` non-stream | `failed_upstream` |
| `previous_response_id + prompt_cache_*` stream | `failed_upstream` |

## Interpretation

This is currently the strongest richer multi-turn real-upstream model profile in the repo.

It shows that `gpt-5.2` can successfully complete several follow-up combinations that fail on other model/provider combinations, including:

- `previous_response_id + tool + stream`
- `previous_response_id + structured output` non-stream
- `previous_response_id + structured output` stream

At the same time, it also shows an upstream/provider limitation for:

- `previous_response_id + prompt_cache_*` follow-up

## Conclusion

`gpt-5.2` is the strongest current real-upstream success sample for richer multi-turn `/v1/responses` behavior on the supported `openai_responses` route.
