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

func mustJSONStringForTest(value string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

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

func TestResponsesHandlerPreservesPreviousResponseIDForResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPreviousResponseID string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotPreviousResponseID, _ = payload["previous_response_id"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_124","object":"response","status":"completed","output":[]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-prev-id",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-prev-id","input":"hello","previous_response_id":"resp_prev_1","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPreviousResponseID != "resp_prev_1" {
		t.Fatalf("expected previous_response_id to be preserved, got %q", gotPreviousResponseID)
	}
}

func TestResponsesHandlerRejectsPreviousResponseIDForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:         "responses-prev-id-chat-upstream",
		ModelName:    "backend-chat",
		APIBaseURL:   "http://example.invalid",
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-prev-id-chat-upstream","input":"hello","previous_response_id":"resp_prev_1","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "previous_response_id") {
		t.Fatalf("expected previous_response_id rejection, got %s", recorder.Body.String())
	}
}

func TestResponsesHandlerRejectsPreviousResponseIDForAnthropicUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:         "responses-prev-id-anthropic-upstream",
		ModelName:    "backend-anthropic",
		APIBaseURL:   "http://example.invalid",
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-prev-id-anthropic-upstream","input":"hello","previous_response_id":"resp_prev_1","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "previous_response_id") {
		t.Fatalf("expected previous_response_id rejection, got %s", recorder.Body.String())
	}
}

func TestResponsesHandlerRejectsPromptCacheForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:         "responses-prompt-cache-chat-upstream",
		ModelName:    "backend-chat",
		APIBaseURL:   "http://example.invalid",
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-prompt-cache-chat-upstream","input":"hello","prompt_cache_key":"cache-key","prompt_cache_retention":"24h","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "prompt_cache") {
		t.Fatalf("expected prompt_cache rejection, got %s", recorder.Body.String())
	}
}

func TestResponsesHandlerRejectsPromptCacheForAnthropicUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:         "responses-prompt-cache-anthropic-upstream",
		ModelName:    "backend-anthropic",
		APIBaseURL:   "http://example.invalid",
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-prompt-cache-anthropic-upstream","input":"hello","prompt_cache_key":"cache-key","prompt_cache_retention":"24h","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "prompt_cache") {
		t.Fatalf("expected prompt_cache rejection, got %s", recorder.Body.String())
	}
}

func TestResponsesHandlerRejectsResponsesOnlyFieldsForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	config := models.ModelConfig{
		Name:         "responses-only-fields-chat-upstream",
		ModelName:    "backend-chat",
		APIBaseURL:   "http://example.invalid",
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-only-fields-chat-upstream","input":"hello","include":["reasoning.encrypted_content"],"store":true,"background":false,"conversation":{"id":"conv_1"},"prompt":{"id":"pmpt_1"},"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	bodyText := recorder.Body.String()
	for _, token := range []string{"include", "store", "background", "conversation", "prompt"} {
		if !strings.Contains(bodyText, token) {
			t.Fatalf("expected %s rejection, got %s", token, bodyText)
		}
	}
}

func TestChatHandlerPassthroughsToChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotAuth string
	var gotModel string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
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
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "auto-chat-passthrough",
		ModelName:    "backend-chat",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"auto-chat-passthrough","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("expected provider /chat/completions path, got %q", gotPath)
	}
	if gotAuth != "Bearer chat-key" {
		t.Fatalf("expected bearer auth header, got %q", gotAuth)
	}
	if gotModel != "backend-chat" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
	if len(gotMessages) != 1 || gotMessages[0]["role"] != "user" || gotMessages[0]["content"] != "hello" {
		t.Fatalf("expected passthrough chat message body, got %+v", gotMessages)
	}
}

func TestChatHandlerPassthroughNormalizesThinkTaggedContent(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_think","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"<think>private reasoning</think>\n\npong"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-think-normalize",
		ModelName:    "backend-chat",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-think-normalize","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode normalized chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != "pong" {
		t.Fatalf("expected think tags removed from content, got %+v", message["content"])
	}
	if message["reasoning_content"] != "private reasoning" {
		t.Fatalf("expected reasoning_content extracted from think tags, got %+v", message["reasoning_content"])
	}
}

