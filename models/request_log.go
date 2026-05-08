package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RequestLog struct {
	ID                   uint      `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt            time.Time `gorm:"index" json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	ModelName            string    `gorm:"index;type:varchar(255)" json:"model_name"`
	BackendConfigID      uint      `gorm:"index" json:"backend_config_id"`
	BackendModelName     string    `gorm:"index;type:varchar(255)" json:"backend_model_name"`
	BackendAPIBaseURL    string    `gorm:"type:varchar(500)" json:"backend_api_base_url"`
	Fingerprint          string    `gorm:"index;type:varchar(32)" json:"fingerprint"`
	ResponseTime         int64     `json:"response_time"`
	FirstTokenLatency    int64     `json:"first_token_latency"`
	AvgTokenLatency      float64   `json:"avg_token_latency"`
	ActiveRequests       int       `json:"active_requests"`
	Request              string    `gorm:"type:longtext" json:"request"`
	Response             string    `gorm:"type:longtext" json:"response"`
	StreamResponse       []byte    `gorm:"type:longblob" json:"stream_response"`
	RequestBytes         int       `json:"request_bytes"`
	ResponseBytes        int       `json:"response_bytes"`
	StreamBytes          int       `json:"stream_bytes"`
	InferredTraceKey     string    `gorm:"index;type:varchar(80)" json:"inferred_trace_key"`
	InferredParentID     uint      `gorm:"index" json:"inferred_parent_id"`
	InferredRequestKey   string    `gorm:"index;type:varchar(64)" json:"inferred_request_key"`
	InferredRootKey      string    `gorm:"index;type:varchar(64)" json:"inferred_root_key"`
	InferredMatchReason  string    `gorm:"type:varchar(64)" json:"inferred_match_reason"`
	InferredConfidence   float64   `json:"inferred_confidence"`
	InferredMessageCount int       `gorm:"index" json:"inferred_message_count"`
	InferredPreview      string    `gorm:"type:varchar(512)" json:"inferred_preview"`
}

func (r *RequestLog) BeforeCreate(tx *gorm.DB) error {
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	r.PrepareInferenceMetadata()
	return nil
}

func (r *RequestLog) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	return nil
}

func (RequestLog) TableName() string {
	return "request_logs"
}

type RequestTraceMetadata struct {
	MessageCount int
	RequestKey   string
	RootKey      string
	PrefixKeys   []PrefixDigest
	Preview      string
}

type PrefixDigest struct {
	Length int
	Key    string
}

func (r *RequestLog) PrepareInferenceMetadata() RequestTraceMetadata {
	r.RequestBytes = len(r.Request)
	r.ResponseBytes = len(r.Response)
	r.StreamBytes = len(r.StreamResponse)

	metadata, ok := BuildRequestTraceMetadata(r.Request)
	if !ok {
		return RequestTraceMetadata{}
	}

	if r.InferredRequestKey == "" {
		r.InferredRequestKey = metadata.RequestKey
	}
	if r.InferredRootKey == "" {
		r.InferredRootKey = metadata.RootKey
	}
	if r.InferredMessageCount == 0 {
		r.InferredMessageCount = metadata.MessageCount
	}
	if r.InferredPreview == "" {
		r.InferredPreview = metadata.Preview
	}
	if r.InferredTraceKey == "" && metadata.RequestKey != "" {
		r.InferredTraceKey = "inferred:" + ShortDigest(metadata.RequestKey)
	}
	if r.InferredConfidence == 0 {
		r.InferredConfidence = 1
	}

	return metadata
}

