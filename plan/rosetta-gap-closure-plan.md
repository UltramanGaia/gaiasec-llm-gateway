# Rosetta Gap Closure Plan

## 目标

在不破坏当前 `chat` / `responses` / `messages` 主路径可用性的前提下，把 `gaiasec-llm-gateway` 从“基础三协议互转”推进到更接近 `llm-rosetta` 的语义覆盖面。

本计划聚焦当前已确认的具体缺口：

- OpenAI Responses 高级 output item / tool 类型缺失
- 顶层 request/response 元数据透传不足
- `refusal` / `annotations` / `audio` 等 richer 语义未建模
- stream 侧仅覆盖文本与 function tool call 主路径
- Anthropic / Chat / Responses 的高级 provider 字段仍未接入

约束：

- 正式 AI 配置入口仍是 `/api/model-configs`
- 正式日志入口仍是 `/api/request-logs`
- 第一优先级是“语义保留或显式拒绝”，不是盲目扩字段

## 当前差距摘要

### 已具备

- 三个公共入口已统一接入协议分发
- `chat <-> responses <-> messages` 主路径已可用
- function tools、基础 reasoning、image/file 主路径已支持
- 主要非流式与关键流式路径已有测试

### 仍缺失

#### P0 缺口

- Responses 非 function 工具类型未打通：
  - `web_search`
  - `file_search`
  - `computer`
  - `code_interpreter`
  - `image_generation`
  - `local_shell`
  - `shell`
  - `custom`
  - `apply_patch`
  - `mcp`
- Responses output item 类型过窄：
  - 仅支持 `message` / `function_call` / `reasoning`
  - 缺少 `custom_tool_call` / `mcp_call` / `web_search_call` / `compaction` 等
- 顶层 request provider fields 未形成统一保留与回放：
  - Responses: `previous_response_id`, `include`, `metadata`, `service_tier`, `store`, `background`, `conversation`, `prompt`, `prompt_cache_*`
  - Chat: `metadata`, `service_tier`, `modalities`, `audio`, `prediction`, `verbosity`, `web_search_options`, `logprobs`, `top_logprobs`, `seed`, `n`, `frequency_penalty`, `presence_penalty`, `logit_bias`
  - Anthropic: `metadata`, `service_tier`, `top_k`, `cache_control`

#### P1 缺口

- `refusal` 未作为一等响应语义建模
- `annotations` / citation 未建模
- Responses / Chat 的 audio 输出未建模
- reasoning 的 response/stream 语义仍偏最小实现
- structured output 在 response/stream 侧缺少更强约束与测试

#### P2 缺口

- 多模态 stream 语义链不完整
- richer stream event 类型未建模
- 日志层还未针对高级 item 类型做结构化保留

## 总体策略

### 原则 1：先扩 IR，再扩 codec

当前主要瓶颈不是 handler，而是 `protocol/types.go` 中的 `IRRequest` / `IRResponse` / `IRPart` / `IRTool` 表达能力不足。

先扩 IR，后改：

- request codec
- non-stream response codec
- stream codec
- handler 日志归一化

### 原则 2：先保留语义，再做跨协议等价映射

分两层推进：

1. 先让 IR 能保住 richer 字段，不在 decode 阶段丢掉
2. 再逐步补齐跨协议 encode 规则

### 原则 3：不能安全映射时显式降级或拒绝

对于目标协议没有等价表达的语义：

- 能保存在 `provider_extensions` 的先保留
- 能以近似语义降级的要明确规则
- 不能安全降级的返回显式错误，不静默吞字段

## 分阶段计划

## Phase 0: 建立差距基线

状态：`已完成（矩阵已建立，后续 phases 需持续回写）`

目标：

- 把当前缺口固化成可验证矩阵，避免后续“做了很多但不知道补到了哪”

工作项：

- 新增“Rosetta 差距矩阵”文档，按 request / response / stream 三层列出：
  - 已支持
  - 已部分支持
  - 未支持
  - 明确拒绝
- 为 Responses 高级类型建立最小样例 fixture 清单

涉及文件：

