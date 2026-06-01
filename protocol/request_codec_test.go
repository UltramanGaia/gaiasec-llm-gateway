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

func TestChatRequestToolChoiceRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"gpt-tools",
		"messages":[{"role":"user","content":"hello"}],
		"tool_choice":{"type":"function","function":{"name":"lookup"}}
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	encoded, err := EncodeOpenAIChatRequest(ir, "backend-tools")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	toolChoice := payload["tool_choice"].(map[string]interface{})
	function := toolChoice["function"].(map[string]interface{})
	if function["name"] != "lookup" {
		t.Fatalf("expected tool_choice function lookup, got %+v", toolChoice)
	}
}

func TestChatRequestPreservesBuiltInTools(t *testing.T) {
	body := []byte(`{
		"model":"gpt-tools-filter",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}},
			{"type":"namespace","name":"shell"},
			{"type":"web_search"}
		],
		"tool_choice":"auto",
		"parallel_tool_calls":true
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if len(ir.Tools) != 3 {
		t.Fatalf("expected all tools to be preserved, got %+v", ir.Tools)
	}
	if ir.Tools[2].Type != "web_search" || len(ir.Tools[2].RawPayload) == 0 {
		t.Fatalf("expected web_search raw payload preserved, got %+v", ir.Tools[2])
	}

	encoded, err := EncodeOpenAIChatRequest(ir, "backend-tools")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	tools := payload["tools"].([]interface{})
	if len(tools) != 3 {
		t.Fatalf("expected all tools to be re-encoded, got %+v", tools)
	}
}

func TestChatRequestKeepsToolChoiceWhenOnlyBuiltInToolsRemain(t *testing.T) {
	body := []byte(`{
		"model":"gpt-tools-filter-empty",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"namespace","name":"shell"},
			{"type":"web_search"}
		],
		"tool_choice":"required",
		"parallel_tool_calls":true
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if len(ir.Tools) != 2 {
		t.Fatalf("expected built-in tools to be preserved, got %+v", ir.Tools)
	}
	if len(ir.ToolChoice) == 0 {
		t.Fatalf("expected tool_choice to remain when tools remain")
	}
	if parallel, _ := ir.ProviderExtensions["parallel_tool_calls"].(bool); !parallel {
		t.Fatalf("expected parallel_tool_calls to remain when tools remain")
	}
}

