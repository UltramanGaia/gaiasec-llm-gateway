# Rosetta Adapter Completion Audit

更新时间：2026-06-01

## 结论

当前仓库**不能**把 `plan/rosetta-gap-closure-plan.md` 视为完成。

已有大量实质进展，但根据当前工作树证据，以下范围仍未完全闭环：

- richer response 的跨协议映射仍不完整
- stream richer event 虽已显著推进，但统一 IR 接管与全协议覆盖仍不完整
- `annotations` stream 只有局部主路径证据，尚未形成稳定全覆盖
- richer response / stream 的 replay 与日志聚合仍不完整
- Phase 7 真实客户端/真实上游验证已增强，但 richer built-in tool、多轮场景与更广模型样本仍不足

因此，该目标当前状态应为：`进行中，而非完成`。

## 审计原则

本审计只基于当前工作树与当前可验证证据，不基于历史意图。

证据来源包括：

- 当前源码
- 当前测试
- 当前脚本
- 当前报告文档

## 分阶段审计

### Phase 0: 建立差距基线

状态：`完成`

证据：

- [plan/rosetta-gap-closure-plan.md](/home/icsl/gaiasec/gaiasec-llm-gateway/plan/rosetta-gap-closure-plan.md:1)
- [plan/rosetta-adapter-compatibility-matrix.md](/home/icsl/gaiasec/gaiasec-llm-gateway/plan/rosetta-adapter-compatibility-matrix.md:1)

### Phase 1: 扩展 IR 模型

状态：`部分完成`

已证明：

- request / response 的 richer 顶层字段已扩入 IR
- `IRTool` 已可承载 `raw_payload`
- `IRPart` 已承载 `refusal/annotations/audio/status/id/call_id`
- `IRStreamEvent` 已具备 richer stream 字段承载能力
- 已有最小 `Responses/Chat -> IRStreamEvent` 归一化 helper
- 已有 `Anthropic -> IRStreamEvent` 归一化 helper 与测试
- `Responses/Anthropic -> IRStreamEvent` 已开始把 `reasoning.* / annotation.added / tool_call.* / audio.*` 等 lifecycle 统一归一化为更稳定的 IR 事件类型，并保留原始 provider event type
- `response.output_text.delta` 现在也已明确归一化到统一 IR `output_text.delta`
- `Responses response.output_item.done` 里自带的最终 tool args / reasoning summary 现在也会直接提升到统一 IR 字段，而不只留在 raw item
- `responses -> chat stream`、`chat -> responses stream`、`responses -> anthropic stream`、`anthropic -> responses stream`、`chat -> anthropic stream`、`anthropic -> chat stream` 主路径已开始消费 IR 事件归一化结果
- `Responses -> Chat/Anthropic` 的 `output_item.done` 分支现在也已优先消费 IR 提升出的最终 args/summary，而不再主要依赖 raw item
- chat-like stream 聚合/日志辅助逻辑也已开始消费 IR 事件归一化结果
- Responses replay fallback 聚合也已开始优先消费 `IRStreamEventsFromResponsesFrame`
- `buildOpenAIStreamLogResponse` 里的 provider-specific chunk struct 解析也已收敛到通用 map + IR 事件聚合
- Anthropic 输出包装层的 token 计量也已开始识别 richer `content_block_start`
- `chat -> anthropic` 与 `responses -> anthropic` 两条输出包装路径都已共用 richer anthropic frame token 检测
- `anthropicFrameHasOutputToken` 也已优先消费 `IRStreamEventsFromAnthropicFrame`

证据：

- [protocol/types.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/types.go:1)
- [protocol/types_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/types_test.go:1)
- [protocol/stream_ir.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_ir.go:1)
- [protocol/stream_ir_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_ir_test.go:1)

未完成点：

- `IRStreamEvent` 已进入六条关键 stream codec 主路径，并继续进入 replay / 日志等外围聚合逻辑；但仍未统一接管所有包装与聚合逻辑
- plan 中列出的 `reasoning_start/reasoning_done/annotation_added/tool_call_start/tool_call_done` 已有部分统一落到 IR 层；但仍未覆盖所有 richer event 与全部外围逻辑

