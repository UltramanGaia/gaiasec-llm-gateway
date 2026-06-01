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

func TestChatChunksFromResponsesFrameDoneWithoutCompletedEmitsFinishChunk(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_done_only","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup"}}`},
		{Event: "", Data: "[DONE]"},
	}

	var lastChunk map[string]interface{}
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		if len(chunks) > 0 {
			lastChunk = chunks[len(chunks)-1]
		}
	}

	if lastChunk == nil {
		t.Fatal("expected final chat chunk on [DONE]")
	}
	choices := lastChunk["choices"].([]map[string]interface{})
	if choices[0]["finish_reason"] != "tool_calls" {
		t.Fatalf("expected [DONE] flush finish_reason tool_calls, got %+v", choices[0]["finish_reason"])
	}
}

func TestChatChunksFromResponsesFrameDoneFlushPreservesUsage(t *testing.T) {
	state := NewResponsesStreamState()
	state.ResponseID = "resp_done_usage"
	state.Model = "resp-model"
	state.PendingUsage = map[string]interface{}{
		"prompt_tokens":     3,
		"completion_tokens": 2,
		"total_tokens":      5,
	}

	chunks, _ := ChatChunksFromResponsesFrame(SSEFrame{Data: "[DONE]"}, state)
	if len(chunks) != 1 {
		t.Fatalf("expected one final chunk, got %+v", chunks)
	}
	usage := chunks[0]["usage"].(map[string]interface{})
	if usage["total_tokens"] != 5 {
		t.Fatalf("expected flush to preserve usage, got %+v", usage)
	}
}

func TestChatChunksFromResponsesFrameToolArgumentsDoneEmitsFinalArguments(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_done","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup"}}`},
		{Event: "response.function_call_arguments.done", Data: `{"type":"response.function_call_arguments.done","output_index":5,"arguments":"{\"q\":\"hello\"}"}`},
	}

	var sawFinalArgs bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			toolCalls, _ := delta["tool_calls"].([]map[string]interface{})
			if len(toolCalls) == 0 {
				continue
			}
			fn := toolCalls[0]["function"].(map[string]interface{})
			if fn["arguments"] == `{"q":"hello"}` {
				sawFinalArgs = true
			}
		}
	}

	if !sawFinalArgs {
		t.Fatal("expected function_call_arguments.done to emit final tool arguments chunk")
	}
}

func TestChatChunksFromResponsesFrameToolOutputItemDoneEmitsFinalArguments(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_done_item","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup"}}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`},
	}

	var sawFinalArgs bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			toolCalls, _ := delta["tool_calls"].([]map[string]interface{})
			if len(toolCalls) == 0 {
				continue
			}
			fn := toolCalls[0]["function"].(map[string]interface{})
			if fn["arguments"] == `{"q":"hello"}` {
				sawFinalArgs = true
			}
		}
	}

	if !sawFinalArgs {
		t.Fatal("expected output_item.done to emit final tool arguments chunk")
	}
}

func TestChatChunksFromResponsesFrameToolOutputItemAddedEmitsInitialArguments(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_added_item","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"in_progress","arguments":"{\"q\":\"hello\"}"}}`},
	}

	var sawInitialArgs bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			toolCalls, _ := delta["tool_calls"].([]map[string]interface{})
			if len(toolCalls) == 0 {
				continue
			}
			fn := toolCalls[0]["function"].(map[string]interface{})
			if fn["arguments"] == `{"q":"hello"}` {
				sawInitialArgs = true
			}
		}
	}

	if !sawInitialArgs {
		t.Fatal("expected output_item.added to emit initial tool arguments chunk")
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

func TestAnthropicEventsFromResponsesFramePreservesMessageStartOnlyRicherContent(t *testing.T) {
	state := NewAnthropicOutboundState()
	events := AnthropicEventsFromResponsesFrame(SSEFrame{
		Event: "response.output_item.added",
		Data:  `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`,
	}, state)

	var sawText, sawAnnotations, sawRefusal, sawAudio bool
	for _, event := range events {
		if event.Event != "content_block_start" {
			continue
		}
		if strings.Contains(event.Data, `"text":"hello"`) {
			sawText = true
		}
		if strings.Contains(event.Data, `"annotations"`) && strings.Contains(event.Data, `"url_citation"`) {
			sawAnnotations = true
		}
		if strings.Contains(event.Data, `"refusal":"blocked"`) {
			sawRefusal = true
		}
		if strings.Contains(event.Data, `"audio"`) && strings.Contains(event.Data, `"aud_1"`) {
			sawAudio = true
		}
	}
	if !sawText || !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected output_item.added message content to populate anthropic blocks, got text=%v annotations=%v refusal=%v audio=%v", sawText, sawAnnotations, sawRefusal, sawAudio)
	}
}

func TestChatChunksFromResponsesFrameRefusalDelta(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_refusal","model":"resp-model"}}`},
		{Event: "response.refusal.delta", Data: `{"type":"response.refusal.delta","output_index":0,"delta":"cannot comply"}`},
	}

	var sawRole, sawRefusal bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if delta["refusal"] == "cannot comply" {
				sawRefusal = true
			}
		}
	}
	if !sawRole || !sawRefusal {
		t.Fatalf("expected role and refusal chunks, got role=%v refusal=%v", sawRole, sawRefusal)
	}
}

