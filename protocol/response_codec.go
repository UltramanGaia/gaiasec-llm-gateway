package protocol

import (
	"encoding/json"
	"strings"
)

type openAIChatResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object,omitempty"`
	Created int64           `json:"created,omitempty"`
	Model   string          `json:"model,omitempty"`
	Choices json.RawMessage `json:"choices,omitempty"`
	Usage   json.RawMessage `json:"usage,omitempty"`
}

type responsesResponse struct {
	ID                string          `json:"id"`
	Object            string          `json:"object,omitempty"`
	CreatedAt         int64           `json:"created_at,omitempty"`
	Status            string          `json:"status,omitempty"`
	Model             string          `json:"model,omitempty"`
	Output            json.RawMessage `json:"output,omitempty"`
	OutputText        string          `json:"output_text,omitempty"`
	Usage             json.RawMessage `json:"usage,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	IncompleteDetails json.RawMessage `json:"incomplete_details,omitempty"`
	Error             json.RawMessage `json:"error,omitempty"`
	Conversation      json.RawMessage `json:"conversation,omitempty"`
	Prompt            json.RawMessage `json:"prompt,omitempty"`
	Reasoning         json.RawMessage `json:"reasoning,omitempty"`
	Text              json.RawMessage `json:"text,omitempty"`
	ToolChoice        json.RawMessage `json:"tool_choice,omitempty"`
	Tools             json.RawMessage `json:"tools,omitempty"`
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
		ID:                resp.ID,
		Model:             resp.Model,
		Status:            resp.Status,
		Usage:             cloneRaw(resp.Usage),
		Metadata:          cloneRaw(resp.Metadata),
		IncompleteDetails: cloneRaw(resp.IncompleteDetails),
		Error:             cloneRaw(resp.Error),
		Conversation:      cloneRaw(resp.Conversation),
		Prompt:            cloneRaw(resp.Prompt),
		TextConfig:        cloneRaw(resp.Text),
		ReasoningConfig:   cloneRaw(resp.Reasoning),
		ToolChoice:        cloneRaw(resp.ToolChoice),
		Tools:             cloneRaw(resp.Tools),
	}
	ir.OutputItems = decodeResponsesOutput(resp.Output)
	ir.FinishReason = inferFinishReasonFromMessages(ir.OutputItems)
	return ir, nil
}

func EncodeResponsesResponse(ir IRResponse, modelName string) ([]byte, error) {
	resp := responsesResponse{
		ID:                ir.ID,
		Object:            "response",
		Status:            firstNonEmpty(ir.Status, "completed"),
		Model:             firstNonEmpty(modelName, ir.Model),
		CreatedAt:         0,
		Usage:             cloneRaw(ir.Usage),
		Metadata:          cloneRaw(ir.Metadata),
		IncompleteDetails: cloneRaw(ir.IncompleteDetails),
		Error:             cloneRaw(ir.Error),
		Conversation:      cloneRaw(ir.Conversation),
		Prompt:            cloneRaw(ir.Prompt),
		Reasoning:         cloneRaw(ir.ReasoningConfig),
		Text:              cloneRaw(ir.TextConfig),
		ToolChoice:        cloneRaw(ir.ToolChoice),
		Tools:             cloneRaw(ir.Tools),
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
		msg := IRMessage{
			Role:   firstNonEmpty(stringValue(message["role"]), "assistant"),
			Status: "completed",
			Type:   "message",
		}
		msg.Content = append(msg.Content, decodeOpenAIChatMessageContent(message["content"])...)
		reasoning := firstNonEmpty(
			stringValue(message["reasoning_content"]),
			extractThinkTaggedReasoning(stringValue(message["content"])),
		)
		if reasoning != "" {
			msg.Content = append(msg.Content, IRPart{Type: "reasoning", Text: reasoning})
		}
		if refusal := stringValue(message["refusal"]); refusal != "" {
			msg.Content = append(msg.Content, IRPart{Type: "refusal", Refusal: refusal, Text: refusal})
		}
		if audio, ok := message["audio"]; ok && audio != nil {
			audioRaw, _ := json.Marshal(audio)
			msg.Content = append(msg.Content, IRPart{Type: "audio", Audio: audioRaw, ProviderExtensions: map[string]interface{}{"raw": audio}})
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
					ID:        stringValue(tc["id"]),
					CallID:    stringValue(tc["id"]),
					Status:    "completed",
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

func decodeOpenAIChatMessageContent(content interface{}) []IRPart {
	if rawText, ok := content.(string); ok {
		text, _ := stripThinkTaggedText(rawText)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []IRPart{{Type: "text", Text: text}}
	}
	return decodeGenericContent(content)
}

func extractThinkTaggedReasoning(content string) string {
	_, reasoning := stripThinkTaggedText(content)
	return reasoning
}

func stripThinkTaggedText(content string) (string, string) {
	if !strings.Contains(content, "<think>") || !strings.Contains(content, "</think>") {
		return content, ""
	}

	var reasoningParts []string
	var output strings.Builder
	rest := content

	for {
		start := strings.Index(rest, "<think>")
		if start < 0 {
			output.WriteString(rest)
			break
		}
		output.WriteString(rest[:start])
		rest = rest[start+len("<think>"):]

		end := strings.Index(rest, "</think>")
		if end < 0 {
			output.WriteString("<think>")
			output.WriteString(rest)
			break
		}

		reasoning := strings.TrimSpace(rest[:end])
		if reasoning != "" {
			reasoningParts = append(reasoningParts, reasoning)
		}
		rest = rest[end+len("</think>"):]
	}

	return strings.TrimSpace(output.String()), strings.Join(reasoningParts, "\n\n")
}

func encodeOpenAIChatChoices(ir IRResponse) []map[string]interface{} {
	message := map[string]interface{}{"role": "assistant"}
	primary := primaryChatOutputItem(ir.OutputItems)
	if primary != nil {
		message["role"] = firstNonEmpty(primary.Role, "assistant")
		message["content"] = encodeOpenAIResponseContent(primary.Content)
		if refusal := encodeRefusalContent(primary.Content); refusal != "" {
			message["refusal"] = refusal
		}
		if audio := encodeAudioContent(primary.Content); len(audio) > 0 {
			message["audio"] = decodeRawToAny(audio)
		}
	} else {
		message["content"] = ""
	}
	combined := aggregateOpenAIChatParts(ir.OutputItems)
	if reasoning := encodeReasoningContent(combined); reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if toolCalls := encodeToolCalls(combined); len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	return []map[string]interface{}{{
		"index":         0,
		"message":       message,
		"finish_reason": firstNonEmpty(ir.FinishReason, inferFinishReasonFromMessages(ir.OutputItems)),
	}}
}

func primaryChatOutputItem(items []IRMessage) *IRMessage {
	for i := range items {
		if items[i].Type == "message" {
			return &items[i]
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}

func aggregateOpenAIChatParts(items []IRMessage) []IRPart {
	parts := make([]IRPart, 0)
	for _, item := range items {
		parts = append(parts, item.Content...)
	}
	return parts
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
			msg := IRMessage{
				ID:     stringValue(item["id"]),
				Type:   "message",
				Role:   firstNonEmpty(stringValue(item["role"]), "assistant"),
				Status: stringValue(item["status"]),
			}
			if content, ok := item["content"].([]interface{}); ok {
				for _, rawPart := range content {
					part, ok := rawPart.(map[string]interface{})
					if !ok {
						continue
					}
					msg.Content = append(msg.Content, decodeResponsesOutputPart(part))
				}
			}
			result = append(result, msg)
		case "function_call":
			result = append(result, IRMessage{
				ID:     stringValue(item["id"]),
				Type:   "function_call",
				Role:   "assistant",
				Status: stringValue(item["status"]),
				Content: []IRPart{{
					Type:      "tool_call",
					ID:        stringValue(item["id"]),
					CallID:    firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
					Status:    stringValue(item["status"]),
					Name:      stringValue(item["name"]),
					Arguments: stringValue(item["arguments"]),
					ProviderExtensions: map[string]interface{}{
						"raw_response_item": item,
						"item_type":         "function_call",
						"id":                firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
					},
				}},
				ProviderExtensions: map[string]interface{}{"raw_response_item": item},
			})
		case "reasoning":
			msg := IRMessage{
				ID:     stringValue(item["id"]),
				Type:   "reasoning",
				Role:   "assistant",
				Status: stringValue(item["status"]),
				Content: []IRPart{{
					Type:   "reasoning",
					ID:     stringValue(item["id"]),
					Status: stringValue(item["status"]),
					Text:   extractResponsesReasoning(item),
					ProviderExtensions: map[string]interface{}{
						"raw_response_item": item,
					},
				}},
				ProviderExtensions: map[string]interface{}{"raw_response_item": item},
			}
			result = append(result, msg)
		case "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call", "compaction":
			result = append(result, decodeResponsesRawOutputItem(item))
		}
	}
	return result
}

func encodeResponsesOutput(messages []IRMessage) (json.RawMessage, string, error) {
	output := make([]map[string]interface{}, 0, len(messages))
	var outputText string
	for _, msg := range messages {
		if raw := rawResponseItem(msg); len(raw) > 0 {
			output = append(output, raw)
			outputText += textFromRawResponseItem(raw)
			continue
		}
		if len(msg.Content) == 0 {
			continue
		}
		text := encodeTextContent(msg.Content)
		if text != "" || hasRenderableResponseParts(msg.Content) {
			outputText += text
			output = append(output, map[string]interface{}{
				"id":      msg.ID,
				"type":    firstNonEmpty(msg.Type, "message"),
				"status":  firstNonEmpty(msg.Status, "completed"),
				"role":    firstNonEmpty(msg.Role, "assistant"),
				"content": encodeResponsesOutputContent(msg.Content),
			})
		}
		for _, part := range msg.Content {
			if part.Type != "tool_call" {
				if part.Type == "reasoning" {
					output = append(output, map[string]interface{}{
						"id":     firstNonEmpty(msg.ID, part.ID),
						"type":   "reasoning",
						"status": firstNonEmpty(msg.Status, part.Status, "completed"),
						"summary": []map[string]interface{}{{
							"type": "summary_text",
							"text": part.Text,
						}},
					})
				}
				continue
			}
			output = append(output, map[string]interface{}{
				"type":      firstNonEmpty(stringValue(part.ProviderExtensions["item_type"]), "function_call"),
				"status":    firstNonEmpty(part.Status, msg.Status, "completed"),
				"id":        firstNonEmpty(part.ID, stringValue(part.ProviderExtensions["id"])),
				"call_id":   firstNonEmpty(part.CallID, stringValue(part.ProviderExtensions["id"])),
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
	msg := IRMessage{Role: "assistant", Type: "message", Status: "completed"}
	for _, item := range content {
		switch stringValue(item["type"]) {
		case "text":
			part := IRPart{Type: "text", Text: stringValue(item["text"]), ProviderExtensions: map[string]interface{}{"raw": item}}
			if annotations, ok := item["annotations"]; ok && annotations != nil {
				annotationsRaw, _ := json.Marshal(annotations)
				part.Annotations = annotationsRaw
			}
			if part.Text != "" || len(part.Annotations) > 0 {
				msg.Content = append(msg.Content, part)
			}
			if refusal := stringValue(item["refusal"]); refusal != "" {
				msg.Content = append(msg.Content, IRPart{
					Type:               "refusal",
					Text:               refusal,
					Refusal:            refusal,
					ProviderExtensions: map[string]interface{}{"raw": item},
				})
			}
			if audio, ok := item["audio"]; ok && audio != nil {
				audioRaw, _ := json.Marshal(audio)
				msg.Content = append(msg.Content, IRPart{
					Type:               "audio",
					Audio:              audioRaw,
					ProviderExtensions: map[string]interface{}{"raw": item},
				})
			}
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
				if part.Text != "" || len(part.Annotations) > 0 {
					item := map[string]interface{}{"type": "text", "text": part.Text}
					if len(part.Annotations) > 0 {
						item["annotations"] = decodeRawToAny(part.Annotations)
					}
					content = append(content, item)
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
			case "refusal":
				content = append(content, map[string]interface{}{
					"type":    "text",
					"text":    firstNonEmpty(part.Refusal, part.Text),
					"refusal": firstNonEmpty(part.Refusal, part.Text),
				})
			case "audio":
				if len(part.Audio) > 0 {
					content = append(content, map[string]interface{}{
						"type":  "text",
						"text":  "",
						"audio": decodeRawToAny(part.Audio),
					})
				}
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

func encodeRefusalContent(parts []IRPart) string {
	for _, part := range parts {
		if part.Type == "refusal" {
			return firstNonEmpty(part.Refusal, part.Text)
		}
	}
	return ""
}

func encodeAudioContent(parts []IRPart) json.RawMessage {
	for _, part := range parts {
		if part.Type == "audio" && len(part.Audio) > 0 {
			return cloneRaw(part.Audio)
		}
	}
	return nil
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
				item := map[string]interface{}{"type": "output_text", "text": part.Text}
				if len(part.Annotations) > 0 {
					item["annotations"] = decodeRawToAny(part.Annotations)
				}
				content = append(content, item)
			}
		case "refusal":
			content = append(content, map[string]interface{}{"type": "refusal", "refusal": firstNonEmpty(part.Refusal, part.Text)})
		case "audio":
			if len(part.Audio) > 0 {
				content = append(content, map[string]interface{}{"type": "output_audio", "audio": decodeRawToAny(part.Audio)})
			}
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": part.Type}))
		}
	}
	return content
}

func decodeResponsesOutputPart(part map[string]interface{}) IRPart {
	partType := firstNonEmpty(stringValue(part["type"]), "output_text")
	irPart := IRPart{
		Type:   normalizeIRPartType(partType),
		ID:     stringValue(part["id"]),
		CallID: firstNonEmpty(stringValue(part["call_id"]), stringValue(part["id"])),
		Status: stringValue(part["status"]),
		Text:   stringValue(part["text"]),
		ProviderExtensions: map[string]interface{}{
			"raw": part,
		},
	}
	switch partType {
	case "refusal":
		irPart.Type = "refusal"
		irPart.Refusal = firstNonEmpty(stringValue(part["refusal"]), stringValue(part["text"]))
		irPart.Text = irPart.Refusal
	case "output_audio", "audio":
		irPart.Type = "audio"
		audioRaw, _ := json.Marshal(firstNonEmptyMap(part["audio"], part))
		irPart.Audio = audioRaw
	case "output_text":
		if annotations, ok := part["annotations"]; ok && annotations != nil {
			annotationsRaw, _ := json.Marshal(annotations)
			irPart.Annotations = annotationsRaw
		}
	}
	return irPart
}

func decodeResponsesRawOutputItem(item map[string]interface{}) IRMessage {
	rawPayload, _ := json.Marshal(item)
	msg := IRMessage{
		ID:     stringValue(item["id"]),
		Type:   stringValue(item["type"]),
		Role:   "assistant",
		Status: stringValue(item["status"]),
		ProviderExtensions: map[string]interface{}{
			"raw_response_item": cloneRaw(rawPayload),
		},
	}
	switch msg.Type {
	case "compaction":
		msg.Content = []IRPart{{
			Type:               "reasoning",
			ID:                 msg.ID,
			Status:             msg.Status,
			Text:               extractResponsesReasoning(item),
			ProviderExtensions: map[string]interface{}{"raw_response_item": item},
		}}
	default:
		msg.Content = []IRPart{{
			Type:   "tool_call",
			ID:     msg.ID,
			CallID: firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
			Status: msg.Status,
			Name:   firstNonEmpty(stringValue(item["name"]), stringValue(item["type"])),
			Arguments: firstNonEmpty(
				stringValue(item["arguments"]),
				stringValue(item["input"]),
			),
			ProviderExtensions: map[string]interface{}{
				"raw_response_item": item,
				"item_type":         msg.Type,
				"id":                firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
			},
		}}
	}
	return msg
}

func rawResponseItem(msg IRMessage) map[string]interface{} {
	if msg.ProviderExtensions == nil {
		return nil
	}
	switch raw := msg.ProviderExtensions["raw_response_item"].(type) {
	case map[string]interface{}:
		return raw
	case json.RawMessage:
		return decodeRawObject(raw)
	case []byte:
		return decodeRawObject(raw)
	default:
		return nil
	}
}

func textFromRawResponseItem(item map[string]interface{}) string {
	if stringValue(item["type"]) == "message" {
		if content, ok := item["content"].([]interface{}); ok {
			var b strings.Builder
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]interface{})
				if !ok {
					continue
				}
				if text := stringValue(part["text"]); text != "" {
					b.WriteString(text)
				}
			}
			return b.String()
		}
	}
	return ""
}

func encodeOpenAIResponseContent(parts []IRPart) interface{} {
	hasStructured := false
	for _, part := range parts {
		if part.Type == "image" || part.Type == "file" || part.Type == "refusal" || part.Type == "audio" || len(part.Annotations) > 0 {
			hasStructured = true
			break
		}
	}
	if !hasStructured {
		return encodeTextContent(parts)
	}
	content := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			item := encodeRawOrFallbackPart(part, map[string]interface{}{"type": "text", "text": part.Text})
			if len(part.Annotations) > 0 {
				item["annotations"] = decodeRawToAny(part.Annotations)
			}
			content = append(content, item)
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "image_url", "file": "input_file"}[part.Type]}))
		}
	}
	return content
}

func firstNonEmptyMap(candidates ...interface{}) interface{} {
	for _, candidate := range candidates {
		switch value := candidate.(type) {
		case map[string]interface{}:
			if len(value) > 0 {
				return value
			}
		default:
			if candidate != nil {
				return candidate
			}
		}
	}
	return nil
}

func hasNonTextParts(parts []IRPart) bool {
	for _, part := range parts {
		if part.Type == "image" || part.Type == "file" {
			return true
		}
	}
	return false
}

func hasRenderableResponseParts(parts []IRPart) bool {
	for _, part := range parts {
		switch part.Type {
		case "image", "file", "refusal", "audio":
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
