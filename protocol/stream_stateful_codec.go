package protocol

import "encoding/json"

type responsesToolState struct {
	ID   string
	Name string
}

type ResponsesStreamState struct {
	ResponseID      string
	Model           string
	RoleSent        bool
	Tools           map[int]responsesToolState
	SawToolCall     bool
	MessageDone     bool
	PendingUsage    map[string]interface{}
	PendingResponse map[string]interface{}
}

func NewResponsesStreamState() *ResponsesStreamState {
	return &ResponsesStreamState{Tools: make(map[int]responsesToolState)}
}

type AnthropicOutboundState struct {
	MessageID        string
	Model            string
	MessageStarted   bool
	TextStarted      bool
	ReasoningStarted bool
	RicherBlockBase   int
	AnnotationSent    bool
	RefusalSent       bool
	AudioSent         bool
	ToolBlocks       map[int]responsesToolState
	PendingFinish    string
	PendingUsage     map[string]interface{}
}

func NewAnthropicOutboundState() *AnthropicOutboundState {
	return &AnthropicOutboundState{ToolBlocks: make(map[int]responsesToolState), RicherBlockBase: 1000}
}

func ChatChunksFromResponsesFrame(frame SSEFrame, state *ResponsesStreamState) ([]map[string]interface{}, bool) {
	if state == nil {
		state = NewResponsesStreamState()
	}
	if frame.Data == "[DONE]" {
		return nil, true
	}
	if frame.Data == "" {
		return nil, false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil, false
	}
	var irEvent IRStreamEvent
	if events := IRStreamEventsFromResponsesFrame(frame); len(events) > 0 {
		irEvent = events[0]
	}
	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}
	chunks := make([]map[string]interface{}, 0, 2)
	switch eventType {
	case "response.created":
		response, _ := payload["response"].(map[string]interface{})
		state.ResponseID = firstNonEmpty(stringValue(response["id"]), stringValue(payload["response_id"]))
		state.Model = firstNonEmpty(stringValue(response["model"]), state.Model)
	case "response.output_item.added":
		item, _ := payload["item"].(map[string]interface{})
		if state.ResponseID == "" {
			state.ResponseID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		if state.Model == "" {
			state.Model = firstNonEmpty(stringValue(payload["model"]), state.Model)
		}
		switch stringValue(item["type"]) {
		case "message":
			if !state.RoleSent {
				chunks = append(chunks, map[string]interface{}{
					"id":      state.ResponseID,
					"object":  "chat.completion.chunk",
					"created": 0,
					"model":   state.Model,
					"choices": []map[string]interface{}{{
						"index": 0,
						"delta": map[string]interface{}{"role": "assistant"},
					}},
				})
				state.RoleSent = true
			}
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			outputIndex := numberToIntDefault(payload["output_index"])
			toolID := firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"]))
			toolName := firstNonEmpty(stringValue(item["name"]), stringValue(item["type"]))
			state.Tools[outputIndex] = responsesToolState{ID: toolID, Name: toolName}
			state.SawToolCall = true
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{
						"tool_calls": []map[string]interface{}{{
							"index": outputIndex,
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
	case "response.output_text.delta":
		if !state.RoleSent {
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{"role": "assistant"},
				}},
			})
			state.RoleSent = true
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{"content": firstNonEmpty(irEvent.Text, irEvent.Delta)},
			}},
		})
	case "response.reasoning.delta":
		if !state.RoleSent {
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{"role": "assistant"},
				}},
			})
			state.RoleSent = true
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{"reasoning_content": firstNonEmpty(irEvent.Text, irEvent.Delta)},
			}},
		})
	case "response.content_part.done":
		part, _ := payload["part"].(map[string]interface{})
		if stringValue(part["type"]) == "output_text" {
			annotations := decodeIRAnnotations(irEvent.Annotations)
			if len(annotations) == 0 {
				annotations, _ = part["annotations"].([]interface{})
			}
			if len(annotations) > 0 {
				if !state.RoleSent {
					chunks = append(chunks, map[string]interface{}{
						"id":      state.ResponseID,
						"object":  "chat.completion.chunk",
						"created": 0,
						"model":   state.Model,
						"choices": []map[string]interface{}{{
							"index": 0,
							"delta": map[string]interface{}{"role": "assistant"},
						}},
					})
					state.RoleSent = true
				}
				chunks = append(chunks, map[string]interface{}{
					"id":      state.ResponseID,
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
		}
	case "response.annotation.added":
		annotations := decodeIRAnnotations(irEvent.Annotations)
		if len(annotations) == 0 {
			annotations, _ = payload["annotations"].([]interface{})
		}
		if len(annotations) > 0 {
			if !state.RoleSent {
				chunks = append(chunks, map[string]interface{}{
					"id":      state.ResponseID,
					"object":  "chat.completion.chunk",
					"created": 0,
					"model":   state.Model,
					"choices": []map[string]interface{}{{
						"index": 0,
						"delta": map[string]interface{}{"role": "assistant"},
					}},
				})
				state.RoleSent = true
			}
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
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
	case "response.refusal.delta":
		if !state.RoleSent {
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{"role": "assistant"},
				}},
			})
			state.RoleSent = true
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{"refusal": firstNonEmpty(irEvent.Refusal, irEvent.Delta)},
			}},
		})
	case "response.audio.delta":
		if !state.RoleSent {
			chunks = append(chunks, map[string]interface{}{
				"id":      state.ResponseID,
				"object":  "chat.completion.chunk",
				"created": 0,
				"model":   state.Model,
				"choices": []map[string]interface{}{{
					"index": 0,
					"delta": map[string]interface{}{"role": "assistant"},
				}},
			})
			state.RoleSent = true
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{"audio": decodeIRAudio(irEvent.Audio, payload["audio"])},
			}},
		})
	case "response.function_call_arguments.delta":
		outputIndex := numberToIntDefault(payload["output_index"])
		tool := state.Tools[outputIndex]
		state.SawToolCall = true
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]interface{}{
					"tool_calls": []map[string]interface{}{{
						"index": outputIndex,
						"id":    tool.ID,
						"type":  "function",
						"function": map[string]interface{}{
							"name":      tool.Name,
							"arguments": stringValue(payload["delta"]),
						},
					}},
				},
			}},
		})
	case "response.function_call_arguments.done":
		state.SawToolCall = true
	case "response.output_item.done":
		item, _ := payload["item"].(map[string]interface{})
		if stringValue(item["type"]) == "message" || stringValue(item["type"]) == "function_call" {
			state.MessageDone = true
		}
	case "response.completed":
		response, _ := payload["response"].(map[string]interface{})
		state.ResponseID = firstNonEmpty(irEvent.ItemID, stringValue(response["id"]))
		state.Model = stringValue(response["model"])
		usage := decodeIRUsage(irEvent.Usage)
		if len(usage) == 0 {
			usage, _ = response["usage"].(map[string]interface{})
		}
		finishReason := "stop"
		if state.SawToolCall {
			finishReason = "tool_calls"
		}
		chunks = append(chunks, map[string]interface{}{
			"id":      state.ResponseID,
			"object":  "chat.completion.chunk",
			"created": 0,
			"model":   state.Model,
			"choices": []map[string]interface{}{{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": finishReason,
			}},
			"usage": map[string]interface{}{
				"prompt_tokens":     numberToIntDefault(usage["input_tokens"]),
				"completion_tokens": numberToIntDefault(usage["output_tokens"]),
				"total_tokens":      numberToIntDefault(usage["total_tokens"]),
			},
		})
	}
	return chunks, false
}