func TestChatChunksFromResponsesFramePreservesMessageStartOnlyRicherContent(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_start_only","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","status":"in_progress","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]},{"type":"refusal","refusal":"blocked"},{"type":"output_audio","audio":{"id":"aud_1","format":"wav"}}]}}`},
	}

	var sawRole, sawText, sawAnnotations, sawRefusal, sawAudio bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
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
	if !sawRole || !sawText || !sawAnnotations || !sawRefusal || !sawAudio {
		t.Fatalf("expected output_item.added richer content to populate chat chunks, got role=%v text=%v annotations=%v refusal=%v audio=%v", sawRole, sawText, sawAnnotations, sawRefusal, sawAudio)
	}
}

func TestAnthropicEventsFromResponsesFramePreservesCustomToolLifecycle(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_custom","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"custom_tool_call","id":"call_1","call_id":"call_1","name":"local_shell","status":"in_progress"}}`},
		{Event: "response.function_call_arguments.delta", Data: `{"type":"response.function_call_arguments.delta","output_index":5,"delta":"{\"cmd\":\"ls\"}"}`},
		{Event: "response.function_call_arguments.done", Data: `{"type":"response.function_call_arguments.done","output_index":5,"arguments":"{\"cmd\":\"ls\"}"}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":5,"item":{"type":"custom_tool_call","id":"call_1","call_id":"call_1","name":"local_shell","status":"completed","input":"{\"cmd\":\"ls\"}"}}`},
	}

	var sawToolStart, sawToolDelta, sawToolArgsDone, sawToolStop bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"tool_use"`) && strings.Contains(event.Data, `"name":"local_shell"`) {
				sawToolStart = true
			}
			if event.Event == "content_block_delta" && strings.Contains(event.Data, `"input_json_delta"`) {
				sawToolDelta = true
				if strings.Contains(event.Data, `"{\"cmd\":\"ls\"}"`) {
					sawToolArgsDone = true
				}
			}
			if event.Event == "content_block_stop" && strings.Contains(event.Data, `"index":7`) {
				sawToolStop = true
			}
		}
	}
	if !sawToolStart || !sawToolDelta || !sawToolArgsDone || !sawToolStop {
		t.Fatalf("expected custom tool lifecycle to map to anthropic tool_use, got start=%v delta=%v argsDone=%v stop=%v", sawToolStart, sawToolDelta, sawToolArgsDone, sawToolStop)
	}
}

func TestAnthropicEventsFromResponsesFramePreservesToolArgumentsFromOutputItemAdded(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_tool_added","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"in_progress","arguments":"{\"q\":\"hello\"}"}}`},
	}

	var sawToolStartWithInput bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"tool_use"`) && strings.Contains(event.Data, `"input":{"q":"hello"}`) {
				sawToolStartWithInput = true
			}
		}
	}
	if !sawToolStartWithInput {
		t.Fatal("expected output_item.added arguments to populate anthropic tool_use start input")
	}
}

func TestAnthropicEventsFromResponsesFrameEmitsToolArgsFromOutputItemDoneWithoutDelta(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_done_item","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"in_progress"}}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"completed","arguments":"{\"q\":\"hello\"}"}}`},
	}

	var sawFinalArgs bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_delta" && strings.Contains(event.Data, `"input_json_delta"`) && strings.Contains(event.Data, `"{\"q\":\"hello\"}"`) {
				sawFinalArgs = true
			}
		}
	}

	if !sawFinalArgs {
		t.Fatal("expected output_item.done to emit anthropic input_json_delta")
	}
}

func TestAnthropicEventsFromResponsesFrameEmitsToolArgsFromArgumentsDoneWithoutDelta(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_done_only","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":5,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"lookup","status":"in_progress"}}`},
		{Event: "response.function_call_arguments.done", Data: `{"type":"response.function_call_arguments.done","output_index":5,"arguments":"{\"q\":\"hello\"}"}`},
	}

	var sawFinalArgs bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_delta" && strings.Contains(event.Data, `"input_json_delta"`) && strings.Contains(event.Data, `"{\"q\":\"hello\"}"`) {
				sawFinalArgs = true
			}
		}
	}

	if !sawFinalArgs {
		t.Fatal("expected function_call_arguments.done to emit anthropic input_json_delta")
	}
}

func TestAnthropicEventsFromResponsesFramePreservesReasoningDelta(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning","model":"resp-model"}}`},
		{Event: "response.reasoning.delta", Data: `{"type":"response.reasoning.delta","output_index":50,"item_id":"rs_1","delta":"think step"}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`},
	}

	var sawThinkingStart, sawThinkingDelta, sawThinkingStop bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"thinking"`) {
				sawThinkingStart = true
			}
			if event.Event == "content_block_delta" && strings.Contains(event.Data, `"thinking":"think step"`) {
				sawThinkingDelta = true
			}
			if event.Event == "content_block_stop" && strings.Contains(event.Data, `"index":1`) {
				sawThinkingStop = true
			}
		}
	}
	if !sawThinkingStart || !sawThinkingDelta || !sawThinkingStop {
		t.Fatalf("expected responses reasoning delta to map to anthropic thinking, got start=%v delta=%v stop=%v", sawThinkingStart, sawThinkingDelta, sawThinkingStop)
	}
}

func TestAnthropicEventsFromResponsesFramePreservesReasoningSummaryFromOutputItemAdded(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning_added","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":"think step"}]}}`},
	}

	var sawThinkingStart bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"thinking"`) && strings.Contains(event.Data, `"thinking":"think step"`) {
				sawThinkingStart = true
			}
		}
	}
	if !sawThinkingStart {
		t.Fatal("expected output_item.added summary to populate anthropic thinking start")
	}
}

func TestAnthropicEventsFromResponsesFrameEmitsReasoningFromOutputItemDoneWithoutDelta(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning_done","model":"resp-model"}}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`},
	}

	var sawThinkingStart, sawThinkingDelta, sawThinkingStop bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"type":"thinking"`) {
				sawThinkingStart = true
			}
			if event.Event == "content_block_delta" && strings.Contains(event.Data, `"thinking":"think step"`) {
				sawThinkingDelta = true
			}
			if event.Event == "content_block_stop" && strings.Contains(event.Data, `"index":1`) {
				sawThinkingStop = true
			}
		}
	}
	if !sawThinkingStart || !sawThinkingDelta || !sawThinkingStop {
		t.Fatalf("expected output_item.done reasoning to map to anthropic thinking, got start=%v delta=%v stop=%v", sawThinkingStart, sawThinkingDelta, sawThinkingStop)
	}
}

func TestAnthropicEventsFromResponsesFramePreservesAnnotationAndAudio(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_rich","model":"resp-model"}}`},
		{Event: "response.annotation.added", Data: `{"type":"response.annotation.added","output_index":0,"annotations":[{"type":"url_citation","title":"doc"}]}`},
		{Event: "response.audio.delta", Data: `{"type":"response.audio.delta","output_index":0,"audio":{"id":"aud_1","format":"wav"}}`},
	}

	var sawAnnotationBlock, sawAudioBlock bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event != "content_block_start" {
				continue
			}
			if strings.Contains(event.Data, `"annotations"`) && strings.Contains(event.Data, `"url_citation"`) {
				sawAnnotationBlock = true
			}
			if strings.Contains(event.Data, `"audio"`) && strings.Contains(event.Data, `"aud_1"`) {
				sawAudioBlock = true
			}
		}
	}
	if !sawAnnotationBlock || !sawAudioBlock {
		t.Fatalf("expected annotation/audio content blocks, got annotation=%v audio=%v", sawAnnotationBlock, sawAudioBlock)
	}
}