func BuildRequestTraceMetadata(request string) (RequestTraceMetadata, bool) {
	systemItems, messages := extractRequestMessages(request)
	if len(messages) == 0 {
		return RequestTraceMetadata{}, false
	}

	canonical := map[string]interface{}{"messages": messages}
	if len(systemItems) > 0 {
		canonical["system"] = systemItems
	}

	rootParts := make([]interface{}, 0, len(systemItems)+1)
	for _, item := range systemItems {
		rootParts = append(rootParts, item)
	}
	if firstUser := firstUserMessage(messages); firstUser != nil {
		rootParts = append(rootParts, firstUser)
	}

	prefixes := make([]PrefixDigest, 0, len(messages)-1)
	for i := 1; i < len(messages); i++ {
		prefix := map[string]interface{}{"messages": messages[:i]}
		if len(systemItems) > 0 {
			prefix["system"] = systemItems
		}
		prefixes = append(prefixes, PrefixDigest{Length: i, Key: digestCanonical(prefix)})
	}

	return RequestTraceMetadata{
		MessageCount: len(messages),
		RequestKey:   digestCanonical(canonical),
		RootKey:      digestCanonical(rootParts),
		PrefixKeys:   prefixes,
		Preview:      inferredPreview(messages),
	}, true
}

func extractRequestMessages(request string) ([]interface{}, []interface{}) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(request), &payload); err != nil {
		return nil, nil
	}

	systemItems := make([]interface{}, 0, 1)
	if system, ok := payload["system"]; ok && system != nil {
		systemItems = append(systemItems, map[string]interface{}{
			"role":    "system",
			"content": normalizeTraceValue(system),
		})
	}

	messages := make([]interface{}, 0)
	if rawMessages, ok := payload["messages"].([]interface{}); ok {
		for _, item := range rawMessages {
			messages = append(messages, normalizeTraceMessage(item))
		}
	}

	if len(messages) == 0 {
		if prompt, ok := payload["prompt"]; ok && prompt != nil {
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": normalizeTraceValue(prompt),
			})
		}
	}

	if len(messages) > 0 {
		if first, ok := messages[0].(map[string]interface{}); ok {
			role, _ := first["role"].(string)
			if role == "system" || role == "developer" {
				systemItems = append(systemItems, first)
			}
		}
	}

	return systemItems, messages
}

func normalizeTraceMessage(value interface{}) interface{} {
	message, ok := value.(map[string]interface{})
	if !ok {
		return map[string]interface{}{"content": normalizeTraceValue(value)}
	}

	keepKeys := []string{"role", "name", "content", "tool_calls", "tool_call_id", "type"}
	normalized := make(map[string]interface{}, len(keepKeys))
	for _, key := range keepKeys {
		if field, ok := message[key]; ok {
			normalized[key] = normalizeTraceValue(field)
		}
	}
	return normalized
}

func normalizeTraceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return strings.Join(strings.Fields(strings.TrimSpace(typed)), " ")
	case []interface{}:
		items := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			items = append(items, normalizeTraceValue(item))
		}
		return items
	case map[string]interface{}:
		ignored := map[string]bool{
			"id":       true,
			"created":  true,
			"model":    true,
			"usage":    true,
			"logprobs": true,
		}
		normalized := make(map[string]interface{}, len(typed))
		for key, field := range typed {
			if ignored[key] {
				continue
			}
			normalized[key] = normalizeTraceValue(field)
		}
		return normalized
	default:
		return typed
	}
}

func digestCanonical(value interface{}) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ShortDigest(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}

func firstUserMessage(messages []interface{}) interface{} {
	for _, item := range messages {
		message, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := message["role"].(string); role == "user" {
			return item
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return messages[0]
}

func inferredPreview(messages []interface{}) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		if role, _ := message["role"].(string); role != "user" {
			continue
		}
		if text := traceContentPreview(message["content"]); text != "" {
			return truncateTracePreview(text)
		}
	}
	return ""
}

func traceContentPreview(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []interface{}:
		for i := len(typed) - 1; i >= 0; i-- {
			if text := traceContentPreview(typed[i]); text != "" {
				return text
			}
		}
	case map[string]interface{}:
		if text, ok := typed["text"].(string); ok {
			return text
		}
		if text, ok := typed["content"].(string); ok {
			return text
		}
	}
	return ""
}

func truncateTracePreview(value string) string {
	const limit = 120
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
