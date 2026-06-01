package handlers

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"llm-gateway/models"
	"llm-gateway/protocol"
)

// LogHandler 处理RequestLog相关的API请求
type LogHandler struct {
	DB *gorm.DB
}

// NewLogHandler 创建LogHandler的新实例
func NewLogHandler(db *gorm.DB) *LogHandler {
	return &LogHandler{
		DB: db,
	}
}

// LogResponse 定义日志响应结构，包含分页信息
type LogResponse struct {
	Total int64        `json:"total"`
	Logs  []LogSummary `json:"logs"`
}

type LogSummary struct {
	ID                uint               `json:"id"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	ModelName         string             `json:"model_name"`
	BackendConfigID   uint               `json:"backend_config_id"`
	BackendModelName  string             `json:"backend_model_name"`
	BackendAPIBaseURL string             `json:"backend_api_base_url"`
	Fingerprint       string             `json:"fingerprint"`
	ResponseTime      int64              `json:"response_time"`
	FirstTokenLatency int64              `json:"first_token_latency"`
	AvgTokenLatency   float64            `json:"avg_token_latency"`
	ActiveRequests    int                `json:"active_requests"`
	RequestPreview    string             `json:"request_preview"`
	RequestBytes      int                `json:"request_bytes"`
	ResponseBytes     int                `json:"response_bytes"`
	StreamBytes       int                `json:"stream_bytes"`
	Semantic          LogSemanticSummary `json:"semantic"`
}

type logListRow struct {
	ID                    uint
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ModelName             string
	BackendConfigID       uint
	BackendModelName      string
	BackendAPIBaseURL     string
	Fingerprint           string
	ResponseTime          int64
	FirstTokenLatency     int64
	AvgTokenLatency       float64
	ActiveRequests        int
	RequestPreviewSource  string
	ResponsePreviewSource string
	RequestBytes          int
	ResponseBytes         int
	StreamBytes           int
}

type LogSemanticSummary struct {
	Protocol         string   `json:"protocol,omitempty"`
	Status           string   `json:"status,omitempty"`
	FinishReason     string   `json:"finish_reason,omitempty"`
	OutputItemTypes  []string `json:"output_item_types,omitempty"`
	ToolTypes        []string `json:"tool_types,omitempty"`
	ToolNames        []string `json:"tool_names,omitempty"`
	ReasoningSummary string   `json:"reasoning_summary,omitempty"`
	HasRefusal       bool     `json:"has_refusal,omitempty"`
	Refusal          string   `json:"refusal,omitempty"`
	AnnotationCount  int      `json:"annotation_count,omitempty"`
	HasAudio         bool     `json:"has_audio,omitempty"`
}

type logDetailResponse struct {
	models.RequestLog
	RequestPreview string             `json:"request_preview,omitempty"`
	Semantic       LogSemanticSummary `json:"semantic"`
}

// GetLogs 获取请求日志列表，可以根据查询参数过滤和分页
func (h *LogHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	query := h.DB

	// Add filters based on query parameters
	if model := queryValue(r, "model"); model != "" {
		query = query.Where("model_name = ?", model)
	}

	if backendConfigID := queryValue(r, "backend_config_id"); backendConfigID != "" {
		query = query.Where("backend_config_id = ?", backendConfigID)
	}

	if backendModel := queryValue(r, "backend_model"); backendModel != "" {
		query = query.Where("backend_model_name = ?", backendModel)
	}

	// 添加日期范围过滤
	if startDate := queryValue(r, "start_date"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}

	if endDate := queryValue(r, "end_date"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	// 获取分页参数
	page := 1
	pageSize := 20

	if pageStr := queryValue(r, "page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := queryValue(r, "page_size", "size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = clampLogPageSize(ps)
		}
	}

	// 获取总记录数
	var total int64
	if err := query.Model(&models.RequestLog{}).Count(&total).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 分页查询
	offset := (page - 1) * pageSize
	var rows []logListRow
	if err := query.
		Model(&models.RequestLog{}).
		Select(`
			id,
			created_at,
			updated_at,
			model_name,
			backend_config_id,
			backend_model_name,
			backend_api_base_url,
			fingerprint,
			response_time,
			first_token_latency,
			avg_token_latency,
			active_requests,
			SUBSTR(request, 1, 4096) AS request_preview_source,
			SUBSTR(response, 1, 8192) AS response_preview_source,
			COALESCE(NULLIF(request_bytes, 0), LENGTH(request), 0) AS request_bytes,
			COALESCE(NULLIF(response_bytes, 0), LENGTH(response), 0) AS response_bytes,
			COALESCE(NULLIF(stream_bytes, 0), LENGTH(stream_response), 0) AS stream_bytes
		`).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logs := make([]LogSummary, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, summarizeLog(row))
	}

	// 返回带分页信息的响应
	response := LogResponse{
		Total: total,
		Logs:  logs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func clampLogPageSize(pageSize int) int {
	const maxLogPageSize = 100
	if pageSize > maxLogPageSize {
		return maxLogPageSize
	}
	return pageSize
}

func summarizeLog(log logListRow) LogSummary {
	return LogSummary{
		ID:                log.ID,
		CreatedAt:         log.CreatedAt,
		UpdatedAt:         log.UpdatedAt,
		ModelName:         log.ModelName,
		BackendConfigID:   log.BackendConfigID,
		BackendModelName:  log.BackendModelName,
		BackendAPIBaseURL: log.BackendAPIBaseURL,
		Fingerprint:       log.Fingerprint,
		ResponseTime:      log.ResponseTime,
		FirstTokenLatency: log.FirstTokenLatency,
		AvgTokenLatency:   log.AvgTokenLatency,
		ActiveRequests:    log.ActiveRequests,
		RequestPreview:    requestPreview(log.RequestPreviewSource),
		RequestBytes:      log.RequestBytes,
		ResponseBytes:     log.ResponseBytes,
		StreamBytes:       log.StreamBytes,
		Semantic:          summarizeResponseSemantics(log.ResponsePreviewSource),
	}
}

func requestPreview(request string) string {
	if request == "" {
		return ""
	}

	var payload struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		Prompt json.RawMessage `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(request), &payload); err != nil {
		return truncateForPreview(strings.ReplaceAll(request, "\n", ""))
	}
	for i := len(payload.Messages) - 1; i >= 0; i-- {
		if payload.Messages[i].Role != "user" {
			continue
		}
		if text := contentPreview(payload.Messages[i].Content); text != "" {
			return truncateForPreview(text)
		}
	}
	return truncateForPreview(contentPreview(payload.Prompt))
}

func contentPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.ReplaceAll(text, "\n", "")
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i].Type == "text" && parts[i].Text != "" {
				return strings.ReplaceAll(parts[i].Text, "\n", "")
			}
		}
	}
	return ""
}

func truncateForPreview(value string) string {
	const limit = 120
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func queryValue(r *http.Request, names ...string) string {
	values := r.URL.Query()
	for _, name := range names {
		if value := strings.TrimSpace(values.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

// GetLogDetail 获取单个日志详情
func (h *LogHandler) GetLogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Log ID is required", http.StatusBadRequest)
		return
	}

	var log models.RequestLog
	if err := h.DB.First(&log, id).Error; err != nil {
		http.Error(w, "Log not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logDetailResponse{
		RequestLog:     log,
		RequestPreview: requestPreview(log.Request),
		Semantic:       summarizeResponseSemantics(log.Response),
	})
}

// ClearLogs 清空所有日志
func (h *LogHandler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.Where("1 = 1").Delete(&models.RequestLog{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "All logs cleared successfully",
	})
}

// ReplayRequest 定义重放请求的结构
type ReplayRequest struct {
	Override map[string]interface{} `json:"override"`
}

// ReplayResponse 定义重放响应的结构
type ReplayResponse struct {
	OriginalRequest  string `json:"original_request"`
	ModifiedRequest  string `json:"modified_request"`
	OriginalResponse string `json:"original_response"`
	NewResponse      string `json:"new_response"`
	ModelName        string `json:"model_name"`
	ActualModelName  string `json:"actual_model_name"`
	ResponseTime     int64  `json:"response_time"`
	Error            string `json:"error,omitempty"`
}

// ReplayLog 重放指定的请求日志，支持覆盖参数
func (h *LogHandler) ReplayLog(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Log ID is required", http.StatusBadRequest)
		return
	}

	var reqLog models.RequestLog
	if err := h.DB.First(&reqLog, id).Error; err != nil {
		http.Error(w, "Log not found", http.StatusNotFound)
		return
	}

	var replayReq ReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&replayReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var originalRequest map[string]interface{}
	if err := json.Unmarshal([]byte(reqLog.Request), &originalRequest); err != nil {
		http.Error(w, "Failed to parse original request", http.StatusInternalServerError)
		return
	}

	modifiedRequest := make(map[string]interface{})
	for k, v := range originalRequest {
		modifiedRequest[k] = v
	}
	for k, v := range replayReq.Override {
		modifiedRequest[k] = v
	}

	modelName, ok := modifiedRequest["model"].(string)
	if !ok || modelName == "" {
		modelName = reqLog.ModelName
		modifiedRequest["model"] = modelName
	}

	modifiedRequestJSON, err := json.Marshal(modifiedRequest)
	if err != nil {
		http.Error(w, "Failed to marshal modified request", http.StatusInternalServerError)
		return
	}

	var modifiedRequestRaw map[string]json.RawMessage
	if err := json.Unmarshal(modifiedRequestJSON, &modifiedRequestRaw); err != nil {
		http.Error(w, "Failed to prepare modified request", http.StatusInternalServerError)
		return
	}

	isStream := false
	if streamValue, ok := modifiedRequest["stream"].(bool); ok && streamValue {
		isStream = true
	}

	chatHandler := NewChatHandler(h.DB)
	configs, err := chatHandler.getModelConfigs(modelName)
	if err != nil {
		http.Error(w, "Model config not found: "+modelName, http.StatusNotFound)
		return
	}

	startTime := time.Now()
	resp, selectedConfig, lease, attempts, err := chatHandler.dispatchProviderRequest(r.Context(), r.Header, modifiedRequestRaw, modelName, configs, isStream)
	if err != nil {
		log.WithFields(log.Fields{
			"model":    modelName,
			"attempts": attempts,
		}).Error("Replay provider dispatch failed")
		response := ReplayResponse{
			OriginalRequest:  reqLog.Request,
			ModifiedRequest:  string(modifiedRequestJSON),
			OriginalResponse: reqLog.Response,
			Error:            err.Error(),
			ModelName:        modelName,
			ActualModelName:  "",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	defer resp.Body.Close()
	defer func() {
		if lease != nil {
			lease.Finish(backendObservation{
				Success:        resp.StatusCode >= 200 && resp.StatusCode < 300,
				ResponseTimeMS: time.Since(startTime).Milliseconds(),
			})
		}
	}()

	updatedBody, err := buildProviderRequestBody(modifiedRequestRaw, selectedConfig)
	if err != nil {
		http.Error(w, "Failed to marshal modified request", http.StatusInternalServerError)
		return
	}

	responseTime := time.Since(startTime).Milliseconds()

	var newResponse string
	if isStream {
		newResponse = h.processStreamResponse(resp)
	} else {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read response", http.StatusInternalServerError)
			return
		}
		newResponse = string(respBody)
	}

	response := ReplayResponse{
		OriginalRequest:  reqLog.Request,
		ModifiedRequest:  string(updatedBody),
		OriginalResponse: reqLog.Response,
		NewResponse:      newResponse,
		ModelName:        modelName,
		ActualModelName:  selectedConfig.ModelName,
		ResponseTime:     responseTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// processStreamResponse 处理流式响应，拼接成完整的非流式格式
func (h *LogHandler) processStreamResponse(resp *http.Response) string {
	var fullResponse strings.Builder

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				entry := log.WithError(err)
				switch {
				case isExpectedStreamTermination(err):
					entry.Debug("Replay stream ended after context cancellation or client disconnect")
				default:
					entry.Error("Replay stream read failed")
				}
			}
			break
		}

		fullResponse.WriteString(line)
	}
	if aggregated, ok := aggregateResponsesReplayStream(fullResponse.String()); ok {
		return aggregated
	}
	if aggregated, ok := aggregateAnthropicReplayStream(fullResponse.String()); ok {
		return aggregated
	}
	if aggregated, err := buildOpenAIStreamLogResponse(fullResponse.String()); err == nil && aggregated.ResponseJSON != "" {
		return aggregated.ResponseJSON
	}
	return fullResponse.String()
}

func aggregateResponsesReplayStream(raw string) (string, bool) {
	reader := bufio.NewReader(strings.NewReader(raw))
	response := map[string]interface{}{
		"object": "response",
		"status": "completed",
	}
	outputItems := make(map[int]map[string]interface{})
	textDeltas := make(map[int]string)
	toolArgDeltas := make(map[int]string)
	contentParts := make(map[int]map[int]map[string]interface{})
	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			break
		}
		if frame.Data == "" || frame.Data == "[DONE]" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
			continue
		}
		irEvents := protocol.IRStreamEventsFromResponsesFrame(frame)
		var irEvent protocol.IRStreamEvent
		if len(irEvents) > 0 {
			irEvent = irEvents[0]
		}
		sourceEventType := frame.Event
		if sourceEventType == "" {
			sourceEventType = stringValue(payload["type"])
		}
		switch irEvent.Type {
		case "response.created":
			if resp, ok := payload["response"].(map[string]interface{}); ok {
				if id := stringValue(resp["id"]); id != "" {
					response["id"] = id
				}
				if model := stringValue(resp["model"]); model != "" {
					response["model"] = model
				}
				if status := stringValue(resp["status"]); status != "" {
					response["status"] = status
				}
			}
		case "tool_call.delta":
			index := numberValueToInt(payload["output_index"])
			toolArgDeltas[index] += firstNonEmptyString(irEvent.Arguments, irEvent.Delta, stringValue(payload["delta"]))
		case "output_text.delta":
			index := numberValueToInt(payload["output_index"])
			textDeltas[index] += firstNonEmptyString(irEvent.Text, irEvent.Delta, stringValue(payload["delta"]))
		case "annotation.added":
			index := numberValueToInt(payload["output_index"])
			if contentParts[index] == nil {
				contentParts[index] = make(map[int]map[string]interface{})
			}
			part := contentParts[index][0]
			if part == nil {
				part = map[string]interface{}{"type": "output_text", "text": textDeltas[index]}
			}
			if annotations := decodeReplayIRAnnotations(irEvent.Annotations, payload["annotations"]); len(annotations) > 0 {
				part["annotations"] = annotations
			}
			contentParts[index][0] = part
		case "refusal.delta":
			index := numberValueToInt(payload["output_index"])
			contentIndex := numberValueToInt(payload["content_index"])
			if contentParts[index] == nil {
				contentParts[index] = make(map[int]map[string]interface{})
			}
			contentParts[index][contentIndex] = map[string]interface{}{
				"type":    "refusal",
				"refusal": firstNonEmptyString(irEvent.Refusal, irEvent.Delta, stringValue(payload["refusal"]), stringValue(payload["delta"])),
			}
		case "audio.delta":
			index := numberValueToInt(payload["output_index"])
			contentIndex := numberValueToInt(payload["content_index"])
			audio := decodeReplayIRAudio(irEvent.Audio, payload["audio"])
			if len(audio) > 0 {
				if contentParts[index] == nil {
					contentParts[index] = make(map[int]map[string]interface{})
				}
				contentParts[index][contentIndex] = map[string]interface{}{
					"type":  "output_audio",
					"audio": audio,
				}
			}
		case "response.completed":
			resp, ok := payload["response"].(map[string]interface{})
			if !ok || len(resp) == 0 {
				continue
			}
			normalized, err := json.Marshal(resp)
			if err != nil {
				return "", false
			}
			return string(normalized), true
		}
		switch sourceEventType {
		case "response.created":
		case "response.output_item.added":
			index := numberValueToInt(payload["output_index"])
			if item := decodeReplayIRItem(irEvent.Item, payload["item"]); len(item) > 0 {
				cloned := cloneMap(item)
				outputItems[index] = cloned
			}
		case "response.output_item.done":
			index := numberValueToInt(payload["output_index"])
			if item := decodeReplayIRItem(irEvent.Item, payload["item"]); len(item) > 0 {
				cloned := cloneMap(item)
				outputItems[index] = cloned
			}
		case "response.content_part.added", "response.content_part.done":
			index := numberValueToInt(payload["output_index"])
			contentIndex := numberValueToInt(payload["content_index"])
			part := decodeReplayIRPart(irEvent, payload["part"])
			if len(part) == 0 {
				continue
			}
			if contentParts[index] == nil {
				contentParts[index] = make(map[int]map[string]interface{})
			}
			contentParts[index][contentIndex] = cloneMap(part)
		case "response.refusal.done":
			index := numberValueToInt(payload["output_index"])
			contentIndex := numberValueToInt(payload["content_index"])
			if contentParts[index] == nil {
				contentParts[index] = make(map[int]map[string]interface{})
			}
			contentParts[index][contentIndex] = map[string]interface{}{
				"type":    "refusal",
				"refusal": stringValue(payload["refusal"]),
			}
		case "response.audio.done":
			// already covered through normalized IR event handling above
		}
	}

	if len(outputItems) == 0 {
		return "", false
	}

	indexes := make([]int, 0, len(outputItems))
	for index := range outputItems {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	output := make([]map[string]interface{}, 0, len(indexes))
	var outputText strings.Builder
	for _, index := range indexes {
		item := outputItems[index]
		switch stringValue(item["type"]) {
		case "message":
			if partMap := contentParts[index]; len(partMap) > 0 {
				contentIndexes := make([]int, 0, len(partMap))
				for contentIndex := range partMap {
					contentIndexes = append(contentIndexes, contentIndex)
				}
				sort.Ints(contentIndexes)
				content := make([]map[string]interface{}, 0, len(contentIndexes))
				for _, contentIndex := range contentIndexes {
					part := cloneMap(partMap[contentIndex])
					if stringValue(part["type"]) == "output_text" {
						if text := textDeltas[index]; text != "" && stringValue(part["text"]) == "" {
							part["text"] = text
						}
						outputText.WriteString(stringValue(part["text"]))
					}
					content = append(content, part)
				}
				item["content"] = content
			} else if text := textDeltas[index]; text != "" {
				item["content"] = []map[string]interface{}{{
					"type": "output_text",
					"text": text,
				}}
				outputText.WriteString(text)
			}
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			if args := toolArgDeltas[index]; args != "" {
				if _, ok := item["arguments"]; ok {
					item["arguments"] = args
				} else {
					item["input"] = args
				}
			}
		}
		output = append(output, item)
	}
	response["output"] = output
	if outputText.Len() > 0 {
		response["output_text"] = outputText.String()
	}
	normalized, err := json.Marshal(response)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func decodeReplayIRAnnotations(raw json.RawMessage, fallback interface{}) []interface{} {
	if len(raw) > 0 {
		var annotations []interface{}
		if err := json.Unmarshal(raw, &annotations); err == nil {
			return annotations
		}
	}
	if annotations, ok := fallback.([]interface{}); ok {
		return annotations
	}
	return nil
}

func decodeReplayIRAudio(raw json.RawMessage, fallback interface{}) map[string]interface{} {
	if len(raw) > 0 {
		var audio map[string]interface{}
		if err := json.Unmarshal(raw, &audio); err == nil {
			return audio
		}
	}
	if audio, ok := fallback.(map[string]interface{}); ok {
		return cloneMap(audio)
	}
	return nil
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func aggregateAnthropicReplayStream(raw string) (string, bool) {
	reader := bufio.NewReader(strings.NewReader(raw))
	var message map[string]interface{}
	blocks := make(map[int]map[string]interface{})
	var stopReason string
	var usage map[string]interface{}

	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			break
		}
		if frame.Data == "" || frame.Data == "[DONE]" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(frame.Data), &payload); err != nil {
			continue
		}
		irEvents := protocol.IRStreamEventsFromAnthropicFrame(frame)
		var irEvent protocol.IRStreamEvent
		if len(irEvents) > 0 {
			irEvent = irEvents[0]
		}
		eventType := frame.Event
		if eventType == "" {
			eventType = stringValue(payload["type"])
		}
		switch eventType {
		case "message_start":
			msg, _ := payload["message"].(map[string]interface{})
			if len(msg) > 0 || irEvent.ItemID != "" {
				if len(msg) == 0 {
					msg = map[string]interface{}{}
				}
				message = map[string]interface{}{
					"id":    firstNonEmptyString(irEvent.ItemID, stringValue(msg["id"])),
					"type":  firstNonEmptyString(stringValue(msg["type"]), "message"),
					"role":  firstNonEmptyString(stringValue(msg["role"]), "assistant"),
					"model": msg["model"],
				}
				if decoded := decodeReplayIRUsage(irEvent.Usage); len(decoded) > 0 {
					usage = decoded
				} else if u, ok := msg["usage"].(map[string]interface{}); ok && len(u) > 0 {
					usage = u
				}
			}
		case "content_block_start":
			index := numberValueToInt(payload["index"])
			var sawIRBlock bool
			for _, event := range irEvents {
				block := anthropicReplayBlockFromIREvent(event)
				if len(block) == 0 {
					continue
				}
				blocks[indexForAnthropicReplayBlock(index, event)] = block
				sawIRBlock = true
			}
			if sawIRBlock {
				continue
			}
			block, _ := payload["content_block"].(map[string]interface{})
			if len(block) == 0 {
				continue
			}
			copied := map[string]interface{}{}
			for k, v := range block {
				copied[k] = v
			}
			blocks[index] = copied
		case "content_block_delta":
			index := numberValueToInt(payload["index"])
			block := blocks[index]
			if block == nil {
				continue
			}
			switch irEvent.Type {
			case "output_text.delta":
				block["text"] = stringValue(block["text"]) + firstNonEmptyString(irEvent.Text, irEvent.Delta)
			case "reasoning.delta":
				block["thinking"] = stringValue(block["thinking"]) + firstNonEmptyString(irEvent.Text, irEvent.Delta)
			case "tool_call.delta":
				accumulated := stringValue(block["_partial_json"]) + firstNonEmptyString(irEvent.Arguments, irEvent.Delta)
				block["_partial_json"] = accumulated
				var decoded interface{}
				if err := json.Unmarshal([]byte(accumulated), &decoded); err == nil {
					block["input"] = decoded
				}
			default:
				delta, _ := payload["delta"].(map[string]interface{})
				switch stringValue(delta["type"]) {
				case "text_delta":
					block["text"] = stringValue(block["text"]) + stringValue(delta["text"])
				case "thinking_delta":
					block["thinking"] = stringValue(block["thinking"]) + stringValue(delta["thinking"])
				case "input_json_delta":
					accumulated := stringValue(block["_partial_json"]) + stringValue(delta["partial_json"])
					block["_partial_json"] = accumulated
					var decoded interface{}
					if err := json.Unmarshal([]byte(accumulated), &decoded); err == nil {
						block["input"] = decoded
					}
				}
			}
		case "message_delta":
			delta, _ := payload["delta"].(map[string]interface{})
			stopReason = firstNonEmptyString(stringValue(delta["stop_reason"]), irEvent.FinishReason)
			if decoded := decodeReplayIRUsage(irEvent.Usage); len(decoded) > 0 {
				usage = decoded
			} else if u, ok := payload["usage"].(map[string]interface{}); ok && len(u) > 0 {
				usage = u
			}
		}
	}

	if message == nil || len(blocks) == 0 {
		return "", false
	}

	indexes := make([]int, 0, len(blocks))
	for index := range blocks {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	content := make([]map[string]interface{}, 0, len(indexes))
	for _, index := range indexes {
		block := blocks[index]
		delete(block, "_partial_json")
		content = append(content, block)
	}
	message["content"] = content
	if stopReason != "" {
		message["stop_reason"] = stopReason
	}
	if len(usage) > 0 {
		message["usage"] = usage
	}
	normalized, err := json.Marshal(message)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}

