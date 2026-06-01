package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/models"
	"llm-gateway/protocol"
)

type capabilityRequirements struct {
	Stream            bool
	Tools             bool
	Reasoning         bool
	JSONSchema        bool
	Vision            bool
	AudioOutput       bool
	PreviousResponse  bool
	Include           bool
	Store             bool
	Background        bool
	Conversation      bool
	Prompt            bool
	ParallelToolCalls bool
	WebSearch         bool
	MCP               bool
	CodeInterpreter   bool
	ImageGeneration   bool
	PromptCache       bool
	ResponsesTools    bool
}

func deriveCapabilityRequirements(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) capabilityRequirements {
	reqs := capabilityRequirements{}
	reqs.Stream = boolField(rawBody["stream"])
	reqs.Tools = requestUsesTools(inbound, rawBody)
	reqs.Reasoning = requestUsesReasoning(inbound, rawBody)
	reqs.JSONSchema = requestUsesJSONSchema(inbound, rawBody)
	reqs.Vision = requestUsesVision(inbound, rawBody)
	reqs.AudioOutput = requestUsesAudioOutput(inbound, rawBody)
	reqs.PreviousResponse = requestUsesPreviousResponseID(inbound, rawBody)
	reqs.Include = requestUsesInclude(inbound, rawBody)
	reqs.Store = requestUsesStore(inbound, rawBody)
	reqs.Background = requestUsesBackground(inbound, rawBody)
	reqs.Conversation = requestUsesConversation(inbound, rawBody)
	reqs.Prompt = requestUsesPrompt(inbound, rawBody)
	reqs.ParallelToolCalls = requestUsesParallelToolCalls(inbound, rawBody)
	reqs.WebSearch = requestUsesBuiltInToolType(rawBody, "web_search", "web_search_preview")
	reqs.MCP = requestUsesBuiltInToolType(rawBody, "mcp")
	reqs.CodeInterpreter = requestUsesBuiltInToolType(rawBody, "code_interpreter")
	reqs.ImageGeneration = requestUsesBuiltInToolType(rawBody, "image_generation")
	reqs.PromptCache = requestUsesPromptCache(inbound, rawBody)
	reqs.ResponsesTools = requestUsesResponsesBuiltinTools(rawBody)
	return reqs
}

func validateModelCapabilities(config models.ModelConfig, reqs capabilityRequirements) error {
	unsupported := make([]string, 0, 5)
	if reqs.Stream && !config.SupportsStream {
		unsupported = append(unsupported, "stream")
	}
	if reqs.Tools && !config.SupportsTools {
		unsupported = append(unsupported, "tools")
	}
	if reqs.Reasoning && !config.SupportsReasoning {
		unsupported = append(unsupported, "reasoning")
	}
	if reqs.JSONSchema && !config.SupportsJSONSchema {
		unsupported = append(unsupported, "json_schema")
	}
	if reqs.Vision && !config.SupportsVision {
		unsupported = append(unsupported, "vision")
	}
	if reqs.AudioOutput && !config.SupportsAudioOutput {
		unsupported = append(unsupported, "audio_output")
	}
	if reqs.PreviousResponse && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "previous_response_id")
	}
	if reqs.Include && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "include")
	}
	if reqs.Store && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "store")
	}
	if reqs.Background && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "background")
	}
	if reqs.Conversation && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "conversation")
	}
	if reqs.Prompt && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "prompt")
	}
	if reqs.WebSearch && !config.SupportsWebSearch {
		unsupported = append(unsupported, "web_search")
	}
	if reqs.MCP && !config.SupportsMCP {
		unsupported = append(unsupported, "mcp")
	}
	if reqs.CodeInterpreter && !config.SupportsCodeInterpreter {
		unsupported = append(unsupported, "code_interpreter")
	}
	if reqs.ImageGeneration && !config.SupportsImageGeneration {
		unsupported = append(unsupported, "image_generation")
	}
	if reqs.PromptCache && !config.SupportsPromptCache {
		unsupported = append(unsupported, "prompt_cache")
	}
	if reqs.ResponsesTools && config.UpstreamType != models.UpstreamTypeOpenAIResponses {
		unsupported = append(unsupported, "responses_builtin_tools")
	}
	if len(unsupported) == 0 {
		return nil
	}
	return fmt.Errorf("backend %q does not support: %s", config.ModelName, strings.Join(unsupported, ", "))
}