- `plan/rosetta-gap-closure-plan.md`
- `plan/rosetta-adapter-compatibility-matrix.md`
- `protocol/*_test.go`

验收标准：

- 可以准确指出每个高级字段目前处于哪种状态
- 后续每个 phase 都能回写矩阵

## Phase 1: 扩展 IR 模型

状态：`进行中（IR 已扩 request/response 顶层字段、tool raw payload 与 richer stream event 字段；统一 stream IR 事件层尚未真正接管 codec）`

目标：

- 让 IR 足以承载 Rosetta 侧更丰富的 request / response / stream 语义

工作项：

- 扩展 `IRTool`
  - 增加 richer tool metadata
  - 至少允许表达 tool subtype 与 subtype-specific raw payload
- 扩展 `IRPart`
  - 增加：
    - `Refusal`
    - `Annotations`
    - `Audio`
    - `Status`
    - `ID`
    - `CallID`
    - `ProviderName`
    - `EncryptedContent`
- 扩展 `IRRequest`
  - 增加 request 级 `ProviderExtensions` 约定字段
  - 增加 `Include`, `Metadata`, `ServiceTier`, `Store`, `Conversation`, `PromptCache`
- 扩展 `IRResponse`
  - 增加：
    - `Status`
    - `Error`
    - `IncompleteDetails`
    - `Metadata`
    - `TextConfig`
    - `ReasoningConfig`
    - `Conversation`
    - `Prompt`
- 扩展 `IRStreamEvent`
  - 支持 richer event types：
    - `reasoning_start`
    - `reasoning_delta`
    - `reasoning_done`
    - `refusal_delta`
    - `annotation_added`
    - `audio_delta`
    - `tool_call_start`
    - `tool_call_delta`
    - `tool_call_done`

涉及文件：

- `protocol/types.go`

验收标准：

- 不修改 encode/decode 逻辑的前提下，IR 定义已能表达 P0 缺口所需语义
- 为新增字段补充类型层单测或 round-trip smoke test

## Phase 2: 补齐 Request Codec

状态：`进行中（request codec 已保留 richer provider fields、built-in tool raw payload 与 Anthropic block-level cache_control；跨协议映射仍待继续细化）`

目标：

- 把 richer request 字段从三协议请求体正确解码到 IR，并在支持时回写到目标协议

工作项：

- Chat request:
  - 接入 `metadata`
  - 接入 `service_tier`
  - 接入 `logprobs` / `top_logprobs`
  - 接入 `seed` / `n`
  - 接入 `frequency_penalty` / `presence_penalty` / `logit_bias`
  - 保留 `audio` / `modalities` / `prediction` / `verbosity` / `web_search_options`
- Responses request:
  - 接入 `previous_response_id`
  - 接入 `include`
  - 接入 `metadata`
  - 接入 `service_tier`
  - 接入 `store`
  - 接入 `background`
  - 接入 `conversation`
  - 接入 `prompt_cache_key` / `prompt_cache_retention`
  - 接入 `prompt`
- Anthropic request:
  - 接入 `metadata`
  - 接入 `service_tier`
  - 接入 `top_k`
  - 保留 block-level `cache_control`
- tool 解析升级：
  - 不再只保留 `function`
  - Responses 非 function tools 先保留 subtype 与原始 payload
  - 对暂不支持跨协议转换的 subtype，先在 capability / dispatch 阶段显式限制

涉及文件：

- `protocol/request_codec.go`
- `handlers/protocol_capabilities.go`
- `handlers/protocol_handler_test.go`
- `protocol/request_codec_test.go`

验收标准：

- decode 后不会静默丢失上述字段
- same-protocol round-trip 能保留字段
- unsupported cross-protocol path 能显式报错或显式降级

## Phase 3: 补齐 Non-Stream Response Codec

状态：`进行中（已接入 status/metadata/error/incomplete_details 与部分 richer output item；conversation/prompt/text/reasoning/tools/tool_choice 已保留到 same-protocol，跨协议 richer 映射仍待继续）`

目标：

- 把 richer response item 和顶层元数据纳入 IR，并在可表达的目标协议中编码出来

工作项：

