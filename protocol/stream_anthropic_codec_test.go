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

func TestChatChunksFromAnthropicFrameMessageStopWithoutMessageDeltaEmitsFinishChunk(t *testing.T) {
	state := NewAnthropicInboundState()
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_stop_only","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`},
		{Event: "message_stop", Data: `{"type":"message_stop"}`},
	}

	var lastChunk map[string]interface{}
	for _, frame := range frames {
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		if len(chunks) > 0 {
			lastChunk = chunks[len(chunks)-1]
		}
	}
	if lastChunk == nil {
		t.Fatal("expected final chat chunk on message_stop")
	}
	choices := lastChunk["choices"].([]map[string]interface{})
	if choices[0]["finish_reason"] != "tool_calls" {
		t.Fatalf("expected message_stop flush finish_reason tool_calls, got %+v", choices[0]["finish_reason"])
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

func TestResponsesEventsFromAnthropicFrameMessageStopWithoutMessageDeltaEmitsCompleted(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_stop_only","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`},
		{Event: "message_stop", Data: `{"type":"message_stop"}`},
	}

	var sawToolDone, sawMessageDone, sawCompleted bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.done":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				switch item["type"] {
				case "function_call":
					sawToolDone = true
				case "message":
					sawMessageDone = true
				}
			case "response.completed":
				sawCompleted = true
			}
		}
	}
	if !sawToolDone || !sawMessageDone || !sawCompleted {
		t.Fatalf("expected message_stop flush lifecycle, got toolDone=%v messageDone=%v completed=%v", sawToolDone, sawMessageDone, sawCompleted)
	}
}

func TestResponsesEventsFromAnthropicFramePreserveImageBlock(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	events, _, _, _ := ResponsesEventsFromAnthropicFrame(SSEFrame{
		Event: "content_block_start",
		Data:  `{"type":"content_block_start","index":1,"content_block":{"type":"image","source":{"type":"url","url":"https://example.com/a.png"}}}`,
	}, state, &seq)

	if len(events) != 1 || events[0].Event != "response.content_part.added" {
		t.Fatalf("expected response.content_part.added event, got %+v", events)
	}
	data := events[0].Data.(map[string]interface{})
	part := data["part"].(map[string]interface{})
	if part["type"] != "output_image" {
		t.Fatalf("expected output_image part, got %+v", part)
	}
}

func TestResponsesEventsFromAnthropicFramePreservesThinkingAsReasoningItem(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_reasoning","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"","signature":""}}`},
		{Event: "content_block_delta", Data: `{"type":"content_block_delta","index":1,"delta":{"type":"thinking_delta","thinking":"think step"}}`},
		{Event: "message_delta", Data: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`},
	}

	var sawReasoningAdded, sawReasoningDelta, sawReasoningDone bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.added":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "reasoning" {
					sawReasoningAdded = true
				}
			case "response.reasoning.delta":
				sawReasoningDelta = true
			case "response.output_item.done":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "reasoning" {
					sawReasoningDone = true
				}
			}
		}
	}
	if !sawReasoningAdded || !sawReasoningDelta || !sawReasoningDone {
		t.Fatalf("expected reasoning lifecycle events, got added=%v delta=%v done=%v", sawReasoningAdded, sawReasoningDelta, sawReasoningDone)
	}
}

func TestResponsesEventsFromAnthropicFramePreservesThinkingTextFromStart(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_reasoning_start","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`},
	}

	var sawReasoningAdded, sawReasoningAddedSummary, sawReasoningDelta bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.added":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "reasoning" {
					sawReasoningAdded = true
					summary := item["summary"].([]map[string]interface{})
					if len(summary) == 1 && summary[0]["text"] == "think step" {
						sawReasoningAddedSummary = true
					}
				}
			case "response.reasoning.delta":
				data := event.Data.(map[string]interface{})
				if data["delta"] == "think step" {
					sawReasoningDelta = true
				}
			}
		}
	}
	if !sawReasoningAdded || !sawReasoningAddedSummary || !sawReasoningDelta {
		t.Fatalf("expected thinking start text to populate responses reasoning, got added=%v summary=%v delta=%v", sawReasoningAdded, sawReasoningAddedSummary, sawReasoningDelta)
	}
}

func TestResponsesEventsFromAnthropicFramePreservesToolUseLifecycle(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_tool","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{}}}`},
		{Event: "content_block_delta", Data: `{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hello\"}"}}`},
		{Event: "message_delta", Data: `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":2}}`},
	}

	var sawAdded, sawDelta, sawArgsDone, sawDone bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.added":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "function_call" && item["name"] == "lookup" {
					sawAdded = true
				}
			case "response.function_call_arguments.delta":
				sawDelta = true
			case "response.function_call_arguments.done":
				data := event.Data.(map[string]interface{})
				if data["arguments"] == `{"q":"hello"}` {
					sawArgsDone = true
				}
			case "response.output_item.done":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "function_call" && item["name"] == "lookup" && item["arguments"] == `{"q":"hello"}` {
					sawDone = true
				}
			}
		}
	}
	if !sawAdded || !sawDelta || !sawArgsDone || !sawDone {
		t.Fatalf("expected tool_use lifecycle events, got added=%v delta=%v argsDone=%v done=%v", sawAdded, sawDelta, sawArgsDone, sawDone)
	}
}