func decodeIRAnnotations(raw json.RawMessage) []interface{} {
	if len(raw) == 0 {
		return nil
	}
	var annotations []interface{}
	if err := json.Unmarshal(raw, &annotations); err != nil {
		return nil
	}
	return annotations
}

func decodeIRAudio(raw json.RawMessage, fallback interface{}) interface{} {
	if len(raw) == 0 {
		return fallback
	}
	var audio interface{}
	if err := json.Unmarshal(raw, &audio); err != nil {
		return fallback
	}
	return audio
}

func decodeIRUsage(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var usage map[string]interface{}
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil
	}
	return usage
}

func AnthropicEventsFromResponsesFrame(frame SSEFrame, state *AnthropicOutboundState) []SSEFrame {
	if state == nil {
		state = NewAnthropicOutboundState()
	}
	if frame.Data == "" {
		return nil
	}
	if frame.Data == "[DONE]" {
		return flushAnthropicFinish(state, true)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
		return nil
	}
	var irEvent IRStreamEvent
	if events := IRStreamEventsFromResponsesFrame(frame); len(events) > 0 {
		irEvent = events[0]
	}
	eventType := frame.Event
	if eventType == "" {
		eventType = stringValue(payload["type"])
	}
	frames := make([]SSEFrame, 0, 3)
	ensureStart := func() {
		if state.MessageStarted {
			return
		}
		frames = append(frames, anthropicFrame("message_start", map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":    state.MessageID,
				"type":  "message",
				"role":  "assistant",
				"model": state.Model,
				"usage": map[string]interface{}{},
			},
		}))
		state.MessageStarted = true
	}

	switch eventType {
	case "response.created":
		response, _ := payload["response"].(map[string]interface{})
		state.MessageID = firstNonEmpty(stringValue(response["id"]), stringValue(payload["response_id"]))
		state.Model = firstNonEmpty(stringValue(response["model"]), state.Model)
	case "response.output_item.added":
		item, _ := payload["item"].(map[string]interface{})
		outputIndex := numberToIntDefault(payload["output_index"])
		if state.MessageID == "" {
			state.MessageID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		if state.Model == "" {
			state.Model = firstNonEmpty(stringValue(payload["model"]), state.Model)
		}
		ensureStart()
		switch stringValue(item["type"]) {
		case "message":
			if !state.TextStarted {
				frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": 0,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}))
				state.TextStarted = true
			}
			if content, ok := item["content"].([]interface{}); ok {
				for idx, rawPart := range content {
					part, ok := rawPart.(map[string]interface{})
					if !ok {
						continue
					}
					switch stringValue(part["type"]) {
					case "output_image":
						frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
							"type":  "content_block_start",
							"index": idx + 10,
							"content_block": map[string]interface{}{
								"type":   "image",
								"source": part["source"],
							},
						}))
					case "output_file", "file":
						frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
							"type":  "content_block_start",
							"index": idx + 10,
							"content_block": map[string]interface{}{
								"type":   "document",
								"source": part["source"],
							},
						}))
					}
				}
			}
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			toolID := firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"]))
			toolName := firstNonEmpty(stringValue(item["name"]), stringValue(item["type"]))
			state.ToolBlocks[outputIndex] = responsesToolState{ID: toolID, Name: toolName}
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": outputIndex + 2,
				"content_block": map[string]interface{}{
					"type":  "tool_use",
					"id":    toolID,
					"name":  toolName,
					"input": map[string]interface{}{},
				},
			}))
		}
	case "response.output_text.delta":
		if state.MessageID == "" {
			state.MessageID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		ensureStart()
		if !state.TextStarted {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]interface{}{
					"type": "text",
					"text": "",
				},
			}))
			state.TextStarted = true
		}
		frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": firstNonEmpty(irEvent.Text, irEvent.Delta),
			},
		}))
	case "response.reasoning.delta":
		if state.MessageID == "" {
			state.MessageID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		ensureStart()
		if !state.ReasoningStarted {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": 1,
				"content_block": map[string]interface{}{
					"type":      "thinking",
					"thinking":  "",
					"signature": "",
				},
			}))
			state.ReasoningStarted = true
		}
		frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]interface{}{
				"type":     "thinking_delta",
				"thinking": firstNonEmpty(irEvent.Text, irEvent.Delta),
			},
		}))
	case "response.refusal.delta":
		if state.MessageID == "" {
			state.MessageID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		ensureStart()
		if !state.RefusalSent {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase + 1,
				"content_block": map[string]interface{}{
					"type":    "text",
					"text":    firstNonEmpty(irEvent.Refusal, irEvent.Delta),
					"refusal": firstNonEmpty(irEvent.Refusal, irEvent.Delta),
				},
			}))
			state.RefusalSent = true
		}
	case "response.annotation.added":
		ensureStart()
		var annotations []interface{}
		if len(irEvent.Annotations) > 0 {
			_ = json.Unmarshal(irEvent.Annotations, &annotations)
		}
		if len(annotations) == 0 {
			annotations, _ = payload["annotations"].([]interface{})
		}
		if len(annotations) > 0 {
			state.AnnotationSent = true
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase,
				"content_block": map[string]interface{}{
					"type":        "text",
					"text":        "",
					"annotations": annotations,
				},
			}))
		}
	case "response.audio.delta":
		ensureStart()
		audio := decodeIRAudio(irEvent.Audio, payload["audio"])
		if audio != nil {
			state.AudioSent = true
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase + 2,
				"content_block": map[string]interface{}{
					"type":  "text",
					"text":  "",
					"audio": audio,
				},
			}))
		}
	case "response.function_call_arguments.delta":
		if state.MessageID == "" {
			state.MessageID = firstNonEmpty(stringValue(payload["response_id"]), stringValue(payload["id"]))
		}
		ensureStart()
		outputIndex := numberToIntDefault(payload["output_index"])
		frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
			"type":  "content_block_delta",
			"index": outputIndex + 2,
			"delta": map[string]interface{}{
				"type":         "input_json_delta",
				"partial_json": stringValue(payload["delta"]),
			},
		}))
	case "response.output_item.done":
		item, _ := payload["item"].(map[string]interface{})
		switch stringValue(item["type"]) {
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			state.PendingFinish = "tool_calls"
		}
	case "response.function_call_arguments.done":
		state.PendingFinish = "tool_calls"
	case "response.completed":
		response, _ := payload["response"].(map[string]interface{})
		state.MessageID = stringValue(response["id"])
		state.Model = stringValue(response["model"])
		usage, _ := response["usage"].(map[string]interface{})
		if state.PendingFinish == "" && len(state.ToolBlocks) > 0 {
			state.PendingFinish = "tool_calls"
		}
		if state.PendingFinish == "" {
			state.PendingFinish = "stop"
		}
		state.PendingUsage = map[string]interface{}{
			"prompt_tokens":     numberToIntDefault(usage["input_tokens"]),
			"completion_tokens": numberToIntDefault(usage["output_tokens"]),
		}
		frames = append(frames, flushAnthropicFinish(state, true)...)
	}
	return frames
}

