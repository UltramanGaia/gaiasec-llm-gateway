package protocol

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type SSEFrame struct {
	Event string
	Data  string
}

type ResponsesStreamEvent struct {
	Event string
	Data  interface{}
}

type ChatToResponsesStreamState struct {
	ResponseCreated bool
	MessageItems    map[int]string
	MessageText     map[int]string
	ToolCalls       map[int]responsesToolState
	ToolArgs        map[int]string
}

func NewChatToResponsesStreamState() *ChatToResponsesStreamState {
	return &ChatToResponsesStreamState{
		MessageItems: make(map[int]string),
		MessageText:  make(map[int]string),
		ToolCalls:    make(map[int]responsesToolState),
		ToolArgs:     make(map[int]string),
	}
}

func ReadSSEFrame(reader *bufio.Reader) (SSEFrame, error) {
	var frame SSEFrame
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && (frame.Event != "" || frame.Data != "") {
				return frame, nil
			}
			return frame, err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			if frame.Event != "" || frame.Data != "" {
				return frame, nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "event:") {
			frame.Event = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}
		if strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if frame.Data == "" {
				frame.Data = data
			} else {
				frame.Data += "\n" + data
			}
		}
	}
}

func FormatResponsesStreamEvent(event ResponsesStreamEvent) string {
	data, _ := json.Marshal(event.Data)
	return "event: " + event.Event + "\ndata: " + string(data) + "\n\n"
}

func FormatSSEFrame(frame SSEFrame) string {
	if frame.Event == "" {
		return "data: " + frame.Data + "\n\n"
	}
	return "event: " + frame.Event + "\ndata: " + frame.Data + "\n\n"
}

func ConvertChatChunkToResponsesEvents(chatChunk map[string]interface{}, seqNum int64) []ResponsesStreamEvent {
	return ConvertChatChunkToResponsesEventsStateful(chatChunk, seqNum, nil)
}