func anthropicReplayBlockFromIREvent(event protocol.IRStreamEvent) map[string]interface{} {
	switch event.Type {
	case "output_text.delta":
		return map[string]interface{}{
			"type": "text",
			"text": firstNonEmptyString(event.Text, event.Delta),
		}
	case "annotation.added":
		annotations := decodeReplayIRAnnotations(event.Annotations, nil)
		if len(annotations) == 0 {
			return nil
		}
		return map[string]interface{}{
			"type":        "text",
			"text":        "",
			"annotations": annotations,
		}
	case "refusal.delta":
		return map[string]interface{}{
			"type":    "text",
			"text":    firstNonEmptyString(event.Refusal, event.Delta),
			"refusal": firstNonEmptyString(event.Refusal, event.Delta),
		}
	case "audio.delta":
		audio := decodeReplayIRAudio(event.Audio, nil)
		if len(audio) == 0 {
			return nil
		}
		return map[string]interface{}{
			"type":  "text",
			"text":  "",
			"audio": audio,
		}
	default:
		if len(event.Item) > 0 {
			if block, ok := decodeReplayRawToAny(event.Item).(map[string]interface{}); ok {
				return cloneMap(block)
			}
		}
		return nil
	}
}

func indexForAnthropicReplayBlock(base int, event protocol.IRStreamEvent) int {
	switch event.Type {
	case "annotation.added":
		return base + 1000
	case "refusal.delta":
		return base + 1001
	case "audio.delta":
		return base + 1002
	default:
		return base
	}
}

