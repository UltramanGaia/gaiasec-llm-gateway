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
	events := make([]ResponsesStreamEvent, 0)

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

			if role, ok := delta["role"].(string); ok && role == "assistant" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"type":   "message",
							"status": "in_progress",
							"role":   "assistant",
						},
					},
				})
			}

			if content, ok := delta["content"].(string); ok && content != "" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_text.delta",
					Data: map[string]interface{}{
						"type":            "response.output_text.delta",
						"sequence_number": seqNum,
						"output_index":    idx,
						"content_index":   0,
						"delta":           content,
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

					if tcID != "" && name != "" {
						events = append(events, ResponsesStreamEvent{
							Event: "response.output_item.added",
							Data: map[string]interface{}{
								"type":            "response.output_item.added",
								"sequence_number": seqNum,
								"output_index":    idx + 100,
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
						events = append(events, ResponsesStreamEvent{
							Event: "response.function_call_arguments.delta",
							Data: map[string]interface{}{
								"type":            "response.function_call_arguments.delta",
								"sequence_number": seqNum,
								"output_index":    idx + 100,
								"delta":           args,
							},
						})
					}
				}
			}

			if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.done",
					Data: map[string]interface{}{
						"type":            "response.output_item.done",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"type":   "message",
							"status": "completed",
							"role":   "assistant",
						},
					},
				})
			}
		}
	}

	return events
}

func BuildResponsesCompletedEvent(id, model string, usage map[string]interface{}, seqNum int64) ResponsesStreamEvent {
	inputTokens := numberToIntDefault(usage["prompt_tokens"])
	outputTokens := numberToIntDefault(usage["completion_tokens"])
	return ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response": map[string]interface{}{
				"id":     id,
				"object": "response",
				"status": "completed",
				"model":  model,
				"usage": map[string]interface{}{
					"input_tokens":  inputTokens,
					"output_tokens": outputTokens,
					"total_tokens":  inputTokens + outputTokens,
				},
			},
		},
	}
}

func BuildResponsesDoneEvent(id, model string, seqNum int64) ResponsesStreamEvent {
	return ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response": map[string]interface{}{
				"id":     id,
				"object": "response",
				"status": "completed",
				"model":  model,
			},
		},
	}
}
