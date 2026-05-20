package handlers

import (
	"encoding/json"
	"strings"
)

func convertResponsesToChatRequest(responsesReq ResponsesRequest) map[string]interface{} {
	chatReq := map[string]interface{}{
		"model": responsesReq.Model,
	}

	messages := convertInputToMessages(responsesReq.Input, responsesReq.Instructions)
	if len(messages) > 0 {
		chatReq["messages"] = messages
	}

	if responsesReq.Temperature != nil {
		chatReq["temperature"] = *responsesReq.Temperature
	}
	if responsesReq.TopP != nil {
		chatReq["top_p"] = *responsesReq.TopP
	}
	if responsesReq.MaxOutputTokens > 0 {
		chatReq["max_tokens"] = responsesReq.MaxOutputTokens
	}
	if responsesReq.Stream {
		chatReq["stream"] = true
	}

	if len(responsesReq.Tools) > 0 {
		chatReq["tools"] = convertResponsesToolsToChatTools(responsesReq.Tools)
	}
	if len(responsesReq.ToolChoice) > 0 && string(responsesReq.ToolChoice) != "null" {
		chatReq["tool_choice"] = responsesReq.ToolChoice
	}

	return chatReq
}

func convertInputToMessages(input json.RawMessage, instructions string) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0)

	if instructions != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": instructions,
		})
	}

	if len(input) == 0 || string(input) == "null" {
		return messages
	}

	trimmed := strings.TrimSpace(string(input))

	if strings.HasPrefix(trimmed, "\"") {
		var text string
		if err := json.Unmarshal(input, &text); err == nil && text != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": text,
			})
		}
		return messages
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []inputItem
		if err := json.Unmarshal(input, &items); err == nil {
			return convertInputItemsToMessages(items, messages)
		}
	}

	if trimmed != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": trimmed,
		})
	}

	return messages
}

type inputItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Output    json.RawMessage `json:"output"`
	ID        string          `json:"id"`
}

func convertInputItemsToMessages(items []inputItem, messages []map[string]interface{}) []map[string]interface{} {
	var pendingToolCalls []map[string]interface{}

	for _, item := range items {
		role := item.Role
		if role == "" {
			role = "user"
		}

		switch item.Type {
		case "function_call", "custom_tool_call":
			toolCall := map[string]interface{}{
				"id":   firstNonEmpty(item.CallID, item.ID),
				"type": "function",
				"function": map[string]interface{}{
					"name":      item.Name,
					"arguments": item.Arguments,
				},
			}
			pendingToolCalls = append(pendingToolCalls, toolCall)

		case "function_call_output", "custom_tool_call_output":
			if len(pendingToolCalls) > 0 {
				messages = append(messages, map[string]interface{}{
					"role":       "assistant",
					"tool_calls": pendingToolCalls,
				})
				pendingToolCalls = nil
			}
			outputText := extractOutputText(item.Output)
			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": firstNonEmpty(item.CallID, item.ID),
				"content":      outputText,
			})

		case "message":
			if role == "system" || role == "developer" {
				content := extractContentText(item.Content)
				if content != "" {
					messages = append(messages, map[string]interface{}{
						"role":    "system",
						"content": content,
					})
				}
			} else if role == "assistant" {
				content := extractContentText(item.Content)
				if len(pendingToolCalls) > 0 {
					messages = append(messages, map[string]interface{}{
						"role":       "assistant",
						"content":    content,
						"tool_calls": pendingToolCalls,
					})
					pendingToolCalls = nil
				} else if content != "" {
					messages = append(messages, map[string]interface{}{
						"role":    "assistant",
						"content": content,
					})
				}
			} else {
				content := extractContentText(item.Content)
				if content != "" {
					messages = append(messages, map[string]interface{}{
						"role":    role,
						"content": content,
					})
				}
			}

		default:
			content := extractContentText(item.Content)
			if len(pendingToolCalls) > 0 && role == "assistant" {
				messages = append(messages, map[string]interface{}{
					"role":       role,
					"content":    content,
					"tool_calls": pendingToolCalls,
				})
				pendingToolCalls = nil
			} else if content != "" {
				messages = append(messages, map[string]interface{}{
					"role":    role,
					"content": content,
				})
			}
		}
	}

	if len(pendingToolCalls) > 0 {
		messages = append(messages, map[string]interface{}{
			"role":       "assistant",
			"tool_calls": pendingToolCalls,
		})
	}

	return messages
}

