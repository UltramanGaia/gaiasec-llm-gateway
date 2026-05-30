package protocol

import (
	"strings"
	"testing"
)

func TestChatChunksFromResponsesFrameToolFinish(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_1","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup"}}`},
		{Event: "response.function_call_arguments.delta", Data: `{"type":"response.function_call_arguments.delta","output_index":5,"delta":"{\"q\":\"hello\"}"}`},
		{Event: "response.completed", Data: `{"type":"response.completed","response":{"id":"resp_1","model":"resp-model","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`},
	}

	var lastChunk map[string]interface{}
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		if len(chunks) > 0 {
			lastChunk = chunks[len(chunks)-1]
		}
	}

	if lastChunk == nil {
		t.Fatal("expected final chat chunk")
	}
	choices := lastChunk["choices"].([]map[string]interface{})
	if choices[0]["finish_reason"] != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %+v", choices[0]["finish_reason"])
	}
}

func TestAnthropicEventsFromResponsesFrame(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_2","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant"}}`},
		{Event: "response.output_text.delta", Data: `{"type":"response.output_text.delta","output_index":0,"delta":"hello"}`},
		{Event: "response.completed", Data: `{"type":"response.completed","response":{"id":"resp_2","model":"resp-model","usage":{"input_tokens":3,"output_tokens":2}}}`},
	}

	var sawStart, sawDelta, sawStop bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			switch event.Event {
			case "message_start":
				sawStart = true
			case "content_block_delta":
				sawDelta = true
			case "message_stop":
				sawStop = true
			}
		}
	}

	if !sawStart || !sawDelta || !sawStop {
		t.Fatalf("expected message_start/content_block_delta/message_stop, got start=%v delta=%v stop=%v", sawStart, sawDelta, sawStop)
	}
}

func TestAnthropicEventsFromResponsesFramePreserveImagePart(t *testing.T) {
	state := NewAnthropicOutboundState()
	events := AnthropicEventsFromResponsesFrame(SSEFrame{
		Event: "response.output_item.added",
		Data:  `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant","content":[{"type":"output_image","source":{"type":"url","url":"https://example.com/a.png"}}]}}`,
	}, state)

	var sawImage bool
	for _, event := range events {
		if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"image"`) {
			sawImage = true
		}
	}
	if !sawImage {
		t.Fatalf("expected anthropic content_block_start with image, got %+v", events)
	}
}

func TestAnthropicFramesFromChatChunk(t *testing.T) {
	state := NewAnthropicOutboundState()
	chunk := map[string]interface{}{
		"id":    "chatcmpl_1",
		"model": "gpt-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"content": "hello",
					"tool_calls": []interface{}{
						map[string]interface{}{
							"index": 1,
							"id":    "call_1",
							"function": map[string]interface{}{
								"name":      "lookup",
								"arguments": "{\"q\":\"hello\"}",
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     3,
			"completion_tokens": 2,
		},
	}

	frames := AnthropicFramesFromChatChunk(chunk, state)
	var sawStart, sawTextDelta, sawToolStart, sawToolDelta bool
	for _, frame := range frames {
		switch frame.Event {
		case "message_start":
			sawStart = true
		case "content_block_delta":
			if strings.Contains(frame.Data, "text_delta") {
				sawTextDelta = true
			}
			if strings.Contains(frame.Data, "input_json_delta") {
				sawToolDelta = true
			}
		case "content_block_start":
			if strings.Contains(frame.Data, "tool_use") {
				sawToolStart = true
			}
		}
	}
	if !sawStart || !sawTextDelta || !sawToolStart || !sawToolDelta {
		t.Fatalf("expected anthropic frames for start/text/tool, got start=%v text=%v toolStart=%v toolDelta=%v", sawStart, sawTextDelta, sawToolStart, sawToolDelta)
	}

	flushed := FlushAnthropicFrames(state, true)
	if len(flushed) == 0 {
		t.Fatal("expected flushed anthropic terminal frames")
	}
}