func TestAnthropicEventsFromResponsesFramePreservesRefusal(t *testing.T) {
	state := NewAnthropicOutboundState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_refusal","model":"resp-model"}}`},
		{Event: "response.refusal.delta", Data: `{"type":"response.refusal.delta","output_index":0,"delta":"blocked"}`},
	}

	var sawRefusalBlock bool
	for _, frame := range frames {
		events := AnthropicEventsFromResponsesFrame(frame, state)
		for _, event := range events {
			if event.Event == "content_block_start" && strings.Contains(event.Data, `"refusal":"blocked"`) {
				sawRefusalBlock = true
			}
		}
	}
	if !sawRefusalBlock {
		t.Fatalf("expected refusal content block")
	}
}

func TestAnthropicFramesFromChatChunkPreserveRicherTextSemantics(t *testing.T) {
	state := NewAnthropicOutboundState()
	chunk := map[string]interface{}{
		"id":    "chatcmpl_rich",
		"model": "chat-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"content":     "hello",
					"refusal":     "blocked",
					"annotations": []interface{}{map[string]interface{}{"type": "url_citation", "title": "doc"}},
					"audio":       map[string]interface{}{"id": "aud_1", "format": "wav"},
				},
				"finish_reason": "stop",
			},
		},
	}

	frames := AnthropicFramesFromChatChunk(chunk, state)
	var sawAnnotationBlock, sawRefusalBlock, sawAudioBlock bool
	for _, frame := range frames {
		if frame.Event != "content_block_start" {
			continue
		}
		if strings.Contains(frame.Data, `"annotations"`) && strings.Contains(frame.Data, `"url_citation"`) {
			sawAnnotationBlock = true
		}
		if strings.Contains(frame.Data, `"refusal":"blocked"`) {
			sawRefusalBlock = true
		}
		if strings.Contains(frame.Data, `"audio"`) && strings.Contains(frame.Data, `"aud_1"`) {
			sawAudioBlock = true
		}
	}
	if !sawAnnotationBlock || !sawRefusalBlock || !sawAudioBlock {
		t.Fatalf("expected richer anthropic blocks, got annotation=%v refusal=%v audio=%v", sawAnnotationBlock, sawRefusalBlock, sawAudioBlock)
	}
}

