package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNonStreamGoldenChatToResponses(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_gold",
		"object":"chat.completion",
		"model":"gpt-gold",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeOpenAIChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatResponse error: %v", err)
	}
	encoded, err := EncodeResponsesResponse(ir, "resp-gold")
	if err != nil {
		t.Fatalf("EncodeResponsesResponse error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded responses: %v", err)
	}
	if payload["object"] != "response" || payload["model"] != "resp-gold" {
		t.Fatalf("unexpected responses payload: %+v", payload)
	}
	output := payload["output"].([]interface{})
	if len(output) != 2 {
		t.Fatalf("expected 2 output items, got %+v", output)
	}
}

func TestNonStreamGoldenResponsesToAnthropic(t *testing.T) {
	body := []byte(`{
		"id":"resp_gold",
		"object":"response",
		"status":"completed",
		"model":"resp-gold",
		"output":[
			{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello"}]},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"think"}],"status":"completed"}
		],
		"usage":{"input_tokens":4,"output_tokens":2,"total_tokens":6}
	}`)

	ir, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse error: %v", err)
	}
	encoded, err := EncodeAnthropicResponse(ir, "claude-gold")
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded anthropic: %v", err)
	}
	if payload["type"] != "message" || payload["model"] != "claude-gold" {
		t.Fatalf("unexpected anthropic payload: %+v", payload)
	}
	content := payload["content"].([]interface{})
	if len(content) != 2 {
		t.Fatalf("expected 2 anthropic content blocks, got %+v", content)
	}
}

func TestNonStreamGoldenAnthropicToChat(t *testing.T) {
	body := []byte(`{
		"id":"msg_gold",
		"type":"message",
		"role":"assistant",
		"model":"claude-gold",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"thinking","thinking":"think","signature":"sig"},
			{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}
		],
		"stop_reason":"tool_use",
		"usage":{"input_tokens":4,"output_tokens":2}
	}`)

	ir, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse error: %v", err)
	}
	encoded, err := EncodeOpenAIChatResponse(ir, "chat-gold")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatResponse error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded chat: %v", err)
	}
	if payload["object"] != "chat.completion" || payload["model"] != "chat-gold" {
		t.Fatalf("unexpected chat payload: %+v", payload)
	}
	choices := payload["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["reasoning_content"] != "think" {
		t.Fatalf("expected reasoning_content think, got %+v", message)
	}
	toolCalls := message["tool_calls"].([]interface{})
	if !reflect.DeepEqual(toolCalls[0].(map[string]interface{})["id"], "call_1") {
		t.Fatalf("expected tool call id call_1, got %+v", toolCalls)
	}
}