func decodeReplayIRUsage(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var usage map[string]interface{}
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil
	}
	return usage
}

func decodeReplayRawToAny(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func decodeReplayIRItem(raw json.RawMessage, fallback interface{}) map[string]interface{} {
	if len(raw) > 0 {
		if item, ok := decodeReplayRawToAny(raw).(map[string]interface{}); ok {
			return item
		}
	}
	if item, ok := fallback.(map[string]interface{}); ok {
		return item
	}
	return nil
}

func decodeReplayIRPart(event protocol.IRStreamEvent, fallback interface{}) map[string]interface{} {
	part := map[string]interface{}{}
	if rawPart, ok := fallback.(map[string]interface{}); ok && len(rawPart) > 0 {
		part = cloneMap(rawPart)
	}
	if event.Text != "" && stringValue(part["text"]) == "" {
		part["text"] = event.Text
	}
	if event.Refusal != "" {
		part["type"] = "refusal"
		part["refusal"] = event.Refusal
	}
	if len(event.Annotations) > 0 {
		if annotations := decodeReplayIRAnnotations(event.Annotations, nil); len(annotations) > 0 {
			part["annotations"] = annotations
		}
		if _, ok := part["type"]; !ok {
			part["type"] = "output_text"
		}
	}
	if len(event.Audio) > 0 {
		if audio := decodeReplayIRAudio(event.Audio, nil); len(audio) > 0 {
			part["type"] = "output_audio"
			part["audio"] = audio
		}
	}
	return part
}

func summarizeResponseSemantics(raw string) LogSemanticSummary {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return LogSemanticSummary{}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return LogSemanticSummary{}
	}

	switch {
	case payload["output"] != nil:
		return summarizeResponsesSemantic(payload)
	case payload["choices"] != nil:
		return summarizeChatSemantic(payload)
	case payload["content"] != nil:
		return summarizeAnthropicSemantic(payload)
	default:
		return LogSemanticSummary{}
	}
}

