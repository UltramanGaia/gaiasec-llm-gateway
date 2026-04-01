# LLM Gateway 架构设计

## 概述
LLM Gateway 是一个纯 Go 的中间件服务，将多种 LLM API 聚合到 OpenAI 兼容接口，并对 GaiaSec 暴露模型配置与请求日志能力。

## 系统架构

![LLM Gateway Architecture](https://i.imgur.com/placeholder.png)

### 核心组件
1. **API 层**：提供 OpenAI 兼容的 RESTful API 接口
2. **路由层**：根据模型配置将请求路由到对应的上游模型
3. **配置管理**：通过 `/api/model-configs` 管理命名模型配置
4. **请求/响应处理**：处理请求转发和响应转换
5. **日志记录**：记录所有请求和响应信息到 MySQL
6. **统计接口**：提供 `/api/stats` 等统计查询能力

## 技术栈
- 后端：Go 语言
- 数据库：MySQL
- ORM：GORM
- Web 框架：标准库 net/http

## 详细设计

### 1. 数据模型
已实现以下数据模型：
- `ModelConfig`：模型配置，包含对外配置名、上游模型名、基础 URL、API Key 和运行参数
- `RequestLog`：请求日志，包含用户令牌、模型名称、请求和响应内容
- `Session`：会话信息，用于请求上下文关联

### 2. API 接口设计
- `/chat/completions`：OpenAI 兼容的聊天完成接口
- `/v1/chat/completions`：OpenAI 兼容别名接口
- `/v1/models`：返回当前可用模型配置
- `/api/model-configs`：模型配置管理接口
- `/api/request-logs`：请求日志查询接口
- `/api/stats`：统计信息接口

### 3. 核心功能实现
- **请求路由**：根据模型配置查找上游模型并转发请求
- **配置管理**：统一维护 GaiaSec 可见的模型配置
- **请求/响应转换**：将不同提供商的 API 格式转换为 OpenAI 兼容格式
- **日志记录**：记录所有请求和响应，支持按时间、模型、用户等维度查询

### 4. 部署与构建
- 构建产物为单个 Go 二进制 `llm-gateway`
- Docker 镜像只需要编译 Go 程序，不再包含前端构建阶段
- 根路径 `/` 返回简单的 `ok` 响应，可用于基础存活探测

## 实现路线图
1. 完善后端 API 接口
2. 增强模型路由和请求转发能力
3. 完善模型配置校验与测试接口
4. 优化请求/响应格式转换
5. 完善日志记录、查询与统计功能
6. 系统测试和优化