func AnthropicFramesFromChatChunk(chatChunk map[string]interface{}, state *AnthropicOutboundState) []SSEFrame {
	if state == nil {
		state = NewAnthropicOutboundState()
	}
	frames := make([]SSEFrame, 0)
	irEvents := IRStreamEventsFromChatChunk(chatChunk)
	if !state.MessageStarted {
		state.MessageID = stringValue(chatChunk["id"])
		state.Model = stringValue(chatChunk["model"])
		message := map[string]interface{}{
			"id":    state.MessageID,
			"type":  "message",
			"role":  "assistant",
			"model": state.Model,
			"usage": map[string]interface{}{},
		}
		frames = append(frames, anthropicFrame("message_start", map[string]interface{}{
			"type":    "message_start",
			"message": message,
		}))
		state.MessageStarted = true
	}

	choices, _ := chatChunk["choices"].([]interface{})
	for _, raw := range choices {
		choice, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		delta, _ := choice["delta"].(map[string]interface{})
		if reasoning := firstNonEmpty(findIRTextDelta(irEvents, "reasoning.delta", 0), stringValue(delta["reasoning_content"])); reasoning != "" {
			if !state.ReasoningStarted {
				frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": 1,
					"content_block": map[string]interface{}{
						"type":      "thinking",
						"thinking":  "",
						"signature": "",
					},
				}))
				state.ReasoningStarted = true
			}
			frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": 1,
				"delta": map[string]interface{}{
					"type":     "thinking_delta",
					"thinking": reasoning,
				},
			}))
		}
		if content := firstNonEmpty(findIRTextDelta(irEvents, "output_text.delta", 0), stringValue(delta["content"])); content != "" {
			if state.ReasoningStarted {
				frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
					"type":  "content_block_stop",
					"index": 1,
				}))
				state.ReasoningStarted = false
			}
			if !state.TextStarted {
				frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
					"type":  "content_block_start",
					"index": 0,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}))
				state.TextStarted = true
			}
			frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": content,
				},
			}))
		}
		if annotations := findIRAnnotations(irEvents, 0); len(annotations) > 0 {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase,
				"content_block": map[string]interface{}{
					"type":        "text",
					"text":        "",
					"annotations": annotations,
				},
			}))
		}
		if refusal := firstNonEmpty(findIRRefusalDelta(irEvents, 0), stringValue(delta["refusal"])); refusal != "" {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase + 1,
				"content_block": map[string]interface{}{
					"type":    "text",
					"text":    refusal,
					"refusal": refusal,
				},
			}))
		}
		if audio := findIRAudio(irEvents, 0); len(audio) > 0 {
			frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": state.RicherBlockBase + 2,
				"content_block": map[string]interface{}{
					"type":  "text",
					"text":  "",
					"audio": audio,
				},
			}))
		}
		if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
			for _, rawTool := range toolCalls {
				toolCall, ok := rawTool.(map[string]interface{})
				if !ok {
					continue
				}
				index := numberToIntDefault(toolCall["index"])
				fn, _ := toolCall["function"].(map[string]interface{})
				name := stringValue(fn["name"])
				id := stringValue(toolCall["id"])
				if _, exists := state.ToolBlocks[index]; !exists {
					state.ToolBlocks[index] = responsesToolState{ID: id, Name: name}
					frames = append(frames, anthropicFrame("content_block_start", map[string]interface{}{
						"type":  "content_block_start",
						"index": index + 2,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    id,
							"name":  name,
							"input": map[string]interface{}{},
						},
					}))
				}
				if args := stringValue(fn["arguments"]); args != "" {
					frames = append(frames, anthropicFrame("content_block_delta", map[string]interface{}{
						"type":  "content_block_delta",
						"index": index + 2,
						"delta": map[string]interface{}{
							"type":         "input_json_delta",
							"partial_json": firstNonEmpty(findIRToolArguments(irEvents, index), args),
						},
					}))
				}
			}
		}
		if finishReason := stringValue(choice["finish_reason"]); finishReason != "" {
			state.PendingFinish = finishReason
		}
	}

	if usage, ok := chatChunk["usage"].(map[string]interface{}); ok && usage != nil {
		state.PendingUsage = usage
	}
	for _, event := range irEvents {
		if event.Type == "usage" && len(event.Usage) > 0 {
			if decoded := decodeIRUsage(event.Usage); len(decoded) > 0 {
				state.PendingUsage = decoded
			}
		}
		if event.Type == "response.completed" && event.FinishReason != "" {
			state.PendingFinish = event.FinishReason
		}
	}

	return frames
}