### Phase 2: 补齐 Request Codec

状态：`大部分完成`

已证明：

- Chat / Responses / Anthropic richer request fields 已能 decode/encode 保留
- built-in Responses tools 已可保留 raw payload
- unsupported cross-protocol capability 已有前置拒绝
- `previous_response_id` 对非 Responses upstream 也已有前置拒绝，且 handler 级已验证不会静默丢语义
- `prompt_cache_*` 对非 Responses upstream 也已有前置拒绝，且 handler 级已验证不会静默丢语义
- `include/store/background/conversation/prompt` 对非 Responses upstream 也已有前置拒绝，且 handler 级已验证不会静默丢语义

证据：

- [protocol/request_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/request_codec.go:1)
- [protocol/request_codec_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/request_codec_test.go:1)
- [handlers/protocol_capabilities.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/protocol_capabilities.go:1)
- [handlers/protocol_capabilities_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/protocol_capabilities_test.go:1)

未完成点：

- “自动降级还是拒绝”的细粒度策略仍未完全覆盖所有 richer tools / request semantics

### Phase 3: 补齐 Non-Stream Response Codec

状态：`部分完成`

已证明：

- Responses 顶层 `status/metadata/error/incomplete_details/conversation/prompt/text/reasoning/tool_choice/tools` 已 same-protocol 保留
- Responses richer output item `custom_tool_call/mcp_call/web_search_call/image_generation_call/compaction` 已能进入 IR 并 same-protocol round-trip
- Chat `refusal/audio` 与 Responses `annotations/refusal/output_audio` 已有 non-stream 保留测试
- `custom_tool_call` 的 `responses -> chat` / `responses -> anthropic` handler 级 mock integration 已有证据
- `Anthropic tool_use -> Responses function_call` handler 级 mock integration 已有证据
- `mcp_call` / `web_search_call` / `image_generation_call` 也已有更多 handler 级跨协议证据
- `Chat structured richer message content (annotations/refusal/audio) -> Responses/Anthropic` non-stream 已有 codec 与 handler 级证据
- `Responses richer message content (annotations/refusal/audio) -> Anthropic` non-stream 已有 codec 与 handler 级证据
- `Anthropic structured richer message content (annotations/refusal/audio) -> Chat/Responses` non-stream 已有 codec 与 handler 级证据
- `responses -> chat` non-stream 在 `reasoning item` 先于 `message` 时也已有回归修复与测试
- `responses -> chat` non-stream 的主消息选择现在不再受 `reasoning item` 排序影响
- richer mock integration 独立脚本已覆盖上述主要非流式 richer 路径
- richer mock integration 独立脚本也已覆盖 richer tool-call stream 的关键跨协议路径

证据：

- [protocol/response_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/response_codec.go:1)
- [protocol/response_codec_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/response_codec_test.go:1)
- [protocol/nonstream_golden_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/nonstream_golden_test.go:1)

未完成点：

- richer response 的跨协议映射仍不完整，但已有更多 handler 级降级/映射证据；覆盖面仍不足
- `annotations` 的跨协议表达已不只停留在同协议保留与局部 chat 路径；`chat -> responses`、`chat -> anthropic`、`responses -> anthropic`、`anthropic -> chat`、`anthropic -> responses` non-stream 都已有保留证据，但仍未形成稳定全覆盖映射

### Phase 4: 补齐 Stream Codec

状态：`部分完成`

已证明：