func ConvertChatChunkToResponsesEventsStateful(chatChunk map[string]interface{}, seqNum int64, state *ChatToResponsesStreamState) []ResponsesStreamEvent {
	if state == nil {
		state = NewChatToResponsesStreamState()
	}
	events := make([]ResponsesStreamEvent, 0)
	responseID := stringValue(chatChunk["id"])
	model := stringValue(chatChunk["model"])

	if !state.ResponseCreated && responseID != "" {
		state.ResponseCreated = true
		events = append(events, ResponsesStreamEvent{
			Event: "response.created",
			Data: map[string]interface{}{
				"type":            "response.created",
				"sequence_number": seqNum,
				"response": map[string]interface{}{
					"id":         responseID,
					"object":     "response",
					"status":     "in_progress",
					"model":      model,
					"output":     []interface{}{},
					"created_at": 0,
				},
			},
		})
		events = append(events, ResponsesStreamEvent{
			Event: "response.in_progress",
			Data: map[string]interface{}{
				"type":            "response.in_progress",
				"sequence_number": seqNum,
				"response": map[string]interface{}{
					"id":         responseID,
					"object":     "response",
					"status":     "in_progress",
					"model":      model,
					"output":     []interface{}{},
					"created_at": 0,
				},
			},
		})
	}

	if choices, ok := chatChunk["choices"].([]interface{}); ok {
		for _, ch := range choices {
			choice, ok := ch.(map[string]interface{})
			if !ok {
				continue
			}
			idx := 0
			if i, ok := choice["index"].(float64); ok {
				idx = int(i)
			}

			delta, ok := choice["delta"].(map[string]interface{})
			if !ok {
				continue
			}

			itemID := state.MessageItems[idx]
			if itemID == "" {
				itemID = firstNonEmpty(responseID, "resp") + "_msg_" + strings.ReplaceAll(strings.TrimSpace(mustMarshalToString(idx)), "\"", "")
				state.MessageItems[idx] = itemID
			}

			if role, ok := delta["role"].(string); ok && role == "assistant" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"id":      itemID,
							"type":   "message",
							"status": "in_progress",
							"role":   "assistant",
							"content": []interface{}{},
						},
					},
				})
			}

			if content, ok := delta["content"].(string); ok && content != "" {
				if _, exists := state.MessageText[idx]; !exists {
					state.MessageText[idx] = ""
					events = append(events, ResponsesStreamEvent{
						Event: "response.content_part.added",
						Data: map[string]interface{}{
							"type":            "response.content_part.added",
							"sequence_number": seqNum,
							"item_id":         itemID,
							"output_index":    idx,
							"content_index":   0,
							"part": map[string]interface{}{
								"type":        "output_text",
								"text":        "",
								"annotations": []interface{}{},
								"logprobs":    []interface{}{},
							},
						},
					})
				}
				state.MessageText[idx] += content
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_text.delta",
					Data: map[string]interface{}{
						"type":            "response.output_text.delta",
						"sequence_number": seqNum,
						"item_id":         itemID,
						"output_index":    idx,
						"content_index":   0,
						"delta":           content,
						"logprobs":        []interface{}{},
					},
				})
			}

			if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
				for _, tc := range toolCalls {
					toolCall, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					tcID, _ := toolCall["id"].(string)
					fn, _ := toolCall["function"].(map[string]interface{})
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					tcIndex := idx + 100

					if tcID != "" && name != "" {
						state.ToolCalls[tcIndex] = responsesToolState{ID: tcID, Name: name}
						events = append(events, ResponsesStreamEvent{
							Event: "response.output_item.added",
							Data: map[string]interface{}{
								"type":            "response.output_item.added",
								"sequence_number": seqNum,
								"output_index":    tcIndex,
								"item": map[string]interface{}{
									"type":    "function_call",
									"id":      tcID,
									"call_id": tcID,
									"name":    name,
									"status":  "in_progress",
								},
							},
						})
					}

					if args != "" {
						state.ToolArgs[tcIndex] += args
						events = append(events, ResponsesStreamEvent{
							Event: "response.function_call_arguments.delta",
							Data: map[string]interface{}{
								"type":            "response.function_call_arguments.delta",
								"sequence_number": seqNum,
								"item_id":         tcID,
								"output_index":    tcIndex,
								"delta":           args,
							},
						})
					}
				}
			}

			if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
				if text, ok := state.MessageText[idx]; ok {
					events = append(events, ResponsesStreamEvent{
						Event: "response.output_text.done",
						Data: map[string]interface{}{
							"type":            "response.output_text.done",
							"sequence_number": seqNum,
							"item_id":         itemID,
							"output_index":    idx,
							"content_index":   0,
							"text":            text,
							"logprobs":        []interface{}{},
						},
					})
					events = append(events, ResponsesStreamEvent{
						Event: "response.content_part.done",
						Data: map[string]interface{}{
							"type":            "response.content_part.done",
							"sequence_number": seqNum,
							"item_id":         itemID,
							"output_index":    idx,
							"content_index":   0,
							"part": map[string]interface{}{
								"type":        "output_text",
								"text":        text,
								"annotations": []interface{}{},
								"logprobs":    []interface{}{},
							},
						},
					})
				}
				if finishReason == "tool_calls" {
					for outputIndex, tool := range state.ToolCalls {
						events = append(events, ResponsesStreamEvent{
							Event: "response.function_call_arguments.done",
							Data: map[string]interface{}{
								"type":            "response.function_call_arguments.done",
								"sequence_number": seqNum,
								"item_id":         tool.ID,
								"output_index":    outputIndex,
								"arguments":       state.ToolArgs[outputIndex],
							},
						})
						events = append(events, ResponsesStreamEvent{
							Event: "response.output_item.done",
							Data: map[string]interface{}{
								"type":            "response.output_item.done",
								"sequence_number": seqNum,
								"output_index":    outputIndex,
								"item": map[string]interface{}{
									"id":      tool.ID,
									"type":    "function_call",
									"call_id": tool.ID,
									"name":    tool.Name,
									"arguments": state.ToolArgs[outputIndex],
									"status":  "completed",
								},
							},
						})
					}
				}
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.done",
					Data: map[string]interface{}{
						"type":            "response.output_item.done",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"id":      itemID,
							"type":   "message",
							"status": "completed",
							"role":   "assistant",
							"content": []map[string]interface{}{{
								"type":        "output_text",
								"text":        state.MessageText[idx],
								"annotations": []interface{}{},
								"logprobs":    []interface{}{},
							}},
						},
					},
				})
			}
		}
	}

	return events
}

func BuildResponsesCompletedEvent(id, model string, usage map[string]interface{}, outputText string, seqNum int64) ResponsesStreamEvent {
	inputTokens := numberToIntDefault(usage["prompt_tokens"])
	outputTokens := numberToIntDefault(usage["completion_tokens"])
	response := map[string]interface{}{
		"id":     id,
		"object": "response",
		"status": "completed",
		"model":  model,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
	if strings.TrimSpace(outputText) != "" {
		response["output_text"] = outputText
		response["output"] = []map[string]interface{}{{
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]interface{}{{
				"type": "output_text",
				"text": outputText,
			}},
		}}
	}
	return ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response":        response,
		},
	}
}

func BuildResponsesDoneEvent(id, model, outputText string, seqNum int64) ResponsesStreamEvent {
	response := map[string]interface{}{
		"id":     id,
		"object": "response",
		"status": "completed",
		"model":  model,
	}
	if strings.TrimSpace(outputText) != "" {
		response["output_text"] = outputText
		response["output"] = []map[string]interface{}{{
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]interface{}{{
				"type": "output_text",
				"text": outputText,
			}},
		}}
	}
	return ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response":        response,
		},
	}
}

func BuildResponsesTerminalDoneEvent(seqNum int64) ResponsesStreamEvent {
	return ResponsesStreamEvent{
		Event: "response.done",
		Data: map[string]interface{}{
			"type":            "response.done",
			"sequence_number": seqNum,
		},
	}
}