func TestChatHandlerAggregatesToolStreamForNonStreamChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotStream bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotStream, _ = payload["stream"].(bool)

		w.Header().Set("Content-Type", "text/event-stream")
		stream := strings.Join([]string{
			`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"content":"","role":"assistant"}}]}`,
			"",
			`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hang"}}]}}]}`,
			"",
			`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"zhou\"}"}}]}}]}`,
			"",
			`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:          "chat-tool-aggregate",
		ModelName:     "backend-chat",
		APIBaseURL:    provider.URL,
		APIKey:        "chat-key",
		UpstreamType:  models.UpstreamTypeOpenAIChat,
		SupportsTools: true,
		Enabled:       true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-tool-aggregate","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}],"tool_choice":"required","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !gotStream {
		t.Fatalf("expected gateway to force upstream stream=true for tool aggregation")
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode aggregated chat response: %v", err)
	}
	choice := payload["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %+v", choice["finish_reason"])
	}
	message := choice["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	fn := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if fn["name"] != "get_weather" || fn["arguments"] != "{\"city\":\"Hangzhou\"}" {
		t.Fatalf("expected aggregated tool call payload, got %+v", toolCalls[0])
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

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses-transformed response: %v", err)
	}
	if payload["object"] != "response" {
		t.Fatalf("expected transformed /v1/responses output to stay in responses shape, got %+v", payload)
	}
	output := payload["output"].([]any)
	if len(output) != 1 || output[0].(map[string]any)["type"] != "message" {
		t.Fatalf("expected transformed responses output item, got %+v", payload)
	}
}

func TestResponsesHandlerNormalizesFunctionToolForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotTools []map[string]any
	var gotToolChoice map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if tools, ok := payload["tools"].([]any); ok {
			for _, item := range tools {
				if m, ok := item.(map[string]any); ok {
					gotTools = append(gotTools, m)
				}
			}
		}
		gotToolChoice, _ = payload["tool_choice"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:          "responses-tools-to-chat",
		ModelName:     "backend-chat",
		APIBaseURL:    provider.URL,
		APIKey:        "key",
		UpstreamType:  models.UpstreamTypeOpenAIChat,
		SupportsTools: true,
		Enabled:       true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-tools-to-chat","input":"hello","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"tool_choice":{"type":"function","name":"lookup"}}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(gotTools) == 0 {
		t.Fatalf("expected at least one normalized tool, got %+v", gotTools)
	}
	if _, ok := gotTools[0]["function"].(map[string]any); !ok {
		t.Fatalf("expected chat upstream tool shape with function field, got %+v", gotTools[0])
	}
	if gotToolChoice == nil || gotToolChoice["type"] != "function" {
		t.Fatalf("expected normalized chat tool_choice, got %+v", gotToolChoice)
	}
}

func TestResponsesHandlerTransformsChatRicherStreamToResponsesEvents(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		"",
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"refusal":"cannot comply"}}]}`,
		"",
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"audio":{"id":"aud_1","format":"wav"}}}]}`,
		"",
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{"annotations":[{"type":"url_citation","title":"doc"}]}}]}`,
		"",
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"backend-chat","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
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
		Name:         "responses-chat-rich-stream",
		ModelName:    "backend-chat",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-chat-rich-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.refusal.delta`) || !strings.Contains(got, `"refusal":"cannot comply"`) {
		t.Fatalf("expected chat refusal stream to convert to responses refusal events, got %s", got)
	}
	if !strings.Contains(got, `event: response.audio.delta`) || !strings.Contains(got, `"id":"aud_1"`) {
		t.Fatalf("expected chat audio stream to convert to responses audio events, got %s", got)
	}
	if !strings.Contains(got, `event: response.annotation.added`) || !strings.Contains(got, `"title":"doc"`) {
		t.Fatalf("expected chat annotations stream to convert to responses annotation events, got %s", got)
	}
}

func TestResponsesHandlerPassthroughsCustomToolCallFromResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_custom","object":"response","status":"completed","model":"backend-responses","output":[{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-custom-tool",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-custom-tool","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses response: %v", err)
	}
	output := payload["output"].([]any)
	if len(output) != 1 || output[0].(map[string]any)["type"] != "custom_tool_call" {
		t.Fatalf("expected custom_tool_call to survive passthrough, got %+v", payload)
	}
}

func TestChatHandlerTransformsResponsesCustomToolCallToChatToolCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_custom_chat","object":"response","status":"completed","model":"backend-responses","output":[{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-custom-tool-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-custom-tool-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	function := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if function["name"] != "local_shell" || function["arguments"] != "ls -la" {
		t.Fatalf("expected custom_tool_call to convert to chat tool_call, got %+v", message)
	}
}

func TestChatHandlerTransformsResponsesMCPCallToChatToolCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_mcp_chat","object":"response","status":"completed","model":"backend-responses","output":[{"type":"mcp_call","id":"mcp_1","call_id":"mcp_1","name":"fetch","arguments":"{\"q\":\"hello\"}","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-mcp-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-mcp-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	function := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if function["name"] != "fetch" || function["arguments"] != "{\"q\":\"hello\"}" {
		t.Fatalf("expected mcp_call to convert to chat tool_call, got %+v", message)
	}
}

func TestChatHandlerTransformsResponsesImageGenerationCallToChatToolCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_img_chat","object":"response","status":"completed","model":"backend-responses","output":[{"type":"image_generation_call","id":"img_1","name":"image_generation_call","input":"generate a cat","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-imagegen-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-imagegen-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	function := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if function["name"] != "image_generation_call" || function["arguments"] != "generate a cat" {
		t.Fatalf("expected image_generation_call to convert to chat tool_call, got %+v", message)
	}
}

func TestResponsesHandlerPassthroughsAnnotationsFromResponsesUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_annotations","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]}]}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-annotations",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-annotations","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses response: %v", err)
	}
	output := payload["output"].([]any)
	content := output[0].(map[string]any)["content"].([]any)
	annotations := content[0].(map[string]any)["annotations"].([]any)
	if len(annotations) != 1 {
		t.Fatalf("expected annotations to survive passthrough, got %+v", payload)
	}
}

func TestChatHandlerTransformsResponsesRefusalToChatMessage(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_refusal","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"refusal","refusal":"cannot comply"}]}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-refusal-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-refusal-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if message["refusal"] != "cannot comply" {
		t.Fatalf("expected refusal to survive responses->chat conversion, got %+v", message)
	}
}

func TestChatHandlerTransformsResponsesAnnotationsToChatMessage(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_annotations_chat","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]}]}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-annotations-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-annotations-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	content := message["content"].([]any)
	annotations := content[0].(map[string]any)["annotations"].([]any)
	if len(annotations) != 1 {
		t.Fatalf("expected annotations to survive responses->chat conversion, got %+v", message)
	}
}

func TestChatHandlerTransformsResponsesAnnotationStreamToChatStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant"}}`,
		"",
		"event: response.content_part.done",
		`data: {"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_stream_annotations","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-annotation-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-annotation-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"annotations":[{"title":"doc","type":"url_citation"}]`) && !strings.Contains(got, `"annotations":[{"type":"url_citation","title":"doc"}]`) {
		t.Fatalf("expected responses annotation stream to convert to chat stream annotations, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesReasoningStreamToChatStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":""}]}}`,
		"",
		"event: response.reasoning.delta",
		`data: {"type":"response.reasoning.delta","output_index":50,"item_id":"rs_1","delta":"think step"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_stream_reasoning","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-reasoning-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-reasoning-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"reasoning_content":"think step"`) {
		t.Fatalf("expected responses reasoning stream to convert to chat reasoning_content, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesNonStreamWithReasoningItemBeforeMessage(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_reasoning_first","object":"response","status":"completed","model":"resp-backend","output":[{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]},{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-responses-reasoning-first-nonstream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-reasoning-first-nonstream","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != "hello" {
		t.Fatalf("expected message content hello, got %+v", message)
	}
	if message["reasoning_content"] != "think step" {
		t.Fatalf("expected reasoning_content think step, got %+v", message)
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

func TestResponsesHandlerPreservesChatStructuredAnnotationsRefusalAndAudio(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_rich_responses","object":"chat.completion","model":"backend-chat","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]},{"type":"refusal","refusal":"blocked"},{"type":"audio","audio":{"id":"aud_1","format":"wav"}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-rich-to-responses",
		ModelName:    "backend-chat",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-rich-to-responses","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses payload: %v", err)
	}
	output := payload["output"].([]any)
	content := output[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["annotations"] == nil {
		t.Fatalf("expected annotations to survive chat->responses conversion, got %+v", content[0])
	}
	if content[1].(map[string]any)["type"] != "refusal" || content[2].(map[string]any)["type"] != "output_audio" {
		t.Fatalf("expected refusal/audio to survive chat->responses conversion, got %+v", content)
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

func TestAnthropicHandlerPreservesChatStructuredAnnotationsRefusalAndAudio(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_rich_anthropic","object":"chat.completion","model":"chat-backend","choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]},{"type":"refusal","refusal":"blocked"},{"type":"audio","audio":{"id":"aud_1","format":"wav"}}]},"finish_reason":"stop"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "chat-rich-to-anthropic",
		ModelName:    "chat-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-rich-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	var sawAnnotations, sawRefusal, sawAudio bool
	for _, raw := range content {
		item := raw.(map[string]any)
		if item["annotations"] != nil {
			sawAnnotations = true
		}
		if item["refusal"] == "blocked" {
			sawRefusal = true
		}
		if _, ok := item["audio"].(map[string]any); ok {
			sawAudio = true
		}
	}
	if !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected annotations/refusal/audio to survive chat->anthropic conversion, got %+v", content)
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

func TestChatHandlerPreservesAnthropicStructuredAnnotationsRefusalAndAudio(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_rich_chat","type":"message","role":"assistant","model":"claude-backend","content":[{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]},{"type":"text","text":"blocked","refusal":"blocked"},{"type":"text","text":"","audio":{"id":"aud_1","format":"wav"}}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-rich-to-chat",
		ModelName:    "claude-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-rich-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	content := message["content"].([]any)
	if content[0].(map[string]any)["annotations"] == nil {
		t.Fatalf("expected annotations to survive anthropic->chat conversion, got %+v", content[0])
	}
	if message["refusal"] != "blocked" {
		t.Fatalf("expected refusal to survive anthropic->chat conversion, got %+v", message)
	}
	if _, ok := message["audio"].(map[string]any); !ok {
		t.Fatalf("expected audio to survive anthropic->chat conversion, got %+v", message)
	}
}

func TestAnthropicHandlerNormalizesToolChoiceForChatUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotToolChoice map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotToolChoice, _ = payload["tool_choice"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion","model":"gpt-backend","choices":[{"index":0,"message":{"role":"assistant","content":"chat says hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":3,"total_tokens":7}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:          "anthropic-tools-to-chat",
		ModelName:     "gpt-backend",
		APIBaseURL:    provider.URL,
		APIKey:        "chat-key",
		UpstreamType:  models.UpstreamTypeOpenAIChat,
		SupportsTools: true,
		Enabled:       true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-tools-to-chat","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"tools":[{"name":"lookup","description":"d","input_schema":{"type":"object"}}],"tool_choice":{"type":"tool","name":"lookup"},"max_tokens":16}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotToolChoice == nil || gotToolChoice["type"] != "function" {
		t.Fatalf("expected chat-normalized tool_choice, got %+v", gotToolChoice)
	}
}

func TestAnthropicHandlerTransformsResponsesCustomToolCallToAnthropicToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_custom_anthropic","object":"response","status":"completed","model":"backend-responses","output":[{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-custom-tool-to-anthropic",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-custom-tool-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	item := content[0].(map[string]any)
	if item["type"] != "tool_use" || item["name"] != "local_shell" {
		t.Fatalf("expected custom_tool_call to convert to anthropic tool_use, got %+v", payload)
	}
}

func TestAnthropicHandlerTransformsResponsesImageGenerationCallToAnthropicToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_img_anthropic","object":"response","status":"completed","model":"backend-responses","output":[{"type":"image_generation_call","id":"img_1","name":"image_generation_call","input":"generate a cat","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-imagegen-to-anthropic",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-imagegen-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	item := content[0].(map[string]any)
	if item["type"] != "tool_use" || item["name"] != "image_generation_call" {
		t.Fatalf("expected image_generation_call to convert to anthropic tool_use, got %+v", payload)
	}
}

func TestAnthropicHandlerTransformsResponsesMCPCallToAnthropicToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_mcp_anthropic","object":"response","status":"completed","model":"backend-responses","output":[{"type":"mcp_call","id":"mcp_1","call_id":"mcp_1","name":"fetch","arguments":"{\"q\":\"hello\"}","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-mcp-to-anthropic",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-mcp-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	item := content[0].(map[string]any)
	if item["type"] != "tool_use" || item["name"] != "fetch" {
		t.Fatalf("expected mcp_call to convert to anthropic tool_use, got %+v", payload)
	}
}

func TestAnthropicHandlerTransformsResponsesWebSearchCallToAnthropicToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_web_anthropic","object":"response","status":"completed","model":"backend-responses","output":[{"type":"web_search_call","id":"ws_1","name":"web_search_call","input":"OpenAI","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-websearch-to-anthropic",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-websearch-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	item := content[0].(map[string]any)
	if item["type"] != "tool_use" || item["name"] != "web_search_call" {
		t.Fatalf("expected web_search_call to convert to anthropic tool_use, got %+v", payload)
	}
}

func TestAnthropicHandlerPreservesResponsesAnnotationsRefusalAndAudio(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_rich_anthropic","object":"response","status":"completed","model":"backend-responses","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-rich-to-anthropic",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-rich-to-anthropic","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode anthropic response: %v", err)
	}
	content := payload["content"].([]any)
	if len(content) < 3 {
		t.Fatalf("expected rich anthropic content blocks, got %+v", payload)
	}
	var sawAnnotations, sawRefusal, sawAudio bool
	for _, raw := range content {
		item := raw.(map[string]any)
		if item["annotations"] != nil {
			sawAnnotations = true
		}
		if item["refusal"] == "blocked" {
			sawRefusal = true
		}
		if _, ok := item["audio"].(map[string]any); ok {
			sawAudio = true
		}
	}
	if !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected annotations/refusal/audio to survive responses->anthropic conversion, got %+v", content)
	}
}

func TestChatHandlerTransformsResponsesWebSearchCallToChatToolCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_web_chat","object":"response","status":"completed","model":"backend-responses","output":[{"type":"web_search_call","id":"ws_1","name":"web_search_call","input":"OpenAI","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-websearch-to-chat",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-websearch-to-chat","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	message := payload["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	toolCalls := message["tool_calls"].([]any)
	function := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if function["name"] != "web_search_call" || function["arguments"] != "OpenAI" {
		t.Fatalf("expected web_search_call to convert to chat tool_call, got %+v", message)
	}
}

func TestAnthropicHandlerPassthroughsToAnthropicUpstream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	var gotPath string
	var gotAPIKey string
	var gotModel string
	var gotSystem string
	var gotMessages []map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		gotModel, _ = payload["model"].(string)
		gotSystem, _ = payload["system"].(string)
		if messages, ok := payload["messages"].([]any); ok {
			for _, item := range messages {
				if msg, ok := item.(map[string]any); ok {
					gotMessages = append(gotMessages, msg)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message","role":"assistant","model":"backend-anthropic","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "auto-anthropic-passthrough",
		ModelName:    "backend-anthropic",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"auto-anthropic-passthrough","system":"sys","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPath != "/messages" {
		t.Fatalf("expected provider /messages path, got %q", gotPath)
	}
	if gotAPIKey != "anthropic-key" {
		t.Fatalf("expected x-api-key header, got %q", gotAPIKey)
	}
	if gotModel != "backend-anthropic" {
		t.Fatalf("expected rewritten backend model, got %q", gotModel)
	}
	if gotSystem != "sys" {
		t.Fatalf("expected passthrough system field, got %q", gotSystem)
	}
	if len(gotMessages) != 1 || gotMessages[0]["role"] != "user" {
		t.Fatalf("expected passthrough anthropic user message, got %+v", gotMessages)
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

func TestResponsesHandlerPreservesAnthropicStructuredAnnotationsRefusalAndAudio(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_rich_responses","type":"message","role":"assistant","model":"claude-backend","content":[{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc","url":"https://example.com"}]},{"type":"text","text":"blocked","refusal":"blocked"},{"type":"text","text":"","audio":{"id":"aud_1","format":"wav"}}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-rich-to-responses",
		ModelName:    "claude-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-rich-to-responses","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses payload: %v", err)
	}
	output := payload["output"].([]any)
	content := output[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["annotations"] == nil {
		t.Fatalf("expected annotations to survive anthropic->responses conversion, got %+v", content[0])
	}
	var sawRefusal, sawAudio bool
	for _, raw := range content {
		item := raw.(map[string]any)
		if item["type"] == "refusal" {
			sawRefusal = true
		}
		if item["type"] == "output_audio" {
			sawAudio = true
		}
	}
	if !sawRefusal || !sawAudio {
		t.Fatalf("expected refusal/audio to survive anthropic->responses conversion, got %+v", content)
	}
}

func TestResponsesHandlerTransformsAnthropicToolUseToResponsesFunctionCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_custom","type":"message","role":"assistant","model":"claude-backend","content":[{"type":"tool_use","id":"ctc_1","name":"local_shell","input":"ls -la"}],"stop_reason":"tool_use","usage":{"input_tokens":6,"output_tokens":4}}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "responses-custom-tool-to-anthropic",
		ModelName:    "claude-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-custom-tool-to-anthropic","input":"hello","stream":false}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode responses payload: %v", err)
	}
	output := payload["output"].([]any)
	first := output[0].(map[string]any)
	if first["type"] != "function_call" || first["name"] != "local_shell" || first["arguments"] != "\"ls -la\"" {
		t.Fatalf("expected anthropic tool_use to convert to responses function_call, got %+v", payload)
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

func TestChatHandlerTransformsAnthropicRicherTextBlockStreamToChatStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_rich","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}`,
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
		Name:         "chat-anthropic-rich-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-rich-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"content":"hello"`) ||
		(!strings.Contains(got, `"annotations":[{"title":"doc","type":"url_citation"}]`) && !strings.Contains(got, `"annotations":[{"type":"url_citation","title":"doc"}]`)) ||
		!strings.Contains(got, `"refusal":"blocked"`) ||
		!strings.Contains(got, `"id":"aud_1"`) {
		t.Fatalf("expected anthropic richer stream to convert to chat richer stream, got %s", got)
	}
}