func FlushAnthropicFrames(state *AnthropicOutboundState, includeStop bool) []SSEFrame {
	return flushAnthropicFinish(state, includeStop)
}

func flushAnthropicFinish(state *AnthropicOutboundState, includeStop bool) []SSEFrame {
	if state == nil {
		return nil
	}
	frames := make([]SSEFrame, 0, len(state.ToolBlocks)+4)
	if state.TextStarted {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": 0,
		}))
		state.TextStarted = false
	}
	if state.ReasoningStarted {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": 1,
		}))
		state.ReasoningStarted = false
	}
	if state.AnnotationSent {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.RicherBlockBase,
		}))
		state.AnnotationSent = false
	}
	if state.RefusalSent {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.RicherBlockBase + 1,
		}))
		state.RefusalSent = false
	}
	if state.AudioSent {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.RicherBlockBase + 2,
		}))
		state.AudioSent = false
	}
	for index := range state.ToolBlocks {
		frames = append(frames, anthropicFrame("content_block_stop", map[string]interface{}{
			"type":  "content_block_stop",
			"index": index + 2,
		}))
	}
	state.ToolBlocks = make(map[int]responsesToolState)
	if state.PendingFinish != "" || len(state.PendingUsage) > 0 {
		frames = append(frames, anthropicFrame("message_delta", map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": anthropicStopReasonFromChatFinish(state.PendingFinish),
			},
			"usage": map[string]interface{}{
				"input_tokens":  numberToIntDefault(state.PendingUsage["prompt_tokens"]),
				"output_tokens": numberToIntDefault(state.PendingUsage["completion_tokens"]),
			},
		}))
		state.PendingFinish = ""
		state.PendingUsage = nil
	}
	if includeStop {
		frames = append(frames, anthropicFrame("message_stop", map[string]interface{}{
			"type": "message_stop",
		}))
	}
	return frames
}

func anthropicFrame(event string, data map[string]interface{}) SSEFrame {
	body, _ := json.Marshal(data)
	return SSEFrame{Event: event, Data: string(body)}
}

func findIRToolArguments(events []IRStreamEvent, index int) string {
	for _, event := range events {
		if event.Type == "tool_call.delta" && event.Index == index {
			return event.Arguments
		}
	}
	return ""
}
