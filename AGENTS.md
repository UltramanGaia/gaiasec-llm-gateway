# AGENTS.md

## 模块定位
`gaiasec-llm-gateway` 是 LLM 聚合网关，提供 OpenAI 兼容接口、模型映射、请求日志，以及配套的前端管理界面。

## 技术栈与入口
- Go 1.23/1.24 toolchain
- 后端入口：`main.go`
- 前端：`frontend/` 中的 Vue 3 + Vite
- 关键依赖：GORM、MySQL、Element Plus

## 关键目录
- `main.go`: 后端入口
- `handlers/`: HTTP 处理、日志、模型映射、会话接口
- `models/`: 持久化模型
- `config/`: 配置加载
- `utils/`: 通用工具
- `prompts/`: 相关提示模板
- `frontend/`: Web 管理界面

## 常用命令
```bash
go test ./...
go run .
make build
cd frontend && npm run build
```

## 协作约定
- OpenAI 兼容 API、模型映射和日志表结构属于外部契约，修改前先评估对调用方的影响。
- 后端代码优先落在 `handlers/`、`models/`、`config/`，前端代码优先落在 `frontend/src/`；不要手改根目录已有 `llm-gateway` 二进制。
- `frontend/node_modules/` 或构建产物不应手改；前端接口字段需与后端同步演进。
- 如果请求转发、认证或日志记录逻辑变化，要同时核对依赖此网关的服务和控制台页面。
