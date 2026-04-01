# LLM Gateway

## Overview
LLM Gateway is a Go service that aggregates multiple LLM API interfaces into a single OpenAI-compatible interface. In GaiaSec it owns model configuration, request routing, and request log querying.

## Features
1. **Unified Interface**: Provides an OpenAI-compatible API to access multiple LLM providers
2. **Model Configuration**: Stores named model configs used by the OpenAI-compatible entrypoints
3. **Request Routing**: Routes requests to the appropriate upstream model endpoint based on config name
4. **Logging**: Records all requests and responses for auditing and analysis
5. **GaiaSec Control-Plane APIs**: Serves `/api/model-configs` and `/api/request-logs`

## Architecture
![LLM Gateway Architecture](https://i.imgur.com/placeholder.png)

### Core Components
- **API Layer**: Provides RESTful API endpoints compatible with OpenAI
- **Routing Layer**: Maps the requested config name to the configured upstream model
- **Credential Manager**: Stores API credentials inside model configs
- **Request/Response Handler**: Processes and transforms requests and responses
- **Logger**: Records all requests and responses

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
### OpenAI Compatible Endpoints
- `POST /chat/completions`: Chat completion API compatible with OpenAI
- `POST /v1/chat/completions`: Chat completion API compatible with OpenAI

### Management Endpoints
- `GET /api/model-configs`: List model configs
- `POST /api/model-configs`: Create a model config
- `PUT /api/model-configs/{id}`: Update a model config
- `DELETE /api/model-configs/{id}`: Delete a model config
- `POST /api/model-configs/{id}/test`: Validate a model config
- `GET /api/request-logs`: Query request logs

### Compatibility Notes
- Public GaiaSec deployment no longer exposes `/api/model-mappings` or `/api/providers`; `/api/model-configs` is the only supported AI configuration contract.
- `/api/model-mappings` may still exist as an internal compatibility route in `llm-gateway`, but it is not part of the supported GaiaSec control-plane surface.
- `/api/logs` is retired at the GaiaSec gateway layer; use `/api/request-logs`.

## Configuration
### Model Configuration
Create a model config with the following fields:
- **Name**: The public config name used by `/chat/completions` and `/v1/models`
- **Model Name**: The actual upstream model name
- **API Base URL**: The upstream OpenAI-compatible base URL
- **API Key**: The upstream credential
- **Max Tokens / Temperature / Description / Enabled**: Optional runtime settings

## License
This project is licensed under the MIT License - see the LICENSE file for details.
