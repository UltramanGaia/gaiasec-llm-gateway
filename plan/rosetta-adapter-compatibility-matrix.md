# Rosetta Adapter Compatibility Matrix

## 字段兼容矩阵

| 语义 | OpenAI Chat | OpenAI Responses | Anthropic Messages | 当前状态 |
| --- | --- | --- | --- | --- |
| 模型名 | `model` | `model` | `model` | 已支持 |
| 系统指令 | `messages[].role=system/developer` | `instructions` | 顶层 `system` | 已支持双向恢复 |
| 用户文本 | `messages[].content` | `input` / `input[].content[]` | `messages[].content[].text` | 已支持 |
| assistant 文本 | `choices[].message.content` | `output[].content[].text` | `content[].text` | 已支持 |
| tools 定义 | `tools[].function` | `tools[]` | `tools[].input_schema` | 已支持 function tools |
| tool choice | `tool_choice` | `tool_choice` | `tool_choice` | 已部分支持 |
| tool call | `message.tool_calls` | `output[].type=function_call` | `content[].type=tool_use` | 已支持 |
| tool result | `role=tool` | `input[].type=function_call_output` | `content[].type=tool_result` | 已支持 |
| max tokens | `max_tokens` | `max_output_tokens` | `max_tokens` | 已支持 |
| temperature | `temperature` | `temperature` | `temperature` | 已支持 |
| top_p | `top_p` | `top_p` | `top_p` | 已支持 |
| stop | `stop` | `stop` | `stop_sequences` | 已支持 |
| reasoning | `reasoning` / `reasoning_content` | `reasoning` / `reasoning item` | `thinking` / thinking delta | 基本支持，stream 侧仍待加强 |
| structured output | `response_format` | `text.format` | 无原生等价 | request 与主路径变换已保留，response/stream 侧仍待加强 |
| vision / file | 多模态 content part | input/output content part | content part | request 与 non-stream response 已保留，stream 侧仍待加强 |
| parallel tool calls | `parallel_tool_calls` | `parallel_tool_calls` | `tool_choice.disable_parallel_tool_use` | 已支持最小收敛 |
| usage | `usage` | `usage` | `usage` | 已支持 |
| finish reason | `finish_reason` | `response.completed` / output done | `stop_reason` | 已支持主路径 |

## 流式事件兼容矩阵

| 上游流式 | 目标流式 | 当前状态 | 说明 |
| --- | --- | --- | --- |
| Chat SSE | Responses SSE | 已支持 | `delta.content` / `delta.tool_calls` / `usage` |
| Chat SSE | Anthropic SSE | 已支持 | `content_block_*` / `message_delta` / `message_stop` |
| Responses SSE | Chat SSE | 已支持 | `response.output_*` / `response.function_call_arguments.*` / `response.completed` |
| Responses SSE | Anthropic SSE | 已支持 | 已有 stateful codec |
| Anthropic SSE | Chat SSE | 已支持 | `message_start` / `content_block_*` / `message_delta` |
| Anthropic SSE | Responses SSE | 已支持 | 已有 stateful codec |

## 风险清单

### 高风险

- `json_schema/json_object` 尚未在 response/stream 侧形成更强的跨协议语义证明。
- `vision/file` 尚未在 stream 侧形成完整语义链路与测试证据。
- 部分 sender/wrapper 仍保留在 handler 中，虽然主路径已切换，但代码收尾尚未完全完成。

### 中风险

- stream codec 虽已大部分迁入 `protocol/`，但仍有少量 handler 特有日志/包装逻辑，后续重构需防止行为回归。
- 现有测试以 round-trip 和集成风格为主，严格意义上的 golden fixtures 还不够系统化。

### 低风险

- `parallel_tool_calls` 已有明确降级策略。
- `tools` / `reasoning` / `json_schema` / `vision` / `stream` 已有入站前能力校验。