func extractContentText(content json.RawMessage) string {
	if len(content) == 0 || string(content) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text
	}
	var parts []map[string]interface{}
	if err := json.Unmarshal(content, &parts); err == nil {
		var result string
		for _, part := range parts {
			if t, ok := part["type"].(string); ok && t == "text" {
				if txt, ok := part["text"].(string); ok {
					result += txt
				}
			}
		}
		return result
	}
	return string(content)
}

func extractOutputText(output json.RawMessage) string {
	if len(output) == 0 || string(output) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(output, &text); err == nil {
		return text
	}
	return string(output)
}

func convertResponsesToolsToChatTools(tools []ResponsesTool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		switch tool.Type {
		case "function":
			result = append(result, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			})
		default:
			result = append(result, map[string]interface{}{
				"type": tool.Type,
				"function": map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  tool.Parameters,
				},
			})
		}
	}
	return result
}

func convertChatResponseToResponses(chatResp map[string]interface{}) ResponsesResponse {
	resp := ResponsesResponse{
		Object: "response",
		Status: "completed",
	}

	if id, ok := chatResp["id"].(string); ok {
		resp.ID = id
	}
	if model, ok := chatResp["model"].(string); ok {
		resp.Model = model
	}

	if choices, ok := chatResp["choices"].([]interface{}); ok && len(choices) > 0 {
		output, outputText := convertChoicesToOutput(choices)
		resp.Output = output
		resp.OutputText = outputText
	}

	if usage, ok := chatResp["usage"].(map[string]interface{}); ok {
		resp.Usage = convertUsage(usage)
	}

	return resp
}

func convertChoicesToOutput(choices []interface{}) ([]ResponsesOutputItem, string) {
	output := make([]ResponsesOutputItem, 0)
	var outputText string

	for _, choice := range choices {
		ch, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		msg, ok := ch["message"].(map[string]interface{})
		if !ok {
			continue
		}

		item := ResponsesOutputItem{
			Type:   "message",
			Status: "completed",
		}

		if role, ok := msg["role"].(string); ok {
			item.Role = role
		}

		content := extractMessageContent(msg)
		if content != "" {
			item.Content = []ResponsesContentPart{
				{Type: "output_text", Text: content},
			}
			outputText += content
		}

		if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
			for _, tc := range toolCalls {
				toolCall, ok := tc.(map[string]interface{})
				if !ok {
					continue
				}
				toolItem := convertToolCallToOutputItem(toolCall)
				output = append(output, toolItem)
			}
		}

		output = append(output, item)
	}

	return output, outputText
}

func extractMessageContent(msg map[string]interface{}) string {
	if content, ok := msg["content"].(string); ok {
		return content
	}
	if parts, ok := msg["content"].([]interface{}); ok {
		var result string
		for _, part := range parts {
			if p, ok := part.(map[string]interface{}); ok {
				if t, ok := p["type"].(string); ok && (t == "text" || t == "output_text") {
					if txt, ok := p["text"].(string); ok {
						result += txt
					}
				}
			}
		}
		return result
	}
	return ""
}

func convertToolCallToOutputItem(toolCall map[string]interface{}) ResponsesOutputItem {
	item := ResponsesOutputItem{
		Type:   "function_call",
		Status: "completed",
	}

	if id, ok := toolCall["id"].(string); ok {
		item.ID = id
		item.CallID = id
	}

	if fn, ok := toolCall["function"].(map[string]interface{}); ok {
		if name, ok := fn["name"].(string); ok {
			item.Name = name
		}
		if args, ok := fn["arguments"].(string); ok {
			item.Arguments = args
		}
	}

	return item
}

func convertUsage(usage map[string]interface{}) ResponsesUsage {
	u := ResponsesUsage{}
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		u.InputTokens = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		u.OutputTokens = int(ct)
	}
	if tt, ok := usage["total_tokens"].(float64); ok {
		u.TotalTokens = int(tt)
	}
	if ptd, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if ct, ok := ptd["cached_tokens"].(float64); ok {
			u.InputTokensDetails.CachedTokens = int(ct)
		}
	}
	return u
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