func TestChatRequestPreservesRichProviderFields(t *testing.T) {
	body := []byte(`{
		"model":"chat-rich",
		"messages":[{"role":"user","content":"hello"}],
		"metadata":{"trace_id":"abc"},
		"service_tier":"flex",
		"modalities":["text","audio"],
		"audio":{"voice":"alloy"},
		"prediction":{"type":"content","content":"pong"},
		"verbosity":{"level":"high"},
		"web_search_options":{"search_context_size":"medium"},
		"logprobs":true,
		"top_logprobs":2,
		"seed":7,
		"n":2,
		"frequency_penalty":0.1,
		"presence_penalty":0.2,
		"logit_bias":{"42":1}
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if ir.ServiceTier != "flex" || string(ir.Metadata) == "" {
		t.Fatalf("expected service_tier and metadata preserved, got %+v", ir)
	}
	if _, ok := ir.ProviderExtensions["audio"]; !ok {
		t.Fatalf("expected audio provider extension, got %+v", ir.ProviderExtensions)
	}

	encoded, err := EncodeOpenAIChatRequest(ir, "backend-chat-rich")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["service_tier"] != "flex" {
		t.Fatalf("expected service_tier preserved, got %+v", payload["service_tier"])
	}
	if _, ok := payload["audio"].(map[string]interface{}); !ok {
		t.Fatalf("expected audio payload preserved, got %+v", payload["audio"])
	}
	if payload["top_logprobs"] != float64(2) {
		t.Fatalf("expected top_logprobs preserved, got %+v", payload["top_logprobs"])
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

func TestChatRequestFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"gpt-file",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"read"},
				{"type":"input_file","file_id":"file_123"}
			]}
		]
	}`)

	ir, err := DecodeOpenAIChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeOpenAIChatRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "file" {
		t.Fatalf("expected file content in IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeOpenAIChatRequest(ir, "backend-file")
	if err != nil {
		t.Fatalf("EncodeOpenAIChatRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["messages"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "input_file" {
		t.Fatalf("expected input_file part to round-trip, got %+v", content[1])
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

func TestResponsesRequestToolChoiceRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"resp-tools",
		"input":"hello",
		"tool_choice":{"type":"function","name":"lookup"}
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	encoded, err := EncodeResponsesRequest(ir, "backend-resp-tools")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	toolChoice := payload["tool_choice"].(map[string]interface{})
	if toolChoice["name"] != "lookup" {
		t.Fatalf("expected responses tool_choice lookup, got %+v", toolChoice)
	}
}

func TestResponsesRequestPreservesBuiltInTools(t *testing.T) {
	body := []byte(`{
		"model":"resp-tools-filter",
		"input":"hello",
		"tools":[
			{"type":"function","name":"lookup","parameters":{"type":"object"}},
			{"type":"namespace","name":"shell"},
			{"type":"web_search"}
		],
		"tool_choice":"auto",
		"parallel_tool_calls":true
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	if len(ir.Tools) != 3 {
		t.Fatalf("expected all tools to be preserved, got %+v", ir.Tools)
	}
	if ir.Tools[2].Type != "web_search" || len(ir.Tools[2].RawPayload) == 0 {
		t.Fatalf("expected web_search raw payload preserved, got %+v", ir.Tools[2])
	}

	encoded, err := EncodeResponsesRequest(ir, "backend-resp-tools")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	tools := payload["tools"].([]interface{})
	if len(tools) != 3 {
		t.Fatalf("expected all tools to be re-encoded, got %+v", tools)
	}
}

func TestResponsesRequestPreservesRichProviderFields(t *testing.T) {
	body := []byte(`{
		"model":"resp-rich",
		"instructions":"sys",
		"input":"hello",
		"previous_response_id":"resp_prev",
		"include":["reasoning.encrypted_content"],
		"metadata":{"trace_id":"abc"},
		"service_tier":"priority",
		"store":true,
		"background":false,
		"conversation":{"id":"conv_1"},
		"prompt":{"id":"pmpt_1","version":"2"},
		"prompt_cache_key":"cache-key",
		"prompt_cache_retention":"persist"
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	if ir.PreviousResponseID != "resp_prev" || ir.ServiceTier != "priority" {
		t.Fatalf("expected rich fields in IR, got %+v", ir)
	}
	if ir.Store == nil || !*ir.Store || ir.Background == nil || *ir.Background {
		t.Fatalf("expected store/background preserved, got store=%v background=%v", ir.Store, ir.Background)
	}
	if string(ir.Metadata) == "" || string(ir.Prompt) == "" || string(ir.Conversation) == "" {
		t.Fatalf("expected metadata/prompt/conversation preserved, got %+v", ir)
	}

	encoded, err := EncodeResponsesRequest(ir, "backend-resp-rich")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["previous_response_id"] != "resp_prev" {
		t.Fatalf("expected previous_response_id preserved, got %+v", payload["previous_response_id"])
	}
	if payload["prompt_cache_key"] != "cache-key" || payload["prompt_cache_retention"] != "persist" {
		t.Fatalf("expected prompt cache fields preserved, got %+v", payload)
	}
	if payload["service_tier"] != "priority" {
		t.Fatalf("expected service_tier preserved, got %+v", payload["service_tier"])
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

func TestResponsesRequestFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"resp-file",
		"input":[
			{"type":"message","role":"user","content":[
				{"type":"input_text","text":"read"},
				{"type":"input_file","file_id":"file_123"}
			]}
		]
	}`)

	ir, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "file" {
		t.Fatalf("expected file content in responses IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeResponsesRequest(ir, "backend-resp-file")
	if err != nil {
		t.Fatalf("EncodeResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["input"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "input_file" {
		t.Fatalf("expected input_file part to round-trip, got %+v", content[1])
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

func TestAnthropicRequestToolChoiceRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"claude-tools",
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"tool_choice":{"type":"tool","name":"lookup"}
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	encoded, err := EncodeAnthropicRequest(ir, "backend-claude-tools")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	toolChoice := payload["tool_choice"].(map[string]interface{})
	if toolChoice["name"] != "lookup" {
		t.Fatalf("expected anthropic tool_choice lookup, got %+v", toolChoice)
	}
}

func TestAnthropicRequestPreservesRichProviderFields(t *testing.T) {
	body := []byte(`{
		"model":"claude-rich",
		"system":"sys",
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"metadata":{"trace_id":"abc"},
		"service_tier":"priority",
		"top_k":50
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	if ir.ServiceTier != "priority" || string(ir.Metadata) == "" {
		t.Fatalf("expected metadata/service_tier preserved, got %+v", ir)
	}
	if _, ok := ir.ProviderExtensions["top_k"]; !ok {
		t.Fatalf("expected top_k provider extension, got %+v", ir.ProviderExtensions)
	}

	encoded, err := EncodeAnthropicRequest(ir, "backend-claude-rich")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["service_tier"] != "priority" || payload["top_k"] != float64(50) {
		t.Fatalf("expected service_tier/top_k preserved, got %+v", payload)
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

func TestAnthropicRequestFileRoundTrip(t *testing.T) {
	body := []byte(`{
		"model":"claude-file",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"read"},
				{"type":"document","source":{"type":"url","url":"https://example.com/a.pdf"}}
			]}
		]
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	if len(ir.Messages) != 1 || len(ir.Messages[0].Content) != 2 || ir.Messages[0].Content[1].Type != "file" {
		t.Fatalf("expected anthropic file content in IR, got %+v", ir.Messages)
	}

	encoded, err := EncodeAnthropicRequest(ir, "backend-claude-file")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	content := payload["messages"].([]interface{})[0].(map[string]interface{})["content"].([]interface{})
	if content[1].(map[string]interface{})["type"] != "document" {
		t.Fatalf("expected document part to round-trip, got %+v", content[1])
	}
}

func TestAnthropicRequestPreservesBlockLevelCacheControl(t *testing.T) {
	body := []byte(`{
		"model":"claude-cache-control",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}
			]},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"},"cache_control":{"type":"ephemeral"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"call_1","content":"ok","cache_control":{"type":"ephemeral"}}
			]}
		]
	}`)

	ir, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest error: %v", err)
	}
	if len(ir.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %+v", ir.Messages)
	}
	if raw, ok := ir.Messages[0].Content[0].ProviderExtensions["raw"].(map[string]interface{}); !ok || raw["cache_control"] == nil {
		t.Fatalf("expected user text raw block with cache_control, got %+v", ir.Messages[0].Content[0])
	}
	if raw, ok := ir.Messages[1].Content[0].ProviderExtensions["raw"].(map[string]interface{}); !ok || raw["cache_control"] == nil {
		t.Fatalf("expected tool_use raw block with cache_control, got %+v", ir.Messages[1].Content[0])
	}
	if raw, ok := ir.Messages[2].Content[0].ProviderExtensions["raw"].(map[string]interface{}); !ok || raw["cache_control"] == nil {
		t.Fatalf("expected tool_result raw block with cache_control, got %+v", ir.Messages[2].Content[0])
	}

	encoded, err := EncodeAnthropicRequest(ir, "backend-claude-cache-control")
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	messages := payload["messages"].([]interface{})
	userContent := messages[0].(map[string]interface{})["content"].([]interface{})
	if userContent[0].(map[string]interface{})["cache_control"] == nil {
		t.Fatalf("expected user text cache_control to round-trip, got %+v", userContent[0])
	}
	assistantContent := messages[1].(map[string]interface{})["content"].([]interface{})
	if assistantContent[0].(map[string]interface{})["cache_control"] == nil {
		t.Fatalf("expected tool_use cache_control to round-trip, got %+v", assistantContent[0])
	}
	toolResultContent := messages[2].(map[string]interface{})["content"].([]interface{})
	if toolResultContent[0].(map[string]interface{})["cache_control"] == nil {
		t.Fatalf("expected tool_result cache_control to round-trip, got %+v", toolResultContent[0])
	}
}
