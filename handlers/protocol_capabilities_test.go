package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"llm-gateway/models"
	"llm-gateway/protocol"
)

func TestDeriveCapabilityRequirementsDetectsResponsesBuiltinsAndPromptCache(t *testing.T) {
	rawBody := map[string]json.RawMessage{
		"tools":                  json.RawMessage(`[{"type":"web_search"},{"type":"image_generation"}]`),
		"prompt_cache_key":       json.RawMessage(`"cache-key"`),
		"prompt_cache_retention": json.RawMessage(`"persist"`),
	}

	reqs := deriveCapabilityRequirements(protocol.InboundProtocolResponses, rawBody)
	if !reqs.WebSearch || !reqs.ImageGeneration || !reqs.PromptCache || !reqs.ResponsesTools {
		t.Fatalf("expected builtin tool and prompt cache requirements, got %+v", reqs)
	}
}

func TestDeriveCapabilityRequirementsDetectsChatAudioOutput(t *testing.T) {
	rawBody := map[string]json.RawMessage{
		"modalities": json.RawMessage(`["text","audio"]`),
		"audio":      json.RawMessage(`{"voice":"alloy"}`),
	}

	reqs := deriveCapabilityRequirements(protocol.InboundProtocolChat, rawBody)
	if !reqs.AudioOutput {
		t.Fatalf("expected audio output requirement, got %+v", reqs)
	}
}

func TestDeriveCapabilityRequirementsDetectsPreviousResponseID(t *testing.T) {
	rawBody := map[string]json.RawMessage{
		"previous_response_id": json.RawMessage(`"resp_prev_1"`),
	}

	reqs := deriveCapabilityRequirements(protocol.InboundProtocolResponses, rawBody)
	if !reqs.PreviousResponse {
		t.Fatalf("expected previous_response_id requirement, got %+v", reqs)
	}
}

func TestDeriveCapabilityRequirementsDetectsResponsesOnlyFields(t *testing.T) {
	rawBody := map[string]json.RawMessage{
		"include":      json.RawMessage(`["reasoning.encrypted_content"]`),
		"store":        json.RawMessage(`true`),
		"background":   json.RawMessage(`false`),
		"conversation": json.RawMessage(`{"id":"conv_1"}`),
		"prompt":       json.RawMessage(`{"id":"pmpt_1"}`),
	}

	reqs := deriveCapabilityRequirements(protocol.InboundProtocolResponses, rawBody)
	if !reqs.Include || !reqs.Store || !reqs.Background || !reqs.Conversation || !reqs.Prompt {
		t.Fatalf("expected responses-only fields detected, got %+v", reqs)
	}
}

func TestValidateModelCapabilitiesRejectsResponsesBuiltinsForChatUpstream(t *testing.T) {
	config := models.ModelConfig{
		ModelName:         "chat-backend",
		UpstreamType:      models.UpstreamTypeOpenAIChat,
		SupportsTools:     true,
		SupportsStream:    true,
		SupportsWebSearch: true,
	}

	err := validateModelCapabilities(config, capabilityRequirements{
		Tools:          true,
		WebSearch:      true,
		ResponsesTools: true,
	})
	if err == nil || !strings.Contains(err.Error(), "responses_builtin_tools") {
		t.Fatalf("expected responses builtin tool rejection, got %v", err)
	}
}

func TestValidateModelCapabilitiesAcceptsSupportedResponsesBuiltins(t *testing.T) {
	config := models.ModelConfig{
		ModelName:               "responses-backend",
		UpstreamType:            models.UpstreamTypeOpenAIResponses,
		SupportsTools:           true,
		SupportsStream:          true,
		SupportsWebSearch:       true,
		SupportsImageGeneration: true,
		SupportsPromptCache:     true,
	}

	err := validateModelCapabilities(config, capabilityRequirements{
		Tools:           true,
		WebSearch:       true,
		ImageGeneration: true,
		PromptCache:     true,
		ResponsesTools:  true,
	})
	if err != nil {
		t.Fatalf("expected capability validation success, got %v", err)
	}
}

func TestValidateModelCapabilitiesRejectsUnsupportedAudioOutput(t *testing.T) {
	config := models.ModelConfig{
		ModelName:      "chat-backend",
		UpstreamType:   models.UpstreamTypeOpenAIChat,
		SupportsStream: true,
	}

	err := validateModelCapabilities(config, capabilityRequirements{
		AudioOutput: true,
	})
	if err == nil || !strings.Contains(err.Error(), "audio_output") {
		t.Fatalf("expected audio_output rejection, got %v", err)
	}
}

func TestValidateModelCapabilitiesRejectsPreviousResponseIDForNonResponsesUpstream(t *testing.T) {
	config := models.ModelConfig{
		ModelName:      "chat-backend",
		UpstreamType:   models.UpstreamTypeOpenAIChat,
		SupportsStream: true,
	}

	err := validateModelCapabilities(config, capabilityRequirements{
		PreviousResponse: true,
	})
	if err == nil || !strings.Contains(err.Error(), "previous_response_id") {
		t.Fatalf("expected previous_response_id rejection, got %v", err)
	}
}

func TestValidateModelCapabilitiesRejectsResponsesOnlyFieldsForNonResponsesUpstream(t *testing.T) {
	config := models.ModelConfig{
		ModelName:      "chat-backend",
		UpstreamType:   models.UpstreamTypeOpenAIChat,
		SupportsStream: true,
	}

	err := validateModelCapabilities(config, capabilityRequirements{
		Include:      true,
		Store:        true,
		Background:   true,
		Conversation: true,
		Prompt:       true,
	})
	if err == nil ||
		!strings.Contains(err.Error(), "include") ||
		!strings.Contains(err.Error(), "store") ||
		!strings.Contains(err.Error(), "background") ||
		!strings.Contains(err.Error(), "conversation") ||
		!strings.Contains(err.Error(), "prompt") {
		t.Fatalf("expected responses-only field rejection, got %v", err)
	}
}
