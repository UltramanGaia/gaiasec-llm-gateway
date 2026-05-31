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