- text delta / function tool call delta 主路径已支持
- refusal stream 已补主路径
- audio stream 已补 Chat <-> Responses 与 stream log 聚合主路径
- Anthropic `thinking` -> Responses reasoning stream 已补主路径
- richer `*_call` lifecycle 已补一部分
- Anthropic `tool_use` -> Responses `function_call` stream 现在已有 `output_item.added` / `arguments.delta` / `arguments.done` / `output_item.done` 完整生命周期，并会累积参数到 done item
- Anthropic `tool_use` 如果在 `content_block_start` 里就已携带完整 `input`，现在也能直接进入 Chat/Responses stream，而不再要求后续 `input_json_delta`
- `tool_use start-only input` 在 Anthropic -> Responses 路径下不再过早重复发 `arguments.done`；现在是 added 带参数、stop 时只收尾一次
- Anthropic `thinking` 如果在 `content_block_start` 里就已携带非空文本，现在也能直接进入 Chat/Responses stream，而不再要求后续 `thinking_delta`
- Anthropic `thinking start-only text` 在 Responses 路径下，`output_item.added.summary` 现在也会和后续 reasoning 文本保持一致，不再是空 summary
- Responses `reasoning/tool output_item.done` -> Anthropic `content_block_stop` 也已有更贴近 provider 语义的即时收尾，不再只依赖 `response.completed` 统一 flush
- Responses `function_call_arguments.done` -> Chat stream 也已不再只是 finish 辅助信号，而会实际发出最终 `tool_calls[].function.arguments` chunk
- Responses `function_call_arguments.done` -> Anthropic stream 现在也会在 only-done 场景下发出最终 `input_json_delta`，不再要求先见到完整 delta
- Responses `output_item.added` 自带的初始 tool args 现在也能直接进入 Chat `tool_calls[].function.arguments`
- Responses `output_item.added` 自带的初始 reasoning summary 现在也能直接进入 Chat `reasoning_content`
- Responses `output_item.added` 自带的初始 tool args 现在也能直接进入 Anthropic `tool_use input`
- Responses `output_item.added` 自带的初始 reasoning summary 现在也能直接进入 Anthropic `thinking` block
- Responses `output_item.done` 中自带的最终 tool args，现在也已能直接进入 Chat `tool_calls[].function.arguments` 与 Anthropic `input_json_delta`
- Responses `output_item.done` 中自带的最终 reasoning summary，现在也已能直接进入 Chat `reasoning_content` 与 Anthropic `thinking` stream
- Responses `output_item.added.item.content` 中自带的 start-only `output_text/annotations/refusal/audio`，现在也能直接进入 Chat/Anthropic stream，而不再要求后续 delta/content_part
- Responses -> Chat 在裸 `[DONE]` 且缺少 `response.completed` 时，现在也会补最终 finish chunk，不再静默结束
- Responses -> Chat 的 `[DONE]` flush 现在也会保留已缓存 usage，不再只补 finish chunk
- Anthropic -> Chat 在 `message_stop` 且缺少 `message_delta` 时，现在也会补最终 finish chunk，不再静默结束
- Anthropic -> Responses 在 `message_stop` 且缺少 `message_delta` 时，现在也会补最终 `output_item.done` / `response.completed` 生命周期
- Responses -> Anthropic 在裸 `[DONE]` 且缺少 `response.completed` 时，现在也会根据 tool block 推断 `stop_reason=tool_use`
- Anthropic -> Responses handler 侧也已避免在已有 `response.completed` 时重复补写 completed
- `Anthropic richer text-block stream (annotations/refusal/audio) -> Chat/Responses` 已有主路径证据
- `Responses/Chat richer text stream (annotations/refusal/audio) -> Anthropic` 也已有 protocol + handler 主路径证据

证据：

- [protocol/stream_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_codec.go:1)
- [protocol/stream_stateful_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_stateful_codec.go:1)
- [protocol/stream_anthropic_codec.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_anthropic_codec.go:1)
- [protocol/stream_codec_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_codec_test.go:1)
- [protocol/stream_stateful_codec_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_stateful_codec_test.go:1)
- [protocol/stream_anthropic_codec_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/protocol/stream_anthropic_codec_test.go:1)
- [handlers/protocol_handler_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/protocol_handler_test.go:1)

未完成点：

