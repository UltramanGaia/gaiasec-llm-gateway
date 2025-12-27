# LLM Gateway

## Overview
LLM Gateway is a middleware service that aggregates multiple LLM API interfaces into a single OpenAI-compatible interface. It provides functionalities such as model mapping, credential management, request routing, and logging.

## Features
1. **Unified Interface**: Provides an OpenAI-compatible API to access multiple LLM providers
2. **Model Mapping**: Allows custom naming for models from different providers
3. **Credential Management**: Securely stores and manages API keys for various LLM providers
4. **Request Routing**: Routes requests to the appropriate LLM provider based on model name
5. **Logging**: Records all requests and responses to a SQLite database for auditing and analysis
6. **Web Interface**: Provides a Vue-based web interface for configuration and monitoring

## Architecture
![LLM Gateway Architecture](https://i.imgur.com/placeholder.png)

### Core Components
- **API Layer**: Provides RESTful API endpoints compatible with OpenAI
- **Routing Layer**: Maps model names to corresponding LLM providers
- **Credential Manager**: Securely stores and manages API credentials
- **Model Mapper**: Maintains custom model name mappings
- **Request/Response Handler**: Processes and transforms requests and responses
- **Logger**: Records all requests and responses
- **Web UI**: Vue-based interface for configuration and monitoring

## Technology Stack
- **Backend**: Go
- **Frontend**: Vue 3 + Vite
- **Database**: SQLite
- **ORM**: GORM
- **UI Framework**: Element Plus
- **HTTP Client**: Axios
- **Router**: Vue Router

## Getting Started

### Prerequisites
- Go 1.21 or later
- Node.js 18 or later
- npm or yarn

### Installation
1. Clone the repository
```bash
git clone https://github.com/yourusername/llm-gateway.git
cd llm-gateway
```

2. Install backend dependencies
```bash
go mod tidy
```

3. Install frontend dependencies
```bash
cd frontend
npm install
```

### Building the Project
1. Build the frontend
```bash
cd frontend
npm run build
```

2. Build the backend
```bash
cd ..
go build -o llm-gateway
```

### Running the Server
```bash
./llm-gateway --host=0.0.0.0 --port=8000
```

### Accessing the Web Interface
Open your browser and navigate to `http://localhost:8000`

## API Documentation
### OpenAI Compatible Endpoints
- `POST /chat/completions`: Chat completion API compatible with OpenAI
- `POST /v1/chat/completions`: Chat completion API compatible with OpenAI

### Management Endpoints
- `GET /api/providers`: List all providers
- `POST /api/providers`: Create a new provider
- `GET /api/model-mappings`: List all model mappings
- `POST /api/model-mappings`: Create a new model mapping
- `POST /api/credentials/generate`: Generate a new API credential
- `GET /api/logs`: Query request logs

## Configuration
### Provider Configuration
To add a new provider, navigate to the Providers page in the web interface and fill in the following information:
- **Name**: A descriptive name for the provider
- **API Key**: The API key for accessing the provider's services
- **Base URL**: The base URL for the provider's API

### Model Mapping Configuration
To create a model mapping, navigate to the Model Mappings page and fill in the following information:
- **Alias**: The custom name you want to use for the model
- **Provider**: The provider this model belongs to
- **Model Name**: The actual model name used by the provider

## License
This project is licensed under the MIT License - see the LICENSE file for details.
