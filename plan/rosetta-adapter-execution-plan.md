# gaiasec-llm-gateway 协议适配执行计划

## 目标

将当前 `gaiasec-llm-gateway` 从“按入口分别处理 + 局部转换”演进为“统一 IR + 完整协议语义转换”的网关，稳定支持以下三类对外接口：

- OpenAI Chat Completions: `/v1/chat/completions`
- OpenAI Responses: `/v1/responses`
- Anthropic Messages: `/v1/messages`

同时支持以下上游协议类型：

- `openai_chat`
- `openai_responses`
- `anthropic_messages`

核心原则：

- `llm-gateway` 对外固定支持三套接口，不做开关。
- 每个 `ModelConfig` 明确声明其上游原生协议和能力。
- 同协议优先直通。
- 异协议必须走统一 IR 转换，且请求、非流式响应、流式响应都必须完整保语义。

## 现状判断

当前仓库已经具备以下基础：

- 三条入口已存在：
  - `handlers/chat_handler.go` 中的 `ChatCompletion`
  - `handlers/chat_handler.go` 中的 `Responses`
  - `handlers/chat_handler.go` 中的 `AnthropicMessages`
- 已有局部转换：
  - `Responses -> Chat` 请求转换：`handlers/responses_converter.go`
  - `Chat -> Responses` 响应/流式转换：`handlers/chat_response.go`
- 已有 Anthropic 透传链路：
  - `dispatchAnthropicRequest`
  - `sendAnthropicRequest`
- 当前配置模型缺少“上游协议类型”和“能力位”：
  - `models/model_mapping.go`

当前主要问题：

- 转换逻辑分散在 handler 内，缺少统一协议抽象。
- 现有 `Responses -> Chat` / `Chat -> Responses` 为特化逻辑，不足以支撑完整三向互转。
- 目前路由决策依赖入口函数，未显式建模“入站协议”与“上游协议”的关系。
- 流式转换尚未形成统一事件层，后续补齐 `messages <-> responses/chat` 会快速复杂化。

## 参考实现结论

参考 `llm-rosetta` 后，应采用 Hub-and-Spoke 结构：

- Provider Request -> `IRRequest` -> Target Provider Request
- Provider Response -> `IRResponse` -> Target Provider Response
- Provider SSE Chunk -> `IRStreamEvent` -> Target Provider SSE Chunk

参考文件：

- `llm-rosetta/references/design/architecture.md`
- `llm-rosetta/references/sdk_ir/openai_chat_ir_mapping.md`
- `llm-rosetta/references/sdk_ir/openai_responses_ir_mapping.md`
- `llm-rosetta/references/sdk_ir/anthropic_ir_mapping.md`
- `llm-rosetta/src/llm_rosetta/types/ir/request.py`
- `llm-rosetta/src/llm_rosetta/types/ir/stream.py`

直接结论：

- 不能继续堆点对点转换函数。
- 必须新增统一 IR 层。
- 流式必须先解码成内部事件，再编码为目标协议 SSE。

## 总体方案

### 1. 配置层建模

为 `ModelConfig` 增加以下字段：

- `UpstreamType string`
  - 枚举：`openai_chat`、`openai_responses`、`anthropic_messages`
- `SupportsTools bool`
- `SupportsStream bool`
- `SupportsReasoning bool`
- `SupportsJSONSchema bool`
- `SupportsVision bool`
- `SupportsParallelToolCalls bool`

可选的第二阶段字段：

- `SupportsPromptCache bool`
- `SupportsPrefill bool`
- `SupportsComputerUse bool`
- `SupportsWebSearch bool`
- `SupportsMCP bool`

约束：

- 第一阶段不要让一个 upstream 同时声明多个“原生协议”。
- 主协议只能单选。
- 能力位用于转换降级判断和前置校验，不直接决定主路由。

### 2. 新增协议适配层

新增目录建议：

- `protocol/`
- `protocol/ir/`
- `protocol/openai_chat/`
- `protocol/openai_responses/`
- `protocol/anthropic/`

推荐最小文件布局：

