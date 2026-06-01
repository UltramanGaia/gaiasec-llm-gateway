package protocol

import "encoding/json"

// IRStreamEventsFromResponsesFrame decodes a Responses SSE frame into one or more
// IR-level stream events without imposing target-protocol encoding rules.
func IRStreamEventsFromResponsesFrame(frame SSEFrame) []IRStreamEvent {
	if frame.Data == "" || frame.Data == "[DONE]" {
		return nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil
	}

	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}
	normalizedType := normalizeResponsesIRStreamEventType(eventType, payload)

	base := IRStreamEvent{
		Type:         normalizedType,
		Index:        numberToIntDefault(payload["output_index"]),
		ItemID:       stringValue(payload["item_id"]),
		ContentIndex: numberToIntDefault(payload["content_index"]),
		CallID:       stringValue(payload["call_id"]),
		Delta:        stringValue(payload["delta"]),
		Text:         stringValue(payload["text"]),
		Refusal:      stringValue(payload["refusal"]),
		Arguments:    stringValue(payload["arguments"]),
		ProviderExtensions: map[string]interface{}{
			"source_event_type": eventType,
		},
	}

	if audio, ok := payload["audio"]; ok && audio != nil {
		if raw, err := json.Marshal(audio); err == nil {
			base.Audio = raw
		}
	}
	if annotations, ok := payload["annotations"]; ok && annotations != nil {
		if raw, err := json.Marshal(annotations); err == nil {
			base.Annotations = raw
		}
	}
	if item, ok := payload["item"]; ok && item != nil {
		itemMap := asMap(item)
		if raw, err := json.Marshal(item); err == nil {
			base.Item = raw
			base.Status = firstNonEmpty(base.Status, stringValue(itemMap["status"]))
			base.CallID = firstNonEmpty(base.CallID, stringValue(itemMap["call_id"]))
			base.ItemID = firstNonEmpty(base.ItemID, stringValue(itemMap["id"]))
			base.Arguments = firstNonEmpty(base.Arguments, stringValue(itemMap["arguments"]), stringValue(itemMap["input"]))
			base.Text = firstNonEmpty(base.Text, extractIRSummaryText(itemMap))
		}
	}

	switch eventType {
	case "response.completed":
		if response, ok := payload["response"]; ok && response != nil {
			respMap := asMap(response)
			if raw, err := json.Marshal(respMap["usage"]); err == nil {
				base.Usage = raw
			}
			base.ItemID = firstNonEmpty(base.ItemID, stringValue(respMap["id"]))
			base.Status = firstNonEmpty(base.Status, stringValue(respMap["status"]))
		}
	case "response.output_item.added", "response.output_item.done":
		base.Status = firstNonEmpty(base.Status, stringValue(asMap(payload["item"])["status"]))
	case "response.content_part.done":
		part := asMap(payload["part"])
		base.Text = firstNonEmpty(base.Text, stringValue(part["text"]))
		base.Refusal = firstNonEmpty(base.Refusal, stringValue(part["refusal"]))
		if annotations, ok := part["annotations"]; ok && annotations != nil {
			if raw, err := json.Marshal(annotations); err == nil {
				base.Annotations = raw
			}
		}
		if audio, ok := part["audio"]; ok && audio != nil {
			if raw, err := json.Marshal(audio); err == nil {
				base.Audio = raw
			}
		}
	}

	return []IRStreamEvent{base}
}

func extractIRSummaryText(item map[string]interface{}) string {
	summary, _ := item["summary"].([]interface{})
	for _, raw := range summary {
		part, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if text := stringValue(part["text"]); text != "" {
			return text
		}
	}
	return ""
}