func summarizeResponsesSemantic(payload map[string]any) LogSemanticSummary {
	summary := LogSemanticSummary{
		Protocol: "responses",
		Status:   stringValue(payload["status"]),
	}
	output, _ := payload["output"].([]any)
	for _, rawItem := range output {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		itemType := stringValue(item["type"])
		addUniqueString(&summary.OutputItemTypes, itemType)
		switch itemType {
		case "message":
			if content, ok := item["content"].([]any); ok {
				for _, rawPart := range content {
					part, ok := rawPart.(map[string]any)
					if !ok {
						continue
					}
					partType := stringValue(part["type"])
					switch partType {
					case "output_text":
						if summary.ReasoningSummary == "" && stringValue(part["text"]) != "" {
							summary.ReasoningSummary = truncateForPreview(stringValue(part["text"]))
						}
						if annotations, ok := part["annotations"].([]any); ok {
							summary.AnnotationCount += len(annotations)
						}
					case "refusal":
						summary.HasRefusal = true
						summary.Refusal = firstNonEmptyString(summary.Refusal, stringValue(part["refusal"]))
					case "output_audio", "audio":
						summary.HasAudio = true
					}
				}
			}
		case "reasoning", "compaction":
			if summary.ReasoningSummary == "" {
				summary.ReasoningSummary = truncateForPreview(extractSummaryText(item))
			}
		default:
			if strings.Contains(itemType, "call") || strings.Contains(itemType, "tool") || strings.Contains(itemType, "search") || strings.Contains(itemType, "shell") {
				addUniqueString(&summary.ToolTypes, itemType)
				addUniqueString(&summary.ToolNames, firstNonEmptyString(stringValue(item["name"]), itemType))
			}
		}
	}
	return summary
}

