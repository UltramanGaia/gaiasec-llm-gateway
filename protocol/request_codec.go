package protocol

import (
	"encoding/json"
	"strings"
)

type openAIChatRequest struct {
	Model             string          `json:"model"`
	Messages          json.RawMessage `json:"messages,omitempty"`
	MaxTokens         int             `json:"max_tokens,omitempty"`
	Temperature       *float64        `json:"temperature,omitempty"`
	TopP              *float64        `json:"top_p,omitempty"`
	Stop              json.RawMessage `json:"stop,omitempty"`
	Tools             json.RawMessage `json:"tools,omitempty"`
	ToolChoice        json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool           `json:"parallel_tool_calls,omitempty"`
	ResponseFormat    json.RawMessage `json:"response_format,omitempty"`
	Reasoning         json.RawMessage `json:"reasoning,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	ServiceTier       string          `json:"service_tier,omitempty"`
	Modalities        json.RawMessage `json:"modalities,omitempty"`
	Audio             json.RawMessage `json:"audio,omitempty"`
	Prediction        json.RawMessage `json:"prediction,omitempty"`
	Verbosity         json.RawMessage `json:"verbosity,omitempty"`
	WebSearchOptions  json.RawMessage `json:"web_search_options,omitempty"`
	Logprobs          json.RawMessage `json:"logprobs,omitempty"`
	TopLogprobs       json.RawMessage `json:"top_logprobs,omitempty"`
	Seed              json.RawMessage `json:"seed,omitempty"`
	N                 json.RawMessage `json:"n,omitempty"`
	FrequencyPenalty  json.RawMessage `json:"frequency_penalty,omitempty"`
	PresencePenalty   json.RawMessage `json:"presence_penalty,omitempty"`
	LogitBias         json.RawMessage `json:"logit_bias,omitempty"`
	Stream            bool            `json:"stream,omitempty"`
}

type responsesRequest struct {
	Model                string          `json:"model"`
	Input                json.RawMessage `json:"input,omitempty"`
	Instructions         string          `json:"instructions,omitempty"`
	MaxOutputTokens      int             `json:"max_output_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	Stop                 json.RawMessage `json:"stop,omitempty"`
	Tools                json.RawMessage `json:"tools,omitempty"`
	ToolChoice           json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool           `json:"parallel_tool_calls,omitempty"`
	Reasoning            json.RawMessage `json:"reasoning,omitempty"`
	Text                 json.RawMessage `json:"text,omitempty"`
	Stream               bool            `json:"stream,omitempty"`
	PreviousResponseID   string          `json:"previous_response_id,omitempty"`
	Include              []string        `json:"include,omitempty"`
	Metadata             json.RawMessage `json:"metadata,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	Background           *bool           `json:"background,omitempty"`
	Conversation         json.RawMessage `json:"conversation,omitempty"`
	Prompt               json.RawMessage `json:"prompt,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
}

