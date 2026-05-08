package handlers

import (
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

	includeSteps := queryBool(r, "include_steps")
	minSteps := queryInt(r, 2, "min_steps")
	if minSteps <= 0 {
		minSteps = 2
	}

	if err := h.materializeRecentInferenceLogs(limit); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var rows []materializedTraceRow
	if err := h.DB.
		Model(&models.RequestLog{}).
		Select(`
			id,
			created_at,
			model_name,
			backend_model_name,
			response_time,
			inferred_trace_key,
			inferred_parent_id,
			inferred_request_key,
			inferred_root_key,
			inferred_match_reason,
			inferred_confidence,
			inferred_message_count,
			inferred_preview,
			COALESCE(NULLIF(request_bytes, 0), LENGTH(request), 0) AS request_bytes,
			COALESCE(NULLIF(response_bytes, 0), LENGTH(response), 0) AS response_bytes
		`).
		Where("inferred_trace_key <> '' AND inferred_message_count > 0").
		Order("created_at DESC, id DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	traces := buildMaterializedInferredTraces(rows, minSteps, includeSteps)

	response := InferredTraceResponse{
		Total:  int64(len(traces)),
		Traces: traces,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LogHandler) materializeRecentInferenceLogs(limit int) error {
	var missing []models.RequestLog
	if err := h.DB.
		Model(&models.RequestLog{}).
		Select("id, created_at, request, response, stream_response").
		Where("request IS NOT NULL AND request <> '' AND (inferred_request_key = '' OR inferred_request_key IS NULL)").
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&missing).Error; err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}

	logs := make([]*models.RequestLog, 0, len(missing))
	for index := range missing {
		reqLog := &missing[index]
		reqLog.PrepareInferenceMetadata()
		if reqLog.InferredRequestKey == "" {
			continue
		}
		if err := h.DB.Model(&models.RequestLog{}).
			Where("id = ?", reqLog.ID).
			Updates(map[string]interface{}{
				"request_bytes":          reqLog.RequestBytes,
				"response_bytes":         reqLog.ResponseBytes,
				"stream_bytes":           reqLog.StreamBytes,
				"inferred_trace_key":     reqLog.InferredTraceKey,
				"inferred_request_key":   reqLog.InferredRequestKey,
				"inferred_root_key":      reqLog.InferredRootKey,
				"inferred_confidence":    reqLog.InferredConfidence,
				"inferred_message_count": reqLog.InferredMessageCount,
				"inferred_preview":       reqLog.InferredPreview,
			}).Error; err != nil {
			return err
		}
		logs = append(logs, reqLog)
	}

	writer := &AsyncLogWriter{db: h.DB}
	writer.resolveInferenceLinks(logs)
	return nil
}

type materializedTraceRow struct {
	ID                   uint
	CreatedAt            time.Time
	ModelName            string
	BackendModelName     string
	ResponseTime         int64
	InferredTraceKey     string
	InferredParentID     uint
	InferredRequestKey   string
	InferredRootKey      string
	InferredMatchReason  string
	InferredConfidence   float64
	InferredMessageCount int
	InferredPreview      string
	RequestBytes         int
	ResponseBytes        int
}

func buildMaterializedInferredTraces(rows []materializedTraceRow, minSteps int, includeSteps bool) []InferredTrace {
	grouped := make(map[string][]materializedTraceRow)
	for _, row := range rows {
		if row.InferredTraceKey == "" {
			continue
		}
		grouped[row.InferredTraceKey] = append(grouped[row.InferredTraceKey], row)
	}

	traces := make([]InferredTrace, 0, len(grouped))
	for traceKey, steps := range grouped {
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

		root := steps[0]
		for _, step := range steps {
			if step.InferredParentID == 0 {
				root = step
				break
			}
		}

		for _, step := range steps {
			if step.ModelName != "" {
				modelSet[step.ModelName] = true
			}
			if step.BackendModelName != "" {
				backendSet[step.BackendModelName] = true
			}

			stepConfidence := step.InferredConfidence
			if stepConfidence == 0 {
				stepConfidence = 1
			}
			if step.InferredParentID != 0 {
				confidenceSum += stepConfidence
				edgeCount++
			}

			if includeSteps {
				traceSteps = append(traceSteps, InferredTraceStep{
					ID:               step.ID,
					ParentID:         step.InferredParentID,
					CreatedAt:        step.CreatedAt,
					ModelName:        step.ModelName,
					BackendModelName: step.BackendModelName,
					ResponseTime:     step.ResponseTime,
					MessageCount:     step.InferredMessageCount,
					RequestBytes:     step.RequestBytes,
					ResponseBytes:    step.ResponseBytes,
					RequestKey:       shortDigest(step.InferredRequestKey),
					RootKey:          shortDigest(step.InferredRootKey),
					MatchReason:      step.InferredMatchReason,
					Confidence:       stepConfidence,
					Preview:          step.InferredPreview,
				})
			}
		}

		confidence := 1.0
		if edgeCount > 0 {
			confidence = confidenceSum / float64(edgeCount)
		}
		startAt := steps[0].CreatedAt
		endAt := steps[len(steps)-1].CreatedAt

		traces = append(traces, InferredTrace{
			TraceKey:     traceKey,
			RootLogID:    root.ID,
			StepCount:    len(steps),
			Confidence:   confidence,
			StartAt:      startAt,
			EndAt:        endAt,
			DurationMS:   endAt.Sub(startAt).Milliseconds(),
			ModelNames:   sortedKeys(modelSet),
			BackendNames: sortedKeys(backendSet),
			Preview:      root.InferredPreview,
			Steps:        traceSteps,
		})
	}

	sort.Slice(traces, func(i, j int) bool {
		return traces[i].EndAt.After(traces[j].EndAt)
	})
	return traces
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
	metadata, ok := models.BuildRequestTraceMetadata(row.Request)
	if !ok {
		return inferredLogFeature{}, false
	}

	prefixes := make([]prefixDigest, 0, len(metadata.PrefixKeys))
	for _, prefix := range metadata.PrefixKeys {
		prefixes = append(prefixes, prefixDigest{Length: prefix.Length, Key: prefix.Key})
	}

	return inferredLogFeature{
		inferredTraceRow: row,
		MessageCount:     metadata.MessageCount,
		RequestKey:       metadata.RequestKey,
		RootKey:          metadata.RootKey,
		PrefixKeys:       prefixes,
		Preview:          metadata.Preview,
	}, true
}

func shortDigest(value string) string {
	return models.ShortDigest(value)
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
