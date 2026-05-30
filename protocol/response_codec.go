package protocol

import "encoding/json"

type openAIChatResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object,omitempty"`
	Created int64           `json:"created,omitempty"`
	Model   string          `json:"model,omitempty"`
	Choices json.RawMessage `json:"choices,omitempty"`
	Usage   json.RawMessage `json:"usage,omitempty"`
}

type responsesResponse struct {
	ID         string          `json:"id"`
	Object     string          `json:"object,omitempty"`
	CreatedAt  int64           `json:"created_at,omitempty"`
	Status     string          `json:"status,omitempty"`
	Model      string          `json:"model,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
	OutputText string          `json:"output_text,omitempty"`
	Usage      json.RawMessage `json:"usage,omitempty"`
}

type anthropicResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type,omitempty"`
	Role         string          `json:"role,omitempty"`
	Model        string          `json:"model,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
	StopReason   string          `json:"stop_reason,omitempty"`
	StopSequence json.RawMessage `json:"stop_sequence,omitempty"`
	Usage        json.RawMessage `json:"usage,omitempty"`
}

func DecodeOpenAIChatResponse(body []byte) (IRResponse, error) {
	var resp openAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return IRResponse{}, err
	}
	ir := IRResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: cloneRaw(resp.Usage),
	}
	outputItems, finishReason := decodeOpenAIChatChoices(resp.Choices)
	ir.OutputItems = outputItems
	ir.FinishReason = finishReason
	return ir, nil
}

func EncodeOpenAIChatResponse(ir IRResponse, modelName string) ([]byte, error) {
	resp := openAIChatResponse{
		ID:      ir.ID,
		Object:  "chat.completion",
		Model:   firstNonEmpty(modelName, ir.Model),
		Created: 0,
		Usage:   cloneRaw(ir.Usage),
	}
	choices, err := json.Marshal(encodeOpenAIChatChoices(ir))
	if err != nil {
		return nil, err
	}
	resp.Choices = choices
	return json.Marshal(resp)
}

func DecodeResponsesResponse(body []byte) (IRResponse, error) {
	var resp responsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return IRResponse{}, err
	}
	ir := IRResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: cloneRaw(resp.Usage),
	}
	ir.OutputItems = decodeResponsesOutput(resp.Output)
	ir.FinishReason = inferFinishReasonFromMessages(ir.OutputItems)
	return ir, nil
}

func EncodeResponsesResponse(ir IRResponse, modelName string) ([]byte, error) {
	resp := responsesResponse{
		ID:        ir.ID,
		Object:    "response",
		Status:    "completed",
		Model:     firstNonEmpty(modelName, ir.Model),
		CreatedAt: 0,
		Usage:     cloneRaw(ir.Usage),
	}
	output, outputText, err := encodeResponsesOutput(ir.OutputItems)
	if err != nil {
		return nil, err
	}
	resp.Output = output
	resp.OutputText = outputText
	return json.Marshal(resp)
}

func DecodeAnthropicResponse(body []byte) (IRResponse, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return IRResponse{}, err
	}
	ir := IRResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		Usage:        normalizeAnthropicUsage(resp.Usage),
		FinishReason: anthropicStopReasonToChatFinish(resp.StopReason),
	}
	ir.OutputItems = decodeAnthropicContent(resp.Content)
	return ir, nil
}

func EncodeAnthropicResponse(ir IRResponse, modelName string) ([]byte, error) {
	resp := anthropicResponse{
		ID:         ir.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      firstNonEmpty(modelName, ir.Model),
		StopReason: anthropicStopReasonFromChatFinish(ir.FinishReason),
		Usage:      denormalizeAnthropicUsage(ir.Usage),
	}
	content, err := encodeAnthropicContent(ir.OutputItems)
	if err != nil {
		return nil, err
	}
	resp.Content = content
	return json.Marshal(resp)
}

func decodeOpenAIChatChoices(raw json.RawMessage) ([]IRMessage, string) {
	var choices []map[string]interface{}
	if err := json.Unmarshal(raw, &choices); err != nil {
		return nil, ""
	}
	result := make([]IRMessage, 0, len(choices))
	finishReason := ""
	for _, choice := range choices {
		message, _ := choice["message"].(map[string]interface{})
		if len(message) == 0 {
			continue
		}
		msg := IRMessage{Role: firstNonEmpty(stringValue(message["role"]), "assistant")}
		msg.Content = append(msg.Content, decodeGenericContent(message["content"])...)
		if reasoning := stringValue(message["reasoning_content"]); reasoning != "" {
			msg.Content = append(msg.Content, IRPart{Type: "reasoning", Text: reasoning})
		}
		if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
			for _, rawTC := range toolCalls {
				tc, ok := rawTC.(map[string]interface{})
				if !ok {
					continue
				}
				fn, _ := tc["function"].(map[string]interface{})
				msg.Content = append(msg.Content, IRPart{
					Type:      "tool_call",
					Name:      stringValue(fn["name"]),
					Arguments: stringValue(fn["arguments"]),
					ProviderExtensions: map[string]interface{}{
						"id": stringValue(tc["id"]),
					},
				})
			}
		}
		result = append(result, msg)
		if reason := stringValue(choice["finish_reason"]); reason != "" {
			finishReason = reason
		}
	}
	return result, finishReason
}

func encodeOpenAIChatChoices(ir IRResponse) []map[string]interface{} {
	message := map[string]interface{}{"role": "assistant"}
	if len(ir.OutputItems) > 0 {
		first := ir.OutputItems[0]
		message["role"] = firstNonEmpty(first.Role, "assistant")
		message["content"] = encodeOpenAIContent(first.Content)
		if reasoning := encodeReasoningContent(first.Content); reasoning != "" {
			message["reasoning_content"] = reasoning
		}
		if toolCalls := encodeToolCalls(first.Content); len(toolCalls) > 0 {
			message["tool_calls"] = toolCalls
		}
	}
	return []map[string]interface{}{{
		"index":         0,
		"message":       message,
		"finish_reason": firstNonEmpty(ir.FinishReason, inferFinishReasonFromMessages(ir.OutputItems)),
	}}
}

func decodeResponsesOutput(raw json.RawMessage) []IRMessage {
	var output []map[string]interface{}
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil
	}
	result := make([]IRMessage, 0, len(output))
	for _, item := range output {
		switch stringValue(item["type"]) {
		case "message":
			msg := IRMessage{Role: firstNonEmpty(stringValue(item["role"]), "assistant")}
			if content, ok := item["content"].([]interface{}); ok {
				for _, rawPart := range content {
					part, ok := rawPart.(map[string]interface{})
					if !ok {
						continue
					}
					msg.Content = append(msg.Content, IRPart{
						Type: normalizeIRPartType(firstNonEmpty(stringValue(part["type"]), "output_text")),
						Text: stringValue(part["text"]),
						ProviderExtensions: map[string]interface{}{
							"raw": part,
						},
					})
				}
			}
			result = append(result, msg)
		case "function_call":
			result = append(result, IRMessage{
				Role: "assistant",
				Content: []IRPart{{
					Type:      "tool_call",
					Name:      stringValue(item["name"]),
					Arguments: stringValue(item["arguments"]),
					ProviderExtensions: map[string]interface{}{
						"id": firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
					},
				}},
			})
		case "reasoning":
			msg := IRMessage{
				Role: "assistant",
				Content: []IRPart{{
					Type: "reasoning",
					Text: extractResponsesReasoning(item),
				}},
			}
			result = append(result, msg)
		}
	}
	return result
}

func encodeResponsesOutput(messages []IRMessage) (json.RawMessage, string, error) {
	output := make([]map[string]interface{}, 0, len(messages))
	var outputText string
	for _, msg := range messages {
		if len(msg.Content) == 0 {
			continue
		}
		text := encodeTextContent(msg.Content)
		if text != "" || hasNonTextParts(msg.Content) {
			outputText += text
			output = append(output, map[string]interface{}{
				"type":    "message",
				"status":  "completed",
				"role":    firstNonEmpty(msg.Role, "assistant"),
				"content": encodeResponsesOutputContent(msg.Content),
			})
		}
		for _, part := range msg.Content {
			if part.Type != "tool_call" {
				if part.Type == "reasoning" {
					output = append(output, map[string]interface{}{
						"type":   "reasoning",
						"status": "completed",
						"summary": []map[string]interface{}{{
							"type": "summary_text",
							"text": part.Text,
						}},
					})
				}
				continue
			}
			output = append(output, map[string]interface{}{
				"type":      "function_call",
				"status":    "completed",
				"id":        stringValue(part.ProviderExtensions["id"]),
				"call_id":   stringValue(part.ProviderExtensions["id"]),
				"name":      part.Name,
				"arguments": part.Arguments,
			})
		}
	}
	raw, err := json.Marshal(output)
	return raw, outputText, err
}

func decodeAnthropicContent(raw json.RawMessage) []IRMessage {
	var content []map[string]interface{}
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil
	}
	msg := IRMessage{Role: "assistant"}
	for _, item := range content {
		switch stringValue(item["type"]) {
		case "text":
			msg.Content = append(msg.Content, IRPart{Type: "text", Text: stringValue(item["text"]), ProviderExtensions: map[string]interface{}{"raw": item}})
		case "thinking":
			msg.Content = append(msg.Content, IRPart{
				Type: "reasoning",
				Text: stringValue(item["thinking"]),
				ProviderExtensions: map[string]interface{}{
					"signature": stringValue(item["signature"]),
				},
			})
		case "image", "document":
			partType := "image"
			if stringValue(item["type"]) == "document" {
				partType = "file"
			}
			msg.Content = append(msg.Content, IRPart{Type: partType, ProviderExtensions: map[string]interface{}{"raw": item}})
		case "tool_use":
			msg.Content = append(msg.Content, IRPart{
				Type:      "tool_call",
				Name:      stringValue(item["name"]),
				Arguments: mustMarshalToString(item["input"]),
				ProviderExtensions: map[string]interface{}{
					"id": stringValue(item["id"]),
				},
			})
		}
	}
	if len(msg.Content) == 0 {
		return nil
	}
	return []IRMessage{msg}
}

func encodeAnthropicContent(messages []IRMessage) (json.RawMessage, error) {
	content := make([]map[string]interface{}, 0)
	for _, msg := range messages {
		for _, part := range msg.Content {
			switch part.Type {
			case "text", "output_text", "":
				if part.Text != "" {
					content = append(content, map[string]interface{}{"type": "text", "text": part.Text})
				}
			case "image", "file":
				content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "image", "file": "document"}[part.Type]}))
			case "tool_call":
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    stringValue(part.ProviderExtensions["id"]),
					"name":  part.Name,
					"input": decodeRawOrString(part.Arguments),
				})
			case "reasoning":
				content = append(content, map[string]interface{}{
					"type":      "thinking",
					"thinking":  part.Text,
					"signature": stringValue(part.ProviderExtensions["signature"]),
				})
			}
		}
	}
	return json.Marshal(content)
}

func inferFinishReasonFromMessages(messages []IRMessage) string {
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.Type == "tool_call" {
				return "tool_calls"
			}
		}
	}
	return "stop"
}

func encodeReasoningContent(parts []IRPart) string {
	for _, part := range parts {
		if part.Type == "reasoning" {
			return part.Text
		}
	}
	return ""
}

func extractResponsesReasoning(item map[string]interface{}) string {
	if summary, ok := item["summary"].([]interface{}); ok {
		for _, raw := range summary {
			part, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if text := stringValue(part["text"]); text != "" {
				return text
			}
		}
	}
	return ""
}

func encodeResponsesOutputContent(parts []IRPart) []map[string]interface{} {
	content := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "output_text", "":
			if part.Text != "" {
				content = append(content, map[string]interface{}{"type": "output_text", "text": part.Text})
			}
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": part.Type}))
		}
	}
	return content
}

func hasNonTextParts(parts []IRPart) bool {
	for _, part := range parts {
		if part.Type == "image" || part.Type == "file" {
			return true
		}
	}
	return false
}

func normalizeAnthropicUsage(raw json.RawMessage) json.RawMessage {
	var usage map[string]interface{}
	if err := json.Unmarshal(raw, &usage); err != nil {
		return cloneRaw(raw)
	}
	normalized, _ := json.Marshal(map[string]interface{}{
		"prompt_tokens":     numberToIntDefault(usage["input_tokens"]),
		"completion_tokens": numberToIntDefault(usage["output_tokens"]),
		"total_tokens":      numberToIntDefault(usage["input_tokens"]) + numberToIntDefault(usage["output_tokens"]),
	})
	return normalized
}

func denormalizeAnthropicUsage(raw json.RawMessage) json.RawMessage {
	var usage map[string]interface{}
	if err := json.Unmarshal(raw, &usage); err != nil {
		return cloneRaw(raw)
	}
	normalized, _ := json.Marshal(map[string]interface{}{
		"input_tokens":  numberToIntDefault(usage["prompt_tokens"]),
		"output_tokens": numberToIntDefault(usage["completion_tokens"]),
	})
	return normalized
}

func anthropicStopReasonToChatFinish(reason string) string {
	switch reason {
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return "stop"
	}
}

func anthropicStopReasonFromChatFinish(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}
