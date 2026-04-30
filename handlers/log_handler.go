package handlers

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"llm-gateway/models"
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
	ID                uint      `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ModelName         string    `json:"model_name"`
	BackendConfigID   uint      `json:"backend_config_id"`
	BackendModelName  string    `json:"backend_model_name"`
	BackendAPIBaseURL string    `json:"backend_api_base_url"`
	Fingerprint       string    `json:"fingerprint"`
	ResponseTime      int64     `json:"response_time"`
	FirstTokenLatency int64     `json:"first_token_latency"`
	AvgTokenLatency   float64   `json:"avg_token_latency"`
	ActiveRequests    int       `json:"active_requests"`
	RequestPreview    string    `json:"request_preview"`
	RequestBytes      int       `json:"request_bytes"`
	ResponseBytes     int       `json:"response_bytes"`
	StreamBytes       int       `json:"stream_bytes"`
}

type logListRow struct {
	ID                   uint
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ModelName            string
	BackendConfigID      uint
	BackendModelName     string
	BackendAPIBaseURL    string
	Fingerprint          string
	ResponseTime         int64
	FirstTokenLatency    int64
	AvgTokenLatency      float64
	ActiveRequests       int
	RequestPreviewSource string
	RequestBytes         int
	ResponseBytes        int
	StreamBytes          int
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

	if userToken := queryValue(r, "user_token"); userToken != "" {
		query = query.Where("user_token = ?", userToken)
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
			COALESCE(LENGTH(request), 0) AS request_bytes,
			COALESCE(LENGTH(response), 0) AS response_bytes,
			COALESCE(LENGTH(stream_response), 0) AS stream_bytes
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
	json.NewEncoder(w).Encode(log)
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
	var contentOnly strings.Builder
	var reasoningContentOnly strings.Builder

	var streamResponse struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			}
			Logprobs     interface{} `json:"logprobs"`
			FinishReason string      `json:"finish_reason"`
		}
		Usage struct {
			PromptTokens       int `json:"prompt_tokens"`
			CompletionTokens   int `json:"completion_tokens"`
			TotalTokens        int `json:"total_tokens"`
			PromptTokensDetail struct {
				CachedTokens int `json:"cached_tokens"`
			}
			PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
		}
	}

	var firstID, firstObject, firstModel string
	var firstCreated int64
	var hasMetadata bool

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.WithError(err).Error("Replay stream read failed")
			}
			break
		}

		fullResponse.WriteString(line)

		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			jsonStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
			if jsonStr != "" && jsonStr != "[DONE]" {
				if err := json.Unmarshal([]byte(jsonStr), &streamResponse); err == nil {
					if !hasMetadata && streamResponse.ID != "" {
						firstID = streamResponse.ID
						firstObject = streamResponse.Object
						firstCreated = streamResponse.Created
						firstModel = streamResponse.Model
						hasMetadata = true
					}
					if len(streamResponse.Choices) > 0 {
						if streamResponse.Choices[0].Delta.Content != "" {
							contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
						}
						if streamResponse.Choices[0].Delta.ReasoningContent != "" {
							reasoningContentOnly.WriteString(streamResponse.Choices[0].Delta.ReasoningContent)
						}
					}
				}
			}
		}
	}

	cachedResp := struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
			} `json:"message"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		} `json:"choices"`
		Usage struct {
			PromptTokens       int `json:"prompt_tokens"`
			CompletionTokens   int `json:"completion_tokens"`
			TotalTokens        int `json:"total_tokens"`
			PromptTokensDetail struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_detail"`
			PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
		} `json:"usage"`
	}{
		ID:      firstID,
		Object:  firstObject,
		Created: firstCreated,
		Model:   firstModel,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
			} `json:"message"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		}{
			{
				Index: 0,
				Message: struct {
					Role             string `json:"role"`
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content,omitempty"`
				}{
					Role:             "assistant",
					Content:          contentOnly.String(),
					ReasoningContent: reasoningContentOnly.String(),
				},
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
	}

	cachedResp.Usage.PromptTokens = streamResponse.Usage.PromptTokens
	cachedResp.Usage.CompletionTokens = streamResponse.Usage.CompletionTokens
	cachedResp.Usage.TotalTokens = streamResponse.Usage.TotalTokens
	cachedResp.Usage.PromptTokensDetail.CachedTokens = streamResponse.Usage.PromptTokensDetail.CachedTokens
	cachedResp.Usage.PromptCacheHitTokens = streamResponse.Usage.PromptCacheHitTokens
	cachedResp.Usage.PromptCacheMissTokens = streamResponse.Usage.PromptCacheMissTokens

	respData, _ := json.Marshal(cachedResp)
	return string(respData)
}