- `annotations/refusal/audio` 的 stream 主跨协议路径证据已明显增强，但 same-protocol 与长尾 richer item/finish lifecycle 仍未全覆盖
- richer reasoning event 统一层仍不完整
- `IRStreamEvent` 已能统一表达更多 `reasoning/tool/annotation/audio` lifecycle，但尚未完全承担所有 stream 包装/聚合的唯一中间层
- 三协议间 richer stream parity 仍未完全证明

### Phase 5: Capability Guard 升级

状态：`大部分完成`

已证明：

- 新 capability flags 已入后端模型
- `/api/model-configs` API 与前端配置页已接入
- request capability 推导已覆盖 `web_search/mcp/code_interpreter/image_generation/prompt_cache`
- request capability 推导已覆盖 `audio_output`

证据：

- [models/model_mapping.go](/home/icsl/gaiasec/gaiasec-llm-gateway/models/model_mapping.go:1)
- [handlers/model_mapping_handler.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/model_mapping_handler.go:1)
- [handlers/model_mapping_handler_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/model_mapping_handler_test.go:1)
- [frontend/src/views/ModelConfig.vue](/home/icsl/gaiasec/gaiasec-llm-gateway/frontend/src/views/ModelConfig.vue:1)
- [handlers/protocol_capabilities.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/protocol_capabilities.go:1)
- `cd frontend && npm run build`

未完成点：

- 更细的自动降级/原样透传策略仍未完整证明

### Phase 6: Logging 与可观测性补强

状态：`部分完成`

已证明：

- `/api/request-logs` 已有 richer semantic summary
- 可见 `tool subtype/reasoning/refusal/annotation_count/audio`
- `Anthropic` richer content 的 `annotations/refusal/audio` 也已进入 semantic summary
- Responses semantic summary 的 `ReasoningSummary` 现在只来自真正的 `reasoning/compaction` item，不再误把普通 assistant `output_text` 当作 reasoning
- stream 日志聚合已保留 `reasoning/tool_calls/refusal/audio`
- chat-like stream 日志聚合在 only-`[DONE]` 且缺少最终 finish chunk 时，也已能根据 tool call 轨迹推断 `finish_reason=tool_calls`
- `Anthropic -> Responses` stream 的 richer `reasoning/annotation/refusal/audio/arguments.done` 事件现在也会被计入输出 token 观测
- `Anthropic -> Responses` stream 中携带实际输出的 `response.output_item.added/done`（如 start-only tool args / reasoning summary）现在也会计入输出 token 观测
- `Chat -> Responses` stream 现在也不再把 role-only / metadata-only chunk 误计为输出 token，而是只在转换后事件真正携带输出时记 token
- `Anthropic -> Chat` 的 start-only `thinking` 文本现在也会计入输出 token 观测
- richer same-protocol replay 已有基础验证证据
- replay 的 chat-like stream 聚合已可保留 `reasoning/refusal/audio/annotations`
- replay 的 Responses stream 已可直接保留 `response.completed` 完整 payload
- replay 的 Anthropic stream 已可聚合为 anthropic message-like payload，并有 richer content `annotations/refusal/audio` 保留证据
- replay 的 Responses stream 在缺少 `response.completed` 时已不只重建 text/tool args，也能重建 richer message content 的 `annotations/refusal/audio`
- Responses replay fallback 的 `output_item/content_part` 也已开始优先消费 IR item/part
- Responses replay fallback 对 `file_search_call` 这类 built-in tool 也已补上 `input/arguments` 重建证据
- Responses replay fallback 对 only-`function_call_arguments.done` 的 `function_call` 也已补上最终 `arguments` 重建证据
- Responses replay fallback 在同时出现 partial `arguments.delta` 与最终 `arguments.done` 时，也已改为优先保留最终完整参数
- Responses replay fallback 对 only-`response.reasoning.delta` 的 `reasoning` item 也已补上 summary 重建证据
- Responses replay fallback 在缺少 `response.completed` 时，顶层 `status` 也已收敛为 `completed`
- Responses replay fallback 在缺少 `response.output_item.done` 时，重建出的 item `status` 也已收敛为 `completed`
- Responses replay fallback 对 only-`response.output_item.added.item.content` 的 start-only message 现在也会补上顶层 `output_text`
- Responses replay fallback 在 start-only richer message content 后又收到后续 text delta 时，现在也不再覆盖丢失既有 richer parts，且会把 text 正确回填到已存在 part
- Responses replay fallback 现在也按 `content_index` 维度独立重建多个 `output_text` part，不再把同一 message 的多个 text part 串错
- Responses replay fallback 现在也会按 `content_index` 正确落位 `response.annotation.added`，不再默认挂到第一个 text part
- Responses replay fallback 对同一 message 的 `output_text/annotation/refusal/audio` 多 part 组合现在也已有更强的独立落位证据
- Anthropic replay 聚合也已开始优先消费 `IRStreamEventsFromAnthropicFrame`
- Anthropic replay 的 delta 聚合也已开始优先消费 IR delta 事件，而不只依赖 raw `delta.type`
- Anthropic replay 的 `content_block_start` 也已优先消费 IR item/block，而不只依赖 raw block payload
- Anthropic replay 的 `message_start` 元数据也已优先消费 IR `item_id/usage`
- Anthropic replay 在 only-`message_stop` 且缺少 `message_delta` 时，也已补上 `stop_reason` 推断
- Anthropic replay 对 `tool_use start-only input` 现在也已有明确保留证据
- Anthropic replay 在缺少 `message_delta/message_stop` 的截断 tool_use 场景下，也已补上 `stop_reason` 兜底推断
- Anthropic replay 现在也能重建合法的空 assistant message（`content: []`），不再因为无 content blocks 直接失败
- Anthropic replay 对 `thinking start-only text` 现在也已有明确保留证据

