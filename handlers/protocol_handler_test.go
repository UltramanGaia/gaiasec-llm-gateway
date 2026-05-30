package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llm-gateway/models"
)

func TestResponsesHandlerPassthroughsToResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotModel string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","status":"completed","output":[]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "auto-responses",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"auto-responses","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/responses" {
		t.Fatalf("expected provider /responses path, got %q", gotPath)
	}
	if gotModel != "backend-responses" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
}

func TestResponsesHandlerTransformsToChatForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotModel string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		if messages, ok := payload["messages"].([]any); ok {
			for _, item := range messages {
				if msg, ok := item.(map[string]any); ok {
					gotMessages = append(gotMessages, msg)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "auto-chat",
		ModelName:    "backend-chat",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"auto-chat","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("expected provider /chat/completions path, got %q", gotPath)
	}
	if gotModel != "backend-chat" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
	if len(gotMessages) != 1 || gotMessages[0]["role"] != "user" || gotMessages[0]["content"] != "hello" {
		t.Fatalf("expected responses request to be converted to chat messages, got %+v", gotMessages)
	}
}

func TestResponsesHandlerPreservesJSONSchemaWhenTransformingToChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotResponseFormat map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotResponseFormat, _ = payload["response_format"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:               "responses-jsonschema-to-chat",
		ModelName:          "backend-chat",
		APIBaseURL:         provider.URL,
		APIKey:             "key",
		UpstreamType:       models.UpstreamTypeOpenAIChat,
		SupportsJSONSchema: true,
		Enabled:            true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-jsonschema-to-chat","input":"hello","text":{"format":{"type":"json_schema","name":"result","schema":{"type":"object"}}}}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotResponseFormat == nil {
		t.Fatalf("expected response_format payload")
	}
	if gotResponseFormat["type"] == "json_schema" {
		return
	}
	format, _ := gotResponseFormat["format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("expected json_schema to be preserved into chat response_format, got %+v", gotResponseFormat)
	}
}

func TestChatHandlerTransformsToResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotModel string
	var gotInstructions string
	var gotInput []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		gotInstructions, _ = payload["instructions"].(string)
		if input, ok := payload["input"].([]any); ok {
			for _, item := range input {
				if m, ok := item.(map[string]any); ok {
					gotInput = append(gotInput, m)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello back"}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-to-responses",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-to-responses","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/responses" {
		t.Fatalf("expected provider /responses path, got %q", gotPath)
	}
	if gotModel != "backend-responses" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
	if gotInstructions != "sys" {
		t.Fatalf("expected system instructions to move to instructions, got %q", gotInstructions)
	}
	if len(gotInput) != 1 || gotInput[0]["role"] != "user" {
		t.Fatalf("expected user message in responses input, got %+v", gotInput)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	choices := payload["choices"].([]any)
	choice := choices[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if message["content"] != "hello back" {
		t.Fatalf("expected converted chat content, got %+v", message)
	}
}

func TestChatHandlerPreservesJSONSchemaWhenTransformingToResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotText map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotText, _ = payload["text"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:               "chat-jsonschema-to-responses",
		ModelName:          "backend-responses",
		APIBaseURL:         provider.URL,
		APIKey:             "key",
		UpstreamType:       models.UpstreamTypeOpenAIResponses,
		SupportsJSONSchema: true,
		Enabled:            true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-jsonschema-to-responses","messages":[{"role":"user","content":"hello"}],"response_format":{"type":"json_schema","json_schema":{"name":"result","schema":{"type":"object"}}}}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotText == nil {
		t.Fatalf("expected responses text payload")
	}
	if gotText["type"] == "json_schema" {
		return
	}
	format, _ := gotText["format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("expected json_schema to be preserved into responses text.format, got %+v", gotText)
	}
}

func TestProtocolRequestWritesRequestLogForTransformPath(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	resetAsyncLogWriterForTests()
	defer resetAsyncLogWriterForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_123","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "log-transform",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"log-transform","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// Flush async logs for deterministic verification.
	GetAsyncLogWriter(db).Stop()

	var count int64
	if err := db.Model(&models.RequestLog{}).Where("model_name = ?", "log-transform").Count(&count).Error; err != nil {
		t.Fatalf("count request logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 request log entry, got %d", count)
	}
}

func TestChatHandlerTransformsToAnthropicUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotAPIKey string
	var gotSystem string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotSystem, _ = payload["system"].(string)
		if messages, ok := payload["messages"].([]any); ok {
			for _, item := range messages {
				if m, ok := item.(map[string]any); ok {
					gotMessages = append(gotMessages, m)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message","role":"assistant","model":"claude-backend","content":[{"type":"text","text":"anthropic hi"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":4}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-to-anthropic",
		ModelName:    "claude-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-to-anthropic","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/messages" {
		t.Fatalf("expected provider /messages path, got %q", gotPath)
	}
	if gotAPIKey != "anthropic-key" {
		t.Fatalf("expected x-api-key header, got %q", gotAPIKey)
	}
	if gotSystem != "sys" {
		t.Fatalf("expected system prompt to map to top-level system, got %q", gotSystem)
	}
	if len(gotMessages) != 1 || gotMessages[0]["role"] != "user" {
		t.Fatalf("expected anthropic user message, got %+v", gotMessages)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != "anthropic hi" {
		t.Fatalf("expected anthropic response to convert back to chat, got %+v", message)
	}
}

func TestAnthropicHandlerTransformsToChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotModel string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		if messages, ok := payload["messages"].([]any); ok {
			for _, item := range messages {
				if m, ok := item.(map[string]any); ok {
					gotMessages = append(gotMessages, m)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion","model":"gpt-backend","choices":[{"index":0,"message":{"role":"assistant","content":"chat says hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":3,"total_tokens":7}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-to-chat",
		ModelName:    "gpt-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-to-chat","system":"sys","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("expected provider /chat/completions path, got %q", gotPath)
	}
	if gotModel != "gpt-backend" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
	if len(gotMessages) < 2 || gotMessages[0]["role"] != "system" || gotMessages[1]["role"] != "user" {
		t.Fatalf("expected anthropic request to map to chat messages, got %+v", gotMessages)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	first := content[0].(map[string]any)
	if first["type"] != "text" || first["text"] != "chat says hi" {
		t.Fatalf("expected chat response to convert to anthropic message, got %+v", payload)
	}
}

func TestResponsesHandlerTransformsToAnthropicUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotSystem string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotSystem, _ = payload["system"].(string)
		if messages, ok := payload["messages"].([]any); ok {
			for _, item := range messages {
				if m, ok := item.(map[string]any); ok {
					gotMessages = append(gotMessages, m)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_234","type":"message","role":"assistant","model":"claude-backend","content":[{"type":"text","text":"anthropic response"}],"stop_reason":"end_turn","usage":{"input_tokens":6,"output_tokens":4}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-to-anthropic",
		ModelName:    "claude-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-to-anthropic","instructions":"sys","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/messages" {
		t.Fatalf("expected provider /messages path, got %q", gotPath)
	}
	if gotSystem != "sys" {
		t.Fatalf("expected responses instructions to map to anthropic system, got %q", gotSystem)
	}
	if len(gotMessages) != 1 || gotMessages[0]["role"] != "user" {
		t.Fatalf("expected anthropic user message, got %+v", gotMessages)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses payload: %v", err)
	}
	output := payload["output"].([]any)
	first := output[0].(map[string]any)
	if first["type"] != "message" {
		t.Fatalf("expected responses message output, got %+v", payload)
	}
}

func TestAnthropicHandlerTransformsToResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotInstructions string
	var gotInput []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotInstructions, _ = payload["instructions"].(string)
		if input, ok := payload["input"].([]any); ok {
			for _, item := range input {
				if m, ok := item.(map[string]any); ok {
					gotInput = append(gotInput, m)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_234","object":"response","status":"completed","model":"responses-backend","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"response says hi"}]}],"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-to-responses",
		ModelName:    "responses-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-to-responses","system":"sys","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/responses" {
		t.Fatalf("expected provider /responses path, got %q", gotPath)
	}
	if gotInstructions != "sys" {
		t.Fatalf("expected anthropic system to map to responses instructions, got %q", gotInstructions)
	}
	if len(gotInput) != 1 || gotInput[0]["role"] != "user" {
		t.Fatalf("expected user input item, got %+v", gotInput)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic payload: %v", err)
	}
	content := payload["content"].([]any)
	first := content[0].(map[string]any)
	if first["type"] != "text" || first["text"] != "response says hi" {
		t.Fatalf("expected responses payload to convert to anthropic message, got %+v", payload)
	}
}

func TestChatHandlerTransformsAnthropicStreamToChatStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-anthropic-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"role":"assistant"`) || !strings.Contains(got, `"content":"hello"`) || !strings.Contains(got, `data: [DONE]`) {
		t.Fatalf("expected anthropic stream to convert to chat stream, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicStreamToResponsesStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_2","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-anthropic-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.output_text.delta`) || !strings.Contains(got, `"delta":"hello"`) || !strings.Contains(got, `data: [DONE]`) {
		t.Fatalf("expected anthropic stream to convert to responses stream, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesStreamToAnthropicStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant"}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-responses-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: message_start`) || !strings.Contains(got, `event: content_block_delta`) || !strings.Contains(got, `event: message_stop`) {
		t.Fatalf("expected responses stream to convert to anthropic stream, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesToolStreamToChatToolFinish(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":5,"delta":"{\"q\":\"he"}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":5,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-responses-tool-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-tool-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"tool_calls"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected responses tool stream to convert to chat tool finish, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesToolStreamToToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool2","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":2,"delta":"{\"q\":\"hello\"}"}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":2,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool2","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-responses-tool-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-tool-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected responses tool stream to convert to anthropic tool_use, got %s", got)
	}
}

func TestChatHandlerRejectsUnsupportedToolsCapability(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:          "tools-unsupported",
		ModelName:     "backend-chat",
		APIBaseURL:    "https://api.example.test",
		APIKey:        "key",
		UpstreamType:  models.UpstreamTypeOpenAIChat,
		SupportsTools: false,
		Enabled:       true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"tools-unsupported","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "tools") {
		t.Fatalf("expected tools capability error, got %s", recorder.Body.String())
	}
}

func TestResponsesHandlerRejectsUnsupportedJSONSchemaCapability(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:               "json-schema-unsupported",
		ModelName:          "backend-responses",
		APIBaseURL:         "https://api.example.test",
		APIKey:             "key",
		UpstreamType:       models.UpstreamTypeOpenAIResponses,
		SupportsJSONSchema: false,
		Enabled:            true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"json-schema-unsupported","input":"hello","text":{"format":{"type":"json_schema","name":"result","schema":{"type":"object"}}}}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "json_schema") {
		t.Fatalf("expected json_schema capability error, got %s", recorder.Body.String())
	}
}

func TestChatHandlerNormalizesParallelToolCallsWhenUnsupported(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotParallel bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode provider request: %v", err)
		}
		gotParallel, _ = payload["parallel_tool_calls"].(bool)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:                      "parallel-normalize",
		ModelName:                 "backend-chat",
		APIBaseURL:                provider.URL,
		APIKey:                    "key",
		UpstreamType:              models.UpstreamTypeOpenAIChat,
		SupportsTools:             true,
		SupportsParallelToolCalls: false,
		Enabled:                   true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"parallel-normalize","messages":[{"role":"user","content":"hello"}],"parallel_tool_calls":true,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotParallel {
		t.Fatalf("expected parallel_tool_calls to be normalized to false")
	}
}