func filterConfigsByCapabilities(configs []models.ModelConfig, reqs capabilityRequirements) ([]models.ModelConfig, error) {
	filtered := make([]models.ModelConfig, 0, len(configs))
	var firstErr error
	for _, config := range configs {
		if err := validateModelCapabilities(config, reqs); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		filtered = append(filtered, config)
	}
	if len(filtered) == 0 {
		if firstErr == nil {
			firstErr = fmt.Errorf("no backend satisfies requested capabilities")
		}
		return nil, firstErr
	}
	return filtered, nil
}

func normalizeEnvelopeForConfig(envelope protocolRequestEnvelope, config models.ModelConfig, inbound protocol.InboundProtocol) protocolRequestEnvelope {
	if config.SupportsParallelToolCalls {
		return envelope
	}

	updatedRaw := normalizeParallelToolCallsRawBody(envelope.rawBody, inbound)
	updatedBody, err := json.Marshal(updatedRaw)
	if err != nil {
		return envelope
	}
	return protocolRequestEnvelope{
		body:      updatedBody,
		rawBody:   updatedRaw,
		modelName: envelope.modelName,
		isStream:  envelope.isStream,
	}
}

func normalizeParallelToolCallsRawBody(rawBody map[string]json.RawMessage, inbound protocol.InboundProtocol) map[string]json.RawMessage {
	if len(rawBody) == 0 {
		return rawBody
	}
	bodyCopy := make(map[string]json.RawMessage, len(rawBody))
	for k, v := range rawBody {
		bodyCopy[k] = v
	}

	switch inbound {
	case protocol.InboundProtocolChat, protocol.InboundProtocolResponses:
		if _, ok := bodyCopy["parallel_tool_calls"]; ok {
			bodyCopy["parallel_tool_calls"] = json.RawMessage("false")
		}
	case protocol.InboundProtocolAnthropic:
		if rawChoice, ok := bodyCopy["tool_choice"]; ok && len(rawChoice) > 0 {
			var choice map[string]any
			if err := json.Unmarshal(rawChoice, &choice); err == nil {
				choice["disable_parallel_tool_use"] = true
				if updated, err := json.Marshal(choice); err == nil {
					bodyCopy["tool_choice"] = updated
				}
			}
		}
	}

	return bodyCopy
}

func boolField(raw json.RawMessage) bool {
	var v bool
	return len(raw) > 0 && json.Unmarshal(raw, &v) == nil && v
}

func requestUsesTools(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	if toolsRaw, ok := rawBody["tools"]; ok && len(toolsRaw) > 0 && string(toolsRaw) != "null" && string(toolsRaw) != "[]" {
		return true
	}
	if inbound == protocol.InboundProtocolAnthropic {
		if rawChoice, ok := rawBody["tool_choice"]; ok && len(rawChoice) > 0 && string(rawChoice) != "null" {
			return true
		}
	}
	return false
}

func requestUsesReasoning(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	if raw, ok := rawBody["reasoning"]; ok && len(raw) > 0 && string(raw) != "null" && string(raw) != "{}" {
		return true
	}
	if inbound == protocol.InboundProtocolAnthropic {
		if raw, ok := rawBody["thinking"]; ok && len(raw) > 0 && string(raw) != "null" && string(raw) != "{}" {
			return true
		}
	}
	return false
}

func requestUsesJSONSchema(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	switch inbound {
	case protocol.InboundProtocolChat:
		if raw, ok := rawBody["response_format"]; ok && len(raw) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err == nil && stringValue(payload["type"]) == "json_schema" {
				return true
			}
		}
	case protocol.InboundProtocolResponses:
		if raw, ok := rawBody["text"]; ok && len(raw) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err == nil {
				if format, ok := payload["format"].(map[string]any); ok && stringValue(format["type"]) == "json_schema" {
					return true
				}
			}
		}
	}
	return false
}

func requestUsesVision(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	switch inbound {
	case protocol.InboundProtocolChat:
		return chatMessagesUseVision(rawBody["messages"])
	case protocol.InboundProtocolResponses:
		return responsesInputUsesVision(rawBody["input"])
	case protocol.InboundProtocolAnthropic:
		return anthropicMessagesUseVision(rawBody["messages"])
	default:
		return false
	}
}

