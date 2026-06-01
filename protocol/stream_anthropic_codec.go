package protocol

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

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
	FinishSent     bool
	ToolIndexes    map[int]anthropicInboundToolState
	ToolArguments  map[int]string
	ReasoningItems map[int]string
	ReasoningText  map[int]string
}

func NewAnthropicInboundState() *AnthropicInboundState {
	return &AnthropicInboundState{
		ToolIndexes:    make(map[int]anthropicInboundToolState),
		ToolArguments:  make(map[int]string),
		ReasoningItems: make(map[int]string),
		ReasoningText:  make(map[int]string),
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
	irEvents := IRStreamEventsFromAnthropicFrame(frame)
	var irEvent IRStreamEvent
	if len(irEvents) > 0 {
		irEvent = irEvents[0]
	}

	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}

	chunks := make([]map[string]interface{}, 0, 2)
	switch eventType {
	case "message_start":
		if msg, ok := payload["message"].(map[string]interface{}); ok {
			state.MessageID = firstNonEmpty(irEvent.ItemID, stringValue(msg["id"]))
			state.Model = stringValue(msg["model"])
			if usage := decodeIRUsage(irEvent.Usage); len(usage) > 0 {
				state.LastUsage = anthropicUsageToChatUsage(usage)
			} else if usage, ok := msg["usage"].(map[string]interface{}); ok {
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
				toolID := firstNonEmpty(irEvent.CallID, stringValue(block["id"]))
				toolName := firstNonEmpty(stringValue(asMap(irEvent.ProviderExtensions)["name"]), stringValue(block["name"]))
				initialArguments := anthropicNonEmptyToolArguments(irEvent.Arguments, block["input"])
				state.ToolArguments[index] = initialArguments
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
									"arguments": initialArguments,
								},
							}},
						},
					}},
				})
			case "thinking":
				initialReasoning := firstNonEmpty(irEvent.Text, stringValue(block["thinking"]))
				if initialReasoning != "" {
					state.ReasoningText[index] += initialReasoning
					chunks = append(chunks, map[string]interface{}{
						"id":      state.MessageID,
						"object":  "chat.completion.chunk",
						"created": 0,
						"model":   state.Model,
						"choices": []map[string]interface{}{{
							"index": 0,
							"delta": map[string]interface{}{
								"reasoning_content": initialReasoning,
							},
						}},
					})
				}
			case "text":
				for _, event := range irEvents {
					switch event.Type {
					case "output_text.delta":
						chunks = append(chunks, map[string]interface{}{
							"id":      state.MessageID,
							"object":  "chat.completion.chunk",
							"created": 0,
							"model":   state.Model,
							"choices": []map[string]interface{}{{
								"index": 0,
								"delta": map[string]interface{}{
									"content": firstNonEmpty(event.Text, event.Delta),
								},
							}},
						})
					case "annotation.added":
						var annotations []interface{}
						_ = json.Unmarshal(event.Annotations, &annotations)
						if len(annotations) > 0 {
							chunks = append(chunks, map[string]interface{}{
								"id":      state.MessageID,
								"object":  "chat.completion.chunk",
								"created": 0,
								"model":   state.Model,
								"choices": []map[string]interface{}{{
									"index": 0,
									"delta": map[string]interface{}{
										"annotations": annotations,
									},
								}},
							})
						}
					case "refusal.delta":
						chunks = append(chunks, map[string]interface{}{
							"id":      state.MessageID,
							"object":  "chat.completion.chunk",
							"created": 0,
							"model":   state.Model,
							"choices": []map[string]interface{}{{
								"index": 0,
								"delta": map[string]interface{}{
									"refusal": firstNonEmpty(event.Refusal, event.Delta),
								},
							}},
						})
					case "audio.delta":
						audio := decodeIRAudio(event.Audio, block["audio"])
						chunks = append(chunks, map[string]interface{}{
							"id":      state.MessageID,
							"object":  "chat.completion.chunk",
							"created": 0,
							"model":   state.Model,
							"choices": []map[string]interface{}{{
								"index": 0,
								"delta": map[string]interface{}{
									"audio": audio,
								},
							}},
						})
					}
				}
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
						"content": firstNonEmpty(irEvent.Text, irEvent.Delta),
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
						"reasoning_content": firstNonEmpty(irEvent.Text, irEvent.Delta),
					},
				}},
			})
		case "input_json_delta":
			tool := state.ToolIndexes[index]
			state.ToolArguments[index] += firstNonEmpty(irEvent.Arguments, irEvent.Delta)
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
								"arguments": firstNonEmpty(irEvent.Arguments, irEvent.Delta),
							},
						}},
					},
				}},
			})
		}
	case "message_delta":
		delta, _ := payload["delta"].(map[string]interface{})
		state.LastStopReason = firstNonEmpty(irEvent.FinishReason, anthropicStopReasonToChatFinish(stringValue(delta["stop_reason"])))
		if usage := decodeIRUsage(irEvent.Usage); len(usage) > 0 {
			state.LastUsage = anthropicUsageToChatUsage(usage)
		} else if usage, ok := payload["usage"].(map[string]interface{}); ok {
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
		state.FinishSent = true
	case "message_stop":
		if !state.FinishSent && state.MessageID != "" {
			finishReason := firstNonEmpty(state.LastStopReason, defaultAnthropicChatFinishReason(state))
			chunk := map[string]interface{}{
				"id":      state.MessageID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index":         0,
					"delta":         map[string]interface{}{},
					"finish_reason": finishReason,
				}},
			}
			if len(state.LastUsage) > 0 {
				chunk["usage"] = state.LastUsage
			}
			chunks = append(chunks, chunk)
			state.FinishSent = true
		}
		return chunks, true
	}

	return chunks, false
}

