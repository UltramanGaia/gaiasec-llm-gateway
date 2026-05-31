package protocol

import (
	"bufio"
	"strings"
	"testing"
)

func TestStreamFixtureOpenAIChatToResponses(t *testing.T) {
	chunk := map[string]interface{}{
		"id":    "chatcmpl_fixture",
		"model": "gpt-fixture",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"role":    "assistant",
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
	}

	events := ConvertChatChunkToResponsesEvents(chunk, 11)
	if len(events) < 8 {
		t.Fatalf("expected responses events from chat fixture, got %d", len(events))
	}
	if events[0].Event != "response.created" || events[1].Event != "response.in_progress" || events[2].Event != "response.output_item.added" || events[3].Event != "response.content_part.added" || events[4].Event != "response.output_text.delta" {
		t.Fatalf("unexpected event order from chat fixture: %+v", events)
	}
}

func TestStreamFixtureResponsesToAnthropic(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_fixture","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant"}}`},
		{Event: "response.output_text.delta", Data: `{"type":"response.output_text.delta","output_index":0,"delta":"hello"}`},
		{Event: "response.completed", Data: `{"type":"response.completed","response":{"id":"resp_fixture","model":"resp-model","usage":{"input_tokens":3,"output_tokens":2}}}`},
	}

	var sawStart, sawText, sawStop bool
	for _, frame := range frames {
		for _, out := range AnthropicEventsFromResponsesFrame(frame, state) {
			if out.Event == "message_start" {
				sawStart = true
			}
			if out.Event == "content_block_delta" && strings.Contains(out.Data, "text_delta") {
				sawText = true
			}
			if out.Event == "message_stop" {
				sawStop = true
			}
		}
	}
	if !sawStart || !sawText || !sawStop {
		t.Fatalf("expected anthropic fixture events, got start=%v text=%v stop=%v", sawStart, sawText, sawStop)
	}
}

func TestStreamFixtureAnthropicToChat(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_fixture","model":"claude","usage":{"input_tokens":3}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		"",
	}, "\n")))

	state := NewAnthropicInboundState()
	var sawRole, sawContent, sawFinish bool
	for {
		frame, err := ReadSSEFrame(reader)
		if err != nil {
			break
		}
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if delta["content"] == "hello" {
				sawContent = true
			}
			if choices[0]["finish_reason"] == "stop" {
				sawFinish = true
			}
		}
	}
	if !sawRole || !sawContent || !sawFinish {
		t.Fatalf("expected chat chunks from anthropic fixture, got role=%v content=%v finish=%v", sawRole, sawContent, sawFinish)
	}
}
