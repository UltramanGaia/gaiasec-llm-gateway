package handlers

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"llm-gateway/protocol"
)

func TestBuildOpenAIStreamLogResponseIncludesToolCalls(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hel"}}]}}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"lo\"}"}}],"reasoning_content":"thinking"}}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != nil {
		t.Fatalf("buildOpenAIStreamLogResponse returned error: %v", err)
	}

	if !result.DoneSeen {
		t.Fatal("expected DoneSeen to be true")
	}
	if result.ToolCallChunks != 2 {
		t.Fatalf("expected 2 tool call chunks, got %d", result.ToolCallChunks)
	}
	if result.ReasoningChunks != 1 {
		t.Fatalf("expected 1 reasoning chunk, got %d", result.ReasoningChunks)
	}
	if result.UsageChunks != 1 {
		t.Fatalf("expected 1 usage chunk, got %d", result.UsageChunks)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if got := result.ToolCalls[0].Function.Name; got != "lookup" {
		t.Fatalf("expected function name lookup, got %q", got)
	}
	if got := result.ToolCalls[0].Function.Arguments; got != "{\"q\":\"hello\"}" {
		t.Fatalf("expected merged function arguments, got %q", got)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Role             string                 `json:"role"`
				Content          string                 `json:"content"`
				ReasoningContent string                 `json:"reasoning_content"`
				ToolCalls        []openAIStreamToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(result.ResponseJSON), &payload); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	if len(payload.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(payload.Choices))
	}
	if got := payload.Choices[0].FinishReason; got != "tool_calls" {
		t.Fatalf("expected finish reason tool_calls, got %q", got)
	}
	if got := payload.Choices[0].Message.ReasoningContent; got != "thinking" {
		t.Fatalf("expected reasoning content to be preserved, got %q", got)
	}
	if len(payload.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected tool calls in response JSON, got %d", len(payload.Choices[0].Message.ToolCalls))
	}
}

func TestBuildOpenAIStreamLogResponseInfersToolCallsFinishWithoutFinalChunk(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-tool-done-only","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-tool-done-only","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"hello\"}"}}]}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != nil {
		t.Fatalf("buildOpenAIStreamLogResponse returned error: %v", err)
	}

	var payload struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(result.ResponseJSON), &payload); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	if len(payload.Choices) != 1 || payload.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected inferred finish reason tool_calls, got %+v", payload.Choices)
	}
}

func TestBuildOpenAIStreamLogResponseIncludesRefusal(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-refusal","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-refusal","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"refusal":"cannot "}}]}`,
		`data: {"id":"chatcmpl-refusal","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"refusal":"comply"}}]}`,
		`data: {"id":"chatcmpl-refusal","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != nil {
		t.Fatalf("buildOpenAIStreamLogResponse returned error: %v", err)
	}
	if result.RefusalChunks != 2 || result.Refusal != "cannot comply" {
		t.Fatalf("expected refusal chunks and merged refusal, got %+v", result)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(result.ResponseJSON), &payload); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	if payload.Choices[0].Message.Refusal != "cannot comply" {
		t.Fatalf("expected refusal to be preserved in aggregated response, got %q", payload.Choices[0].Message.Refusal)
	}
}

func TestBuildOpenAIStreamLogResponseIncludesAudio(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-audio","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-audio","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"audio":{"id":"aud_1","format":"wav"}}}]}`,
		`data: {"id":"chatcmpl-audio","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != nil {
		t.Fatalf("buildOpenAIStreamLogResponse returned error: %v", err)
	}
	if result.AudioChunks != 1 || result.Audio["id"] != "aud_1" {
		t.Fatalf("expected audio chunk and aggregated audio payload, got %+v", result)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Audio map[string]interface{} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(result.ResponseJSON), &payload); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	if payload.Choices[0].Message.Audio["id"] != "aud_1" {
		t.Fatalf("expected audio to be preserved in aggregated response, got %+v", payload.Choices[0].Message.Audio)
	}
}

func TestBuildOpenAIStreamLogResponseIncludesAnnotations(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-annotations","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-annotations","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"annotations":[{"type":"url_citation","title":"doc"}]}}]}`,
		`data: {"id":"chatcmpl-annotations","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != nil {
		t.Fatalf("buildOpenAIStreamLogResponse returned error: %v", err)
	}
	if result.AnnotationCount != 1 {
		t.Fatalf("expected annotation count 1, got %+v", result)
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Annotations []interface{} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(result.ResponseJSON), &payload); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}
	if len(payload.Choices[0].Message.Annotations) != 1 {
		t.Fatalf("expected annotations to be preserved in aggregated response, got %+v", payload.Choices[0].Message.Annotations)
	}
}

func TestBuildOpenAIStreamLogResponseReturnsEOFForMetadataOnlyStream(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl-2","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"id":"chatcmpl-2","object":"chat.completion.chunk","created":1,"model":"glm-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":0,"total_tokens":3}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, err := buildOpenAIStreamLogResponse(rawStream)
	if err != io.EOF {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if !result.DoneSeen {
		t.Fatal("expected DoneSeen to be true")
	}
	if result.DataEvents != 2 {
		t.Fatalf("expected 2 data events, got %d", result.DataEvents)
	}
}

func TestLineHasOpenAIStreamTokenDetectsToolCalls(t *testing.T) {
	line := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]}}]}`
	if !lineHasOpenAIStreamToken(line) {
		t.Fatal("expected tool call chunk to count as stream token")
	}
}