func requestUsesAudioOutput(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	switch inbound {
	case protocol.InboundProtocolChat:
		if raw, ok := rawBody["audio"]; ok && hasUsableRawField(raw) {
			return true
		}
		if raw, ok := rawBody["modalities"]; ok && len(raw) > 0 {
			var modalities []string
			if err := json.Unmarshal(raw, &modalities); err == nil {
				for _, modality := range modalities {
					if strings.EqualFold(strings.TrimSpace(modality), "audio") {
						return true
					}
				}
			}
		}
	case protocol.InboundProtocolResponses:
		if raw, ok := rawBody["modalities"]; ok && len(raw) > 0 {
			var modalities []string
			if err := json.Unmarshal(raw, &modalities); err == nil {
				for _, modality := range modalities {
					if strings.EqualFold(strings.TrimSpace(modality), "audio") {
						return true
					}
				}
			}
		}
	}
	return false
}

func requestUsesPreviousResponseID(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	if inbound != protocol.InboundProtocolResponses {
		return false
	}
	if raw, ok := rawBody["previous_response_id"]; ok && hasUsableRawField(raw) {
		return true
	}
	return false
}

func requestUsesInclude(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	return inbound == protocol.InboundProtocolResponses && hasUsableArrayField(rawBody["include"])
}

func requestUsesStore(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	return inbound == protocol.InboundProtocolResponses && hasUsableRawField(rawBody["store"])
}

func requestUsesBackground(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	return inbound == protocol.InboundProtocolResponses && hasUsableRawField(rawBody["background"])
}

func requestUsesConversation(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	return inbound == protocol.InboundProtocolResponses && hasUsableRawField(rawBody["conversation"])
}

func requestUsesPrompt(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	return inbound == protocol.InboundProtocolResponses && hasUsableRawField(rawBody["prompt"])
}

func requestUsesParallelToolCalls(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	if inbound == protocol.InboundProtocolAnthropic {
		if rawChoice, ok := rawBody["tool_choice"]; ok && len(rawChoice) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(rawChoice, &payload); err == nil {
				if disabled, ok := payload["disable_parallel_tool_use"].(bool); ok {
					return !disabled
				}
			}
		}
		return false
	}
	return boolField(rawBody["parallel_tool_calls"])
}

func requestUsesPromptCache(inbound protocol.InboundProtocol, rawBody map[string]json.RawMessage) bool {
	if inbound != protocol.InboundProtocolResponses {
		return false
	}
	return hasUsableRawField(rawBody["prompt_cache_key"]) || hasUsableRawField(rawBody["prompt_cache_retention"])
}

func requestUsesResponsesBuiltinTools(rawBody map[string]json.RawMessage) bool {
	toolTypes := decodeToolTypes(rawBody["tools"])
	for _, toolType := range toolTypes {
		if toolType != "" && toolType != "function" {
			return true
		}
	}
	return false
}

func requestUsesBuiltInToolType(rawBody map[string]json.RawMessage, candidates ...string) bool {
	toolTypes := decodeToolTypes(rawBody["tools"])
	for _, toolType := range toolTypes {
		for _, candidate := range candidates {
			if toolType == candidate {
				return true
			}
		}
	}
	return false
}

func decodeToolTypes(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tools []map[string]any
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		result = append(result, stringValue(tool["type"]))
	}
	return result
}

func hasUsableRawField(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed != "\"\""
}

func hasUsableArrayField(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed != "[]"
}

func chatMessagesUseVision(raw json.RawMessage) bool {
	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return false
	}
	for _, msg := range messages {
		switch content := msg["content"].(type) {
		case []any:
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]any)
				if !ok {
					continue
				}
				partType := stringValue(part["type"])
				if strings.Contains(partType, "image") || strings.Contains(partType, "file") {
					return true
				}
			}
		}
	}
	return false
}

func responsesInputUsesVision(raw json.RawMessage) bool {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	for _, item := range items {
		if content, ok := item["content"].([]any); ok {
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]any)
				if !ok {
					continue
				}
				partType := stringValue(part["type"])
				if strings.Contains(partType, "image") || strings.Contains(partType, "file") {
					return true
				}
			}
		}
	}
	return false
}

func anthropicMessagesUseVision(raw json.RawMessage) bool {
	var messages []map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		return false
	}
	for _, msg := range messages {
		if content, ok := msg["content"].([]any); ok {
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]any)
				if !ok {
					continue
				}
				partType := stringValue(part["type"])
				if strings.Contains(partType, "image") || strings.Contains(partType, "file") {
					return true
				}
			}
		}
	}
	return false
}
