# LLM Gateway

## Overview
LLM Gateway is a Go service that exposes multiple LLM-facing protocols behind one gateway. In GaiaSec it owns:
- protocol-compatible inference entrypoints
- model configuration management
- request routing and upstream failover
- request log querying

## Features
1. **Multi-Protocol Gateway**: Supports `/v1/chat/completions`, `/v1/responses`, `/v1/messages`
2. **Explicit Upstream Modeling**: Each `ModelConfig` declares `upstream_type` and capability flags
3. **Protocol Adapter Layer**: Uses a shared `protocol/` package to convert request/response/stream semantics
4. **Routing and Failover**: Selects a backend config by name with retry and concurrency control
5. **Logging**: Records requests and responses for auditing and analysis

## Architecture
The gateway now uses a hub-and-spoke adapter shape:

- inbound protocol request
- decode to shared IR-like protocol model in `protocol/`
- encode to target upstream protocol
- call upstream
- decode upstream response/stream
- encode back to inbound protocol

### Core Components
- **API Layer**: `handlers/chat_handler.go` exposes the three public inference protocols
- **Protocol Adapter Layer**: `protocol/` contains request codec, non-stream response codec, and stream codec/state
- **Dispatch Layer**: `handlers/protocol_handler.go` resolves passthrough vs transform and executes upstream calls
- **Capability Guard**: `handlers/protocol_capabilities.go` validates `tools`, `stream`, `reasoning`, `json_schema`, `vision`, `parallel_tool_calls`
- **Routing Layer**: backend selection, retry, and concurrency control
- **Logger**: async request/response logging

## Technology Stack
- **Backend**: Go
- **Database**: MySQL
- **ORM**: GORM
- **HTTP Server**: Go standard library (`net/http`)

## Getting Started

### Prerequisites
- Go 1.23 or later
- MySQL

### Installation
1. Clone the repository
```bash
git clone https://github.com/yourusername/llm-gateway.git
cd llm-gateway
```

2. Install dependencies
```bash
go mod tidy
```

### Building the Project
Build the server binary
```bash
go build -o llm-gateway
```

### Running the Server
```bash
./llm-gateway --host=0.0.0.0 --port=8090
```

### Verifying the Service
The root path returns a simple readiness response:
```bash
curl http://localhost:8090/
```

## API Documentation
### Inference Endpoints
- `POST /chat/completions`
- `POST /v1/chat/completions`
- `POST /v1/responses`
- `POST /responses`
- `POST /v1/messages`

### Management Endpoints
- `GET /api/model-configs`: List model configs
- `POST /api/model-configs`: Create a model config
- `PUT /api/model-configs/{id}`: Update a model config
- `DELETE /api/model-configs/{id}`: Delete a model config
- `POST /api/model-configs/{id}/test`: Validate a model config
- `GET /api/request-logs`: Query request logs

### Compatibility Notes
- Public GaiaSec deployment no longer exposes `/api/model-mappings` or `/api/providers`; `/api/model-configs` is the only supported AI configuration contract.
- `/api/model-mappings` may still exist as an internal compatibility route in `llm-gateway`
- `/api/logs` is retired at the GaiaSec gateway layer; use `/api/request-logs`.

## Configuration
### Model Configuration
Create a model config with the following fields:
- **Name**: The public config name used by `/chat/completions` and `/v1/models`
- **Model Name**: The actual upstream model name
- **API Base URL**: The upstream OpenAI-compatible base URL
- **API Key**: The upstream credential
- **Upstream Type**: One of `openai_chat`, `openai_responses`, `anthropic_messages`
- **Capability Flags**: `supports_tools`, `supports_stream`, `supports_reasoning`, `supports_json_schema`, `supports_vision`, `supports_parallel_tool_calls`
- **Priority / Max Tokens / Max Concurrency / Temperature / Description / Enabled**: Optional runtime settings. Lower `priority` is preferred first. `max_concurrency=0` means no limit.

## License
This project is licensed under the MIT License - see the LICENSE file for details.