func TestLineHasOpenAIStreamTokenDetectsRefusal(t *testing.T) {
	line := `data: {"choices":[{"index":0,"delta":{"refusal":"blocked"}}]}`
	if !lineHasOpenAIStreamToken(line) {
		t.Fatal("expected refusal chunk to count as stream token")
	}
}

func TestLineHasOpenAIStreamTokenDetectsAudio(t *testing.T) {
	line := `data: {"choices":[{"index":0,"delta":{"audio":{"id":"aud_1"}}}]}`
	if !lineHasOpenAIStreamToken(line) {
		t.Fatal("expected audio chunk to count as stream token")
	}
}

func TestLineHasOpenAIStreamTokenDetectsAnnotations(t *testing.T) {
	line := `data: {"choices":[{"index":0,"delta":{"annotations":[{"type":"url_citation"}]}}]}`
	if !lineHasOpenAIStreamToken(line) {
		t.Fatal("expected annotations chunk to count as stream token")
	}
}

func TestAnthropicFrameHasOutputTokenDetectsRicherContentBlockStart(t *testing.T) {
	frame := protocol.SSEFrame{
		Event: "content_block_start",
		Data:  `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"hello","annotations":[{"type":"url_citation"}],"refusal":"blocked","audio":{"id":"aud_1"}}}`,
	}
	if !anthropicFrameHasOutputToken(frame) {
		t.Fatal("expected richer anthropic content_block_start to count as token")
	}
}

func TestAnthropicFrameHasOutputTokenDetectsThinkingStartWithText(t *testing.T) {
	frame := protocol.SSEFrame{
		Event: "content_block_start",
		Data:  `{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"think step","signature":""}}`,
	}
	if !anthropicFrameHasOutputToken(frame) {
		t.Fatal("expected thinking start with text to count as token")
	}
}

func TestResponsesEventHasOutputTokenDetectsRicherEvents(t *testing.T) {
	cases := []protocol.ResponsesStreamEvent{
		{Event: "response.reasoning.delta"},
		{Event: "response.annotation.added"},
		{Event: "response.refusal.done"},
		{Event: "response.audio.done"},
		{Event: "response.function_call_arguments.done"},
	}
	for _, event := range cases {
		if !responsesEventHasOutputToken(event) {
			t.Fatalf("expected %s to count as output token", event.Event)
		}
	}
}

func TestResponsesEventHasOutputTokenDetectsOutputItemWithArgumentsOrSummary(t *testing.T) {
	cases := []protocol.ResponsesStreamEvent{
		{
			Event: "response.output_item.added",
			Data: map[string]interface{}{
				"item": map[string]interface{}{
					"type":      "function_call",
					"arguments": `{"q":"hello"}`,
				},
			},
		},
		{
			Event: "response.output_item.done",
			Data: map[string]interface{}{
				"item": map[string]interface{}{
					"type": "reasoning",
					"summary": []interface{}{
						map[string]interface{}{"type": "summary_text", "text": "think step"},
					},
				},
			},
		},
	}
	for _, event := range cases {
		if !responsesEventHasOutputToken(event) {
			t.Fatalf("expected %s with payload to count as output token", event.Event)
		}
	}
}

func TestResponsesEventsHasOutputTokenSkipsRoleOnlyAndDetectsRealOutput(t *testing.T) {
	roleOnly := []protocol.ResponsesStreamEvent{
		{
			Event: "response.output_item.added",
			Data: map[string]interface{}{
				"item": map[string]interface{}{
					"type":   "message",
					"role":   "assistant",
					"status": "in_progress",
				},
			},
		},
	}
	if responsesEventsHasOutputToken(roleOnly) {
		t.Fatal("expected role-only responses events not to count as output token")
	}

	withOutput := append(roleOnly, protocol.ResponsesStreamEvent{Event: "response.reasoning.delta"})
	if !responsesEventsHasOutputToken(withOutput) {
		t.Fatal("expected reasoning delta to count as output token")
	}
}