- `protocol/types.go`
- `protocol/route.go`
- `protocol/ir_request.go`
- `protocol/ir_response.go`
- `protocol/ir_stream.go`
- `protocol/openai_chat_decoder.go`
- `protocol/openai_chat_encoder.go`
- `protocol/openai_responses_decoder.go`
- `protocol/openai_responses_encoder.go`
- `protocol/anthropic_decoder.go`
- `protocol/anthropic_encoder.go`

### 3. IR 设计

第一版 `IRRequest` 至少覆盖：

- `Model`
- `Messages`
- `SystemInstruction`
- `Tools`
- `ToolChoice`
- `ToolConfig`
- `Generation`
- `ResponseFormat`
- `Reasoning`
- `Stream`
- `ProviderExtensions`

`IRMessage` / `IRPart` 至少覆盖：

- `system`
- `developer`
- `user`
- `assistant`
- `tool`
- `text`
- `image`
- `file`
- `reasoning`
- `tool_call`
- `tool_result`
- `refusal`

`IRResponse` 至少覆盖：

- `ID`
- `Model`
- `Created`
- `OutputItems`
- `Usage`
- `FinishReason`
- `ProviderExtensions`

`IRStreamEvent` 至少覆盖：

- `stream_start`
- `content_block_start`
- `text_delta`
- `reasoning_delta`
- `tool_call_start`
- `tool_call_delta`
- `content_block_end`
- `finish`
- `usage`
- `stream_end`

要求：

- IR 必须允许保留 provider-specific 字段，避免强行丢语义。
- 不支持的能力不能静默吞掉，必须可记录告警或返回显式错误。

## 路由与分发设计

新增统一判定函数：

- `resolveInboundProtocol(path) InboundProtocol`
- `resolveDispatchMode(inbound, upstream) DispatchMode`

推荐枚举：

- `InboundProtocolChat`
- `InboundProtocolResponses`
- `InboundProtocolAnthropic`

- `DispatchPassthrough`
- `DispatchTransform`

判定规则：

- `chat -> openai_chat`：直通
- `responses -> openai_responses`：直通
- `messages -> anthropic_messages`：直通
- 其他全部转换

注意：

- “直通”不代表完全原样转发，仍然需要：
  - 替换模型名
  - 规范鉴权头
  - 透传必要请求头
  - 记录日志
  - 做能力位校验

## Handler 重构计划

### Phase 1: 解耦入口与协议转换

保留现有 3 个入口 handler，但把核心逻辑收敛到统一函数：

- `handleProtocolRequest(w, r, inboundProtocol)`

该函数负责：

- 解析 trace/log 上下文
- 读取 body
- 提取 model name
- 查询 `ModelConfig`
- 根据 `inboundProtocol + upstreamType` 决定直通或转换
- 执行上游请求
- 记录请求日志

### Phase 2: 替换分散转换逻辑

逐步淘汰或收编以下特化逻辑：

- `handlers/responses_converter.go`
- `handleResponsesNonStreamResponse`
- `handleResponsesStreamResponse`
- `buildAnthropicProviderRequestBody`
- 其他散落在 `chat_handler.go` / `chat_response.go` 内的协议特化逻辑

目标是把它们改造成：

- `Decode<Request/Response/Stream>(provider format -> IR)`
- `Encode<Request/Response/Stream>(IR -> provider format)`

## 分阶段执行

### 阶段 0：基线梳理

任务：

- 盘点当前所有请求入口、日志入口、配置入口
- 盘点现有字段映射与流式事件处理
- 识别当前测试覆盖缺口

产出：

- 字段兼容矩阵
- 流式事件兼容矩阵
- 风险清单

### 阶段 1：配置模型升级

任务：

- 扩展 `models.ModelConfig`
- 更新数据库 schema 校验
- 更新 `/api/model-configs` 的创建、修改、列表返回
- 更新前端 `ModelConfig.vue`
- 更新“测试连接”逻辑，使其按 `upstream_type` 打不同测试端点

验收：

