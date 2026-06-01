package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llm-gateway/models"
	"llm-gateway/protocol"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDecodeReplayIRPartPrefersIRFields(t *testing.T) {
	event := protocol.IRStreamEvent{
		Text:        "hello",
		Refusal:     "blocked",
		Annotations: json.RawMessage(`[{"type":"url_citation","title":"doc"}]`),
		Audio:       json.RawMessage(`{"id":"aud_1","format":"wav"}`),
	}

	part := decodeReplayIRPart(event, map[string]interface{}{"type": "output_text"})
	if part["refusal"] != "blocked" {
		t.Fatalf("expected refusal from IR, got %+v", part)
	}
	if _, ok := part["annotations"].([]interface{}); !ok {
		t.Fatalf("expected annotations from IR, got %+v", part)
	}
	if _, ok := part["audio"].(map[string]interface{}); !ok {
		t.Fatalf("expected audio from IR, got %+v", part)
	}
}

func TestLogQueryValueSupportsSnakeCase(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/request-logs?backend_config_id=7", nil)

	value := queryValue(request, "backend_config_id")

	if value != "7" {
		t.Fatalf("expected snake_case value, got %q", value)
	}
}

func TestGetLogsReturnsSummaryWithoutLargeBodies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	requestBody := `{"messages":[{"role":"user","content":"` + strings.Repeat("hello ", 50) + `"}]}`
	if err := db.Create(&models.RequestLog{
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		ModelName:         "gpt-test",
		BackendConfigID:   7,
		BackendModelName:  "backend-test",
		BackendAPIBaseURL: "http://provider",
		Request:           requestBody,
		Response:          `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"custom_tool_call","id":"ctc_1","name":"local_shell","status":"completed"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1"}}]}]}`,
		StreamResponse:    []byte(strings.Repeat("stream", 1000)),
		ResponseTime:      123,
	}).Error; err != nil {
		t.Fatalf("create request log: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/request-logs?page_size=500", nil)
	NewLogHandler(db).GetLogs(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response LogResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Logs) != 1 {
		t.Fatalf("expected one log, total=%d len=%d", response.Total, len(response.Logs))
	}

	log := response.Logs[0]
	if log.RequestPreview == "" || len(log.RequestPreview) > 123 {
		t.Fatalf("unexpected request preview %q", log.RequestPreview)
	}
	if log.ResponseBytes == 0 || log.StreamBytes == 0 {
		t.Fatalf("expected body sizes, got response=%d stream=%d", log.ResponseBytes, log.StreamBytes)
	}
	if log.Semantic.Protocol != "responses" {
		t.Fatalf("expected semantic protocol responses, got %+v", log.Semantic)
	}
	if len(log.Semantic.OutputItemTypes) == 0 || log.Semantic.OutputItemTypes[0] != "custom_tool_call" {
		t.Fatalf("expected output item types in semantic summary, got %+v", log.Semantic)
	}
	if !log.Semantic.HasRefusal || !log.Semantic.HasAudio || log.Semantic.AnnotationCount != 1 {
		t.Fatalf("expected refusal/audio/annotations in semantic summary, got %+v", log.Semantic)
	}
	if log.Semantic.ReasoningSummary != "" {
		t.Fatalf("expected plain assistant output_text not to populate reasoning summary, got %+v", log.Semantic)
	}
}

func TestResponsesSemanticSummaryUsesReasoningItemNotMessageText(t *testing.T) {
	summary := summarizeResponseSemantics(`{"id":"resp_reasoning","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"final answer"}]},{"type":"reasoning","summary":[{"type":"summary_text","text":"private reasoning"}]}]}`)

	if summary.Protocol != "responses" {
		t.Fatalf("expected responses protocol, got %+v", summary)
	}
	if summary.ReasoningSummary != "private reasoning" {
		t.Fatalf("expected reasoning summary from reasoning item, got %+v", summary)
	}
}

