package protocol

import "encoding/json"

type InboundProtocol string

const (
	InboundProtocolChat      InboundProtocol = "chat"
	InboundProtocolResponses InboundProtocol = "responses"
	InboundProtocolAnthropic InboundProtocol = "anthropic_messages"
)

type DispatchMode string

const (
	DispatchPassthrough DispatchMode = "passthrough"
	DispatchTransform   DispatchMode = "transform"
	DispatchUnsupported DispatchMode = "unsupported"
)

type IRRequest struct {
	Model                string                 `json:"model,omitempty"`
	SystemInstruction    string                 `json:"system_instruction,omitempty"`
	Messages             []IRMessage            `json:"messages,omitempty"`
	Tools                []IRTool               `json:"tools,omitempty"`
	ToolChoice           json.RawMessage        `json:"tool_choice,omitempty"`
	PreviousResponseID   string                 `json:"previous_response_id,omitempty"`
	Include              []string               `json:"include,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
	ServiceTier          string                 `json:"service_tier,omitempty"`
	Store                *bool                  `json:"store,omitempty"`
	Background           *bool                  `json:"background,omitempty"`
	Conversation         json.RawMessage        `json:"conversation,omitempty"`
	Prompt               json.RawMessage        `json:"prompt,omitempty"`
	PromptCacheKey       string                 `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                 `json:"prompt_cache_retention,omitempty"`
	Generation           IRGeneration           `json:"generation,omitempty"`
	ResponseFormat       json.RawMessage        `json:"response_format,omitempty"`
	Reasoning            json.RawMessage        `json:"reasoning,omitempty"`
	Stream               bool                   `json:"stream,omitempty"`
	ProviderExtensions   map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRMessage struct {
	ID                 string                 `json:"id,omitempty"`
	Type               string                 `json:"type,omitempty"`
	Role               string                 `json:"role,omitempty"`
	Status             string                 `json:"status,omitempty"`
	Content            []IRPart               `json:"content,omitempty"`
	ToolCallID         string                 `json:"tool_call_id,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRPart struct {
	Type               string                 `json:"type,omitempty"`
	ID                 string                 `json:"id,omitempty"`
	CallID             string                 `json:"call_id,omitempty"`
	Status             string                 `json:"status,omitempty"`
	Text               string                 `json:"text,omitempty"`
	Refusal            string                 `json:"refusal,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Arguments          string                 `json:"arguments,omitempty"`
	Output             string                 `json:"output,omitempty"`
	Annotations        json.RawMessage        `json:"annotations,omitempty"`
	Audio              json.RawMessage        `json:"audio,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRTool struct {
	Type               string                 `json:"type,omitempty"`
	Subtype            string                 `json:"subtype,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Description        string                 `json:"description,omitempty"`
	Parameters         json.RawMessage        `json:"parameters,omitempty"`
	RawPayload         json.RawMessage        `json:"raw_payload,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRGeneration struct {
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type IRResponse struct {
	ID                 string                 `json:"id,omitempty"`
	Model              string                 `json:"model,omitempty"`
	Status             string                 `json:"status,omitempty"`
	OutputItems        []IRMessage            `json:"output_items,omitempty"`
	Usage              json.RawMessage        `json:"usage,omitempty"`
	FinishReason       string                 `json:"finish_reason,omitempty"`
	Metadata           json.RawMessage        `json:"metadata,omitempty"`
	Error              json.RawMessage        `json:"error,omitempty"`
	IncompleteDetails  json.RawMessage        `json:"incomplete_details,omitempty"`
	Conversation       json.RawMessage        `json:"conversation,omitempty"`
	Prompt             json.RawMessage        `json:"prompt,omitempty"`
	TextConfig         json.RawMessage        `json:"text_config,omitempty"`
	ReasoningConfig    json.RawMessage        `json:"reasoning_config,omitempty"`
	ToolChoice         json.RawMessage        `json:"tool_choice,omitempty"`
	Tools              json.RawMessage        `json:"tools,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRStreamEvent struct {
	Type               string                 `json:"type,omitempty"`
	Index              int                    `json:"index,omitempty"`
	ItemID             string                 `json:"item_id,omitempty"`
	ContentIndex       int                    `json:"content_index,omitempty"`
	CallID             string                 `json:"call_id,omitempty"`
	Status             string                 `json:"status,omitempty"`
	Delta              string                 `json:"delta,omitempty"`
	Text               string                 `json:"text,omitempty"`
	Refusal            string                 `json:"refusal,omitempty"`
	Arguments          string                 `json:"arguments,omitempty"`
	Annotations        json.RawMessage        `json:"annotations,omitempty"`
	Audio              json.RawMessage        `json:"audio,omitempty"`
	Item               json.RawMessage        `json:"item,omitempty"`
	Usage              json.RawMessage        `json:"usage,omitempty"`
	FinishReason       string                 `json:"finish_reason,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}
