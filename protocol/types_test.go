package protocol

import (
	"encoding/json"
	"testing"
)

func TestIRStreamEventSupportsRicherFields(t *testing.T) {
	event := IRStreamEvent{
		Type:         "annotation_added",
		Index:        2,
		ItemID:       "msg_1",
		ContentIndex: 1,
		CallID:       "call_1",
		Status:       "completed",
		Delta:        "delta",
		Text:         "hello",
		Refusal:      "cannot comply",
		Arguments:    `{"q":"hello"}`,
		Annotations:  json.RawMessage(`[{"type":"url_citation","title":"doc"}]`),
		Audio:        json.RawMessage(`{"id":"aud_1","format":"wav"}`),
		Item:         json.RawMessage(`{"type":"reasoning","status":"completed"}`),
		Usage:        json.RawMessage(`{"input_tokens":3,"output_tokens":2}`),
		FinishReason: "stop",
		ProviderExtensions: map[string]interface{}{
			"source_event": "response.content_part.done",
		},
	}

	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal IRStreamEvent: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal IRStreamEvent JSON: %v", err)
	}

	for _, key := range []string{
		"type",
		"item_id",
		"content_index",
		"call_id",
		"status",
		"delta",
		"text",
		"refusal",
		"arguments",
		"annotations",
		"audio",
		"item",
		"usage",
		"finish_reason",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("expected field %q in marshaled IRStreamEvent, got %s", key, string(raw))
		}
	}
}