func TestGetLogDetailReturnsSemanticSummary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	entry := models.RequestLog{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ModelName: "gpt-chat",
		Request:   `{"messages":[{"role":"user","content":"hello"}]}`,
		Response:  `{"id":"chatcmpl_1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong","reasoning_content":"private reasoning","refusal":"cannot comply","audio":{"id":"aud_1"},"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create request log: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/request-logs/detail?id=1", nil)
	NewLogHandler(db).GetLogDetail(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		RequestPreview string             `json:"request_preview"`
		Semantic       LogSemanticSummary `json:"semantic"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if payload.RequestPreview != "hello" {
		t.Fatalf("expected request preview hello, got %q", payload.RequestPreview)
	}
	if payload.Semantic.Protocol != "chat" || payload.Semantic.FinishReason != "tool_calls" {
		t.Fatalf("expected chat semantic summary, got %+v", payload.Semantic)
	}
	if payload.Semantic.ReasoningSummary != "private reasoning" || !payload.Semantic.HasRefusal || !payload.Semantic.HasAudio {
		t.Fatalf("expected reasoning/refusal/audio in semantic summary, got %+v", payload.Semantic)
	}
	if len(payload.Semantic.ToolNames) != 1 || payload.Semantic.ToolNames[0] != "lookup" {
		t.Fatalf("expected tool names in semantic summary, got %+v", payload.Semantic)
	}
}

