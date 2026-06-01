package protocol

import (
	"encoding/json"
	"testing"
)

func TestOpenAIChatResponseIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"model":"gpt-test",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello","reasoning_content":"think","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if ir.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls finish, got %q", ir.FinishReason)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) < 2 {
		t.Fatalf("expected assistant text + tool call, got %+v", ir.OutputItems)
	}
	if got := encodeReasoningContent(ir.OutputItems[0].Content); got != "think" {
		t.Fatalf("expected reasoning content think, got %q", got)
	}
	encoded, err := EncodeOpenAIChatResponse(ir, "backend-chat")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat response: %v", err)
	}
	if payload["model"] != "backend-chat" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	message := payload["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["reasoning_content"] != "think" {
		t.Fatalf("expected reasoning_content to round-trip, got %+v", message["reasoning_content"])
	}
}

func TestOpenAIChatResponseExtractsThinkTagsFromContent(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_think",
		"object":"chat.completion",
		"model":"gpt-think",
		"choices":[{"index":0,"message":{"role":"assistant","content":"<think>private reasoning</think>\n\npong"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 {
		t.Fatalf("expected one output item, got %+v", ir.OutputItems)
	}
	if got := encodeTextContent(ir.OutputItems[0].Content); got != "pong" {
		t.Fatalf("expected think tags removed from content, got %q", got)
	}
	if got := encodeReasoningContent(ir.OutputItems[0].Content); got != "private reasoning" {
		t.Fatalf("expected reasoning extracted from think tags, got %q", got)
	}
}

func TestOpenAIChatResponseVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_img",
		"object":"chat.completion",
		"model":"gpt-vision",
		"choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "image" {
		t.Fatalf("expected image part in chat response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeOpenAIChatResponse(ir, "backend-chat")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat response: %v", err)
	}
	content := payload["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "image_url" {
		t.Fatalf("expected image_url part to round-trip, got %+v", content[1])
	}
}

func TestOpenAIChatResponseFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_file",
		"object":"chat.completion",
		"model":"gpt-file",
		"choices":[{"index":0,"message":{"role":"assistant","content":[{"type":"text","text":"hello"},{"type":"input_file","file_id":"file_123"}]},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "file" {
		t.Fatalf("expected file part in chat response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeOpenAIChatResponse(ir, "backend-chat")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat response: %v", err)
	}
	content := payload["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "input_file" {
		t.Fatalf("expected input_file part to round-trip, got %+v", content[1])
	}
}

func TestResponsesResponseIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"resp_1",
		"object":"response",
		"status":"completed",
		"model":"resp-test",
		"metadata":{"trace_id":"abc"},
		"incomplete_details":{"reason":"max_output_tokens"},
		"error":{"message":"soft failure","type":"server_error"},
		"conversation":{"id":"conv_1"},
		"prompt":{"id":"pmpt_1","version":"2"},
		"reasoning":{"effort":"medium"},
		"text":{"format":{"type":"json_schema"}},
		"tool_choice":{"type":"auto"},
		"tools":[{"type":"function","name":"lookup"}],
		"output":[
			{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"hello\"}","status":"completed"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"think"}],"status":"completed"},
			{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]}
		],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	if ir.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls finish, got %q", ir.FinishReason)
	}
	if ir.Status != "completed" || string(ir.Metadata) == "" || string(ir.IncompleteDetails) == "" || string(ir.Error) == "" {
		t.Fatalf("expected top-level metadata preserved, got %+v", ir)
	}
	if string(ir.Conversation) == "" || string(ir.Prompt) == "" || string(ir.ReasoningConfig) == "" || string(ir.TextConfig) == "" || string(ir.ToolChoice) == "" || string(ir.Tools) == "" {
		t.Fatalf("expected top-level responses configs preserved, got %+v", ir)
	}
	var sawReasoning bool
	for _, msg := range ir.OutputItems {
		for _, part := range msg.Content {
			if part.Type == "reasoning" && part.Text == "think" {
				sawReasoning = true
			}
		}
	}
	if !sawReasoning {
		t.Fatalf("expected reasoning part in IR output, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeResponsesResponse(ir, "backend-resp")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	if payload["model"] != "backend-resp" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	if payload["status"] != "completed" {
		t.Fatalf("expected status round-trip, got %+v", payload["status"])
	}
	if _, ok := payload["metadata"].(map[string]interface{}); !ok {
		t.Fatalf("expected metadata to round-trip, got %+v", payload["metadata"])
	}
	if _, ok := payload["conversation"].(map[string]interface{}); !ok {
		t.Fatalf("expected conversation to round-trip, got %+v", payload["conversation"])
	}
	if _, ok := payload["tools"].([]interface{}); !ok {
		t.Fatalf("expected tools to round-trip, got %+v", payload["tools"])
	}
	output := payload["output"].([]interface{})
	var sawReasoningItem bool
	for _, raw := range output {
		item := raw.(map[string]interface{})
		if item["type"] == "reasoning" {
			sawReasoningItem = true
		}
	}
	if !sawReasoningItem {
		t.Fatalf("expected reasoning item to round-trip, got %+v", output)
	}
}

