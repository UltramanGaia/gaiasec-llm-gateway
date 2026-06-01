package protocol

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadSSEFrame(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("event: response.output_text.delta\ndata: {\"delta\":\"hi\"}\n\n"))
	frame, err := ReadSSEFrame(reader)
	if err != nil {
		t.Fatalf("ReadSSEFrame error: %v", err)
	}
	if frame.Event != "response.output_text.delta" {
		t.Fatalf("unexpected event %q", frame.Event)
	}
	if frame.Data != "{\"delta\":\"hi\"}" {
		t.Fatalf("unexpected data %q", frame.Data)
	}
}

func TestConvertChatChunkToResponsesEvents(t *testing.T) {
	chunk := map[string]interface{}{
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

	events := ConvertChatChunkToResponsesEvents(chunk, 7)
	if len(events) < 7 {
		t.Fatalf("expected multiple responses events, got %d", len(events))
	}
	if events[0].Event != "response.output_item.added" {
		t.Fatalf("unexpected first event %q", events[0].Event)
	}
	if events[1].Event != "response.content_part.added" {
		t.Fatalf("unexpected second event %q", events[1].Event)
	}
	if events[2].Event != "response.output_text.delta" {
		t.Fatalf("unexpected third event %q", events[2].Event)
	}
	if events[3].Event != "response.output_item.added" {
		t.Fatalf("unexpected fourth event %q", events[3].Event)
	}
}

func TestResponsesCompletedAndDoneEventFormatting(t *testing.T) {
	completed := BuildResponsesCompletedEvent("resp_1", "model-a", map[string]interface{}{
		"prompt_tokens":     3,
		"completion_tokens": 2,
	}, "hello", 9)
	done := BuildResponsesDoneEvent("resp_1", "model-a", "hello", 10)

	completedText := FormatResponsesStreamEvent(completed)
	doneText := FormatResponsesStreamEvent(done)

	if !strings.Contains(completedText, "response.completed") || !strings.Contains(completedText, "\"total_tokens\":5") {
		t.Fatalf("unexpected completed event text %s", completedText)
	}
	if !strings.Contains(doneText, "\"model\":\"model-a\"") {
		t.Fatalf("unexpected done event text %s", doneText)
	}
}

func TestConvertChatChunkToResponsesEventsIncludesAudio(t *testing.T) {
	chunk := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0.0,
				"delta": map[string]interface{}{
					"role":  "assistant",
					"audio": map[string]interface{}{"id": "aud_1", "format": "wav"},
				},
				"finish_reason": "stop",
			},
		},
	}

	events := ConvertChatChunkToResponsesEvents(chunk, 8)
	var sawAudioAdded, sawAudioDelta, sawAudioDone bool
	for _, event := range events {
		switch event.Event {
		case "response.content_part.added":
			data := event.Data.(map[string]interface{})
			part := data["part"].(map[string]interface{})
			if part["type"] == "output_audio" {
				sawAudioAdded = true
			}
		case "response.audio.delta":
			sawAudioDelta = true
		case "response.audio.done":
			sawAudioDone = true
		}
	}
	if !sawAudioAdded || !sawAudioDelta || !sawAudioDone {
		t.Fatalf("expected output_audio lifecycle, got added=%v delta=%v done=%v", sawAudioAdded, sawAudioDelta, sawAudioDone)
	}
}