func TestAnthropicSemanticSummaryIncludesAnnotationsRefusalAndAudio(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	entry := models.RequestLog{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ModelName: "claude-rich",
		Request:   `{"messages":[{"role":"user","content":"hello"}]}`,
		Response:  `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello","annotations":[{"type":"url_citation"}]},{"type":"text","text":"blocked","refusal":"blocked"},{"type":"text","text":"","audio":{"id":"aud_1"}},{"type":"thinking","thinking":"private reasoning"}],"stop_reason":"end_turn"}`,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create request log: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/request-logs/detail?id=1", nil)
	NewLogHandler(db).GetLogDetail(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Semantic LogSemanticSummary `json:"semantic"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if payload.Semantic.Protocol != "anthropic_messages" {
		t.Fatalf("expected anthropic semantic summary, got %+v", payload.Semantic)
	}
	if payload.Semantic.AnnotationCount != 1 || !payload.Semantic.HasRefusal || !payload.Semantic.HasAudio {
		t.Fatalf("expected annotations/refusal/audio in anthropic semantic summary, got %+v", payload.Semantic)
	}
	if payload.Semantic.Refusal != "blocked" || payload.Semantic.ReasoningSummary != "private reasoning" {
		t.Fatalf("expected refusal/reasoning preserved in anthropic semantic summary, got %+v", payload.Semantic)
	}
}

func TestReplayLogPreservesRicherResponsesPayload(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateAllModelConfigCache()
	db := newModelConfigTestDB(t)
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	var gotPreviousResponseID string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode replay provider request: %v", err)
		}
		gotPreviousResponseID, _ = payload["previous_response_id"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_replayed","object":"response","status":"completed","output":[{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:         "replay-rich",
		ModelName:    "backend-responses",
		APIBaseURL:   provider.URL,
		APIKey:       "key",
		UpstreamType: models.UpstreamTypeOpenAIResponses,
		Enabled:      true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	logEntry := models.RequestLog{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ModelName: "replay-rich",
		Request:   `{"model":"replay-rich","input":"hello","previous_response_id":"resp_prev_replay","stream":false}`,
		Response:  `{"id":"resp_original","object":"response","status":"completed","output":[]}`,
	}
	if err := db.Create(&logEntry).Error; err != nil {
		t.Fatalf("failed to seed request log: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/request-logs/replay?id=1", bytes.NewReader([]byte(`{"override":{}}`)))
	NewLogHandler(db).ReplayLog(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotPreviousResponseID != "resp_prev_replay" {
		t.Fatalf("expected previous_response_id preserved in replay, got %q", gotPreviousResponseID)
	}

	var payload ReplayResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if !strings.Contains(payload.ModifiedRequest, `"previous_response_id":"resp_prev_replay"`) {
		t.Fatalf("expected modified request to preserve previous_response_id, got %s", payload.ModifiedRequest)
	}
	if !strings.Contains(payload.NewResponse, `"type":"custom_tool_call"`) {
		t.Fatalf("expected replayed richer response payload, got %s", payload.NewResponse)
	}
}

func TestProcessStreamResponsePreservesRicherChatPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"reasoning_content":"think"}}]}`,
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"annotations":[{"type":"url_citation","title":"doc"}]}}]}`,
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"refusal":"cannot comply"}}]}`,
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"audio":{"id":"aud_1","format":"wav"}}}]}`,
		`data: {"id":"chatcmpl-rich","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"reasoning_content":"think"`) {
		t.Fatalf("expected reasoning_content preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"refusal":"cannot comply"`) {
		t.Fatalf("expected refusal preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"audio":{"format":"wav","id":"aud_1"}`) && !strings.Contains(aggregated, `"audio":{"id":"aud_1","format":"wav"}`) {
		t.Fatalf("expected audio preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"annotations":[{"title":"doc","type":"url_citation"}]`) && !strings.Contains(aggregated, `"annotations":[{"type":"url_citation","title":"doc"}]`) {
		t.Fatalf("expected annotations preserved, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesResponsesCompletedPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","status":"in_progress"}}`,
		"",
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_replay","object":"response","status":"completed","model":"resp-backend","output":[{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"}]}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"object":"response"`) || !strings.Contains(aggregated, `"type":"custom_tool_call"`) {
		t.Fatalf("expected responses completed payload preserved, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesPayloadWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reconstruct","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"ls -la"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"object":"response"`) || !strings.Contains(aggregated, `"type":"custom_tool_call"`) {
		t.Fatalf("expected reconstructed responses payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"input":"ls -la"`) && !strings.Contains(aggregated, `"arguments":"ls -la"`) {
		t.Fatalf("expected reconstructed tool arguments, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"status":"completed"`) {
		t.Fatalf("expected reconstructed response status completed, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"type":"custom_tool_call"`) || !strings.Contains(aggregated, `"status":"completed"`) {
		t.Fatalf("expected reconstructed output item status completed, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesPayloadFromArgumentsDoneWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reconstruct_done","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"object":"response"`) || !strings.Contains(aggregated, `"type":"function_call"`) {
		t.Fatalf("expected reconstructed responses payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected reconstructed tool arguments from arguments.done, got %s", aggregated)
	}
}

func TestProcessStreamResponsePrefersFinalArgumentsDoneOverPartialDelta(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reconstruct_done_final","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"fc_1","name":"lookup","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"q\":\"he"}`,
		"",
		"event: response.function_call_arguments.done",
		`data: {"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"q\":\"hello\"}"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"arguments":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected final arguments.done to override partial delta, got %s", aggregated)
	}
	if strings.Contains(aggregated, `"arguments":"{\"q\":\"he"`) {
		t.Fatalf("expected partial delta not to survive after arguments.done, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesFileSearchPayloadWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_file_search","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"file_search_call","id":"fs_1","call_id":"fs_1","name":"file_search","status":"in_progress"}}`,
		"",
		"event: response.function_call_arguments.delta",
		`data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"search docs"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"object":"response"`) || !strings.Contains(aggregated, `"type":"file_search_call"`) {
		t.Fatalf("expected reconstructed file_search_call payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"input":"search docs"`) {
		t.Fatalf("expected reconstructed file_search input, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesRicherMessageWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_rich_reconstruct","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`,
		"",
		"event: response.annotation.added",
		`data: {"type":"response.annotation.added","output_index":0,"annotations":[{"type":"url_citation","title":"doc"}]}`,
		"",
		"event: response.refusal.done",
		`data: {"type":"response.refusal.done","output_index":0,"content_index":1,"refusal":"blocked"}`,
		"",
		"event: response.audio.done",
		`data: {"type":"response.audio.done","output_index":0,"content_index":2,"audio":{"id":"aud_1","format":"wav"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"object":"response"`) || !strings.Contains(aggregated, `"type":"message"`) {
		t.Fatalf("expected reconstructed responses message payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"annotations":[{"title":"doc","type":"url_citation"}]`) && !strings.Contains(aggregated, `"annotations":[{"type":"url_citation","title":"doc"}]`) {
		t.Fatalf("expected reconstructed annotations, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"type":"refusal"`) || !strings.Contains(aggregated, `"refusal":"blocked"`) {
		t.Fatalf("expected reconstructed refusal, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"type":"output_audio"`) || !strings.Contains(aggregated, `"id":"aud_1"`) {
		t.Fatalf("expected reconstructed audio, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesStartOnlyMessageOutputTextWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_start_only","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"output_text":"hello"`) {
		t.Fatalf("expected start-only message content to populate output_text, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesStartOnlyRicherPartsWhenLaterTextDeltaArrives(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_mixed_reconstruct","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[{"type":"output_text","text":""},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"text":"hello"`) {
		t.Fatalf("expected reconstructed text, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"type":"output_audio"`) || !strings.Contains(aggregated, `"id":"aud_1"`) {
		t.Fatalf("expected start-only richer part preserved, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsMultipleOutputTextPartsIndependently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_multi_text","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":1,"part":{"type":"output_text","text":""}}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`,
		"",
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","output_index":0,"content_index":1,"delta":"world"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"content":[{"text":"hello","type":"output_text"},{"text":"world","type":"output_text"}]`) &&
		!strings.Contains(aggregated, `"content":[{"type":"output_text","text":"hello"},{"type":"output_text","text":"world"}]`) {
		t.Fatalf("expected independent output_text parts, got %s", aggregated)
	}
}

func TestProcessStreamResponsePlacesAnnotationsOnCorrectContentPart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_annotations_parts","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":"hello"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":1,"part":{"type":"output_text","text":"world"}}`,
		"",
		"event: response.annotation.added",
		`data: {"type":"response.annotation.added","output_index":0,"content_index":1,"annotations":[{"type":"url_citation","title":"doc"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"content":[{"text":"hello","type":"output_text"},{"annotations":[{"title":"doc","type":"url_citation"}],"text":"world","type":"output_text"}]`) &&
		!strings.Contains(aggregated, `"content":[{"type":"output_text","text":"hello"},{"type":"output_text","text":"world","annotations":[{"type":"url_citation","title":"doc"}]}]`) {
		t.Fatalf("expected annotation on second content part, got %s", aggregated)
	}
}

func TestProcessStreamResponsePlacesRicherPartsOnCorrectContentIndexes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_richer_parts","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":"hello"}}`,
		"",
		"event: response.content_part.added",
		`data: {"type":"response.content_part.added","output_index":0,"content_index":1,"part":{"type":"output_text","text":"world"}}`,
		"",
		"event: response.annotation.added",
		`data: {"type":"response.annotation.added","output_index":0,"content_index":1,"annotations":[{"type":"url_citation","title":"doc"}]}`,
		"",
		"event: response.refusal.done",
		`data: {"type":"response.refusal.done","output_index":0,"content_index":2,"refusal":"blocked"}`,
		"",
		"event: response.audio.done",
		`data: {"type":"response.audio.done","output_index":0,"content_index":3,"audio":{"id":"aud_1","format":"wav"}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"content":[{"text":"hello","type":"output_text"},{"annotations":[{"title":"doc","type":"url_citation"}],"text":"world","type":"output_text"},{"refusal":"blocked","type":"refusal"},{"audio":{"format":"wav","id":"aud_1"},"type":"output_audio"}]`) &&
		!strings.Contains(aggregated, `"content":[{"type":"output_text","text":"hello"},{"type":"output_text","text":"world","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]`) {
		t.Fatalf("expected richer parts on correct content indexes, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsResponsesReasoningSummaryWithoutCompletedEvent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_reasoning_reconstruct","model":"resp-backend","status":"in_progress"}}`,
		"",
		"event: response.output_item.added",
		`data: {"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":""}]}}`,
		"",
		"event: response.reasoning.delta",
		`data: {"type":"response.reasoning.delta","output_index":50,"item_id":"rs_1","delta":"think step"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"reasoning"`) {
		t.Fatalf("expected reconstructed reasoning item, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"summary":[{"text":"think step","type":"summary_text"}]`) &&
		!strings.Contains(aggregated, `"summary":[{"type":"summary_text","text":"think step"}]`) {
		t.Fatalf("expected reconstructed reasoning summary, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesAnthropicCompletedPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"thinking_delta","thinking":"think step"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hello\"}"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":3,"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"thinking"`) || !strings.Contains(aggregated, `"type":"tool_use"`) {
		t.Fatalf("expected anthropic completed payload preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected anthropic stop_reason preserved, got %s", aggregated)
	}
}

func TestProcessStreamResponseInfersAnthropicStopReasonFromMessageStopWithoutDelta(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_stop_only","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"tool_use"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected anthropic stop_reason inferred from message_stop, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesAnthropicToolUseStartInput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_start","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"tool_use"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"input":{"q":"hello"}`) {
		t.Fatalf("expected anthropic replay to preserve tool_use start input, got %s", aggregated)
	}
}

func TestProcessStreamResponseInfersAnthropicStopReasonFromTruncatedToolUseReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_tool_truncated","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"tool_use"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected anthropic stop_reason inferred from truncated replay, got %s", aggregated)
	}
}

func TestProcessStreamResponseReconstructsAnthropicEmptyMessageReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_empty","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":3,"output_tokens":0}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"content":[]`) {
		t.Fatalf("expected anthropic empty message replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"stop_reason":"end_turn"`) {
		t.Fatalf("expected anthropic empty message stop_reason preserved, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesAnthropicThinkingStartText(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_reasoning_start","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"thinking"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"thinking":"think step"`) {
		t.Fatalf("expected anthropic replay to preserve thinking start text, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesAnthropicThinkingStartTextWithoutDelta(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_reasoning_start_only","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) || !strings.Contains(aggregated, `"type":"thinking"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"thinking":"think step"`) {
		t.Fatalf("expected anthropic replay to preserve thinking start-only text, got %s", aggregated)
	}
}

func TestProcessStreamResponsePreservesAnthropicRicherContentPayload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	handler := NewLogHandler(db)

	rawStream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_rich","type":"message","role":"assistant","model":"claude"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":"blocked","refusal":"blocked"}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":2,"content_block":{"type":"text","text":"","audio":{"id":"aud_1","format":"wav"}}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":3,"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(rawStream))}
	aggregated := handler.processStreamResponse(resp)

	if !strings.Contains(aggregated, `"type":"message"`) {
		t.Fatalf("expected anthropic replay payload, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"annotations":[{"title":"doc","type":"url_citation"}]`) && !strings.Contains(aggregated, `"annotations":[{"type":"url_citation","title":"doc"}]`) {
		t.Fatalf("expected anthropic replay annotations preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"refusal":"blocked"`) {
		t.Fatalf("expected anthropic replay refusal preserved, got %s", aggregated)
	}
	if !strings.Contains(aggregated, `"audio":{"format":"wav","id":"aud_1"}`) && !strings.Contains(aggregated, `"audio":{"id":"aud_1","format":"wav"}`) {
		t.Fatalf("expected anthropic replay audio preserved, got %s", aggregated)
	}
}
