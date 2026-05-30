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
	Model              string                 `json:"model,omitempty"`
	SystemInstruction  string                 `json:"system_instruction,omitempty"`
	Messages           []IRMessage            `json:"messages,omitempty"`
	Tools              []IRTool               `json:"tools,omitempty"`
	ToolChoice         json.RawMessage        `json:"tool_choice,omitempty"`
	Generation         IRGeneration           `json:"generation,omitempty"`
	ResponseFormat     json.RawMessage        `json:"response_format,omitempty"`
	Reasoning          json.RawMessage        `json:"reasoning,omitempty"`
	Stream             bool                   `json:"stream,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRMessage struct {
	Role               string                 `json:"role,omitempty"`
	Content            []IRPart               `json:"content,omitempty"`
	ToolCallID         string                 `json:"tool_call_id,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRPart struct {
	Type               string                 `json:"type,omitempty"`
	Text               string                 `json:"text,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Arguments          string                 `json:"arguments,omitempty"`
	Output             string                 `json:"output,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRTool struct {
	Type               string                 `json:"type,omitempty"`
	Name               string                 `json:"name,omitempty"`
	Description        string                 `json:"description,omitempty"`
	Parameters         json.RawMessage        `json:"parameters,omitempty"`
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
	OutputItems        []IRMessage            `json:"output_items,omitempty"`
	Usage              json.RawMessage        `json:"usage,omitempty"`
	FinishReason       string                 `json:"finish_reason,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}

type IRStreamEvent struct {
	Type               string                 `json:"type,omitempty"`
	Index              int                    `json:"index,omitempty"`
	Delta              string                 `json:"delta,omitempty"`
	Usage              json.RawMessage        `json:"usage,omitempty"`
	FinishReason       string                 `json:"finish_reason,omitempty"`
	ProviderExtensions map[string]interface{} `json:"provider_extensions,omitempty"`
}
