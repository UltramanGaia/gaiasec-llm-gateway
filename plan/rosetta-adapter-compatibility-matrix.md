# Rosetta Adapter Compatibility Matrix

更新时间：2026-06-01

状态标签：

- `支持`：当前实现已有明确 codec/handler 路径与测试证据
- `部分支持`：至少可保留或同协议 round-trip，但跨协议/stream/guard 尚不完整
- `未支持`：当前实现没有可靠保留或映射
- `显式拒绝`：在 capability/dispatch 前置拦截，不再静默丢失

## Request

| 项目 | Chat | Responses | Anthropic | 当前状态 | 证据 |
| --- | --- | --- | --- | --- | --- |
| 基础 text/tool/function 主路径 | 支持 | 支持 | 支持 | 支持 | `protocol/request_codec_test.go` |
| vision/file content | 支持 | 支持 | 支持 | 支持 | `protocol/request_codec_test.go` |
| `parallel_tool_calls` | 支持 | 支持 | 部分支持 | 支持 | `protocol/request_codec_test.go`, `handlers/protocol_capabilities.go` |
| Responses `previous_response_id/include/metadata/service_tier/store/background/conversation/prompt/prompt_cache_*` | N/A | 支持 | N/A | 部分支持 | `protocol/request_codec.go`, `protocol/request_codec_test.go` |
| `previous_response_id` 转非 Responses upstream | N/A | N/A | N/A | 显式拒绝 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go`, `handlers/protocol_handler_test.go` |
| `prompt_cache_*` 转非 Responses upstream | N/A | N/A | N/A | 显式拒绝 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go`, `handlers/protocol_handler_test.go` |
| `include/store/background/conversation/prompt` 转非 Responses upstream | N/A | N/A | N/A | 显式拒绝 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go`, `handlers/protocol_handler_test.go` |
| Chat `metadata/service_tier/audio/modalities/prediction/verbosity/web_search_options/logprobs/top_logprobs/seed/n/*penalty/logit_bias` | 支持 | N/A | N/A | 部分支持 | `protocol/request_codec.go`, `protocol/request_codec_test.go` |
| Anthropic `metadata/service_tier/top_k` | N/A | N/A | 支持 | 部分支持 | `protocol/request_codec.go`, `protocol/request_codec_test.go` |
| Responses 非 function tools 原样保留 | 部分支持 | 支持 | N/A | 部分支持 | `protocol/request_codec.go`, `protocol/request_codec_test.go` |
| Responses built-in tools 转非 Responses upstream | N/A | N/A | N/A | 显式拒绝 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go` |

## Response

| 项目 | 当前状态 | 备注 |
| --- | --- | --- |
| Chat `reasoning_content` | 支持 | 已有 non-stream / stream 测试 |
| Responses `message/function_call/reasoning` | 支持 | 主路径可用 |
| Responses richer output item (`custom_tool_call/mcp_call/web_search_call/compaction/...`) | 部分支持 | 已可 decode 并 same-protocol 原样 round-trip；跨协议映射仍待补 |
| `refusal` / `annotations` / `audio` 一等建模 | 部分支持 | Chat/Responses non-stream 已保留并有测试；`chat -> anthropic`、`responses -> anthropic` non-stream 也已保留扩展字段，stream 与其余跨协议仍待补 |
| 顶层 `status/error/incomplete_details/metadata` 保留 | 支持 | Responses non-stream same-protocol 已保留 |

## Stream

| 项目 | 当前状态 | 备注 |
| --- | --- | --- |
| text delta | 支持 | 三协议主路径已有覆盖，且 `response.output_text.delta` 现在已明确归一化到统一 IR `output_text.delta` |
| function tool call delta | 支持 | 已有聚合与 codec 测试 |
| reasoning delta / lifecycle IR | 部分支持 | Chat/Anthropic 主路径已支持，`Responses/Anthropic -> IRStreamEvent` 已开始统一归一化 `reasoning.start/delta/done` 的一部分 lifecycle，且 `Responses response.output_item.done` 里的最终 reasoning summary 已直接提升到 IR 并被 `Responses -> Chat/Anthropic` output_item.done 分支优先消费；同时 Anthropic `thinking` start-only 文本现在也能直接进入 Chat/Responses stream，且在 Responses 路径下 added item summary 也保持一致；反向的 `Responses output_item.added.summary` 现在也能直接进入 Anthropic `thinking` block，并进入 Chat `reasoning_content`；仍未全覆盖 |
| refusal stream | 部分支持 | Chat <-> Responses、Chat -> Anthropic、Responses -> Anthropic、Anthropic -> Chat/Responses 的主跨协议路径已有证据；其中 Responses `output_item.added.item.content` 自带的 start-only refusal 现在也能直接进入 Chat/Anthropic stream；same-protocol 与长尾 lifecycle 仍不完整 |
| audio stream | 部分支持 | Chat <-> Responses、Chat -> Anthropic、Responses -> Anthropic、Anthropic -> Chat/Responses 的主跨协议路径已有证据；其中 Responses `output_item.added.item.content` 自带的 start-only audio 现在也能直接进入 Chat/Anthropic stream |
| annotation stream | 部分支持 | Chat -> Responses、Responses -> Chat、Chat/Responses -> Anthropic、Anthropic -> Chat/Responses 与 stream log 聚合均已有主路径证据，且 `response.annotation.added` 已进入统一 IR 事件层；其中 Responses `output_item.added.item.content` 自带的 start-only annotations 现在也能直接进入 Chat/Anthropic stream；same-protocol 与长尾仍缺 |
| richer tool lifecycle/status | 部分支持 | Responses `*_call` 到 Chat/Anthropic 已保留基础 lifecycle，`Responses/Anthropic -> IRStreamEvent` 已开始统一归一化 `tool_call.start/delta/done`；其中 Anthropic `tool_use -> Responses function_call` 已有 `added/delta/arguments.done/output_item.done` 与参数累积证据，且 `content_block_start.input` 中自带的完整参数现在也能直接进入 Chat/Responses stream，并避免了 start-only input 过早重复发 `arguments.done`；反向路径中，`Responses -> Chat` 的 `output_item.added` 自带初始 tool args 现在也能直接进入 `tool_calls[].function.arguments`，而 `Responses -> Anthropic` 的 `output_item.added` 自带初始 tool args 现在也能直接进入 `tool_use input`；Anthropic `thinking` start-only 文本现在也能直接进入 Chat/Responses stream；Responses `tool/reasoning output_item.done -> Anthropic content_block_stop` 也已即时收尾，Responses `function_call_arguments.done -> Chat` 会发出最终 arguments chunk，`function_call_arguments.done -> Anthropic` 在 only-done 场景下也会发出最终 `input_json_delta`，`output_item.done` 自带的最终 tool args/ reasoning summary 现在也会直接提升到 IR 并被 Chat/Anthropic output_item.done 分支优先消费，而 Responses -> Chat 的 only `[DONE]` flush 现在也会保留 usage；Anthropic -> Chat 的 only `message_stop`、Anthropic -> Responses 的 only `message_stop`、Responses -> Anthropic 的 only `[DONE]` tool path 在无最终 completed/delta 时也都会补最终收尾语义；Anthropic -> Responses handler 侧也已避免重复 `response.completed`；整体语义仍有降级 |

## Capability Guard

| 项目 | 当前状态 | 证据 |
| --- | --- | --- |
| stream/tools/reasoning/json_schema/vision | 支持 | `handlers/protocol_capabilities.go` |
| audio output capability gating | 支持 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go` |
| web_search / mcp / code_interpreter / image_generation / prompt_cache | 支持 | `handlers/protocol_capabilities.go`, `handlers/protocol_capabilities_test.go` |
| builtin responses tools 非目标协议前置拒绝 | 支持 | `handlers/protocol_capabilities_test.go` |
| `/api/model-configs` 暴露并可编辑扩展 capability flags | 支持 | `handlers/model_mapping_handler.go`, `handlers/model_mapping_handler_test.go`, `frontend/src/views/ModelConfig.vue` |

## Logging

| 项目 | 当前状态 | 证据 |
| --- | --- | --- |
| `/api/request-logs` 可见 richer output item types | 部分支持 | `handlers/log_handler.go`, `handlers/log_handler_test.go` |
| `/api/request-logs` 可见 reasoning/refusal/annotations/audio 摘要 | 部分支持 | Chat/Responses/Anthropic 的主要 richer content 已可进入 semantic summary；其中 Responses `ReasoningSummary` 已收紧到真正的 `reasoning/compaction` item，不再误读普通 assistant `output_text`；更广 richer item 仍未全覆盖 |
| stream 日志聚合保留 richer 结构 | 部分支持 | 已保留 reasoning/tool_calls/refusal；其中 chat-like stream 聚合在 only-`[DONE]` 无最终 finish chunk 时也会推断 `finish_reason=tool_calls`，`Anthropic -> Responses` richer `reasoning/annotation/refusal/audio/arguments.done` 事件及携带实际输出的 `response.output_item.added/done` 现在也会计入输出 token 观测，`Chat -> Responses` 不再把 role-only / metadata-only chunk 误计为输出 token，而 `Anthropic -> Chat` 的 start-only `thinking` 文本现在也会计入输出 token；annotations/audio/richer item 仍未完整聚合 |
| richer same-protocol replay 验证 | 部分支持 | Chat-like / Responses / Anthropic replay 都已有证据；Responses fallback 与 Anthropic replay 都已覆盖 annotations/refusal/audio，且 Responses fallback 已补 `file_search_call` built-in tool 输入重建、only-`function_call_arguments.done` 的 `function_call.arguments` 重建、partial `arguments.delta` + final `arguments.done` 时的最终参数优先保留、only-`response.reasoning.delta` 的 `reasoning.summary` 重建、only-`response.output_item.added.item.content` 的 start-only `output_text` 汇总、start-only richer parts + 后续 text delta 时的 part/text 合并、同一 message 多 `output_text` part` 的按 `content_index` 独立重建、`response.annotation.added` 的按 `content_index` 正确落位，以及 `output_text/annotation/refusal/audio` 多 part 组合的独立落位和无 `response.completed`/`response.output_item.done` 时的顶层与 item-level `status=completed` 收尾；Anthropic replay 也已补 only-`message_stop` 场景下的 `stop_reason` 推断、截断 tool_use 场景下的 `stop_reason` 兜底、`tool_use start-only input` 保留、`thinking start-only text` 保留，以及空 assistant message 重建；跨协议 replay 仍不足 |

## Integration

| 项目 | 当前状态 | 证据 |
| --- | --- | --- |
| responses `previous_response_id` mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `custom_tool_call` mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `annotations` mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `refusal` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `annotations` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| chat structured richer content (`annotations/refusal/audio`) -> responses non-stream | 支持 | `handlers/protocol_handler_test.go`, `protocol/response_codec_test.go` |
| chat structured richer content (`annotations/refusal/audio`) -> anthropic non-stream | 支持 | `handlers/protocol_handler_test.go`, `protocol/response_codec_test.go` |
| anthropic structured richer content (`annotations/refusal/audio`) -> chat non-stream | 支持 | `handlers/protocol_handler_test.go`, `protocol/response_codec_test.go` |
| anthropic structured richer content (`annotations/refusal/audio`) -> responses non-stream | 支持 | `handlers/protocol_handler_test.go`, `protocol/response_codec_test.go` |
| responses richer content (`annotations/refusal/audio`) -> anthropic non-stream | 支持 | `handlers/protocol_handler_test.go`, `protocol/response_codec_test.go` |
| responses `custom_tool_call` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `custom_tool_call` -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `mcp_call` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `web_search_call` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `image_generation_call` -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `image_generation_call` -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `mcp_call` -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `web_search_call` -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `annotations` stream -> chat mock integration | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_stateful_codec_test.go` |
| responses `reasoning` stream -> chat mock integration | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_stateful_codec_test.go` |
| anthropic richer text-block stream (`annotations/refusal/audio`) -> chat | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_anthropic_codec_test.go` |
| anthropic richer text-block stream (`annotations/refusal/audio`) -> responses | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_anthropic_codec_test.go` |
| responses richer text stream (`annotations/refusal/audio`) -> anthropic | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_stateful_codec_test.go` |
| chat richer text stream (`annotations/refusal/audio`) -> anthropic | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_stateful_codec_test.go` |
| responses `custom_tool_call` stream -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `custom_tool_call` stream -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses built-in tool streams -> chat mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses built-in tool streams -> anthropic mock integration | 支持 | `handlers/protocol_handler_test.go` |
| anthropic `thinking` -> responses reasoning stream mock integration | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_anthropic_codec_test.go` |
| anthropic `tool_use` stream -> responses function_call mock integration | 支持 | `handlers/protocol_handler_test.go`, `protocol/stream_anthropic_codec_test.go` |
| anthropic `tool_use` stream -> chat tool_calls mock integration | 支持 | `handlers/protocol_handler_test.go` |
| responses `reasoning` stream -> anthropic thinking mock integration | 支持 | `handlers/protocol_handler_test.go` |
| chat richer stream -> responses richer events mock integration | 支持 | `handlers/protocol_handler_test.go` |
| richer mock integration script | 支持 | `scripts/e2e_gateway_rich_mock.sh`, `reports/2026-05-31-rosetta-rich-mock-e2e.md` |
| gateway smoke optional richer scenarios | 部分支持 | `scripts/e2e_gateway_smoke.sh`, `reports/2026-05-31-gateway-client-e2e.md` |
| expanded real-upstream smoke for `/v1/responses` + `/v1/messages` | 支持 | `reports/2026-06-01-gateway-smoke-expanded.md` |
| real-upstream structured output smoke | 支持 | `reports/2026-06-01-gateway-structured-vision-smoke.md` |
| real-upstream responses structured output smoke | 支持 | `reports/2026-06-01-gateway-structured-vision-smoke.md` |
| dedicated real-upstream responses-upstream smoke | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-smoke.md` |
| second-model real-upstream responses-upstream smoke (`gpt-5.2`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-smoke.md` |
| consolidated real-upstream follow-up matrix (`gpt-5.2`) | 支持 | `reports/2026-06-01-gateway-responses-upstream-gpt52-followup-matrix.md` |
| third-model real-upstream responses-upstream smoke (`gpt-5.3-codex`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt53codex-smoke.md` |
| real-upstream `previous_response_id + structured output` smoke (`gpt-5.2`) | 支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-structured-followup-smoke.md` |
| real-upstream `previous_response_id + structured output + stream` smoke (`gpt-5.2`) | 支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-structured-stream-followup-smoke.md` |
| real-upstream `previous_response_id + tool + stream` smoke (`gpt-5.2`) | 支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-tool-stream-followup-smoke.md` |
| real-upstream `prompt_cache_*` smoke (`gpt-5.2`) | 支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-smoke.md` |
| real-upstream `previous_response_id + prompt_cache_*` classification (`gpt-5.2`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-followup-smoke.md` |
| real-upstream `previous_response_id + include` smoke (`gpt-5.2`) | 支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt52-include-followup-smoke.md` |
| real-upstream `previous_response_id + structured output` classification (`gpt-5.4-mini`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt54mini-structured-smoke.md` |
| fourth-model real-upstream responses-upstream smoke (`gpt-5.4`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt54-smoke.md` |
| fifth-model real-upstream responses-upstream smoke (`gpt-5.4-mini`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt54mini-smoke.md` |
| sixth-model real-upstream responses-upstream smoke (`gpt-5.5`) | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-responses-upstream-gpt55-smoke.md` |
| real-upstream responses previous_response_id smoke | 支持 | `reports/2026-06-01-gateway-previous-response-smoke.md` |
| real-upstream responses previous_response_id stream smoke | 支持 | `reports/2026-06-01-gateway-previous-response-stream-smoke.md` |
| real-upstream responses previous_response_id + tool smoke | 支持 | `reports/2026-06-01-gateway-previous-response-tool-smoke.md` |
| real-upstream responses previous_response_id + tool stream smoke | 支持 | `reports/2026-06-01-gateway-previous-response-tool-stream-smoke.md` |
| real-upstream responses-upstream previous_response_id classification | 部分支持 | `scripts/e2e_gateway_responses_upstream_smoke.sh`, `reports/2026-06-01-gateway-previous-response-responses-upstream-smoke.md` |
| real-upstream vision smoke classification | 部分支持 | `reports/2026-06-01-gateway-structured-vision-smoke.md` |
| real-upstream reasoning stream smoke | 支持 | `reports/2026-06-01-gateway-reasoning-stream-smoke.md` |
| real-upstream file smoke classification | 部分支持 | `reports/2026-06-01-gateway-file-smoke.md` |
| full optional real-upstream smoke run | 支持 | `reports/2026-06-01-gateway-smoke-full.md` |
| real-upstream responses/messages tool-path smoke | 支持 | `reports/2026-06-01-gateway-toolpath-smoke.md` |
| real-upstream responses/messages tool-stream smoke | 支持 | `reports/2026-06-01-gateway-tool-stream-smoke.md` |
| max real-upstream smoke bundle | 支持 | `reports/2026-06-01-gateway-smoke-max.md` |

## 下一步回填点

- Phase 3：扩 non-stream response codec 到 richer output item / refusal / annotations / audio
- Phase 4：补 unified stream event 与 richer lifecycle
- Phase 6：把高级 item 结构写入 `/api/request-logs`