func defaultAnthropicChatFinishReason(state *AnthropicInboundState) string {
	if state == nil {
		return "stop"
	}
	if len(state.ToolIndexes) > 0 {
		return "tool_calls"
	}
	return "stop"
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
	irEvents := IRStreamEventsFromAnthropicFrame(frame)
	var irEvent IRStreamEvent
	if len(irEvents) > 0 {
		irEvent = irEvents[0]
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
			state.MessageID = firstNonEmpty(irEvent.ItemID, stringValue(msg["id"]))
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
				toolID := firstNonEmpty(irEvent.CallID, stringValue(block["id"]))
				toolName := firstNonEmpty(stringValue(asMap(irEvent.ProviderExtensions)["name"]), stringValue(block["name"]))
				initialArguments := anthropicNonEmptyToolArguments(irEvent.Arguments, block["input"])
				state.ToolArguments[index] = initialArguments
				state.ToolIndexes[index] = anthropicInboundToolState{ID: toolID, Name: toolName}
				item := map[string]interface{}{
					"type":    "function_call",
					"id":      toolID,
					"call_id": toolID,
					"name":    toolName,
					"status":  "in_progress",
				}
				if initialArguments != "" {
					item["arguments"] = initialArguments
				}
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": nextSeq(),
						"output_index":    index + 100,
						"item":            item,
					},
				})
			case "image", "document":
				part := anthropicContentBlockToResponsesPart(block)
				if len(irEvent.Item) > 0 {
					if rawItem := asMap(jsonRawToAny(irEvent.Item)); len(rawItem) > 0 {
						part = anthropicContentBlockToResponsesPart(rawItem)
					}
				}
				events = append(events, ResponsesStreamEvent{
					Event: "response.content_part.added",
					Data: map[string]interface{}{
						"type":            "response.content_part.added",
						"sequence_number": nextSeq(),
						"output_index":    0,
						"content_index":   index,
						"part":            part,
					},
				})
			case "thinking":
				itemID := state.MessageID + "_reasoning_" + strconv.Itoa(index)
				state.ReasoningItems[index] = itemID
				initialReasoning := firstNonEmpty(irEvent.Text, stringValue(block["thinking"]))
				state.ReasoningText[index] = initialReasoning
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": nextSeq(),
						"output_index":    index + 50,
						"item": map[string]interface{}{
							"id":     itemID,
							"type":   "reasoning",
							"status": "in_progress",
							"summary": []map[string]interface{}{{
								"type": "summary_text",
								"text": initialReasoning,
							}},
						},
					},
				})
				if initialReasoning != "" {
					events = append(events, ResponsesStreamEvent{
						Event: "response.reasoning.delta",
						Data: map[string]interface{}{
							"type":            "response.reasoning.delta",
							"sequence_number": nextSeq(),
							"item_id":         itemID,
							"output_index":    index + 50,
							"delta":           initialReasoning,
						},
					})
				}
			case "text":
				contentIndex := index
				events = append(events, ResponsesStreamEvent{
					Event: "response.content_part.added",
					Data: map[string]interface{}{
						"type":            "response.content_part.added",
						"sequence_number": nextSeq(),
						"output_index":    0,
						"content_index":   contentIndex,
						"part": map[string]interface{}{
							"type":        "output_text",
							"text":        "",
							"annotations": []interface{}{},
						},
					},
				})
				for _, event := range irEvents {
					switch event.Type {
					case "output_text.delta":
						events = append(events, ResponsesStreamEvent{
							Event: "response.output_text.delta",
							Data: map[string]interface{}{
								"type":            "response.output_text.delta",
								"sequence_number": nextSeq(),
								"output_index":    0,
								"content_index":   contentIndex,
								"delta":           firstNonEmpty(event.Text, event.Delta),
							},
						})
					case "annotation.added":
						var annotations []interface{}
						_ = json.Unmarshal(event.Annotations, &annotations)
						if len(annotations) > 0 {
							events = append(events, ResponsesStreamEvent{
								Event: "response.annotation.added",
								Data: map[string]interface{}{
									"type":            "response.annotation.added",
									"sequence_number": nextSeq(),
									"output_index":    0,
									"annotations":     annotations,
								},
							})
						}
					case "refusal.delta":
						events = append(events, ResponsesStreamEvent{
							Event: "response.refusal.done",
							Data: map[string]interface{}{
								"type":            "response.refusal.done",
								"sequence_number": nextSeq(),
								"output_index":    0,
								"content_index":   contentIndex + 1,
								"refusal":         firstNonEmpty(event.Refusal, event.Delta),
							},
						})
					case "audio.delta":
						audio := decodeIRAudio(event.Audio, block["audio"])
						events = append(events, ResponsesStreamEvent{
							Event: "response.audio.done",
							Data: map[string]interface{}{
								"type":            "response.audio.done",
								"sequence_number": nextSeq(),
								"output_index":    0,
								"content_index":   contentIndex + 2,
								"audio":           audio,
							},
						})
					}
				}
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
					"delta":           firstNonEmpty(irEvent.Text, irEvent.Delta),
				},
			})
		case "input_json_delta":
			state.ToolArguments[index] += firstNonEmpty(irEvent.Arguments, irEvent.Delta)
			events = append(events, ResponsesStreamEvent{
				Event: "response.function_call_arguments.delta",
				Data: map[string]interface{}{
					"type":            "response.function_call_arguments.delta",
					"sequence_number": nextSeq(),
					"output_index":    index + 100,
					"delta":           firstNonEmpty(irEvent.Arguments, irEvent.Delta),
				},
			})
		case "thinking_delta":
			reasoning := firstNonEmpty(irEvent.Text, irEvent.Delta)
			state.ReasoningText[index] += reasoning
			events = append(events, ResponsesStreamEvent{
				Event: "response.reasoning.delta",
				Data: map[string]interface{}{
					"type":            "response.reasoning.delta",
					"sequence_number": nextSeq(),
					"item_id":         state.ReasoningItems[index],
					"output_index":    index + 50,
					"delta":           reasoning,
				},
			})
		}
	case "message_delta":
		delta, _ := payload["delta"].(map[string]interface{})
		state.LastStopReason = firstNonEmpty(irEvent.FinishReason, anthropicStopReasonToChatFinish(stringValue(delta["stop_reason"])))
		usage := decodeIRUsage(irEvent.Usage)
		if len(usage) == 0 {
			usage, _ = payload["usage"].(map[string]interface{})
		}
		if len(usage) > 0 {
			state.LastUsage = anthropicUsageToChatUsage(usage)
		}
		events = append(events, flushResponsesFromAnthropicStop(state, nextSeq)...)
		state.FinishSent = true
	case "message_stop":
		if !state.FinishSent && state.MessageID != "" {
			events = append(events, flushResponsesFromAnthropicStop(state, nextSeq)...)
			state.FinishSent = true
		}
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