func TestChatChunksFromResponsesFrameAudioDelta(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_audio","model":"resp-model"}}`},
		{Event: "response.audio.delta", Data: `{"type":"response.audio.delta","output_index":0,"audio":{"id":"aud_1","format":"wav"}}`},
	}

	var sawRole, sawAudio bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if _, ok := delta["audio"].(map[string]interface{}); ok {
				sawAudio = true
			}
		}
	}
	if !sawRole || !sawAudio {
		t.Fatalf("expected role and audio chunks, got role=%v audio=%v", sawRole, sawAudio)
	}
}

func TestChatChunksFromResponsesFrameAnnotationDone(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_annotations","model":"resp-model"}}`},
		{Event: "response.content_part.done", Data: `{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"output_text","text":"hello","annotations":[{"type":"url_citation","title":"doc"}]}}`},
	}

	var sawRole, sawAnnotations bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if annotations, ok := delta["annotations"].([]interface{}); ok && len(annotations) == 1 {
				sawAnnotations = true
			}
		}
	}
	if !sawRole || !sawAnnotations {
		t.Fatalf("expected role and annotations chunks, got role=%v annotations=%v", sawRole, sawAnnotations)
	}
}

func TestChatChunksFromResponsesFrameAnnotationAdded(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_annotations_added","model":"resp-model"}}`},
		{Event: "response.annotation.added", Data: `{"type":"response.annotation.added","output_index":0,"item_id":"msg_1","annotations":[{"type":"url_citation","title":"doc"}]}`},
	}

	var sawRole, sawAnnotations bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if annotations, ok := delta["annotations"].([]interface{}); ok && len(annotations) == 1 {
				sawAnnotations = true
			}
		}
	}
	if !sawRole || !sawAnnotations {
		t.Fatalf("expected role and annotations chunks, got role=%v annotations=%v", sawRole, sawAnnotations)
	}
}

func TestChatChunksFromResponsesFrameReasoningDelta(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning","model":"resp-model"}}`},
		{Event: "response.reasoning.delta", Data: `{"type":"response.reasoning.delta","output_index":50,"delta":"think step"}`},
	}

	var sawRole, sawReasoning bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if delta["reasoning_content"] == "think step" {
				sawReasoning = true
			}
		}
	}
	if !sawRole || !sawReasoning {
		t.Fatalf("expected role and reasoning chunks, got role=%v reasoning=%v", sawRole, sawReasoning)
	}
}