func TestEncodeOpenAIChatResponseUsesMessageContentWhenReasoningItemComesFirst(t *testing.T) {
	ir := IRResponse{
		ID:          "resp_reasoning_first",
		Model:       "resp-model",
		FinishReason: "stop",
		OutputItems: []IRMessage{
			{
				ID:     "rs_1",
				Type:   "reasoning",
				Role:   "assistant",
				Status: "completed",
				Content: []IRPart{{
					Type: "reasoning",
					Text: "think step",
				}},
			},
			{
				ID:     "msg_1",
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []IRPart{{
					Type: "output_text",
					Text: "hello",
				}},
			},
		},
	}

	encoded, err := EncodeOpenAIChatResponse(ir, "backend-chat")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat response: %v", err)
	}
	message := payload["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["content"] != "hello" {
		t.Fatalf("expected message content from later message item, got %+v", message["content"])
	}
	if message["reasoning_content"] != "think step" {
		t.Fatalf("expected reasoning_content preserved, got %+v", message["reasoning_content"])
	}
}

func TestResponsesResponsePreservesRichOutputItems(t *testing.T) {
	body := []byte(`{
		"id":"resp_rich",
		"object":"response",
		"status":"incomplete",
		"model":"resp-rich",
		"output":[
			{"type":"custom_tool_call","id":"ctc_1","call_id":"ctc_1","name":"local_shell","input":"ls -la","status":"completed"},
			{"type":"mcp_call","id":"mcp_1","call_id":"mcp_1","name":"fetch","arguments":"{\"q\":\"hello\"}","status":"completed"},
			{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"OpenAI"}},
			{"type":"file_search_call","id":"fs_1","name":"file_search_call","input":"search docs","status":"completed"},
			{"type":"image_generation_call","id":"img_1","status":"completed","result":"image_123"},
			{"type":"compaction","id":"cmp_1","status":"completed","summary":[{"type":"summary_text","text":"compressed"}]}
		],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	if len(ir.OutputItems) != 6 {
		t.Fatalf("expected 6 output items, got %+v", ir.OutputItems)
	}
	if ir.OutputItems[0].Type != "custom_tool_call" || ir.OutputItems[3].Type != "file_search_call" || ir.OutputItems[5].Type != "compaction" {
		t.Fatalf("expected richer item types preserved, got %+v", ir.OutputItems)
	}

	encoded, err := EncodeResponsesResponse(ir, "backend-resp-rich")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	output := payload["output"].([]interface{})
	if len(output) != 6 {
		t.Fatalf("expected 6 output items to round-trip, got %+v", output)
	}
	if output[0].(map[string]interface{})["type"] != "custom_tool_call" || output[3].(map[string]interface{})["type"] != "file_search_call" || output[5].(map[string]interface{})["type"] != "compaction" {
		t.Fatalf("expected richer output item types to round-trip, got %+v", output)
	}
}

func TestOpenAIChatResponsePreservesRefusalAndAudio(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_refusal",
		"object":"chat.completion",
		"model":"gpt-refusal",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello","refusal":"cannot comply","audio":{"id":"aud_1","expires_at":123}},"finish_reason":"stop"}]
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 {
		t.Fatalf("expected one output item, got %+v", ir.OutputItems)
	}
	var sawRefusal, sawAudio bool
	for _, part := range ir.OutputItems[0].Content {
		if part.Type == "refusal" && part.Refusal == "cannot comply" {
			sawRefusal = true
		}
		if part.Type == "audio" && len(part.Audio) > 0 {
			sawAudio = true
		}
	}
	if !sawRefusal || !sawAudio {
		t.Fatalf("expected refusal and audio parts, got %+v", ir.OutputItems[0].Content)
	}

	encoded, err := EncodeOpenAIChatResponse(ir, "backend-chat")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat response: %v", err)
	}
	message := payload["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["refusal"] != "cannot comply" {
		t.Fatalf("expected refusal to round-trip, got %+v", message["refusal"])
	}
	if _, ok := message["audio"].(map[string]interface{}); !ok {
		t.Fatalf("expected audio to round-trip, got %+v", message["audio"])
	}
}

func TestOpenAIChatResponseStructuredContentPreservesAnnotationsRefusalAndAudioAcrossProtocols(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_structured_rich",
		"object":"chat.completion",
		"model":"gpt-rich",
		"choices":[{"index":0,"message":{"role":"assistant","content":[
			{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},
			{"type":"refusal","refusal":"blocked"},
			{"type":"audio","audio":{"id":"aud_1","format":"wav"}}
		]},"finish_reason":"stop"}]
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 3 {
		t.Fatalf("expected structured rich content in IR, got %+v", ir.OutputItems)
	}
	if len(ir.OutputItems[0].Content[0].Annotations) == 0 {
		t.Fatalf("expected annotations preserved, got %+v", ir.OutputItems[0].Content[0])
	}
	if ir.OutputItems[0].Content[1].Type != "refusal" || ir.OutputItems[0].Content[1].Refusal != "blocked" {
		t.Fatalf("expected refusal preserved, got %+v", ir.OutputItems[0].Content)
	}
	if ir.OutputItems[0].Content[2].Type != "audio" || len(ir.OutputItems[0].Content[2].Audio) == 0 {
		t.Fatalf("expected audio preserved, got %+v", ir.OutputItems[0].Content)
	}

	responsesEncoded, err := EncodeResponsesResponse(ir, "backend-responses")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var responsesPayload map[string]interface{}
	if err := json.Unmarshal(responsesEncoded, &responsesPayload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	content := responsesPayload["output"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[0].(map[string]interface{})["annotations"] == nil {
		t.Fatalf("expected annotations to survive chat->responses conversion, got %+v", content[0])
	}
	if content[1].(map[string]interface{})["type"] != "refusal" || content[2].(map[string]interface{})["type"] != "output_audio" {
		t.Fatalf("expected refusal/audio to survive chat->responses conversion, got %+v", content)
	}

	anthropicEncoded, err := EncodeAnthropicResponse(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var anthropicPayload map[string]interface{}
	if err := json.Unmarshal(anthropicEncoded, &anthropicPayload); err != nil {
		t.Fatalf("unmarshal encoded anthropic response: %v", err)
	}
	anthropicContent := anthropicPayload["content"].([]interface{})
	var sawAnnotations, sawRefusal, sawAudio bool
	for _, raw := range anthropicContent {
		item := raw.(map[string]interface{})
		if item["annotations"] != nil {
			sawAnnotations = true
		}
		if item["refusal"] == "blocked" {
			sawRefusal = true
		}
		if _, ok := item["audio"].(map[string]interface{}); ok {
			sawAudio = true
		}
	}
	if !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected annotations/refusal/audio to survive chat->anthropic conversion, got %+v", anthropicContent)
	}
}

func TestResponsesResponsePreservesRefusalAnnotationsAndAudioContent(t *testing.T) {
	body := []byte(`{
		"id":"resp_content_rich",
		"object":"response",
		"status":"completed",
		"model":"resp-content-rich",
		"output":[
			{"type":"message","role":"assistant","status":"completed","content":[
				{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},
				{"type":"refusal","refusal":"blocked"},
				{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}
			]}
		]
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 3 {
		t.Fatalf("expected rich message content, got %+v", ir.OutputItems)
	}
	if ir.OutputItems[0].Content[0].Type != "output_text" || len(ir.OutputItems[0].Content[0].Annotations) == 0 {
		t.Fatalf("expected annotations preserved, got %+v", ir.OutputItems[0].Content[0])
	}
	if ir.OutputItems[0].Content[1].Type != "refusal" || ir.OutputItems[0].Content[2].Type != "audio" {
		t.Fatalf("expected refusal/audio parts preserved, got %+v", ir.OutputItems[0].Content)
	}

	encoded, err := EncodeResponsesResponse(ir, "backend-resp")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	content := payload["output"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "refusal" || content[2].(map[string]interface{})["type"] != "output_audio" {
		t.Fatalf("expected refusal/audio content to round-trip, got %+v", content)
	}
}

func TestResponsesResponseVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"resp_img",
		"object":"response",
		"status":"completed",
		"model":"resp-vision",
		"output":[
			{"type":"message","role":"assistant","status":"completed","content":[
				{"type":"output_text","text":"hello"},
				{"type":"image","url":"https://example.com/a.png"}
			]}
		],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "image" {
		t.Fatalf("expected image part in responses response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeResponsesResponse(ir, "backend-resp")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	content := payload["output"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "image" {
		t.Fatalf("expected image part to round-trip, got %+v", content[1])
	}
}

func TestResponsesResponseFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"resp_file",
		"object":"response",
		"status":"completed",
		"model":"resp-file",
		"output":[
			{"type":"message","role":"assistant","status":"completed","content":[
				{"type":"output_text","text":"hello"},
				{"type":"file","file_id":"file_123"}
			]}
		],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "file" {
		t.Fatalf("expected file part in responses response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeResponsesResponse(ir, "backend-resp")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses response: %v", err)
	}
	content := payload["output"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "file" {
		t.Fatalf("expected file part to round-trip, got %+v", content[1])
	}
}

func TestAnthropicResponseIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"msg_1",
		"type":"message",
		"role":"assistant",
		"model":"claude-test",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"thinking","thinking":"think","signature":"sig_1"},
			{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}
		],
		"stop_reason":"tool_use",
		"usage":{"input_tokens":4,"output_tokens":2}
	}`)

	ir, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse error: %v", err)
	}
	if ir.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls finish, got %q", ir.FinishReason)
	}
	var sawReasoning bool
	for _, part := range ir.OutputItems[0].Content {
		if part.Type == "reasoning" && part.Text == "think" {
			sawReasoning = true
		}
	}
	if !sawReasoning {
		t.Fatalf("expected anthropic thinking to decode into reasoning, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeAnthropicResponse(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded anthropic response: %v", err)
	}
	if payload["model"] != "backend-claude" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	if payload["stop_reason"] != "tool_use" {
		t.Fatalf("expected stop_reason tool_use, got %v", payload["stop_reason"])
	}
	content := payload["content"].([]interface{})
	var sawThinking bool
	for _, raw := range content {
		item := raw.(map[string]interface{})
		if item["type"] == "thinking" && item["thinking"] == "think" {
			sawThinking = true
		}
	}
	if !sawThinking {
		t.Fatalf("expected thinking block to round-trip, got %+v", payload["content"])
	}
}

func TestAnthropicResponsePreservesAnnotationsRefusalAndAudioExtensions(t *testing.T) {
	body := []byte(`{
		"id":"msg_rich",
		"type":"message",
		"role":"assistant",
		"model":"claude-rich",
		"content":[
			{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},
			{"type":"text","text":"blocked","refusal":"blocked"},
			{"type":"text","text":"","audio":{"id":"aud_1","format":"wav"}}
		],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":4,"output_tokens":2}
	}`)

	ir, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 4 {
		t.Fatalf("expected rich anthropic content to decode, got %+v", ir.OutputItems)
	}
	if ir.OutputItems[0].Content[0].Type != "text" || len(ir.OutputItems[0].Content[0].Annotations) == 0 {
		t.Fatalf("expected annotations on text part, got %+v", ir.OutputItems[0].Content[0])
	}
	if ir.OutputItems[0].Content[2].Type != "refusal" || ir.OutputItems[0].Content[2].Refusal != "blocked" {
		t.Fatalf("expected refusal part preserved, got %+v", ir.OutputItems[0].Content)
	}
	if ir.OutputItems[0].Content[3].Type != "audio" || len(ir.OutputItems[0].Content[3].Audio) == 0 {
		t.Fatalf("expected audio part preserved, got %+v", ir.OutputItems[0].Content)
	}

	encoded, err := EncodeAnthropicResponse(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded anthropic response: %v", err)
	}
	content := payload["content"].([]interface{})
	if content[0].(map[string]interface{})["annotations"] == nil {
		t.Fatalf("expected annotations extension to round-trip, got %+v", content[0])
	}
	if content[2].(map[string]interface{})["refusal"] != "blocked" {
		t.Fatalf("expected refusal extension to round-trip, got %+v", content[2])
	}
	if _, ok := content[3].(map[string]interface{})["audio"].(map[string]interface{}); !ok {
		t.Fatalf("expected audio extension to round-trip, got %+v", content[3])
	}
}

func TestAnthropicResponseVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"msg_img",
		"type":"message",
		"role":"assistant",
		"model":"claude-vision",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}
		],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":4,"output_tokens":2}
	}`)

	ir, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "image" {
		t.Fatalf("expected image part in anthropic response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeAnthropicResponse(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded anthropic response: %v", err)
	}
	content := payload["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "image" {
		t.Fatalf("expected anthropic image part to round-trip, got %+v", content[1])
	}
}

func TestAnthropicResponseFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"msg_file",
		"type":"message",
		"role":"assistant",
		"model":"claude-file",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}}
		],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":4,"output_tokens":2}
	}`)

	ir, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse error: %v", err)
	}
	if len(ir.OutputItems) != 1 || len(ir.OutputItems[0].Content) != 2 || ir.OutputItems[0].Content[1].Type != "file" {
		t.Fatalf("expected file part in anthropic response IR, got %+v", ir.OutputItems)
	}
	encoded, err := EncodeAnthropicResponse(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded anthropic response: %v", err)
	}
	content := payload["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "document" {
		t.Fatalf("expected anthropic document part to round-trip, got %+v", content[1])
	}
}