func flushResponsesFromAnthropicStop(state *AnthropicInboundState, nextSeq func() int64) []ResponsesStreamEvent {
	if state == nil || state.MessageID == "" {
		return nil
	}
	events := make([]ResponsesStreamEvent, 0, len(state.ToolIndexes)+len(state.ReasoningItems)+2)

	toolIndexes := make([]int, 0, len(state.ToolIndexes))
	for index := range state.ToolIndexes {
		toolIndexes = append(toolIndexes, index)
	}
	sort.Ints(toolIndexes)
	for _, index := range toolIndexes {
		tool := state.ToolIndexes[index]
		if arguments := state.ToolArguments[index]; arguments != "" {
			events = append(events, ResponsesStreamEvent{
				Event: "response.function_call_arguments.done",
				Data: map[string]interface{}{
					"type":            "response.function_call_arguments.done",
					"sequence_number": nextSeq(),
					"output_index":    index + 100,
					"arguments":       arguments,
				},
			})
		}
		item := map[string]interface{}{
			"type":    "function_call",
			"id":      tool.ID,
			"call_id": tool.ID,
			"name":    tool.Name,
			"status":  "completed",
		}
		if arguments := state.ToolArguments[index]; arguments != "" {
			item["arguments"] = arguments
		}
		events = append(events, ResponsesStreamEvent{
			Event: "response.output_item.done",
			Data: map[string]interface{}{
				"type":            "response.output_item.done",
				"sequence_number": nextSeq(),
				"output_index":    index + 100,
				"item":            item,
			},
		})
	}
	state.ToolIndexes = make(map[int]anthropicInboundToolState)
	state.ToolArguments = make(map[int]string)

	reasoningIndexes := make([]int, 0, len(state.ReasoningItems))
	for index := range state.ReasoningItems {
		reasoningIndexes = append(reasoningIndexes, index)
	}
	sort.Ints(reasoningIndexes)
	for _, index := range reasoningIndexes {
		itemID := state.ReasoningItems[index]
		events = append(events, ResponsesStreamEvent{
			Event: "response.output_item.done",
			Data: map[string]interface{}{
				"type":            "response.output_item.done",
				"sequence_number": nextSeq(),
				"output_index":    index + 50,
				"item": map[string]interface{}{
					"id":     itemID,
					"type":   "reasoning",
					"status": "completed",
					"summary": []map[string]interface{}{{
						"type": "summary_text",
						"text": state.ReasoningText[index],
					}},
				},
			},
		})
	}
	state.ReasoningItems = make(map[int]string)
	state.ReasoningText = make(map[int]string)

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

	return events
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

func anthropicInitialToolArguments(input interface{}) string {
	switch v := input.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return ""
		}
	case nil:
		return ""
	}
	return mustMarshalToString(input)
}

func anthropicNonEmptyToolArguments(irArguments string, input interface{}) string {
	if strings.TrimSpace(irArguments) != "" && strings.TrimSpace(irArguments) != "{}" {
		return irArguments
	}
	return anthropicInitialToolArguments(input)
}

func jsonRawToAny(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}