证据：

- [handlers/log_handler.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/log_handler.go:1)
- [handlers/log_handler_test.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/log_handler_test.go:1)
- [handlers/chat_response.go](/home/icsl/gaiasec/gaiasec-llm-gateway/handlers/chat_response.go:1)

未完成点：

- richer item replay 仍只有 same-protocol 为主的证据，覆盖面不够
- stream 聚合仍非完整 richer 结构，但 Responses 与 Anthropic 的 same-protocol replay 已明显补强，且 Responses fallback 重建已覆盖更多 richer message content

### Phase 7: E2E 与真实客户端验证

状态：`部分完成`

已证明：

- `go test ./...` 通过
- richer mock integration 已补独立脚本
- 已有真实客户端报告
- `/v1/responses` / `/v1/messages` 的 expanded real-upstream smoke 已实跑通过

证据：

- [scripts/e2e_gateway_rich_mock.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_rich_mock.sh:1)
- [scripts/e2e_gateway_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_smoke.sh:1)
- [scripts/e2e_gateway_responses_upstream_smoke.sh](/home/icsl/gaiasec/gaiasec-llm-gateway/scripts/e2e_gateway_responses_upstream_smoke.sh:1)
- [reports/2026-05-31-gateway-client-e2e.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-05-31-gateway-client-e2e.md:1)
- [reports/2026-05-31-rosetta-rich-mock-e2e.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-05-31-rosetta-rich-mock-e2e.md:1)
- [reports/2026-06-01-gateway-smoke-expanded.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-smoke-expanded.md:1)
- [reports/2026-06-01-gateway-structured-vision-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-structured-vision-smoke.md:1)
- [reports/2026-06-01-gateway-reasoning-stream-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-reasoning-stream-smoke.md:1)
- [reports/2026-06-01-gateway-file-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-file-smoke.md:1)
- [reports/2026-06-01-gateway-previous-response-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-previous-response-smoke.md:1)
- [reports/2026-06-01-gateway-previous-response-stream-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-previous-response-stream-smoke.md:1)
- [reports/2026-06-01-gateway-previous-response-tool-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-previous-response-tool-smoke.md:1)
- [reports/2026-06-01-gateway-previous-response-tool-stream-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-previous-response-tool-stream-smoke.md:1)
- [reports/2026-06-01-gateway-previous-response-responses-upstream-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-previous-response-responses-upstream-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-followup-matrix.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-followup-matrix.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-structured-followup-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-structured-followup-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-structured-stream-followup-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-structured-stream-followup-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-tool-stream-followup-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-tool-stream-followup-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-followup-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-prompt-cache-followup-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt52-include-followup-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt52-include-followup-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt53codex-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt53codex-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt54-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt54-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt54mini-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt54mini-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt54mini-structured-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt54mini-structured-smoke.md:1)
- [reports/2026-06-01-gateway-responses-upstream-gpt55-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-responses-upstream-gpt55-smoke.md:1)
- [reports/2026-06-01-gateway-smoke-full.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-smoke-full.md:1)
- [reports/2026-06-01-gateway-toolpath-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-toolpath-smoke.md:1)
- [reports/2026-06-01-gateway-tool-stream-smoke.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-tool-stream-smoke.md:1)
- [reports/2026-06-01-gateway-smoke-max.md](/home/icsl/gaiasec/gaiasec-llm-gateway/reports/2026-06-01-gateway-smoke-max.md:1)
- `go test ./...`