func summarizeChatSemantic(payload map[string]any) LogSemanticSummary {
	summary := LogSemanticSummary{Protocol: "chat"}
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return summary
	}
	choice, _ := choices[0].(map[string]any)
	summary.FinishReason = stringValue(choice["finish_reason"])
	message, _ := choice["message"].(map[string]any)
	if message == nil {
		return summary
	}
	if reasoning := stringValue(message["reasoning_content"]); reasoning != "" {
		summary.ReasoningSummary = truncateForPreview(reasoning)
	}
	if refusal := stringValue(message["refusal"]); refusal != "" {
		summary.HasRefusal = true
		summary.Refusal = refusal
	}
	if _, ok := message["audio"].(map[string]any); ok {
		summary.HasAudio = true
	}
	if annotations, ok := message["annotations"].([]any); ok {
		summary.AnnotationCount += len(annotations)
	}
	if toolCalls, ok := message["tool_calls"].([]any); ok {
		addUniqueString(&summary.OutputItemTypes, "function_call")
		for _, rawTool := range toolCalls {
			tool, ok := rawTool.(map[string]any)
			if !ok {
				continue
			}
			addUniqueString(&summary.ToolTypes, firstNonEmptyString(stringValue(tool["type"]), "function"))
			if fn, ok := tool["function"].(map[string]any); ok {
				addUniqueString(&summary.ToolNames, stringValue(fn["name"]))
			}
		}
	}
	if content, ok := message["content"].([]any); ok {
		for _, rawPart := range content {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			if annotations, ok := part["annotations"].([]any); ok {
				summary.AnnotationCount += len(annotations)
			}
		}
	}
	return summary
}