func TestResponsesEventsFromAnthropicFramePreservesToolUseInputFromStart(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_tool_start","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`},
		{Event: "message_stop", Data: `{"type":"message_stop"}`},
	}

	var sawAddedArgs, sawArgsDone bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_item.added":
				data := event.Data.(map[string]interface{})
				item := data["item"].(map[string]interface{})
				if item["type"] == "function_call" && item["arguments"] == `{"q":"hello"}` {
					sawAddedArgs = true
				}
			case "response.function_call_arguments.done":
				data := event.Data.(map[string]interface{})
				if data["arguments"] == `{"q":"hello"}` {
					sawArgsDone = true
				}
			}
		}
	}
	if !sawAddedArgs || !sawArgsDone {
		t.Fatalf("expected tool_use start input to populate responses lifecycle, got added=%v argsDone=%v", sawAddedArgs, sawArgsDone)
	}
}

func TestChatChunksFromAnthropicFramePreservesRicherTextBlockStart(t *testing.T) {
	state := NewAnthropicInboundState()
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_rich","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}`},
	}

	var sawText, sawAnnotations, sawRefusal, sawAudio bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["content"] == "hello" {
				sawText = true
			}
			if _, ok := delta["annotations"].([]interface{}); ok {
				sawAnnotations = true
			}
			if delta["refusal"] == "blocked" {
				sawRefusal = true
			}
			if _, ok := delta["audio"].(map[string]interface{}); ok {
				sawAudio = true
			}
		}
	}
	if !sawText || !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected richer text block chat chunks, got text=%v annotations=%v refusal=%v audio=%v", sawText, sawAnnotations, sawRefusal, sawAudio)
	}
}

func TestChatChunksFromAnthropicFramePreservesThinkingTextFromStart(t *testing.T) {
	state := NewAnthropicInboundState()
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_reasoning_start","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`},
	}

	var sawReasoning bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["reasoning_content"] == "think step" {
				sawReasoning = true
			}
		}
	}
	if !sawReasoning {
		t.Fatal("expected thinking start text to populate chat reasoning_content")
	}
}

func TestChatChunksFromAnthropicFramePreservesToolUseInputFromStart(t *testing.T) {
	state := NewAnthropicInboundState()
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_tool_start","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`},
	}

	var sawArguments bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromAnthropicFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			toolCalls, _ := delta["tool_calls"].([]map[string]interface{})
			if len(toolCalls) == 0 {
				continue
			}
			fn := toolCalls[0]["function"].(map[string]interface{})
			if fn["arguments"] == `{"q":"hello"}` {
				sawArguments = true
			}
		}
	}
	if !sawArguments {
		t.Fatal("expected tool_use start input to populate chat tool arguments")
	}
}

func TestResponsesEventsFromAnthropicFramePreservesRicherTextBlockStart(t *testing.T) {
	state := NewAnthropicInboundState()
	var seq int64
	frames := []SSEFrame{
		{Event: "message_start", Data: `{"type":"message_start","message":{"id":"msg_rich","model":"claude","usage":{"input_tokens":3}}}`},
		{Event: "content_block_start", Data: `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}`},
	}

	var sawText, sawAnnotations, sawRefusal, sawAudio bool
	for _, frame := range frames {
		events, _, _, _ := ResponsesEventsFromAnthropicFrame(frame, state, &seq)
		for _, event := range events {
			switch event.Event {
			case "response.output_text.delta":
				sawText = true
			case "response.annotation.added":
				sawAnnotations = true
			case "response.refusal.done":
				sawRefusal = true
			case "response.audio.done":
				sawAudio = true
			}
		}
	}
	if !sawText || !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected richer text block responses events, got text=%v annotations=%v refusal=%v audio=%v", sawText, sawAnnotations, sawRefusal, sawAudio)
	}
}