未完成点：

- real-upstream smoke 已覆盖三入口的基础 non-stream/stream 主路径、chat/responses structured output、responses `previous_response_id` non-stream/stream follow-up、responses `previous_response_id + tool` non-stream/stream follow-up、tool-path（含 stream），并对 vision/file 给出真实结果；最新 `max6` bundle 已全量实跑通过
- 但在当前实现真正允许的 `openai_responses` upstream 路由下，专用 real-smoke 已证明：baseline `/v1/responses` non-stream/stream 能通过，而 `previous_response_id` 的四类 follow-up 变体都会稳定分类为 `failed_upstream`，当前上游返回 `404 not found`
- 第二个模型样本 `gpt-5.2` 的专用 `openai_responses` real-smoke 也已补上，证明 `previous_response_id` follow-up 的真实行为会随模型/提供方组合变化
- `gpt-5.2` 的 richer follow-up 结果现在也已收束成一份单独矩阵报告，便于整体判断其多轮能力边界
- `gpt-5.2` 的专用 real-smoke 还补出了多条 richer 多轮成功案例：`previous_response_id + tool + stream`、`previous_response_id + structured output`（non-stream / stream）均可通过
- `gpt-5.2` 的专用 real-smoke 还补出了 `previous_response_id + include` 的真实成功案例
- `gpt-5.2` 的专用 real-smoke 还补出了 `prompt_cache_key/prompt_cache_retention` 的真实成功案例
- `gpt-5.2` 的专用 real-smoke 也补出了一条多轮请求语义限制案例：`previous_response_id + prompt_cache_*` 当前会被上游拒绝
- 第三个模型样本 `gpt-5.3-codex` 的专用 `openai_responses` real-smoke 也已补上，继续证明 `previous_response_id` follow-up 的真实行为会随模型/提供方组合变化
- 第四个模型样本 `gpt-5.4` 的专用 `openai_responses` real-smoke 也已补上，继续证明 `previous_response_id` follow-up 的真实行为会随模型/提供方组合变化
- 第五个模型样本 `gpt-5.4-mini` 的专用 `openai_responses` real-smoke 也已补上，继续证明 `previous_response_id` follow-up 的真实行为会随模型/提供方组合变化
- `gpt-5.4-mini` 的专用 real-smoke 还补出了一条对照样本：`previous_response_id + structured output` 在 non-stream / stream 都会分类为 `failed_upstream`
- 第六个模型样本 `gpt-5.5` 的专用 `openai_responses` real-smoke 也已补上，进一步强化了“更多模型样本”这一维度的真实证据
- 但 richer built-in tool 的更广真实上游场景、更多多轮场景与更广模型样本仍不足

