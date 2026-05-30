package protocol

import "encoding/json"

type anthropicInboundToolState struct {
	ID   string
	Name string
}

type AnthropicInboundState struct {
	MessageID      string
	Model          string
	RoleSent       bool
	LastUsage      map[string]interface{}
	LastStopReason string
	ToolIndexes    map[int]anthropicInboundToolState
}

func NewAnthropicInboundState() *AnthropicInboundState {
	return &AnthropicInboundState{
		ToolIndexes: make(map[int]anthropicInboundToolState),
	}
}

func ChatChunksFromAnthropicFrame(frame SSEFrame, state *AnthropicInboundState) ([]map[string]interface{}, bool) {
	if state == nil {
		state = NewAnthropicInboundState()
	}
	if frame.Data == "" {
		return nil, false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil, false
	}

	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}

	chunks := make([]map[string]interface{}, 0, 2)
	switch eventType {
	case "message_start":
		if msg, ok := payload["message"].(map[string]interface{}); ok {
			state.MessageID = stringValue(msg["id"])
			state.Model = stringValue(msg["model"])
			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				state.LastUsage = anthropicUsageToChatUsage(usage)
			}
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.MessageID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{
					"role": "assistant",
				},
			}},
		})
		state.RoleSent = true
	case "content_block_start":
		index := numberToIntDefault(payload["index"])
		if block, ok := payload["content_block"].(map[string]interface{}); ok {
			switch stringValue(block["type"]) {
			case "tool_use":
				toolID := stringValue(block["id"])
				toolName := stringValue(block["name"])
				state.ToolIndexes[index] = anthropicInboundToolState{ID: toolID, Name: toolName}
				chunks = append(chunks, map[string]interface{}{
					"id":      state.MessageID,
					"object":  "chat.completion.chunk",
					"created": 0,
					"model":   state.Model,
					"choices": []map[string]interface{}{{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []map[string]interface{}{{
								"index": index,
								"id":    toolID,
								"type":  "function",
								"function": map[string]interface{}{
									"name":      toolName,
									"arguments": "",
								},
							}},
						},
					}},
				})
			}
		}
	case "content_block_delta":
		index := numberToIntDefault(payload["index"])
		delta, _ := payload["delta"].(map[string]interface{})
		switch stringValue(delta["type"]) {
		case "text_delta":
			chunks = append(chunks, map[string]interface{}{
				"id":      state.MessageID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{
						"content": stringValue(delta["text"]),
					},
				}},
			})
		case "thinking_delta":
			chunks = append(chunks, map[string]interface{}{
				"id":      state.MessageID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{
						"reasoning_content": stringValue(delta["thinking"]),
					},
				}},
			})
		case "input_json_delta":
			tool := state.ToolIndexes[index]
			chunks = append(chunks, map[string]interface{}{
				"id":      state.MessageID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{{
							"index": index,
							"id":    tool.ID,
							"type":  "function",
							"function": map[string]interface{}{
								"name":      tool.Name,
								"arguments": stringValue(delta["partial_json"]),
							},
						}},
					},
				}},
			})
		}
	case "message_delta":
		delta, _ := payload["delta"].(map[string]interface{})
		state.LastStopReason = anthropicStopReasonToChatFinish(stringValue(delta["stop_reason"]))
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			state.LastUsage = anthropicUsageToChatUsage(usage)
		}
		chunk := map[string]interface{}{
			"id":      state.MessageID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": firstNonEmpty(state.LastStopReason, "stop"),
			}},
		}
		if len(state.LastUsage) > 0 {
			chunk["usage"] = state.LastUsage
		}
		chunks = append(chunks, chunk)
	case "message_stop":
		return nil, true
	}

	return chunks, false
}