func summarizeAnthropicSemantic(payload map[string]any) LogSemanticSummary {
	summary := LogSemanticSummary{
		Protocol:     "anthropic_messages",
		FinishReason: stringValue(payload["stop_reason"]),
	}
	content, _ := payload["content"].([]any)
	for _, rawPart := range content {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		partType := stringValue(part["type"])
		addUniqueString(&summary.OutputItemTypes, partType)
		if annotations, ok := part["annotations"].([]any); ok {
			summary.AnnotationCount += len(annotations)
		}
		if refusal := stringValue(part["refusal"]); refusal != "" {
			summary.HasRefusal = true
			summary.Refusal = firstNonEmptyString(summary.Refusal, refusal)
		}
		if _, ok := part["audio"].(map[string]any); ok {
			summary.HasAudio = true
		}
		switch partType {
		case "thinking":
			if summary.ReasoningSummary == "" {
				summary.ReasoningSummary = truncateForPreview(stringValue(part["thinking"]))
			}
		case "tool_use":
			addUniqueString(&summary.ToolTypes, "tool_use")
			addUniqueString(&summary.ToolNames, stringValue(part["name"]))
		}
	}
	return summary
}

func extractSummaryText(item map[string]any) string {
	if summary, ok := item["summary"].([]any); ok {
		for _, rawPart := range summary {
			part, ok := rawPart.(map[string]any)
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

func addUniqueString(dst *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range *dst {
		if existing == value {
			return
		}
	}
	*dst = append(*dst, value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
