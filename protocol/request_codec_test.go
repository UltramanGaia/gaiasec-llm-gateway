package protocol

import (
	"encoding/json"
	"testing"
)

func TestChatRequestIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"gpt-test",
		"messages":[
			{"role":"system","content":"sys"},
			{"role":"user","content":"hello"},
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]}
		],
		"tools":[{"type":"function","function":{"name":"lookup","description":"d","parameters":{"type":"object"}}}],
		"parallel_tool_calls":true,
		"response_format":{"type":"json_schema"},
		"reasoning":{"effort":"high"},
		"stream":true
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if ir.SystemInstruction != "sys" {
		t.Fatalf("expected system message to move to system_instruction, got %q", ir.SystemInstruction)
	}
	if len(ir.Messages) != 2 {
		t.Fatalf("expected 2 non-system messages, got %d", len(ir.Messages))
	}
	if len(ir.Tools) != 1 || ir.Tools[0].Name != "lookup" {
		t.Fatalf("expected tool lookup, got %+v", ir.Tools)
	}
	if parallel, _ := ir.ProviderExtensions["parallel_tool_calls"].(bool); !parallel {
		t.Fatalf("expected parallel_tool_calls in provider extensions")
	}

	encoded, err := EncodeOpenAIChatRequest(ir, "backend-model")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["model"] != "backend-model" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("expected stream true, got %v", payload["stream"])
	}
	if payload["response_format"].(map[string]interface{})["type"] != "json_schema" {
		t.Fatalf("expected response_format json_schema to round-trip, got %+v", payload["response_format"])
	}
	messages := payload["messages"].([]interface{})
	if len(messages) == 0 || messages[0].(map[string]interface{})["role"] != "system" {
		t.Fatalf("expected system message to be re-encoded into chat messages, got %+v", payload["messages"])
	}
}

func TestChatRequestVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"gpt-vision",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"describe"},
				{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}
			]}
		]
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "image" {
		t.Fatalf("expected image content in IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeOpenAIChatRequest(ir, "backend-vision")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["messages"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "image_url" {
		t.Fatalf("expected image_url part to round-trip, got %+v", content[1])
	}
}

func TestResponsesRequestIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"resp-test",
		"instructions":"sys",
		"text":{"format":{"type":"json_schema","name":"result","schema":{"type":"object"}}},
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},
			{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"hello\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		],
		"parallel_tool_calls":true,
		"stream":true
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	if ir.SystemInstruction != "sys" {
		t.Fatalf("expected instructions in IR, got %q", ir.SystemInstruction)
	}
	if len(ir.Messages) != 3 {
		t.Fatalf("expected 3 IR messages, got %d", len(ir.Messages))
	}
	encoded, err := EncodeResponsesRequest(ir, "backend-resp")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["model"] != "backend-resp" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	if payload["instructions"] != "sys" {
		t.Fatalf("expected instructions preserved, got %v", payload["instructions"])
	}
	text := payload["text"].(map[string]interface{})
	format := text["format"].(map[string]interface{})
	if format["type"] != "json_schema" {
		t.Fatalf("expected text.format json_schema to round-trip, got %+v", payload["text"])
	}
}

func TestResponsesRequestVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"resp-vision",
		"input":[
			{"type":"message","role":"user","content":[
				{"type":"input_text","text":"describe"},
				{"type":"input_image","image_url":"https://example.com/a.png"}
			]}
		]
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "image" {
		t.Fatalf("expected image content in responses IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeResponsesRequest(ir, "backend-resp-vision")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["input"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "input_image" {
		t.Fatalf("expected input_image part to round-trip, got %+v", content[1])
	}
}

func TestAnthropicRequestIRRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"claude-test",
		"system":"sys",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"hello"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"ok"}]}
		],
		"tools":[{"name":"lookup","description":"d","input_schema":{"type":"object"}}],
		"thinking":{"type":"enabled"},
		"stream":true
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	if ir.SystemInstruction != "sys" {
		t.Fatalf("expected system instruction, got %q", ir.SystemInstruction)
	}
	if len(ir.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(ir.Messages))
	}
	if len(ir.Tools) != 1 || ir.Tools[0].Name != "lookup" {
		t.Fatalf("expected lookup tool, got %+v", ir.Tools)
	}
	encoded, err := EncodeAnthropicRequest(ir, "backend-claude")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["model"] != "backend-claude" {
		t.Fatalf("expected rewritten model, got %v", payload["model"])
	}
	if payload["system"] != "sys" {
		t.Fatalf("expected system preserved, got %v", payload["system"])
	}
}

func TestAnthropicRequestVisionRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"claude-vision",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"describe"},
				{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}
			]}
		]
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "image" {
		t.Fatalf("expected anthropic image content in IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeAnthropicRequest(ir, "backend-claude-vision")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["messages"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "image" {
		t.Fatalf("expected anthropic image part to round-trip, got %+v", content[1])
	}
}
