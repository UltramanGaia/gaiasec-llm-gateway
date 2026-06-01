package protocol

import (
	"encoding/json"
	"testing"
)

func TestIRStreamEventsFromResponsesFrameSupportsRicherEvents(t *testing.T) {
	frame := SSEFrame{
		Event: "response.content_part.done",
		Data:  `{"type":"response.content_part.done","output_index":0,"item_id":"msg_1","content_index":0,"part":{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]}}`,
	}

	events := IRStreamEventsFromResponsesFrame(frame)
	if len(events) != 1 {
		t.Fatalf("expected 1 IR stream event, got %+v", events)
	}
	if events[0].Type != "response.content_part.done" || events[0].ItemID != "msg_1" || events[0].Text != "hello" {
		t.Fatalf("unexpected IR stream event %+v", events[0])
	}
	if asMap(events[0].ProviderExtensions)["source_event_type"] != "response.content_part.done" {
		t.Fatalf("expected source event type preserved, got %+v", events[0].ProviderExtensions)
	}
	var annotations []map[string]interface{}
	if err := json.Unmarshal(events[0].Annotations, &annotations); err != nil || len(annotations) != 1 {
		t.Fatalf("expected annotations payload, got %s err=%v", string(events[0].Annotations), err)
	}
}

func TestIRStreamEventsFromResponsesFrameSupportsReasoningDelta(t *testing.T) {
	frame := SSEFrame{
		Event: "response.reasoning.delta",
		Data:  `{"type":"response.reasoning.delta","output_index":50,"item_id":"rs_1","delta":"think step"}`,
	}

	events := IRStreamEventsFromResponsesFrame(frame)
	if len(events) != 1 || events[0].Type != "reasoning.delta" || events[0].Delta != "think step" {
		t.Fatalf("expected reasoning delta IR event, got %+v", events)
	}
}

func TestIRStreamEventsFromResponsesFrameSupportsAnnotationAdded(t *testing.T) {
	frame := SSEFrame{
		Event: "response.annotation.added",
		Data:  `{"type":"response.annotation.added","output_index":0,"item_id":"msg_1","annotations":[{"type":"url_citation","title":"doc"}]}`,
	}

	events := IRStreamEventsFromResponsesFrame(frame)
	if len(events) != 1 || events[0].Type != "annotation.added" || events[0].ItemID != "msg_1" {
		t.Fatalf("expected annotation IR event, got %+v", events)
	}
	var annotations []map[string]interface{}
	if err := json.Unmarshal(events[0].Annotations, &annotations); err != nil || len(annotations) != 1 {
		t.Fatalf("expected annotation payload, got %s err=%v", string(events[0].Annotations), err)
	}
}

func TestIRStreamEventsFromResponsesFrameSupportsAudioDone(t *testing.T) {
	frame := SSEFrame{
		Event: "response.audio.done",
		Data:  `{"type":"response.audio.done","output_index":0,"item_id":"msg_1","audio":{"id":"aud_1","format":"wav"}}`,
	}

	events := IRStreamEventsFromResponsesFrame(frame)
	if len(events) != 1 || events[0].Type != "audio.delta" || events[0].ItemID != "msg_1" || len(events[0].Audio) == 0 {
		t.Fatalf("expected audio IR event, got %+v", events)
	}
}

func TestIRStreamEventsFromResponsesFrameNormalizesToolLifecycle(t *testing.T) {
	frame := SSEFrame{
		Event: "response.output_item.added",
		Data:  `{"type":"response.output_item.added","output_index":100,"item":{"type":"custom_tool_call","id":"call_1","call_id":"call_1","name":"local_shell","status":"in_progress"}}`,
	}

	events := IRStreamEventsFromResponsesFrame(frame)
	if len(events) != 1 || events[0].Type != "tool_call.start" || events[0].CallID != "call_1" || events[0].ItemID != "call_1" {
		t.Fatalf("expected normalized tool_call.start event, got %+v", events)
	}
}