type anthropicRequest struct {
	Model         string          `json:"model"`
	System        json.RawMessage `json:"system,omitempty"`
	Messages      json.RawMessage `json:"messages,omitempty"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences json.RawMessage `json:"stop_sequences,omitempty"`
	Tools         json.RawMessage `json:"tools,omitempty"`
	ToolChoice    json.RawMessage `json:"tool_choice,omitempty"`
	Thinking      json.RawMessage `json:"thinking,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	ServiceTier   string          `json:"service_tier,omitempty"`
	TopK          json.RawMessage `json:"top_k,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}

func DecodeOpenAIChatRequest(body []byte) (IRRequest, error) {
	var req openAIChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return IRRequest{}, err
	}
	decodedMessages := decodeOpenAIChatMessages(req.Messages)
	systemInstruction, messages := splitSystemInstruction(decodedMessages)
	ir := IRRequest{
		Model:             req.Model,
		SystemInstruction: systemInstruction,
		ToolChoice:        cloneRaw(req.ToolChoice),
		Metadata:          cloneRaw(req.Metadata),
		ServiceTier:       req.ServiceTier,
		ResponseFormat:    cloneRaw(req.ResponseFormat),
		Reasoning:         cloneRaw(req.Reasoning),
		Stream:            req.Stream,
		Generation: IRGeneration{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stop:        normalizeStop(req.Stop),
		},
	}
	ir.Messages = messages
	ir.Tools = decodeOpenAIChatTools(req.Tools)
	ir.ProviderExtensions = map[string]interface{}{}
	if req.ParallelToolCalls != nil {
		ir.ProviderExtensions["parallel_tool_calls"] = *req.ParallelToolCalls
	}
	addRawExtension(ir.ProviderExtensions, "modalities", req.Modalities)
	addRawExtension(ir.ProviderExtensions, "audio", req.Audio)
	addRawExtension(ir.ProviderExtensions, "prediction", req.Prediction)
	addRawExtension(ir.ProviderExtensions, "verbosity", req.Verbosity)
	addRawExtension(ir.ProviderExtensions, "web_search_options", req.WebSearchOptions)
	addRawExtension(ir.ProviderExtensions, "logprobs", req.Logprobs)
	addRawExtension(ir.ProviderExtensions, "top_logprobs", req.TopLogprobs)
	addRawExtension(ir.ProviderExtensions, "seed", req.Seed)
	addRawExtension(ir.ProviderExtensions, "n", req.N)
	addRawExtension(ir.ProviderExtensions, "frequency_penalty", req.FrequencyPenalty)
	addRawExtension(ir.ProviderExtensions, "presence_penalty", req.PresencePenalty)
	addRawExtension(ir.ProviderExtensions, "logit_bias", req.LogitBias)
	normalizeIRToolConfig(&ir, rawHasUsableArray(req.Tools))
	return ir, nil
}

func EncodeOpenAIChatRequest(ir IRRequest, modelName string) ([]byte, error) {
	req := openAIChatRequest{
		Model:          firstNonEmpty(modelName, ir.Model),
		MaxTokens:      ir.Generation.MaxTokens,
		Temperature:    ir.Generation.Temperature,
		TopP:           ir.Generation.TopP,
		ToolChoice:     normalizeOpenAIChatToolChoice(ir.ToolChoice),
		Metadata:       cloneRaw(ir.Metadata),
		ServiceTier:    ir.ServiceTier,
		ResponseFormat: normalizeOpenAIChatResponseFormat(ir.ResponseFormat),
		Reasoning:      cloneRaw(ir.Reasoning),
		Stream:         ir.Stream,
	}
	if len(ir.Generation.Stop) > 0 {
		stop, _ := json.Marshal(ir.Generation.Stop)
		req.Stop = stop
	}
	if len(ir.Tools) > 0 {
		tools, _ := json.Marshal(encodeOpenAIChatTools(ir.Tools))
		req.Tools = tools
	}
	if messages := encodeOpenAIChatMessages(ir); len(messages) > 0 {
		raw, _ := json.Marshal(messages)
		req.Messages = raw
	}
	if parallel, ok := ir.ProviderExtensions["parallel_tool_calls"].(bool); ok {
		req.ParallelToolCalls = &parallel
	}
	req.Modalities = rawExtension(ir.ProviderExtensions, "modalities")
	req.Audio = rawExtension(ir.ProviderExtensions, "audio")
	req.Prediction = rawExtension(ir.ProviderExtensions, "prediction")
	req.Verbosity = rawExtension(ir.ProviderExtensions, "verbosity")
	req.WebSearchOptions = rawExtension(ir.ProviderExtensions, "web_search_options")
	req.Logprobs = rawExtension(ir.ProviderExtensions, "logprobs")
	req.TopLogprobs = rawExtension(ir.ProviderExtensions, "top_logprobs")
	req.Seed = rawExtension(ir.ProviderExtensions, "seed")
	req.N = rawExtension(ir.ProviderExtensions, "n")
	req.FrequencyPenalty = rawExtension(ir.ProviderExtensions, "frequency_penalty")
	req.PresencePenalty = rawExtension(ir.ProviderExtensions, "presence_penalty")
	req.LogitBias = rawExtension(ir.ProviderExtensions, "logit_bias")
	return json.Marshal(req)
}

func DecodeResponsesRequest(body []byte) (IRRequest, error) {
	var req responsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return IRRequest{}, err
	}
	ir := IRRequest{
		Model:                req.Model,
		SystemInstruction:    req.Instructions,
		ToolChoice:           cloneRaw(req.ToolChoice),
		PreviousResponseID:   req.PreviousResponseID,
		Include:              append([]string(nil), req.Include...),
		Metadata:             cloneRaw(req.Metadata),
		ServiceTier:          req.ServiceTier,
		Store:                req.Store,
		Background:           req.Background,
		Conversation:         cloneRaw(req.Conversation),
		Prompt:               cloneRaw(req.Prompt),
		PromptCacheKey:       req.PromptCacheKey,
		PromptCacheRetention: req.PromptCacheRetention,
		ResponseFormat:       cloneRaw(req.Text),
		Reasoning:            cloneRaw(req.Reasoning),
		Stream:               req.Stream,
		Generation: IRGeneration{
			MaxTokens:   req.MaxOutputTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stop:        normalizeStop(req.Stop),
		},
	}
	ir.Messages = decodeResponsesInput(req.Input)
	ir.Tools = decodeResponsesTools(req.Tools)
	ir.ProviderExtensions = map[string]interface{}{}
	if req.ParallelToolCalls != nil {
		ir.ProviderExtensions["parallel_tool_calls"] = *req.ParallelToolCalls
	}
	normalizeIRToolConfig(&ir, rawHasUsableArray(req.Tools))
	return ir, nil
}

func EncodeResponsesRequest(ir IRRequest, modelName string) ([]byte, error) {
	req := responsesRequest{
		Model:                firstNonEmpty(modelName, ir.Model),
		Instructions:         ir.SystemInstruction,
		MaxOutputTokens:      ir.Generation.MaxTokens,
		Temperature:          ir.Generation.Temperature,
		TopP:                 ir.Generation.TopP,
		ToolChoice:           normalizeResponsesToolChoice(ir.ToolChoice),
		PreviousResponseID:   ir.PreviousResponseID,
		Include:              append([]string(nil), ir.Include...),
		Metadata:             cloneRaw(ir.Metadata),
		ServiceTier:          ir.ServiceTier,
		Store:                ir.Store,
		Background:           ir.Background,
		Conversation:         cloneRaw(ir.Conversation),
		Prompt:               cloneRaw(ir.Prompt),
		PromptCacheKey:       ir.PromptCacheKey,
		PromptCacheRetention: ir.PromptCacheRetention,
		Reasoning:            cloneRaw(ir.Reasoning),
		Text:                 cloneRaw(ir.ResponseFormat),
		Stream:               ir.Stream,
	}
	if len(ir.Generation.Stop) > 0 {
		stop, _ := json.Marshal(ir.Generation.Stop)
		req.Stop = stop
	}
	if len(ir.Tools) > 0 {
		tools, _ := json.Marshal(encodeResponsesTools(ir.Tools))
		req.Tools = tools
	}
	if input := encodeResponsesInput(ir.Messages); len(input) > 0 {
		raw, _ := json.Marshal(input)
		req.Input = raw
	}
	if parallel, ok := ir.ProviderExtensions["parallel_tool_calls"].(bool); ok {
		req.ParallelToolCalls = &parallel
	}
	return json.Marshal(req)
}

func DecodeAnthropicRequest(body []byte) (IRRequest, error) {
	var req anthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return IRRequest{}, err
	}
	ir := IRRequest{
		Model:             req.Model,
		SystemInstruction: decodeAnthropicSystem(req.System),
		ToolChoice:        cloneRaw(req.ToolChoice),
		Metadata:          cloneRaw(req.Metadata),
		ServiceTier:       req.ServiceTier,
		Reasoning:         cloneRaw(req.Thinking),
		Stream:            req.Stream,
		Generation: IRGeneration{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			Stop:        normalizeStop(req.StopSequences),
		},
	}
	ir.Messages = decodeAnthropicMessages(req.Messages)
	ir.Tools = decodeAnthropicTools(req.Tools)
	ir.ProviderExtensions = map[string]interface{}{}
	addRawExtension(ir.ProviderExtensions, "top_k", req.TopK)
	return ir, nil
}

func EncodeAnthropicRequest(ir IRRequest, modelName string) ([]byte, error) {
	req := anthropicRequest{
		Model:       firstNonEmpty(modelName, ir.Model),
		MaxTokens:   ir.Generation.MaxTokens,
		Temperature: ir.Generation.Temperature,
		TopP:        ir.Generation.TopP,
		ToolChoice:  normalizeAnthropicToolChoice(ir.ToolChoice),
		Metadata:    cloneRaw(ir.Metadata),
		ServiceTier: ir.ServiceTier,
		Thinking:    cloneRaw(ir.Reasoning),
		Stream:      ir.Stream,
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 1024
	}
	if ir.SystemInstruction != "" {
		system, _ := json.Marshal(ir.SystemInstruction)
		req.System = system
	}
	if len(ir.Generation.Stop) > 0 {
		stop, _ := json.Marshal(ir.Generation.Stop)
		req.StopSequences = stop
	}
	if len(ir.Tools) > 0 {
		tools, _ := json.Marshal(encodeAnthropicTools(ir.Tools))
		req.Tools = tools
	}
	req.TopK = rawExtension(ir.ProviderExtensions, "top_k")
	if messages := encodeAnthropicMessages(ir.Messages); len(messages) > 0 {
		raw, _ := json.Marshal(messages)
		req.Messages = raw
	}
	return json.Marshal(req)
}

func decodeOpenAIChatMessages(raw json.RawMessage) []IRMessage {
	var msgs []map[string]interface{}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil
	}
	result := make([]IRMessage, 0, len(msgs))
	for _, msg := range msgs {
		ir := IRMessage{Role: stringValue(msg["role"])}
		if ir.Role == "" {
			continue
		}
		if toolID := stringValue(msg["tool_call_id"]); toolID != "" {
			ir.ToolCallID = toolID
		}
		ir.Content = append(ir.Content, decodeGenericContent(msg["content"])...)
		if toolCalls, ok := msg["tool_calls"].([]interface{}); ok {
			for _, rawTC := range toolCalls {
				tc, ok := rawTC.(map[string]interface{})
				if !ok {
					continue
				}
				fn, _ := tc["function"].(map[string]interface{})
				ir.Content = append(ir.Content, IRPart{
					Type:      "tool_call",
					Name:      stringValue(fn["name"]),
					Arguments: stringValue(fn["arguments"]),
					ProviderExtensions: map[string]interface{}{
						"id": stringValue(tc["id"]),
					},
				})
			}
		}
		result = append(result, ir)
	}
	return result
}

func encodeOpenAIChatMessages(ir IRRequest) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(ir.Messages)+1)
	if ir.SystemInstruction != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": ir.SystemInstruction,
		})
	}
	for _, msg := range ir.Messages {
		item := map[string]interface{}{"role": msg.Role}
		content := encodeOpenAIContent(msg.Content)
		if msg.Role == "tool" {
			item["tool_call_id"] = msg.ToolCallID
			item["content"] = content
			messages = append(messages, item)
			continue
		}
		toolCalls := encodeToolCalls(msg.Content)
		if len(toolCalls) > 0 {
			item["tool_calls"] = toolCalls
		}
		if content != nil || len(toolCalls) == 0 {
			item["content"] = content
		}
		messages = append(messages, item)
	}
	return messages
}

func splitSystemInstruction(messages []IRMessage) (string, []IRMessage) {
	var systemParts []string
	filtered := make([]IRMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			if text := encodeTextContent(msg.Content); text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}
		filtered = append(filtered, msg)
	}
	return strings.Join(systemParts, "\n\n"), filtered
}

func decodeOpenAIChatTools(raw json.RawMessage) []IRTool {
	var tools []map[string]interface{}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	result := make([]IRTool, 0, len(tools))
	for _, tool := range tools {
		rawPayload, _ := json.Marshal(tool)
		toolType := firstNonEmpty(stringValue(tool["type"]), "function")
		if toolType != "function" {
			result = append(result, IRTool{
				Type:       toolType,
				Subtype:    toolType,
				Name:       stringValue(tool["name"]),
				RawPayload: rawPayload,
			})
			continue
		}
		fn, _ := tool["function"].(map[string]interface{})
		if fn == nil || strings.TrimSpace(stringValue(fn["name"])) == "" {
			continue
		}
		parameters, _ := json.Marshal(fn["parameters"])
		result = append(result, IRTool{
			Type:        toolType,
			Subtype:     toolType,
			Name:        stringValue(fn["name"]),
			Description: stringValue(fn["description"]),
			Parameters:  parameters,
			RawPayload:  rawPayload,
			ProviderExtensions: map[string]interface{}{
				"source_protocol": "chat",
			},
		})
	}
	return result
}

func normalizeIRToolConfig(ir *IRRequest, hadTools bool) {
	if ir == nil {
		return
	}
	if !hadTools || len(ir.Tools) > 0 {
		return
	}
	ir.ToolChoice = nil
	if ir.ProviderExtensions != nil {
		delete(ir.ProviderExtensions, "parallel_tool_calls")
	}
}

func rawHasUsableArray(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed != "[]"
}

func encodeOpenAIChatTools(tools []IRTool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if raw := decodeRawObject(tool.RawPayload); len(raw) > 0 && toolSourceProtocol(tool) == "chat" {
			result = append(result, raw)
			continue
		}
		result = append(result, map[string]interface{}{
			"type": firstNonEmpty(tool.Type, "function"),
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  decodeRawToAny(tool.Parameters),
			},
		})
	}
	return result
}

func decodeResponsesInput(raw json.RawMessage) []IRMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "\"") {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return []IRMessage{{Role: "user", Content: []IRPart{{Type: "text", Text: text}}}}
		}
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	result := make([]IRMessage, 0, len(items))
	for _, item := range items {
		switch stringValue(item["type"]) {
		case "message":
			role := firstNonEmpty(stringValue(item["role"]), "user")
			msg := IRMessage{Role: role}
			if content, ok := item["content"].([]interface{}); ok {
				for _, rawPart := range content {
					part, ok := rawPart.(map[string]interface{})
					if !ok {
						continue
					}
					msg.Content = append(msg.Content, IRPart{
						Type: normalizeIRPartType(firstNonEmpty(stringValue(part["type"]), "text")),
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
		case "function_call_output":
			result = append(result, IRMessage{
				Role:       "tool",
				ToolCallID: firstNonEmpty(stringValue(item["call_id"]), stringValue(item["id"])),
				Content: []IRPart{{
					Type:   "tool_result",
					Output: stringValue(item["output"]),
					Text:   stringValue(item["output"]),
				}},
			})
		}
	}
	return result
}

func encodeResponsesInput(messages []IRMessage) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "tool":
			result = append(result, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": msg.ToolCallID,
				"output":  firstNonEmpty(extractOutput(msg.Content), encodeTextContent(msg.Content)),
			})
		case "assistant":
			for _, part := range msg.Content {
				if part.Type == "tool_call" {
					result = append(result, map[string]interface{}{
						"type":      "function_call",
						"call_id":   stringValue(part.ProviderExtensions["id"]),
						"id":        stringValue(part.ProviderExtensions["id"]),
						"name":      part.Name,
						"arguments": part.Arguments,
					})
				}
			}
			fallthrough
		default:
			content := encodeResponsesContent(msg.Content)
			if len(content) > 0 || msg.Role != "assistant" {
				result = append(result, map[string]interface{}{
					"type":    "message",
					"role":    msg.Role,
					"content": content,
				})
			}
		}
	}
	return result
}

func decodeResponsesTools(raw json.RawMessage) []IRTool {
	var tools []map[string]interface{}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	result := make([]IRTool, 0, len(tools))
	for _, tool := range tools {
		rawPayload, _ := json.Marshal(tool)
		toolType := firstNonEmpty(stringValue(tool["type"]), "function")
		if toolType != "function" {
			result = append(result, IRTool{
				Type:       toolType,
				Subtype:    toolType,
				Name:       stringValue(tool["name"]),
				RawPayload: rawPayload,
			})
			continue
		}
		name := stringValue(tool["name"])
		if strings.TrimSpace(name) == "" {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				name = stringValue(fn["name"])
			}
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		description := stringValue(tool["description"])
		if description == "" {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				description = stringValue(fn["description"])
			}
		}
		var paramsSource interface{} = tool["parameters"]
		if paramsSource == nil {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				paramsSource = fn["parameters"]
			}
		}
		parameters, _ := json.Marshal(paramsSource)
		result = append(result, IRTool{
			Type:        "function",
			Subtype:     toolType,
			Name:        name,
			Description: description,
			Parameters:  parameters,
			RawPayload:  rawPayload,
			ProviderExtensions: map[string]interface{}{
				"source_protocol": "responses",
			},
		})
	}
	return result
}

func encodeResponsesTools(tools []IRTool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if raw := decodeRawObject(tool.RawPayload); len(raw) > 0 && toolSourceProtocol(tool) == "responses" {
			result = append(result, raw)
			continue
		}
		result = append(result, map[string]interface{}{
			"type":        firstNonEmpty(tool.Type, "function"),
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  decodeRawToAny(tool.Parameters),
		})
	}
	return result
}

func decodeAnthropicSystem(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return ""
}

func decodeAnthropicMessages(raw json.RawMessage) []IRMessage {
	var msgs []map[string]interface{}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil
	}
	result := make([]IRMessage, 0, len(msgs))
	for _, msg := range msgs {
		ir := IRMessage{Role: stringValue(msg["role"])}
		content, _ := msg["content"].([]interface{})
		for _, rawPart := range content {
			part, ok := rawPart.(map[string]interface{})
			if !ok {
				continue
			}
			switch stringValue(part["type"]) {
			case "text":
				ir.Content = append(ir.Content, IRPart{Type: "text", Text: stringValue(part["text"]), ProviderExtensions: map[string]interface{}{"raw": part}})
			case "image", "document":
				partType := "image"
				if stringValue(part["type"]) == "document" {
					partType = "file"
				}
				ir.Content = append(ir.Content, IRPart{Type: partType, ProviderExtensions: map[string]interface{}{"raw": part}})
			case "tool_use":
				ir.Content = append(ir.Content, IRPart{
					Type:      "tool_call",
					Name:      stringValue(part["name"]),
					Arguments: mustMarshalToString(part["input"]),
					ProviderExtensions: map[string]interface{}{
						"id":  stringValue(part["id"]),
						"raw": part,
					},
				})
			case "tool_result":
				ir.Role = "tool"
				ir.ToolCallID = stringValue(part["tool_use_id"])
				ir.Content = append(ir.Content, IRPart{
					Type:   "tool_result",
					Output: stringValue(part["content"]),
					Text:   stringValue(part["content"]),
					ProviderExtensions: map[string]interface{}{
						"raw": part,
					},
				})
			}
		}
		result = append(result, ir)
	}
	return result
}

func encodeAnthropicMessages(messages []IRMessage) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "tool":
			if len(msg.Content) > 0 {
				part := msg.Content[0]
				result = append(result, map[string]interface{}{
					"role": "user",
					"content": []map[string]interface{}{
						encodeRawOrFallbackPart(part, map[string]interface{}{
							"type":        "tool_result",
							"tool_use_id": msg.ToolCallID,
							"content":     firstNonEmpty(extractOutput(msg.Content), encodeTextContent(msg.Content)),
						}),
					},
				})
				continue
			}
			result = append(result, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{{
					"type":        "tool_result",
					"tool_use_id": msg.ToolCallID,
					"content":     firstNonEmpty(extractOutput(msg.Content), encodeTextContent(msg.Content)),
				}},
			})
		case "assistant":
			content := make([]map[string]interface{}, 0)
			for _, part := range msg.Content {
				switch part.Type {
				case "text":
					content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": "text", "text": part.Text}))
				case "image", "file":
					content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "image", "file": "document"}[part.Type]}))
				case "tool_call":
					content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{
						"type":  "tool_use",
						"id":    stringValue(part.ProviderExtensions["id"]),
						"name":  part.Name,
						"input": decodeRawOrString(part.Arguments),
					}))
				}
			}
			result = append(result, map[string]interface{}{"role": "assistant", "content": content})
		default:
			result = append(result, map[string]interface{}{
				"role":    msg.Role,
				"content": encodeAnthropicContentParts(msg.Content),
			})
		}
	}
	return result
}

func decodeAnthropicTools(raw json.RawMessage) []IRTool {
	var tools []map[string]interface{}
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	result := make([]IRTool, 0, len(tools))
	for _, tool := range tools {
		rawPayload, _ := json.Marshal(tool)
		parameters, _ := json.Marshal(tool["input_schema"])
		result = append(result, IRTool{
			Type:        "function",
			Subtype:     "function",
			Name:        stringValue(tool["name"]),
			Description: stringValue(tool["description"]),
			Parameters:  parameters,
			RawPayload:  rawPayload,
			ProviderExtensions: map[string]interface{}{
				"source_protocol": "anthropic",
			},
		})
	}
	return result
}

func encodeAnthropicTools(tools []IRTool) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if raw := decodeRawObject(tool.RawPayload); len(raw) > 0 && firstNonEmpty(tool.Type, "function") == "function" && toolSourceProtocol(tool) == "anthropic" {
			result = append(result, raw)
			continue
		}
		result = append(result, map[string]interface{}{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": decodeRawToAny(tool.Parameters),
		})
	}
	return result
}

func decodeGenericContent(content interface{}) []IRPart {
	switch c := content.(type) {
	case string:
		return []IRPart{{Type: "text", Text: c}}
	case []interface{}:
		parts := make([]IRPart, 0, len(c))
		for _, rawPart := range c {
			part, ok := rawPart.(map[string]interface{})
			if !ok {
				continue
			}
			rawType := firstNonEmpty(stringValue(part["type"]), "text")
			partType := normalizeIRPartType(rawType)
			irPart := IRPart{
				Type:               partType,
				Text:               stringValue(part["text"]),
				ProviderExtensions: map[string]interface{}{"raw": part},
			}
			switch rawType {
			case "refusal":
				irPart.Type = "refusal"
				irPart.Refusal = firstNonEmpty(stringValue(part["refusal"]), stringValue(part["text"]))
				irPart.Text = irPart.Refusal
			case "output_audio", "audio":
				irPart.Type = "audio"
				audioRaw, _ := json.Marshal(firstNonEmptyMap(part["audio"], part))
				irPart.Audio = audioRaw
			default:
				if annotations, ok := part["annotations"]; ok && annotations != nil {
					annotationsRaw, _ := json.Marshal(annotations)
					irPart.Annotations = annotationsRaw
				}
			}
			parts = append(parts, irPart)
		}
		return parts
	default:
		return nil
	}
}

func encodeTextContent(parts []IRPart) string {
	var b strings.Builder
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

func encodeOpenAIContent(parts []IRPart) interface{} {
	hasNonText := false
	for _, part := range parts {
		if part.Type == "image" || part.Type == "file" {
			hasNonText = true
			break
		}
	}
	if !hasNonText {
		return encodeTextContent(parts)
	}
	content := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			content = append(content, map[string]interface{}{"type": "text", "text": part.Text})
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "image_url", "file": "input_file"}[part.Type]}))
		}
	}
	return content
}

func encodeResponsesContent(parts []IRPart) []map[string]interface{} {
	content := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			content = append(content, map[string]interface{}{"type": "input_text", "text": part.Text})
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "input_image", "file": "input_file"}[part.Type]}))
		}
	}
	return content
}

func encodeAnthropicContentParts(parts []IRPart) []map[string]interface{} {
	content := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": "text", "text": part.Text}))
		case "image", "file":
			content = append(content, encodeRawOrFallbackPart(part, map[string]interface{}{"type": map[string]string{"image": "image", "file": "document"}[part.Type]}))
		}
	}
	return content
}

func encodeRawOrFallbackPart(part IRPart, fallback map[string]interface{}) map[string]interface{} {
	if raw, ok := part.ProviderExtensions["raw"].(map[string]interface{}); ok {
		return raw
	}
	return fallback
}

func addRawExtension(dst map[string]interface{}, key string, raw json.RawMessage) {
	if dst == nil || len(raw) == 0 || string(raw) == "null" {
		return
	}
	dst[key] = cloneRaw(raw)
}

func rawExtension(src map[string]interface{}, key string) json.RawMessage {
	if src == nil {
		return nil
	}
	switch value := src[key].(type) {
	case json.RawMessage:
		return cloneRaw(value)
	case []byte:
		return cloneRaw(value)
	default:
		if value == nil {
			return nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		return raw
	}
}

func decodeRawObject(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj
}

func toolSourceProtocol(tool IRTool) string {
	if tool.ProviderExtensions == nil {
		return ""
	}
	if source, ok := tool.ProviderExtensions["source_protocol"].(string); ok {
		return source
	}
	return ""
}

func normalizeOpenAIChatToolChoice(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cloneRaw(raw)
	}
	if _, ok := payload["function"].(map[string]interface{}); ok {
		return cloneRaw(raw)
	}
	name := firstNonEmpty(stringValue(payload["name"]), stringValue(asMap(payload["tool"])["name"]))
	switch stringValue(payload["type"]) {
	case "function", "tool":
		if name != "" {
			normalized, err := json.Marshal(map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": name,
				},
			})
			if err == nil {
				return normalized
			}
		}
	}
	return cloneRaw(raw)
}

func normalizeResponsesToolChoice(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cloneRaw(raw)
	}
	if _, ok := payload["name"]; ok {
		return cloneRaw(raw)
	}
	if function, ok := payload["function"].(map[string]interface{}); ok {
		name := stringValue(function["name"])
		if name != "" {
			normalized, err := json.Marshal(map[string]interface{}{
				"type": "function",
				"name": name,
			})
			if err == nil {
				return normalized
			}
		}
	}
	if stringValue(payload["type"]) == "tool" {
		name := stringValue(payload["name"])
		if name != "" {
			normalized, err := json.Marshal(map[string]interface{}{
				"type": "function",
				"name": name,
			})
			if err == nil {
				return normalized
			}
		}
	}
	return cloneRaw(raw)
}

func normalizeAnthropicToolChoice(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cloneRaw(raw)
	}
	if stringValue(payload["type"]) == "tool" {
		return cloneRaw(raw)
	}
	if function, ok := payload["function"].(map[string]interface{}); ok {
		name := stringValue(function["name"])
		if name != "" {
			normalized, err := json.Marshal(map[string]interface{}{
				"type": "tool",
				"name": name,
			})
			if err == nil {
				return normalized
			}
		}
	}
	if stringValue(payload["type"]) == "function" {
		name := stringValue(payload["name"])
		if name != "" {
			normalized, err := json.Marshal(map[string]interface{}{
				"type": "tool",
				"name": name,
			})
			if err == nil {
				return normalized
			}
		}
	}
	return cloneRaw(raw)
}

func normalizeOpenAIChatResponseFormat(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return cloneRaw(raw)
	}
	if format, ok := payload["format"].(map[string]interface{}); ok && len(format) > 0 {
		if stringValue(format["type"]) == "json_schema" {
			jsonSchema := map[string]interface{}{}
			for key, value := range format {
				if key == "type" {
					continue
				}
				jsonSchema[key] = value
			}
			normalized, err := json.Marshal(map[string]interface{}{
				"type":        "json_schema",
				"json_schema": jsonSchema,
			})
			if err == nil {
				return normalized
			}
		}
		normalized, err := json.Marshal(format)
		if err == nil {
			return normalized
		}
	}
	return cloneRaw(raw)
}

func normalizeIRPartType(partType string) string {
	switch {
	case strings.Contains(partType, "image"):
		return "image"
	case strings.Contains(partType, "file"), strings.Contains(partType, "document"):
		return "file"
	default:
		return partType
	}
}

func extractOutput(parts []IRPart) string {
	for _, part := range parts {
		if part.Output != "" {
			return part.Output
		}
	}
	return ""
}

func encodeToolCalls(parts []IRPart) []map[string]interface{} {
	toolCalls := make([]map[string]interface{}, 0)
	for _, part := range parts {
		if part.Type != "tool_call" {
			continue
		}
		toolCalls = append(toolCalls, map[string]interface{}{
			"id":   stringValue(part.ProviderExtensions["id"]),
			"type": "function",
			"function": map[string]interface{}{
				"name":      part.Name,
				"arguments": part.Arguments,
			},
		})
	}
	return toolCalls
}

func normalizeStop(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []string{single}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	return nil
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return json.RawMessage(out)
}

func decodeRawToAny(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}

func decodeRawOrString(raw string) interface{} {
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

func mustMarshalToString(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func numberToInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func numberToIntDefault(v interface{}) int {
	n, _ := numberToInt(v)
	return n
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