- 能创建三种上游协议类型的模型配置
- `/api/model-configs` 返回新字段
- 前端可正确编辑与展示

### 阶段 2：引入最小 IR 类型

任务：

- 新建 `protocol` 包
- 定义最小 `IRRequest`、`IRResponse`、`IRStreamEvent`
- 先只纳入现有已支持字段，不一次性做完所有 provider 扩展

验收：

- IR 类型可编译
- 不影响现有 handler 行为

### 阶段 3：请求方向三协议解码/编码

任务：

- `OpenAI Chat <-> IRRequest`
- `OpenAI Responses <-> IRRequest`
- `Anthropic Messages <-> IRRequest`

要求：

- 完整支持消息角色映射
- 完整支持 tools / tool_choice / parallel settings
- 完整支持 max tokens / temperature / top_p / stop / reasoning / response format
- 保留 provider-specific 扩展

验收：

- 三协议请求均可 round-trip
- 不丢失系统指令、工具定义、工具调用约束

### 阶段 4：非流式响应方向三协议解码/编码

任务：

- `OpenAI Chat <-> IRResponse`
- `OpenAI Responses <-> IRResponse`
- `Anthropic Messages <-> IRResponse`

要求：

- 完整支持 assistant text
- 完整支持 reasoning/thinking
- 完整支持 tool calls / tool use
- 完整支持 finish reason
- 完整支持 usage

验收：

- 非流式三向互转通过 golden tests

### 阶段 5：流式事件方向三协议解码/编码

任务：

- 定义统一 `IRStreamEvent`
- 解码 OpenAI Chat SSE
- 解码 OpenAI Responses SSE
- 解码 Anthropic SSE
- 再编码为目标 SSE 协议

重点：

- Anthropic `message_start` / `content_block_*` / `message_delta`
- Responses `response.output_*` / `response.function_call_arguments.*`
- Chat `delta.content` / `delta.tool_calls` / usage chunk

验收：

- 支持以下流式路径：
  - `chat -> responses`
  - `responses -> chat`
  - `messages -> chat`
  - `chat -> messages`
  - `responses -> messages`
  - `messages -> responses`

### 阶段 6：统一 dispatch 替换旧链路

任务：

- 用统一协议分发替代当前多个 `dispatch...Request`
- 引入“直通/转换”决策层
- 旧逻辑保留一段时间作为回退路径，随后删除

验收：

- 三个对外入口全部走统一适配层
- 日志与负载均衡逻辑不回退

### 阶段 7：能力位校验与降级策略

任务：

- 请求入站前校验目标 upstream 是否支持相关能力
- 明确“不支持”的处理方式：
  - 返回 4xx
  - 自动降级
  - 记录警告并继续

建议策略：

- `tools`、`json_schema`、`reasoning`、`vision` 默认不静默降级
- `parallel_tool_calls` 可根据 upstream 能力自动收敛为串行

验收：

- 用户能从响应中明确知道哪项能力不被支持

### 阶段 8：清理旧实现

任务：

- 删除冗余转换函数
- 收敛重复的 stream 解析逻辑
- 更新 README / ARCHITECTURE 文档

验收：

- 主路径仅保留统一适配实现

## 语义完整性重点

以下语义必须作为强约束，不允许第一版省略：

### 1. 系统指令

- OpenAI Chat: `system/developer` 在 messages 中
- OpenAI Responses: `instructions`
- Anthropic: 顶层 `system`

必须可双向恢复，避免丢失。

### 2. Tool 定义与 tool choice

- OpenAI Chat 和 Anthropic 都支持 function-style tools，但字段结构不同。
- OpenAI Responses 的 tool 类型更丰富，第一阶段至少保证 function tools 完整。
- `tool_choice=auto/none/required/tool` 必须完整保留。

### 3. Tool call / tool result

- assistant 发起工具调用
- tool 返回结果
- 需要支持多次、并行、增量参数流式拼接

### 4. Reasoning / thinking

- Chat 常见为 `reasoning_effort` 或扩展字段
- Responses 有更强语义
- Anthropic 有 `thinking`

必须在 IR 中单独建模，不能混到普通文本里。