func TestIRStreamEventsFromChatChunkSupportsRicherDeltas(t *testing.T) {
	chunk := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"reasoning_content": "think",
					"refusal":           "cannot comply",
					"annotations":       []interface{}{map[string]interface{}{"type": "url_citation"}},
					"audio":             map[string]interface{}{"id": "aud_1"},
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{"prompt_tokens": 3, "completion_tokens": 2, "total_tokens": 5},
	}

	events := IRStreamEventsFromChatChunk(chunk)
	if len(events) < 5 {
		t.Fatalf("expected richer IR events from chat chunk, got %+v", events)
	}

	var sawReasoning, sawRefusal, sawAnnotations, sawAudio, sawFinish, sawUsage bool
	for _, event := range events {
		switch event.Type {
		case "reasoning.delta":
			sawReasoning = event.Delta == "think"
		case "refusal.delta":
			sawRefusal = event.Refusal == "cannot comply"
		case "annotation.added":
			sawAnnotations = len(event.Annotations) > 0
		case "audio.delta":
			sawAudio = len(event.Audio) > 0
		case "response.completed":
			sawFinish = event.FinishReason == "stop"
		case "usage":
			sawUsage = len(event.Usage) > 0
		}
	}
	if !sawReasoning || !sawRefusal || !sawAnnotations || !sawAudio || !sawFinish || !sawUsage {
		t.Fatalf("expected all richer chat IR events, got %+v", events)
	}
}

func TestIRStreamEventsFromAnthropicFrameSupportsThinkingDelta(t *testing.T) {
	frame := SSEFrame{
		Event: "content_block_delta",
		Data:  `{"type":"content_block_delta","index":1,"delta":{"type":"thinking_delta","thinking":"think step"}}`,
	}

	events := IRStreamEventsFromAnthropicFrame(frame)
	if len(events) != 1 || events[0].Type != "reasoning.delta" || events[0].Delta != "think step" {
		t.Fatalf("expected anthropic thinking IR event, got %+v", events)
	}
}

func TestIRStreamEventsFromAnthropicFrameSupportsToolUseStart(t *testing.T) {
	frame := SSEFrame{
		Event: "content_block_start",
		Data:  `{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hello"}}}`,
	}

	events := IRStreamEventsFromAnthropicFrame(frame)
	if len(events) != 1 || events[0].Type != "tool_call.start" || events[0].CallID != "call_1" {
		t.Fatalf("expected anthropic tool_use IR event, got %+v", events)
	}
	if events[0].Arguments != `{"q":"hello"}` {
		t.Fatalf("expected tool input marshaled into arguments, got %+v", events[0])
	}
}

func TestIRStreamEventsFromAnthropicFrameSupportsRicherTextBlockStart(t *testing.T) {
	frame := SSEFrame{
		Event: "content_block_start",
		Data:  `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}],"refusal":"blocked","audio":{"id":"aud_1","format":"wav"}}}`,
	}

	events := IRStreamEventsFromAnthropicFrame(frame)
	if len(events) < 4 {
		t.Fatalf("expected richer anthropic text block events, got %+v", events)
	}
	var sawText, sawAnnotations, sawRefusal, sawAudio bool
	for _, event := range events {
		switch event.Type {
		case "output_text.delta":
			sawText = event.Delta == "hello"
		case "annotation.added":
			sawAnnotations = len(event.Annotations) > 0
		case "refusal.delta":
			sawRefusal = event.Refusal == "blocked"
		case "audio.delta":
			sawAudio = len(event.Audio) > 0
		}
	}
	if !sawText || !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected anthropic richer text block IR events, got %+v", events)
	}
}

func TestIRStreamEventsFromAnthropicFrameSupportsMessageDeltaUsage(t *testing.T) {
	frame := SSEFrame{
		Event: "message_delta",
		Data:  `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":3,"output_tokens":2}}`,
	}

	events := IRStreamEventsFromAnthropicFrame(frame)
	if len(events) != 1 || events[0].Type != "response.completed" || events[0].FinishReason != "stop" || len(events[0].Usage) == 0 {
		t.Fatalf("expected anthropic message_delta IR event with usage, got %+v", events)
	}
}