- Chat response:
  - 解码 `refusal`
  - 解码 `annotations`
  - 解码 `audio`
- Responses response:
  - 解码顶层：
    - `status`
    - `metadata`
    - `error`
    - `incomplete_details`
    - `conversation`
    - `prompt`
    - `reasoning`
    - `text`
    - `tool_choice`
    - `tools`
  - 解码 output items：
    - `custom_tool_call`
    - `mcp_call`
    - `web_search_call`
    - `image_generation_call`
    - `compaction`
    - richer reasoning content
- Anthropic response:
  - 更完整保留 `thinking` 相关字段
  - 保留 richer content block raw payload
- 编码策略：
  - Chat 目标协议优先编码：
    - `content`
    - `reasoning_content`
    - `tool_calls`
    - `refusal`
    - `annotations`
  - Responses 目标协议优先保留原始 item 语义
  - Anthropic 目标协议尽量映射成：
    - `text`
    - `thinking`
    - `tool_use`
    - 必要时在 `provider_extensions` 留痕

涉及文件：

- `protocol/response_codec.go`
- `protocol/response_codec_test.go`
- `protocol/nonstream_golden_test.go`

验收标准：

- richer response 不再在 decode 阶段直接丢失
- 至少 same-protocol round-trip 保真
- `refusal` / `annotations` / richer reasoning 有测试证据

## Phase 4: 补齐 Stream Codec

状态：`进行中（已补 refusal/audio/reasoning 主路径、部分 richer *_call lifecycle，并新增最小 stream -> IR 归一化 helper，且六条关键 stream 主路径已开始消费 IR 事件；annotation stream 的其余协议对与更完整三协议对齐仍待继续）`

目标：

- 把当前“文本 + function tool call”为主的流式转换，升级为更接近 Rosetta 的统一流式事件层

工作项：

- Chat SSE -> IRStreamEvent:
  - 接入 `reasoning_content` delta
  - 接入可能的 `refusal` delta
  - 接入 richer usage / finish metadata
- Responses SSE -> IRStreamEvent:
  - 接入 richer `response.output_*`
  - 接入 reasoning events
  - 接入 non-message item events
  - 接入 refusal / annotation / audio 相关事件
- Anthropic SSE -> IRStreamEvent:
  - 完整建模 `thinking` block 生命周期
  - 保留 `tool_use` / `tool_result` / message delta 更细粒度状态
- IRStreamEvent -> target SSE:
  - `chat -> responses`
  - `responses -> chat`
  - `responses -> anthropic`
  - `anthropic -> responses`
  - `chat -> anthropic`
  - `anthropic -> chat`

重点优先顺序：

1. reasoning stream
2. refusal stream
3. richer tool-call stream
4. annotation/audio stream

涉及文件：

- `protocol/stream_codec.go`
- `protocol/stream_stateful_codec.go`
- `protocol/stream_anthropic_codec.go`
- `protocol/stream_codec_test.go`
- `protocol/stream_stateful_codec_test.go`
- `protocol/stream_anthropic_codec_test.go`
- `protocol/stream_fixtures_test.go`

验收标准：

- `reasoning` stream 在三协议间至少主路径可验证
- richer tool-call stream 不丢调用 ID / status / arguments lifecycle
- 关键路径有 fixture 或 golden tests

## Phase 5: Capability Guard 升级

状态：`进行中（capability guard、后端模型能力字段与 /api/model-configs UI 已补齐；更细的自动降级/拒绝策略和更多能力推导仍待继续）`

目标：

- 把“是否支持高级语义”的判断前移，避免运行时半途丢语义

工作项：

- 扩展 `ModelConfig` capability flags，新增候选字段：
  - `supports_refusal`
  - `supports_annotations`
  - `supports_audio_output`
  - `supports_web_search`
  - `supports_mcp`
  - `supports_code_interpreter`
  - `supports_image_generation`
  - `supports_prompt_cache`
- request capability 推导逻辑升级：
  - 能识别 richer tools
  - 能识别 audio / web_search / prompt_cache / previous_response_id
- dispatch 前执行：
  - 显式拒绝
  - 自动降级
  - 原样透传

