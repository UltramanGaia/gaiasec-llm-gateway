# AGENTS.md

## 这个模块
- `gaiasec-llm-gateway` 提供 OpenAI 兼容接口，以及 `/api/model-configs`、`/api/request-logs`。

## 先看哪里
- `main.go`
- `handlers/`
- `models/`
- `config/`
- `.github/workflows/build.yml`
- `Dockerfile`

## 约束
- 正式 AI 配置入口是 `/api/model-configs`。
- 正式日志入口是 `/api/request-logs`