// IRStreamEventsFromChatChunk decodes a chat.completion.chunk payload into IR events.
func IRStreamEventsFromChatChunk(chatChunk map[string]interface{}) []IRStreamEvent {
	choices, _ := chatChunk["choices"].([]interface{})
	events := make([]IRStreamEvent, 0, len(choices)*4)
	for _, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]interface{})
		if !ok {
			continue
		}
		index := numberToIntDefault(choice["index"])
		delta, _ := choice["delta"].(map[string]interface{})
		if text := stringValue(delta["content"]); text != "" {
			events = append(events, IRStreamEvent{Type: "output_text.delta", Index: index, Delta: text, Text: text})
		}
		if reasoning := stringValue(delta["reasoning_content"]); reasoning != "" {
			events = append(events, IRStreamEvent{Type: "reasoning.delta", Index: index, Delta: reasoning, Text: reasoning})
		}
		if refusal := stringValue(delta["refusal"]); refusal != "" {
			events = append(events, IRStreamEvent{Type: "refusal.delta", Index: index, Delta: refusal, Refusal: refusal})
		}
		if annotations, ok := delta["annotations"]; ok && annotations != nil {
			raw, _ := json.Marshal(annotations)
			events = append(events, IRStreamEvent{Type: "annotation.added", Index: index, Annotations: raw})
		}
		if audio, ok := delta["audio"]; ok && audio != nil {
			raw, _ := json.Marshal(audio)
			events = append(events, IRStreamEvent{Type: "audio.delta", Index: index, Audio: raw})
		}
		if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
			for _, rawTool := range toolCalls {
				tool, ok := rawTool.(map[string]interface{})
				if !ok {
					continue
				}
				fn := asMap(tool["function"])
				events = append(events, IRStreamEvent{
					Type:      "tool_call.delta",
					Index:     numberToIntDefault(tool["index"]),
					ItemID:    stringValue(tool["id"]),
					CallID:    stringValue(tool["id"]),
					Arguments: stringValue(fn["arguments"]),
					ProviderExtensions: map[string]interface{}{
						"name": stringValue(fn["name"]),
						"type": firstNonEmpty(stringValue(tool["type"]), "function"),
					},
				})
			}
		}
		if finishReason := stringValue(choice["finish_reason"]); finishReason != "" {
			events = append(events, IRStreamEvent{Type: "response.completed", Index: index, FinishReason: finishReason})
		}
	}
	if usage, ok := chatChunk["usage"]; ok && usage != nil {
		raw, _ := json.Marshal(usage)
		events = append(events, IRStreamEvent{Type: "usage", Usage: raw})
	}
	return events
}

func asMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// IRStreamEventsFromAnthropicFrame decodes an Anthropic SSE frame into IR-level events.
func IRStreamEventsFromAnthropicFrame(frame SSEFrame) []IRStreamEvent {
	if frame.Data == "" {
		return nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil
	}

	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}
	normalizedType := normalizeAnthropicIRStreamEventType(eventType, payload)

	switch eventType {
	case "message_start":
		msg := asMap(payload["message"])
		var usage json.RawMessage
		if raw, err := json.Marshal(msg["usage"]); err == nil {
			usage = raw
		}
		return []IRStreamEvent{{
			Type:   normalizedType,
			ItemID: stringValue(msg["id"]),
			Status: "in_progress",
			Usage:  usage,
			ProviderExtensions: map[string]interface{}{
				"source_event_type": eventType,
			},
		}}
	case "content_block_start":
		index := numberToIntDefault(payload["index"])
		block := asMap(payload["content_block"])
		blockType := stringValue(block["type"])
		event := IRStreamEvent{
			Type:   normalizedType,
			Index:  index,
			ItemID: stringValue(block["id"]),
			Status: "in_progress",
			ProviderExtensions: map[string]interface{}{
				"source_event_type": eventType,
			},
		}
		if raw, err := json.Marshal(block); err == nil {
			event.Item = raw
		}
		switch blockType {
		case "tool_use":
			event.CallID = stringValue(block["id"])
			event.Arguments = mustMarshalToString(block["input"])
			event.ProviderExtensions = map[string]interface{}{"name": stringValue(block["name"])}
		case "image", "document":
			event.ProviderExtensions = map[string]interface{}{"part_type": blockType}
		case "thinking":
			event.Text = stringValue(block["thinking"])
		case "text":
			events := make([]IRStreamEvent, 0, 4)
			if text := stringValue(block["text"]); text != "" {
				events = append(events, IRStreamEvent{
					Type:  "output_text.delta",
					Index: index,
					Text:  text,
					Delta: text,
					ProviderExtensions: map[string]interface{}{
						"source_event_type": eventType,
					},
				})
			}
			if annotations, ok := block["annotations"]; ok && annotations != nil {
				raw, _ := json.Marshal(annotations)
				events = append(events, IRStreamEvent{
					Type:        "annotation.added",
					Index:       index,
					Annotations: raw,
					ProviderExtensions: map[string]interface{}{
						"source_event_type": eventType,
					},
				})
			}
			if refusal := stringValue(block["refusal"]); refusal != "" {
				events = append(events, IRStreamEvent{
					Type:    "refusal.delta",
					Index:   index,
					Refusal: refusal,
					Delta:   refusal,
					ProviderExtensions: map[string]interface{}{
						"source_event_type": eventType,
					},
				})
			}
			if audio, ok := block["audio"]; ok && audio != nil {
				raw, _ := json.Marshal(audio)
				events = append(events, IRStreamEvent{
					Type:  "audio.delta",
					Index: index,
					Audio: raw,
					ProviderExtensions: map[string]interface{}{
						"source_event_type": eventType,
					},
				})
			}
			if len(events) > 0 {
				return events
			}
		}
		return []IRStreamEvent{event}
	case "content_block_delta":
		index := numberToIntDefault(payload["index"])
		delta := asMap(payload["delta"])
		deltaType := stringValue(delta["type"])
		event := IRStreamEvent{
			Type:  normalizedType,
			Index: index,
			ProviderExtensions: map[string]interface{}{
				"source_event_type": eventType,
			},
		}
		switch deltaType {
		case "text_delta":
			event.Text = stringValue(delta["text"])
			event.Delta = event.Text
		case "thinking_delta":
			event.Text = stringValue(delta["thinking"])
			event.Delta = event.Text
		case "input_json_delta":
			event.Arguments = stringValue(delta["partial_json"])
			event.Delta = event.Arguments
		}
		return []IRStreamEvent{event}
	case "message_delta":
		delta := asMap(payload["delta"])
		var usage json.RawMessage
		if raw, err := json.Marshal(payload["usage"]); err == nil {
			usage = raw
		}
		return []IRStreamEvent{{
			Type:         normalizedType,
			FinishReason: anthropicStopReasonToChatFinish(stringValue(delta["stop_reason"])),
			Usage:        usage,
			ProviderExtensions: map[string]interface{}{
				"source_event_type": eventType,
			},
		}}
	case "message_stop":
		return []IRStreamEvent{{
			Type: normalizedType,
			ProviderExtensions: map[string]interface{}{
				"source_event_type": eventType,
			},
		}}
	default:
		return nil
	}
}

func normalizeResponsesIRStreamEventType(eventType string, payload map[string]interface{}) string {
	switch eventType {
	case "response.output_text.delta":
		return "output_text.delta"
	case "response.reasoning.delta":
		return "reasoning.delta"
	case "response.annotation.added":
		return "annotation.added"
	case "response.refusal.delta":
		return "refusal.delta"
	case "response.audio.delta", "response.audio.done":
		return "audio.delta"
	case "response.function_call_arguments.delta":
		return "tool_call.delta"
	case "response.function_call_arguments.done":
		return "tool_call.done"
	case "response.output_item.added":
		item := asMap(payload["item"])
		switch stringValue(item["type"]) {
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			return "tool_call.start"
		case "reasoning":
			return "reasoning.start"
		}
	case "response.output_item.done":
		item := asMap(payload["item"])
		switch stringValue(item["type"]) {
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			return "tool_call.done"
		case "reasoning":
			return "reasoning.done"
		}
	case "response.content_part.done":
		part := asMap(payload["part"])
		switch stringValue(part["type"]) {
		case "refusal":
			return "refusal.delta"
		case "output_audio":
			return "audio.delta"
		}
	}
	return eventType
}

func normalizeAnthropicIRStreamEventType(eventType string, payload map[string]interface{}) string {
	switch eventType {
	case "message_start":
		return "message.start"
	case "message_delta":
		return "response.completed"
	case "message_stop":
		return "message.stop"
	case "content_block_start":
		block := asMap(payload["content_block"])
		switch stringValue(block["type"]) {
		case "tool_use":
			return "tool_call.start"
		case "thinking":
			return "reasoning.start"
		}
	case "content_block_delta":
		delta := asMap(payload["delta"])
		switch stringValue(delta["type"]) {
		case "text_delta":
			return "output_text.delta"
		case "thinking_delta":
			return "reasoning.delta"
		case "input_json_delta":
			return "tool_call.delta"
		}
	}
	return eventType
}