func TestChatHandlerTransformsAnthropicToolUseStreamToChatToolCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hello\"}"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`,
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
		Name:         "chat-anthropic-tool-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-tool-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"tool_calls"`) || !strings.Contains(got, `"name":"lookup"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected anthropic tool_use stream to convert to chat tool_calls, got %s", got)
	}
}

func TestChatHandlerTransformsAnthropicMessageStopWithoutDeltaToChatFinish(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_stop_only","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
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
		Name:         "chat-anthropic-stop-only-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-stop-only-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"finish_reason":"tool_calls"`) || !strings.Contains(got, `data: [DONE]`) {
		t.Fatalf("expected message_stop flush to emit chat finish chunk, got %s", got)
	}
}

func TestChatHandlerTransformsAnthropicToolUseStartInputToChatToolArgs(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_start","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`,
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
		Name:         "chat-anthropic-tool-start-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-tool-start-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"name":"lookup"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected tool_use start input to convert to chat tool args, got %s", got)
	}
}

func TestChatHandlerTransformsAnthropicThinkingStartTextToChatReasoning(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_reasoning_start","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`,
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
		Name:         "chat-anthropic-reasoning-start-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-anthropic-reasoning-start-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"reasoning_content":"think step"`) {
		t.Fatalf("expected anthropic thinking start text to convert to chat reasoning, got %s", got)
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

func TestResponsesHandlerTransformsAnthropicRicherTextBlockStreamToResponsesStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_rich","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}`,
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
		Name:         "responses-anthropic-rich-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-rich-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.output_text.delta`) ||
		!strings.Contains(got, `"delta":"hello"`) ||
		!strings.Contains(got, `event: response.annotation.added`) ||
		!strings.Contains(got, `"refusal":"blocked"`) ||
		!strings.Contains(got, `"id":"aud_1"`) {
		t.Fatalf("expected anthropic richer stream to convert to responses richer stream, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesRicherStreamToAnthropicStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_rich","object":"response","status":"in_progress","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant"}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"delta":"hello"}`,
		"",
		"event: response.annotation.added",
		`data: {"type":"response.annotation.added","output_index":0,"annotations":[{"type":"url_citation","title":"doc"}]}`,
		"",
		"event: response.refusal.delta",
		`data: {"type":"response.refusal.delta","output_index":0,"delta":"blocked"}`,
		"",
		"event: response.audio.delta",
		`data: {"type":"response.audio.delta","output_index":0,"audio":{"id":"aud_1","format":"wav"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_rich","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2}}}`,
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
		Name:         "anthropic-responses-rich-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-rich-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: content_block_delta`) ||
		!strings.Contains(got, `"text":"hello"`) ||
		!strings.Contains(got, `"annotations"`) ||
		!strings.Contains(got, `"url_citation"`) ||
		!strings.Contains(got, `"refusal":"blocked"`) ||
		!strings.Contains(got, `"audio"`) ||
		!strings.Contains(got, `"aud_1"`) {
		t.Fatalf("expected responses richer stream to convert to anthropic richer stream, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesAddedRicherContentToAnthropicStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_rich_added","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2}}}`,
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
		Name:         "anthropic-responses-rich-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-rich-added-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"text":"hello"`) || !strings.Contains(got, `"annotations"`) || !strings.Contains(got, `"refusal":"blocked"`) || !strings.Contains(got, `"aud_1"`) {
		t.Fatalf("expected output_item.added richer content to convert to anthropic stream, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicThinkingStreamToResponsesReasoningStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_reasoning","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"thinking_delta","thinking":"think step"}}`,
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
		Name:         "responses-anthropic-reasoning-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-reasoning-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.reasoning.delta`) || !strings.Contains(got, `"delta":"think step"`) || !strings.Contains(got, `"type":"reasoning"`) {
		t.Fatalf("expected anthropic thinking stream to convert to responses reasoning stream, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicThinkingStartTextToResponsesReasoning(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_reasoning_start","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`,
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
		Name:         "responses-anthropic-reasoning-start-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-reasoning-start-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.reasoning.delta`) || !strings.Contains(got, `"delta":"think step"`) {
		t.Fatalf("expected anthropic thinking start text to convert to responses reasoning, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicToolUseStreamToResponsesFunctionCallStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hello\"}"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`,
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
		Name:         "responses-anthropic-tool-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-tool-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.output_item.added`) ||
		!strings.Contains(got, `"type":"function_call"`) ||
		!strings.Contains(got, `"delta":"{\"q\":\"hello\"}"`) ||
		!strings.Contains(got, `event: response.function_call_arguments.done`) ||
		!strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) ||
		!strings.Contains(got, `event: response.output_item.done`) ||
		!strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected anthropic tool_use stream to convert to responses function_call stream, got %s", got)
	}
	if strings.Count(got, `event: response.completed`) != 1 {
		t.Fatalf("expected exactly one response.completed event, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicMessageStopWithoutDeltaToResponsesCompleted(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_stop_only","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
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
		Name:         "responses-anthropic-stop-only-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-stop-only-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: response.output_item.done`) || !strings.Contains(got, `event: response.completed`) || !strings.Contains(got, `data: [DONE]`) {
		t.Fatalf("expected message_stop flush to emit responses completion lifecycle, got %s", got)
	}
	if strings.Count(got, `event: response.completed`) != 1 {
		t.Fatalf("expected exactly one response.completed event, got %s", got)
	}
}

func TestResponsesHandlerTransformsAnthropicToolUseStartInputToResponsesFunctionCall(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_start","type":"message","role":"assistant","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`,
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
		Name:         "responses-anthropic-tool-start-stream",
		ModelName:    "claude-stream",
		APIBaseURL:   provider.URL,
		APIKey:       "anthropic-key",
		UpstreamType: models.UpstreamTypeAnthropicMessages,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"responses-anthropic-tool-start-stream","input":"hello","stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).Responses(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"function_call"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected tool_use start input to convert to responses function_call args, got %s", got)
	}
	if strings.Count(got, `event: response.function_call_arguments.done`) != 1 {
		t.Fatalf("expected exactly one arguments.done event for start-only input, got %s", got)
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

func TestAnthropicHandlerTransformsChatRicherStreamToAnthropicStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"chat-backend","choices":[{"index":0,"delta":{"content":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}]}`,
		`data: {"id":"chatcmpl_rich","object":"chat.completion.chunk","created":1,"model":"chat-backend","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(stream))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "anthropic-chat-rich-stream",
		ModelName:    "chat-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "chat-key",
		UpstreamType: models.UpstreamTypeOpenAIChat,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-chat-rich-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: content_block_delta`) ||
		!strings.Contains(got, `"text":"hello"`) ||
		!strings.Contains(got, `"annotations"`) ||
		!strings.Contains(got, `"url_citation"`) ||
		!strings.Contains(got, `"refusal":"blocked"`) ||
		!strings.Contains(got, `"audio"`) ||
		!strings.Contains(got, `"aud_1"`) {
		t.Fatalf("expected chat richer stream to convert to anthropic richer stream, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesReasoningStreamToAnthropicThinkingStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":""}]}}`,
		"",
		"event: response.reasoning.delta",
		`data: {"type":"response.reasoning.delta","output_index":50,"item_id":"rs_1","delta":"think step"}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-reasoning-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-reasoning-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: content_block_start`) || !strings.Contains(got, `"type":"thinking"`) || !strings.Contains(got, `"thinking":"think step"`) || !strings.Contains(got, `event: content_block_stop`) {
		t.Fatalf("expected responses reasoning stream to convert to anthropic thinking stream, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesReasoningOutputItemDoneToAnthropicThinking(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reasoning_done","object":"response","status":"in_progress","model":"resp-backend"}}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning_done","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-reasoning-item-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-reasoning-item-done-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: content_block_start`) || !strings.Contains(got, `"type":"thinking"`) || !strings.Contains(got, `"thinking":"think step"`) || !strings.Contains(got, `event: content_block_stop`) {
		t.Fatalf("expected responses reasoning output_item.done to convert to anthropic thinking, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesReasoningAddedSummaryToAnthropicThinking(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reasoning_added","object":"response","status":"in_progress","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":"think step"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning_added","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-reasoning-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-reasoning-added-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `event: content_block_start`) || !strings.Contains(got, `"type":"thinking"`) || !strings.Contains(got, `"thinking":"think step"`) {
		t.Fatalf("expected responses reasoning added summary to convert to anthropic thinking start, got %s", got)
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

func TestChatHandlerTransformsResponsesToolArgumentsDoneToChatToolArgs(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_done","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":5,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_done","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-tool-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-tool-done-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"name":"lookup"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected responses tool arguments.done to convert to chat tool args, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesToolOutputItemDoneToChatToolArgs(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_item_done","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_item_done","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-tool-item-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-tool-item-done-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"name":"lookup"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected responses output_item.done to convert to chat tool args, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesToolAddedArgumentsToChatToolArgs(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_added","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress","arguments":"{\"q\":\"hello\"}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_added","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-tool-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-tool-added-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"name":"lookup"`) || !strings.Contains(got, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected output_item.added arguments to populate chat tool args, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesDoneWithoutCompletedToChatFinish(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_done_only","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
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
		Name:         "chat-responses-done-only-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-done-only-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"finish_reason":"tool_calls"`) || !strings.Contains(got, `data: [DONE]`) {
		t.Fatalf("expected [DONE] flush to emit chat finish chunk, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesReasoningOutputItemDoneToChatReasoning(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reasoning_item_done","model":"resp-backend"}}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning_item_done","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-reasoning-item-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-reasoning-item-done-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"reasoning_content":"think step"`) {
		t.Fatalf("expected responses reasoning output_item.done to convert to chat reasoning, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesReasoningAddedSummaryToChatReasoning(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reasoning_added","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":"think step"}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_reasoning_added","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-reasoning-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-reasoning-added-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"reasoning_content":"think step"`) {
		t.Fatalf("expected responses reasoning added summary to convert to chat reasoning, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesAddedRicherContentToChatStream(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_start_only","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_start_only","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-rich-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-rich-added-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"content":"hello"`) || !strings.Contains(got, `"annotations"`) || !strings.Contains(got, `"refusal":"blocked"`) || !strings.Contains(got, `"aud_1"`) {
		t.Fatalf("expected output_item.added richer content to convert to chat stream, got %s", got)
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
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`,
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
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `event: content_block_stop`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected responses tool stream to convert to anthropic tool_use, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesToolAddedArgumentsToToolUseInput(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_added","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"in_progress","arguments":"{\"q\":\"hello\"}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_added","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-tool-added-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-tool-added-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"input":{"q":"hello"}`) {
		t.Fatalf("expected output_item.added arguments to populate anthropic tool_use start input, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesDoneWithoutCompletedToAnthropicToolUseStop(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_done_only","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"in_progress"}}`,
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
		Name:         "anthropic-responses-done-only-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-done-only-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `event: message_delta`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected [DONE] flush to infer anthropic tool_use stop_reason, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesToolArgumentsDoneToToolUseInput(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_done_only","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":2,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_done_only","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-tool-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-tool-done-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"partial_json":"{\"q\":\"hello\"}"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected responses function_call_arguments.done to convert to anthropic tool input, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesToolOutputItemDoneToToolUseInput(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_item_done","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.output_item.done",
		`data: {"type":"response.output_item.done","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"fc_2","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_item_done","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-tool-item-done-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-tool-item-done-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"partial_json":"{\"q\":\"hello\"}"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected responses output_item.done to convert to anthropic tool input, got %s", got)
	}
}

func TestChatHandlerTransformsResponsesCustomToolStreamToChatToolFinish(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_tool_custom","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":5,"delta":"ls -la"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_tool_custom","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "chat-responses-custom-tool-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"chat-responses-custom-tool-stream","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).ChatCompletion(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"name":"local_shell"`) || !strings.Contains(got, `"arguments":"ls -la"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
		t.Fatalf("expected responses custom tool stream to convert to chat tool finish, got %s", got)
	}
}

func TestAnthropicHandlerTransformsResponsesCustomToolStreamToToolUse(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)

	stream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_custom_stream","model":"resp-backend"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"custom_tool_call","id":"ctc_2","call_id":"ctc_2","name":"local_shell","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":2,"delta":"ls -la"}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_custom_stream","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
		Name:         "anthropic-responses-custom-tool-stream",
		ModelName:    "resp-backend",
		APIBaseURL:   provider.URL,
		APIKey:       "responses-key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{"model":"anthropic-responses-custom-tool-stream","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	NewChatHandler(db).AnthropicMessages(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	got := recorder.Body.String()
	if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"name":"local_shell"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected responses custom tool stream to convert to anthropic tool_use, got %s", got)
	}
}

func TestChatHandlerTransformsAdditionalResponsesBuiltInStreamsToChatToolFinish(t *testing.T) {
	cases := []struct {
		name     string
		itemType string
		itemName string
		delta    string
		wantName string
		wantArgs string
	}{
		{name: "mcp", itemType: "mcp_call", itemName: "fetch", delta: `{"q":"hello"}`, wantName: "fetch", wantArgs: `{"q":"hello"}`},
		{name: "web-search", itemType: "web_search_call", itemName: "web_search_call", delta: `OpenAI`, wantName: "web_search_call", wantArgs: `OpenAI`},
		{name: "image-generation", itemType: "image_generation_call", itemName: "image_generation_call", delta: `generate a cat`, wantName: "image_generation_call", wantArgs: `generate a cat`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetBackendRuntimeManagerForTests()
			InvalidateAllModelConfigCache()
			db := newModelConfigTestDB(t)

			stream := strings.Join([]string{
				"event: response.created",
				`data: {"type":"response.created","response":{"id":"resp_builtin_stream","model":"resp-backend"}}`,
				"",
				"event: response.output_item.added",
				`data: {"type":"response.output_item.added","output_index":5,"item":{"type":"` + tc.itemType + `","id":"call_1","call_id":"call_1","name":"` + tc.itemName + `","status":"in_progress"}}`,
				"",
				"event: response.function_call_arguments.delta",
				`data: {"type":"response.function_call_arguments.delta","output_index":5,"delta":` + mustJSONStringForTest(tc.delta) + `}`,
				"",
				"event: response.completed",
				`data: {"type":"response.completed","response":{"id":"resp_builtin_stream","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
				Name:         "chat-responses-builtin-stream-" + tc.name,
				ModelName:    "resp-backend",
				APIBaseURL:   provider.URL,
				APIKey:       "responses-key",
				UpstreamType: models.UpstreamTypeOpenAIResponses,
				Enabled:      true,
			}
			if err := db.Create(&config).Error; err != nil {
				t.Fatalf("failed to seed model config: %v", err)
			}

			body := []byte(`{"model":"chat-responses-builtin-stream-` + tc.name + `","messages":[{"role":"user","content":"hello"}],"stream":true}`)
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			recorder := httptest.NewRecorder()

			NewChatHandler(db).ChatCompletion(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			got := recorder.Body.String()
			if !strings.Contains(got, `"name":"`+tc.wantName+`"`) || !strings.Contains(got, `"arguments":"`+strings.ReplaceAll(tc.wantArgs, `"`, `\"`)+`"`) || !strings.Contains(got, `"finish_reason":"tool_calls"`) {
				t.Fatalf("expected %s stream to convert to chat tool finish, got %s", tc.itemType, got)
			}
		})
	}
}

func TestAnthropicHandlerTransformsAdditionalResponsesBuiltInStreamsToToolUse(t *testing.T) {
	cases := []struct {
		name     string
		itemType string
		itemName string
		delta    string
		wantName string
	}{
		{name: "mcp", itemType: "mcp_call", itemName: "fetch", delta: `{"q":"hello"}`, wantName: "fetch"},
		{name: "web-search", itemType: "web_search_call", itemName: "web_search_call", delta: `OpenAI`, wantName: "web_search_call"},
		{name: "image-generation", itemType: "image_generation_call", itemName: "image_generation_call", delta: `generate a cat`, wantName: "image_generation_call"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetBackendRuntimeManagerForTests()
			InvalidateAllModelConfigCache()
			db := newModelConfigTestDB(t)

			stream := strings.Join([]string{
				"event: response.created",
				`data: {"type":"response.created","response":{"id":"resp_builtin_stream","model":"resp-backend"}}`,
				"",
				"event: response.output_item.added",
				`data: {"type":"response.output_item.added","output_index":2,"item":{"type":"` + tc.itemType + `","id":"call_2","call_id":"call_2","name":"` + tc.itemName + `","status":"in_progress"}}`,
				"",
				"event: response.function_call_arguments.delta",
				`data: {"type":"response.function_call_arguments.delta","output_index":2,"delta":` + mustJSONStringForTest(tc.delta) + `}`,
				"",
				"event: response.completed",
				`data: {"type":"response.completed","response":{"id":"resp_builtin_stream","object":"response","status":"completed","model":"resp-backend","usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}}}`,
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
				Name:         "anthropic-responses-builtin-stream-" + tc.name,
				ModelName:    "resp-backend",
				APIBaseURL:   provider.URL,
				APIKey:       "responses-key",
				UpstreamType: models.UpstreamTypeOpenAIResponses,
				Enabled:      true,
			}
			if err := db.Create(&config).Error; err != nil {
				t.Fatalf("failed to seed model config: %v", err)
			}

			body := []byte(`{"model":"anthropic-responses-builtin-stream-` + tc.name + `","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16,"stream":true}`)
			request := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			recorder := httptest.NewRecorder()

			NewChatHandler(db).AnthropicMessages(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			got := recorder.Body.String()
			if !strings.Contains(got, `"type":"tool_use"`) || !strings.Contains(got, `"name":"`+tc.wantName+`"`) || !strings.Contains(got, `"stop_reason":"tool_use"`) {
				t.Fatalf("expected %s stream to convert to anthropic tool_use, got %s", tc.itemType, got)
			}
		})
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
