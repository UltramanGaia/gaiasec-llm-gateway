package protocol

import "testing"

func TestChatChunksFromAnthropicFrame(t *testing.T) {
	state := NewAnthropicInboundState()
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_1","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_delta", Data: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`},
		{Event: "message_delta", Data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`},
	}

	var lastChunk map[string]interface{}
	for _, frame := range frames {
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		if len(chunks) > 0 {
			lastChunk = chunks[len(chunks)-1]
		}
	}
	if lastChunk == nil {
		t.Fatal("expected chat chunk")
	}
	choices := lastChunk["choices"].([]map[string]interface{})
	if choices[0]["finish_reason"] != "stop" {
		t.Fatalf("expected finish_reason stop, got %+v", choices[0]["finish_reason"])
	}
}

func TestResponsesEventsFromAnthropicFrame(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_2","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_delta", Data: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`},
		{Event: "message_delta", Data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`},
	}

	var sawAdded, sawDelta, sawCompleted bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.added":
				sawAdded = true
			case "response.output_text.delta":
				sawDelta = true
			case "response.completed":
				sawCompleted = true
			}
		}
	}
	if !sawAdded || !sawDelta || !sawCompleted {
		t.Fatalf("expected added/delta/completed events, got added=%v delta=%v completed=%v", sawAdded, sawDelta, sawCompleted)
	}
}
