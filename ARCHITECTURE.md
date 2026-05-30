# LLM Gateway 架构设计

## 概述
LLM Gateway 是一个纯 Go 的协议适配网关。当前对外固定暴露三套推理接口：

- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /v1/messages`

对上游则按 `ModelConfig.upstream_type` 区分三类原生协议：

- `openai_chat`
- `openai_responses`
- `anthropic_messages`

网关会根据“入站协议 + 上游协议”决定直通或转换，并在转换时通过 `protocol/` 包里的统一 codec 完成请求、非流式响应、流式响应的语义映射。

## 系统架构

### 核心组件
1. **API 层**：`handlers/chat_handler.go` 暴露三套公共推理入口。
2. **统一分发层**：`handlers/protocol_handler.go` 收敛入口逻辑，负责协议识别、模型查找、直通/转换判定、上游请求执行。
3. **协议适配层**：`protocol/` 负责 request codec、response codec、stream codec/stateful stream codec。
4. **能力位校验层**：`handlers/protocol_capabilities.go` 根据模型能力位拒绝不支持的能力，或对 `parallel_tool_calls` 做最小收敛。
5. **路由与运行时层**：`handlers/model_routing.go` 和运行时状态负责优先级、并发限制、重试与失败切换。
6. **日志与统计层**：请求日志、运行状态与统计接口。

## 技术栈
- 后端：Go 语言
- 数据库：MySQL
- ORM：GORM
- Web 框架：标准库 net/http

## 详细设计

### 1. 数据模型
- `ModelConfig`
  - 对外模型名 `name`
  - 上游模型名 `model_name`
  - 上游协议类型 `upstream_type`
  - 能力位 `supports_*`
  - 路由/运行参数
- `RequestLog`
- `Session`

### 2. API 接口设计
- 推理接口
  - `/chat/completions`
  - `/v1/chat/completions`
  - `/v1/responses`
  - `/responses`
  - `/v1/messages`
- 管理接口
  - `/api/model-configs`
  - `/api/request-logs`
  - `/api/stats`

### 3. 核心功能实现
- **统一入口收敛**
  - 三个入口最终都走 `handleProtocolRequest`
- **协议转换**
  - request codec：`protocol/request_codec.go`
  - non-stream response codec：`protocol/response_codec.go`
  - stream codec：`protocol/stream_codec.go`
  - stateful stream codec：`protocol/stream_stateful_codec.go`、`protocol/stream_anthropic_codec.go`
- **能力位处理**
  - `tools` / `reasoning` / `json_schema` / `vision` / `stream` 默认显式拒绝
  - `parallel_tool_calls` 自动收敛
- **日志记录与追踪**
  - trace id 贯穿入口、分发和日志

### 4. 部署与构建
- 构建产物为单个 Go 二进制 `llm-gateway`
- Docker 镜像只需要编译 Go 程序，不再包含前端构建阶段
- 根路径 `/` 返回简单的 `ok` 响应，可用于基础存活探测

## 当前状态
- 统一协议分发已落地。
- `protocol/` 已承载 request codec、non-stream response codec 和大部分 stream codec。
- handler 中仍有少量收尾逻辑与兼容层需要继续清理。
- 语义完整性方面，`reasoning`、`structured output`、`vision` 仍需进一步补强。