### 5. Structured output

- `json_object`
- `json_schema`

不支持时必须明确失败，不得静默退化成纯文本。

### 6. 流式 usage 和 finish reason

- 不能只转文本 delta
- 必须保留结束原因和 token usage

### 7. 多模态

- 文本、图片、文件至少要有 IR 占位
- 第一阶段即使上游能力不足，也不能在类型设计上封死

## 测试计划

### 1. 单元测试

新增测试目录建议：

- `protocol/.../*_test.go`

覆盖：

- request decode
- request encode
- response decode
- response encode
- stream decode
- stream encode

### 2. Round-trip 测试

关键用例：

- `chat -> IR -> chat`
- `responses -> IR -> responses`
- `messages -> IR -> messages`

目标：

- 语义等价
- 关键字段不丢

### 3. Cross-protocol 测试

关键用例：

- `chat -> anthropic`
- `anthropic -> chat`
- `chat -> responses`
- `responses -> chat`
- `responses -> anthropic`
- `anthropic -> responses`

### 4. 流式 golden tests

准备原始 SSE fixture：

- OpenAI Chat stream fixture
- OpenAI Responses stream fixture
- Anthropic stream fixture

校验：

- 事件顺序
- 文本拼接
- tool call 增量合并
- finish reason
- usage

### 5. 网关集成测试

在现有 `handlers/*_test.go` 基础上补充：

- 不同 `upstream_type` 的配置选择
- passthrough 与 transform 两种路径
- request log 是否仍然正确记录

## 风险点

### 高风险

- Responses 的 `output items` 语义比 Chat 更强，向 Chat/Anthropic 映射时容易丢字段。
- Anthropic streaming 事件模型与 OpenAI 差异大，流式互转复杂度高。
- 当前日志记录依赖特定响应结构，重构后可能出现日志内容失真。

### 中风险

- 旧测试大量绑定当前 handler 行为，重构期间会频繁改测试。
- 数据库 schema 变更会影响已有部署。
- 前端模型配置表单改动会牵动现有用户操作习惯。

### 低风险

- 入口路由本身不需要大改。
- 并发控制、负载均衡、失败重试机制理论上可基本复用。

## 建议实施顺序

建议按以下顺序提交：

1. `ModelConfig` schema 与 API 扩展
2. `protocol` 包最小 IR 落地
3. 请求方向三协议适配
4. 非流式响应方向适配
5. 流式适配
6. 统一 dispatch 接入
7. 能力校验与错误模型
8. 文档与清理

这样做的原因：

- 先把配置入口建好，避免后续逻辑没有落脚点。
- 先做请求和非流式响应，能更快建立可测闭环。
- 流式最后上，风险更集中、更容易隔离问题。

## 最小可用里程碑

### M1

- `ModelConfig` 支持 `upstream_type`
- 路由层能区分 passthrough / transform
- 保持现有 `responses <-> chat` 功能不退化

### M2

- 引入统一 `IRRequest` / `IRResponse`
- `chat <-> responses <-> anthropic` 非流式三向互转可用

### M3

- 三向流式互转可用
- tools / reasoning / usage / finish reason 完整保留

### M4

- 能力位校验完善
- 旧分散转换逻辑移除

## 明确不建议的做法

- 不要继续在 `handlers/chat_handler.go` 中堆更多 if/else 转换。
- 不要用“3 个 checkbox 表示 upstream 原生协议”。
- 不要只做文本层转换，忽略 tool/reasoning/usage。
- 不要把 provider 特有字段在第一版直接丢弃。
- 不要在没有统一 stream IR 的情况下扩写更多 SSE 特判。

## 交付结果定义

项目完成后，应满足：

- 创建模型配置时可声明上游原生协议与能力。
- 网关对外稳定支持三套接口。
- 任意入站协议请求都可根据上游协议直通或转换。
- 工具、推理、结构化输出、流式 usage、finish reason 等核心语义可保留。
- 日志系统仍能记录可重放请求与响应。
- 关键协议互转路径均有自动化测试覆盖。
