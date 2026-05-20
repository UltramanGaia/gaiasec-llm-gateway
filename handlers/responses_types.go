package handlers

import "encoding/json"

type ResponsesRequest struct {
	Model                string          `json:"model"`
	Input                json.RawMessage `json:"input,omitempty"`
	Instructions         string          `json:"instructions,omitempty"`
	MaxOutputTokens      int             `json:"max_output_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	Stop                 json.RawMessage `json:"stop,omitempty"`
	Tools                []ResponsesTool `json:"tools,omitempty"`
	ToolChoice           json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool           `json:"parallel_tool_calls,omitempty"`
	Stream               bool            `json:"stream,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	PreviousResponseID   string          `json:"previous_response_id,omitempty"`
	Include              []string        `json:"include,omitempty"`
	Reasoning            map[string]any  `json:"reasoning,omitempty"`
	Text                 map[string]any  `json:"text,omitempty"`
	ServiceTier          string          `json:"service_tier,omitempty"`
	ClientMetadata       map[string]any  `json:"client_metadata,omitempty"`
	Metadata             map[string]any  `json:"metadata,omitempty"`
	User                 string          `json:"user,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
}

type ResponsesTool struct {
	Type               string          `json:"type"`
	Name               string          `json:"name,omitempty"`
	Description        string          `json:"description,omitempty"`
	Parameters         map[string]any  `json:"parameters,omitempty"`
	Strict             *bool           `json:"strict,omitempty"`
	Format             map[string]any  `json:"format,omitempty"`
	Tools              []ResponsesTool `json:"tools,omitempty"`
	ExternalWebAccess  *bool           `json:"external_web_access,omitempty"`
	SearchContentTypes []string        `json:"search_content_types,omitempty"`
	MaxNumResults      int             `json:"max_num_results,omitempty"`
	DisplayWidth       int             `json:"display_width,omitempty"`
	DisplayHeight      int             `json:"display_height,omitempty"`
}

type ResponsesResponse struct {
	ID                string                      `json:"id"`
	Object            string                      `json:"object"`
	CreatedAt         int64                       `json:"created_at,omitempty"`
	Status            string                      `json:"status"`
	Model             string                      `json:"model,omitempty"`
	Output            []ResponsesOutputItem       `json:"output"`
	OutputText        string                      `json:"output_text,omitempty"`
	Usage             ResponsesUsage              `json:"usage,omitempty"`
	Metadata          map[string]any              `json:"metadata,omitempty"`
	IncompleteDetails *ResponsesIncompleteDetails `json:"incomplete_details,omitempty"`
	Error             *ResponsesErrorObject       `json:"error,omitempty"`
}

type ResponsesOutputItem struct {
	Type      string                      `json:"type"`
	ID        string                      `json:"id,omitempty"`
	Status    string                      `json:"status,omitempty"`
	Role      string                      `json:"role,omitempty"`
	Content   []ResponsesContentPart      `json:"content,omitempty"`
	CallID    string                      `json:"call_id,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Namespace string                      `json:"namespace,omitempty"`
	Arguments string                      `json:"arguments,omitempty"`
	Input     string                      `json:"input,omitempty"`
	Action    *ResponsesToolAction        `json:"action,omitempty"`
	Summary   []ResponsesReasoningSummary `json:"summary,omitempty"`
}

type ResponsesToolAction struct {
	Type             string            `json:"type,omitempty"`
	Command          []string          `json:"command,omitempty"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	TimeoutMS        int               `json:"timeout_ms,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Query            string            `json:"query,omitempty"`
	Queries          []string          `json:"queries,omitempty"`
	URL              string            `json:"url,omitempty"`
	Pattern          string            `json:"pattern,omitempty"`
}

type ResponsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ResponsesUsage struct {
	InputTokens         int                          `json:"input_tokens,omitempty"`
	OutputTokens        int                          `json:"output_tokens,omitempty"`
	TotalTokens         int                          `json:"total_tokens"`
	InputTokensDetails  ResponsesInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokensDetails ResponsesOutputTokensDetails `json:"output_tokens_details,omitempty"`
}

type ResponsesInputTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type ResponsesOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type ResponsesIncompleteDetails struct {
	Reason string `json:"reason"`
}

type ResponsesErrorResponse struct {
	Error ResponsesErrorObject `json:"error"`
}

type ResponsesErrorObject struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

type ResponsesReasoningSummary struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Signature string `json:"signature,omitempty"`
}
