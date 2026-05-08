package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"llm-gateway/models"
)

const (
	defaultTraceLimit         = 500
	maxTraceLimit             = 3000
	defaultTraceWindowMinutes = 20
)

type InferredTraceResponse struct {
	Total  int64           `json:"total"`
	Traces []InferredTrace `json:"traces"`
}

type InferredTrace struct {
	TraceKey     string              `json:"trace_key"`
	RootLogID    uint                `json:"root_log_id"`
	StepCount    int                 `json:"step_count"`
	Confidence   float64             `json:"confidence"`
	StartAt      time.Time           `json:"start_at"`
	EndAt        time.Time           `json:"end_at"`
	DurationMS   int64               `json:"duration_ms"`
	ModelNames   []string            `json:"model_names"`
	BackendNames []string            `json:"backend_names"`
	Preview      string              `json:"preview"`
	Steps        []InferredTraceStep `json:"steps,omitempty"`
}

type InferredTraceStep struct {
	ID               uint      `json:"id"`
	ParentID         uint      `json:"parent_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ModelName        string    `json:"model_name"`
	BackendModelName string    `json:"backend_model_name"`
	ResponseTime     int64     `json:"response_time"`
	MessageCount     int       `json:"message_count"`
	RequestBytes     int       `json:"request_bytes"`
	ResponseBytes    int       `json:"response_bytes"`
	RequestKey       string    `json:"request_key"`
	RootKey          string    `json:"root_key"`
	MatchReason      string    `json:"match_reason"`
	Confidence       float64   `json:"confidence"`
	Preview          string    `json:"preview"`
}

type inferredTraceRow struct {
	ID               uint
	CreatedAt        time.Time
	ModelName        string
	BackendModelName string
	ResponseTime     int64
	Request          string
	RequestBytes     int
	ResponseBytes    int
}

type inferredLogFeature struct {
	inferredTraceRow
	MessageCount int
	RequestKey   string
	RootKey      string
	PrefixKeys   []prefixDigest
	Preview      string
}

type prefixDigest struct {
	Length int
	Key    string
}

type inferredEdge struct {
	ParentID   uint
	Reason     string
	Confidence float64
}

func (h *LogHandler) GetInferredTraces(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, defaultTraceLimit, "limit")
	if limit <= 0 {
		limit = defaultTraceLimit
	}
	if limit > maxTraceLimit {
		limit = maxTraceLimit
	}

	windowMinutes := queryInt(r, defaultTraceWindowMinutes, "window_minutes")
	if windowMinutes <= 0 {
		windowMinutes = defaultTraceWindowMinutes
	}

	includeSteps := queryBool(r, "include_steps")
	minSteps := queryInt(r, 2, "min_steps")
	if minSteps <= 0 {
		minSteps = 2
	}

	var rows []inferredTraceRow
	if err := h.DB.
		Model(&models.RequestLog{}).
		Select(`
			id,
			created_at,
			model_name,
			backend_model_name,
			response_time,
			request,
			COALESCE(LENGTH(request), 0) AS request_bytes,
			COALESCE(LENGTH(response), 0) AS response_bytes
		`).
		Where("request IS NOT NULL AND request <> ''").
		Order("created_at DESC, id DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].CreatedAt.Before(rows[j].CreatedAt)
	})

	features := make([]inferredLogFeature, 0, len(rows))
	for _, row := range rows {
		if feature, ok := buildInferredLogFeature(row); ok {
			features = append(features, feature)
		}
	}

	edges := inferRequestEdges(features, time.Duration(windowMinutes)*time.Minute)
	traces := buildInferredTraces(features, edges, minSteps, includeSteps)

	response := InferredTraceResponse{
		Total:  int64(len(traces)),
		Traces: traces,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func queryInt(r *http.Request, fallback int, names ...string) int {
	value := queryValue(r, names...)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryBool(r *http.Request, names ...string) bool {
	value := strings.ToLower(queryValue(r, names...))
	return value == "1" || value == "true" || value == "yes"
}

func buildInferredLogFeature(row inferredTraceRow) (inferredLogFeature, bool) {
	systemItems, messages := extractRequestMessages(row.Request)
	if len(messages) == 0 {
		return inferredLogFeature{}, false
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

	prefixes := make([]prefixDigest, 0, len(messages)-1)
	for i := 1; i < len(messages); i++ {
		prefix := map[string]interface{}{"messages": messages[:i]}
		if len(systemItems) > 0 {
			prefix["system"] = systemItems
		}
		prefixes = append(prefixes, prefixDigest{Length: i, Key: digestCanonical(prefix)})
	}

	return inferredLogFeature{
		inferredTraceRow: row,
		MessageCount:     len(messages),
		RequestKey:       digestCanonical(canonical),
		RootKey:          digestCanonical(rootParts),
		PrefixKeys:       prefixes,
		Preview:          inferredPreview(messages),
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

func shortDigest(value string) string {
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
			return truncateForPreview(text)
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

func inferRequestEdges(features []inferredLogFeature, window time.Duration) map[uint]inferredEdge {
	byRequestKey := make(map[string]inferredLogFeature)
	recentByRoot := make(map[string][]inferredLogFeature)
	edges := make(map[uint]inferredEdge)

	for _, feature := range features {
		var best *inferredLogFeature
		bestPrefixLen := -1
		for _, prefix := range feature.PrefixKeys {
			candidate, ok := byRequestKey[prefix.Key]
			if !ok || candidate.ID == feature.ID || candidate.CreatedAt.After(feature.CreatedAt) {
				continue
			}
			if prefix.Length > bestPrefixLen {
				copied := candidate
				best = &copied
				bestPrefixLen = prefix.Length
			}
		}

		if best != nil {
			edges[feature.ID] = inferredEdge{
				ParentID:   best.ID,
				Reason:     "prefix:" + strconv.Itoa(bestPrefixLen),
				Confidence: 0.98,
			}
		} else {
			for i := len(recentByRoot[feature.RootKey]) - 1; i >= 0; i-- {
				candidate := recentByRoot[feature.RootKey][i]
				if feature.CreatedAt.Sub(candidate.CreatedAt) > window {
					continue
				}
				grew := candidate.RequestBytes < feature.RequestBytes || candidate.MessageCount < feature.MessageCount
				if !grew {
					continue
				}
				edges[feature.ID] = inferredEdge{
					ParentID:   candidate.ID,
					Reason:     "root+time+len",
					Confidence: 0.72,
				}
				break
			}
		}

		byRequestKey[feature.RequestKey] = feature
		if feature.RootKey != "" {
			recentByRoot[feature.RootKey] = append(recentByRoot[feature.RootKey], feature)
		}
	}

	return edges
}

func buildInferredTraces(features []inferredLogFeature, edges map[uint]inferredEdge, minSteps int, includeSteps bool) []InferredTrace {
	byID := make(map[uint]inferredLogFeature, len(features))
	for _, feature := range features {
		byID[feature.ID] = feature
	}

	grouped := make(map[uint][]inferredLogFeature)
	for _, feature := range features {
		rootID := traceRootID(feature.ID, edges)
		grouped[rootID] = append(grouped[rootID], feature)
	}

	traces := make([]InferredTrace, 0, len(grouped))
	for rootID, steps := range grouped {
		if len(steps) < minSteps {
			continue
		}
		sort.Slice(steps, func(i, j int) bool {
			if steps[i].CreatedAt.Equal(steps[j].CreatedAt) {
				return steps[i].ID < steps[j].ID
			}
			return steps[i].CreatedAt.Before(steps[j].CreatedAt)
		})

		modelSet := make(map[string]bool)
		backendSet := make(map[string]bool)
		confidenceSum := 0.0
		edgeCount := 0
		traceSteps := make([]InferredTraceStep, 0, len(steps))

		for _, step := range steps {
			if step.ModelName != "" {
				modelSet[step.ModelName] = true
			}
			if step.BackendModelName != "" {
				backendSet[step.BackendModelName] = true
			}

			edge := edges[step.ID]
			stepConfidence := 1.0
			if edge.ParentID != 0 {
				stepConfidence = edge.Confidence
				confidenceSum += edge.Confidence
				edgeCount++
			}

			if includeSteps {
				traceSteps = append(traceSteps, InferredTraceStep{
					ID:               step.ID,
					ParentID:         edge.ParentID,
					CreatedAt:        step.CreatedAt,
					ModelName:        step.ModelName,
					BackendModelName: step.BackendModelName,
					ResponseTime:     step.ResponseTime,
					MessageCount:     step.MessageCount,
					RequestBytes:     step.RequestBytes,
					ResponseBytes:    step.ResponseBytes,
					RequestKey:       shortDigest(step.RequestKey),
					RootKey:          shortDigest(step.RootKey),
					MatchReason:      edge.Reason,
					Confidence:       stepConfidence,
					Preview:          step.Preview,
				})
			}
		}

		confidence := 1.0
		if edgeCount > 0 {
			confidence = confidenceSum / float64(edgeCount)
		}

		startAt := steps[0].CreatedAt
		endAt := steps[len(steps)-1].CreatedAt
		durationMS := endAt.Sub(startAt).Milliseconds()
		rootFeature := byID[rootID]
		if rootFeature.ID == 0 {
			rootFeature = steps[0]
		}

		trace := InferredTrace{
			TraceKey:     "inferred:" + shortDigest(rootFeature.RequestKey),
			RootLogID:    rootID,
			StepCount:    len(steps),
			Confidence:   confidence,
			StartAt:      startAt,
			EndAt:        endAt,
			DurationMS:   durationMS,
			ModelNames:   sortedKeys(modelSet),
			BackendNames: sortedKeys(backendSet),
			Preview:      rootFeature.Preview,
			Steps:        traceSteps,
		}
		traces = append(traces, trace)
	}

	sort.Slice(traces, func(i, j int) bool {
		return traces[i].EndAt.After(traces[j].EndAt)
	})

	return traces
}

func traceRootID(id uint, edges map[uint]inferredEdge) uint {
	seen := make(map[uint]bool)
	current := id
	for {
		edge, ok := edges[current]
		if !ok || edge.ParentID == 0 || seen[current] {
			return current
		}
		seen[current] = true
		current = edge.ParentID
	}
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