func TestChatChunksFromResponsesFrameReasoningOutputItemDoneEmitsFinalSummary(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning_done","model":"resp-model"}}`},
		{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"think step"}]}}`},
	}

	var sawRole, sawReasoning bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if delta["reasoning_content"] == "think step" {
				sawReasoning = true
			}
		}
	}
	if !sawRole || !sawReasoning {
		t.Fatalf("expected role and reasoning summary chunks, got role=%v reasoning=%v", sawRole, sawReasoning)
	}
}

func TestChatChunksFromResponsesFrameReasoningOutputItemAddedEmitsInitialSummary(t *testing.T) {
	state := NewResponsesStreamState()
	frames := []SSEFrame{
		{Event: "response.created", Data: `{"type":"response.created","response":{"id":"resp_reasoning_added","model":"resp-model"}}`},
		{Event: "response.output_item.added", Data: `{"type":"response.output_item.added","output_index":50,"item":{"type":"reasoning","id":"rs_1","status":"in_progress","summary":[{"type":"summary_text","text":"think step"}]}}`},
	}

	var sawRole, sawReasoning bool
	for _, frame := range frames {
		chunks, _ := ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			choices := chunk["choices"].([]map[string]interface{})
			delta := choices[0]["delta"].(map[string]interface{})
			if delta["role"] == "assistant" {
				sawRole = true
			}
			if delta["reasoning_content"] == "think step" {
				sawReasoning = true
			}
		}
	}
	if !sawRole || !sawReasoning {
		t.Fatalf("expected role and initial reasoning summary chunks, got role=%v reasoning=%v", sawRole, sawReasoning)
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

func TestFlushAnthropicFramesInfersToolUseStopReasonWithoutCompleted(t *testing.T) {
	state := NewAnthropicOutboundState()
	state.MessageStarted = true
	state.MessageID = "msg_tool_done_only"
	state.Model = "gpt-model"
	state.ToolBlocks[1] = responsesToolState{ID: "call_1", Name: "lookup"}

	frames := FlushAnthropicFrames(state, true)

	var sawMessageDelta, sawToolUseStop bool
	for _, frame := range frames {
		switch frame.Event {
		case "content_block_stop":
			if strings.Contains(frame.Data, `"index":3`) {
				sawToolUseStop = true
			}
		case "message_delta":
			if strings.Contains(frame.Data, `"stop_reason":"tool_use"`) {
				sawMessageDelta = true
			}
		}
	}
	if !sawToolUseStop || !sawMessageDelta {
		t.Fatalf("expected flush to infer tool_use stop_reason, got stop=%v delta=%v frames=%+v", sawToolUseStop, sawMessageDelta, frames)
	}
}

func TestAnthropicFramesFromChatChunkStopsThinkingBeforeText(t *testing.T) {
	state := NewAnthropicOutboundState()
	chunk := map[string]interface{}{
		"id":    "chatcmpl_reasoning",
		"model": "gpt-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"reasoning_content": "think",
				},
			},
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"content": "answer",
				},
				"finish_reason": "stop",
			},
		},
	}

	frames := AnthropicFramesFromChatChunk(chunk, state)
	var sawThinkingStart, sawThinkingStop, sawTextStart, sawTextDelta bool
	for _, frame := range frames {
		switch frame.Event {
		case "content_block_start":
			if strings.Contains(frame.Data, `"type":"thinking"`) {
				sawThinkingStart = true
			}
			if strings.Contains(frame.Data, `"type":"text"`) {
				sawTextStart = true
			}
		case "content_block_stop":
			if strings.Contains(frame.Data, `"index":1`) {
				sawThinkingStop = true
			}
		case "content_block_delta":
			if strings.Contains(frame.Data, `"text":"answer"`) {
				sawTextDelta = true
			}
		}
	}
	if !sawThinkingStart || !sawThinkingStop || !sawTextStart || !sawTextDelta {
		t.Fatalf("expected thinking->stop->text sequence, got thinkingStart=%v thinkingStop=%v textStart=%v textDelta=%v", sawThinkingStart, sawThinkingStop, sawTextStart, sawTextDelta)
	}
}