## 当前最关键未完成项

按 plan 目标与现状对齐，当前最关键的未完成项是：

1. `annotations` 的 response/stream 语义主跨协议路径已明显补强，但仍未达到完全覆盖
2. richer stream event 虽已进入 IR，但尚未完全统一接管
3. richer response / stream 的跨协议映射仍有明显空洞
4. richer replay 与更强 E2E 证据仍不足

## P0/P1/P2 缺口核对

下表直接对应 [rosetta-gap-closure-plan.md](/home/icsl/gaiasec/gaiasec-llm-gateway/plan/rosetta-gap-closure-plan.md:1) 中列出的明确缺口。

状态定义：

- `已完成`：当前工作树与证据足以证明该项已满足阶段目标
- `部分完成`：已有实现与证据，但仍有明显边界、协议对或验证空洞
- `未完成`：当前证据仍不足以证明已满足阶段目标

### P0

| 缺口 | 当前状态 | 依据 |
| --- | --- | --- |
| Responses 非 function 工具类型未打通 | `部分完成` | 已有 `custom_tool_call/mcp_call/web_search_call/image_generation_call` 的 IR、handler mock 与部分 stream/mock evidence；但更广 built-in tool 与真实上游覆盖仍不足 |
| Responses output item 类型过窄 | `部分完成` | 已补 `custom_tool_call/mcp_call/web_search_call/image_generation_call/compaction` 的 same-protocol 与部分跨协议证据；仍非全覆盖 |
| 顶层 request provider fields 未统一保留与回放 | `部分完成` | Responses/Chat/Anthropic 主要字段已保留；更细 provider-specific 语义与跨协议回放仍未完全证明 |

### P1

| 缺口 | 当前状态 | 依据 |
| --- | --- | --- |
| `refusal` 未作为一等响应语义建模 | `部分完成` | non-stream、stream 与日志层均已有主路径证据；其余协议对和更广真实场景仍不足 |
| `annotations` / citation 未建模 | `部分完成` | non-stream、`responses -> chat stream`、日志聚合已有证据；全协议对覆盖仍不足 |
| Responses / Chat audio 输出未建模 | `部分完成` | non-stream、`chat <-> responses stream` 与日志聚合已有证据；更广真实场景不足 |
| reasoning 的 response/stream 语义仍偏最小实现 | `部分完成` | `reasoning/thinking` 的 non-stream、stream、real smoke 已显著补强；统一 richer event 生命周期仍不完整 |
| structured output 在 response/stream 侧缺少更强约束与测试 | `部分完成` | chat/responses non-stream 与真实 upstream structured output 已有证据；`gpt-5.2` 上的 `previous_response_id + structured output` stream 也已有真实成功案例，但整体 stream structured coverage 仍不足 |

### P2

| 缺口 | 当前状态 | 依据 |
| --- | --- | --- |
| 多模态 stream 语义链不完整 | `未完成` | vision/file 真实 upstream 已分类为当前模型限制，但多模态 stream 本身仍无完整语义链证据 |
| richer stream event 类型未建模 | `部分完成` | `IRStreamEvent` 与 `stream_ir.go` 已落地，并已把 `reasoning/tool/annotation/audio` 的一部分 lifecycle 统一归一化；但未完全统一接管所有包装/聚合与更细事件类型 |
| 日志层还未针对高级 item 类型做结构化保留 | `部分完成` | semantic summary 已可见较多 richer 语义；非 chat-like richer item 与 replay/stream 聚合仍未全覆盖 |

## 当前可支撑的结论

当前仓库可以支撑以下结论：

- 该目标**已取得大幅进展**
- 请求、非流式响应、关键流式主路径已有较强证据
- 但还**不能**声称 `plan/rosetta-gap-closure-plan.md` 已完成

如果后续要宣称完成，至少还需要补：

- `annotations` response/stream 的更完整跨协议实现或显式拒绝策略
- richer stream event 的剩余空洞
- Phase 7 计划项的更完整验证证据