func ResponsesEventsFromAnthropicFrame(frame SSEFrame, state *AnthropicInboundState, seqNum *int64) ([]ResponsesStreamEvent, bool, string, string) {
	if state == nil {
		state = NewAnthropicInboundState()
	}
	if frame.Data == "" {
		return nil, false, state.MessageID, state.Model
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil, false, state.MessageID, state.Model
	}
	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}
	events := make([]ResponsesStreamEvent, 0)
	nextSeq := func() int64 {
		if seqNum != nil {
			*seqNum = *seqNum + 1
			return *seqNum
		}
		return 0
	}

	switch eventType {
	case "message_start":
		if msg, ok := payload["message"].(map[string]interface{}); ok {
			state.MessageID = stringValue(msg["id"])
			state.Model = stringValue(msg["model"])
			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				state.LastUsage = anthropicUsageToChatUsage(usage)
			}
		}
		events = append(events, ResponsesStreamEvent{
			Event: "response.output_item.added",
			Data: map[string]interface{}{
				"type":            "response.output_item.added",
				"sequence_number": nextSeq(),
				"output_index":    0,
				"item": map[string]interface{}{
					"type":   "message",
					"status": "in_progress",
					"role":   "assistant",
				},
			},
		})
	case "content_block_start":
		index := numberToIntDefault(payload["index"])
		if block, ok := payload["content_block"].(map[string]interface{}); ok {
			switch stringValue(block["type"]) {
			case "tool_use":
				toolID := stringValue(block["id"])
				toolName := stringValue(block["name"])
				state.ToolIndexes[index] = anthropicInboundToolState{ID: toolID, Name: toolName}
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": nextSeq(),
						"output_index":    index + 100,
						"item": map[string]interface{}{
							"type":    "function_call",
							"id":      toolID,
							"call_id": toolID,
							"name":    toolName,
							"status":  "in_progress",
						},
					},
				})
			case "image", "document":
				events = append(events, ResponsesStreamEvent{
					Event: "response.content_part.added",
					Data: map[string]interface{}{
						"type":            "response.content_part.added",
						"sequence_number": nextSeq(),
						"output_index":    0,
						"content_index":   index,
						"part":            anthropicContentBlockToResponsesPart(block),
					},
				})
			}
		}
	case "content_block_delta":
		index := numberToIntDefault(payload["index"])
		delta, _ := payload["delta"].(map[string]interface{})
		switch stringValue(delta["type"]) {
		case "text_delta":
			events = append(events, ResponsesStreamEvent{
				Event: "response.output_text.delta",
				Data: map[string]interface{}{
					"type":            "response.output_text.delta",
					"sequence_number": nextSeq(),
					"output_index":    0,
					"content_index":   0,
					"delta":           stringValue(delta["text"]),
				},
			})
		case "input_json_delta":
			events = append(events, ResponsesStreamEvent{
				Event: "response.function_call_arguments.delta",
				Data: map[string]interface{}{
					"type":            "response.function_call_arguments.delta",
					"sequence_number": nextSeq(),
					"output_index":    index + 100,
					"delta":           stringValue(delta["partial_json"]),
				},
			})
		}
	case "message_delta":
		delta, _ := payload["delta"].(map[string]interface{})
		if stopReason := stringValue(delta["stop_reason"]); stopReason != "" {
			events = append(events, ResponsesStreamEvent{
				Event: "response.output_item.done",
				Data: map[string]interface{}{
					"type":            "response.output_item.done",
					"sequence_number": nextSeq(),
					"output_index":    0,
					"item": map[string]interface{}{
						"type":   "message",
						"status": "completed",
						"role":   "assistant",
					},
				},
			})
		}
		if usage, ok := payload["usage"].(map[string]interface{}); ok {
			state.LastUsage = anthropicUsageToChatUsage(usage)
			events = append(events, ResponsesStreamEvent{
				Event: "response.completed",
				Data: map[string]interface{}{
					"type":            "response.completed",
					"sequence_number": nextSeq(),
					"response": map[string]interface{}{
						"id":     state.MessageID,
						"object": "response",
						"status": "completed",
						"model":  state.Model,
						"usage": map[string]interface{}{
							"input_tokens":  numberToIntDefault(state.LastUsage["prompt_tokens"]),
							"output_tokens": numberToIntDefault(state.LastUsage["completion_tokens"]),
							"total_tokens":  numberToIntDefault(state.LastUsage["total_tokens"]),
						},
					},
				},
			})
		}
	case "message_stop":
		return events, true, state.MessageID, state.Model
	}

	return events, false, state.MessageID, state.Model
}

func anthropicUsageToChatUsage(usage map[string]interface{}) map[string]interface{} {
	prompt := numberToIntDefault(usage["input_tokens"])
	completion := numberToIntDefault(usage["output_tokens"])
	return map[string]interface{}{
		"prompt_tokens":     prompt,
		"completion_tokens": completion,
		"total_tokens":      prompt + completion,
	}
}

func anthropicContentBlockToResponsesPart(block map[string]interface{}) map[string]interface{} {
	switch stringValue(block["type"]) {
	case "image":
		return map[string]interface{}{
			"type":   "output_image",
			"source": block["source"],
		}
	case "document":
		return map[string]interface{}{
			"type":   "output_file",
			"source": block["source"],
		}
	default:
		return block
	}
}