涉及文件：

- `models/model_mapping.go`
- `handlers/model_mapping_handler.go`
- `frontend/src/views/ModelConfig.vue`
- `handlers/protocol_capabilities.go`
- `handlers/model_mapping_handler_test.go`

验收标准：

- 对用户请求的高级能力能做出可解释的前置判定
- 不再把明显不兼容请求转发到不支持的 upstream 再等 400

## Phase 6: Logging 与可观测性补强

状态：`进行中（/api/request-logs 已增加 richer semantic summary；stream 聚合仅部分保留 richer 结构，replay 验证仍待继续）`

目标：

- 让高级语义在 `/api/request-logs` 中可见，便于问题定位与回放

工作项：

- 日志归一化结果增加：
  - refusal
  - reasoning summary
  - tool subtype
  - annotations
  - richer finish details
- stream 日志聚合器升级：
  - 不只拼文本
  - 能保留高级 item 结构
- replay 能力评估：
  - 对 same-protocol richer payload 的 replay 验证

涉及文件：

- `models/request_log.go`
- `handlers/chat_response.go`
- `handlers/log_handler.go`
- `handlers/log_handler_test.go`

验收标准：

- request logs 可以反映 richer output item 类型
- 至少能从日志中分辨 function call、custom tool call、reasoning、refusal

## Phase 7: E2E 与真实客户端验证

状态：`进行中（已补 richer mock integration，并扩展 `scripts/e2e_gateway_smoke.sh` 以支持 structured output / image-file non-stream 的可选 smoke；真实客户端与更多上游样本仍待继续）`

目标：

- 确保新增能力不会破坏已有 Codex / Claude / curl 主路径

工作项：

- 扩展 gateway smoke：
  - structured output
  - reasoning stream
  - image/file non-stream
- 增加 richer mock integration tests：
  - responses custom tool
  - responses refusal
  - responses annotations
  - responses previous_response_id
- 保留真实客户端验证：
  - Codex via `/v1/responses`
  - Claude via `/v1/messages`
- 若有上游样本，再加：
  - web_search
  - MCP
  - prompt cache

涉及文件：

- `scripts/e2e_gateway_smoke.sh`
- `scripts/e2e_codex_noninteractive.sh`
- `scripts/e2e_claude_noninteractive.sh`
- `reports/*.md`

验收标准：

- `go test ./...` 通过
- 现有 E2E 不回归
- 新增 richer path 至少有 mock 级覆盖

## 实施优先级

### 第一批必须做

- Phase 1
- Phase 2
- Phase 3
- Phase 5

原因：

- 这是“字段先别丢”的最低闭环

### 第二批建议紧接着做

- Phase 4
- Phase 6

原因：

- 当前最大风险点在 stream 与日志，两者都容易造成“实际上丢语义但表面看起来成功”

### 第三批按资源推进

- Phase 7

原因：

- 真实上游样本和真实客户端验证成本更高，但必须在核心实现稳定后补上

## 里程碑定义

### M1: Rich Request Preservation

完成标准：

- richer request fields 不再在 decode 阶段丢失
- same-protocol round-trip 保持主要 provider fields

### M2: Rich Response Preservation

完成标准：

- richer response items 不再只剩 text / function_call / reasoning
- `refusal` / `annotations` / `status` / `incomplete_details` 可进入 IR

### M3: Rich Stream Compatibility

完成标准：

- reasoning stream 主路径在三协议间打通
- richer tool-call lifecycle 不丢失关键状态

### M4: Operational Readiness

完成标准：

- capability guard、日志、E2E 同步补齐
- 能清晰判断哪些语义：
  - 已完整支持
  - 已保留但未完整跨协议映射
  - 明确不支持

## 建议的落地顺序

1. 先改 `protocol/types.go`
2. 再补 `protocol/request_codec.go`
3. 再补 `protocol/response_codec.go`
4. 再补 `protocol/stream_*`
5. 最后补 capability、日志、前端和 E2E

这个顺序的原因很简单：

- 先让内部模型能装下语义
- 再让编解码不丢
- 最后再做运行时约束和验证
