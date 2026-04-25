package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"

	"gorm.io/gorm"
)

type ModelConfigHandler struct {
	DB *gorm.DB
}

const modelConfigConnectionTestTimeout = 2 * time.Minute

type modelConfigResponse struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	ModelName      string    `json:"model_name"`
	APIBaseURL     string    `json:"api_base_url"`
	APIKeySet      bool      `json:"api_key_set"`
	MaxTokens      int       `json:"max_tokens"`
	Priority       int       `json:"priority"`
	MaxConcurrency int       `json:"max_concurrency"`
	Temperature    float64   `json:"temperature"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Enabled        bool      `json:"enabled"`
}

func NewModelConfigHandler(db *gorm.DB) *ModelConfigHandler {
	return &ModelConfigHandler{
		DB: db,
	}
}

func toModelConfigResponse(config models.ModelConfig) modelConfigResponse {
	return modelConfigResponse{
		ID:             config.ID,
		Name:           config.Name,
		ModelName:      config.ModelName,
		APIBaseURL:     config.APIBaseURL,
		APIKeySet:      strings.TrimSpace(config.APIKey) != "",
		MaxTokens:      config.MaxTokens,
		Priority:       config.Priority,
		MaxConcurrency: config.MaxConcurrency,
		Temperature:    config.Temperature,
		Description:    config.Description,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
		Enabled:        config.Enabled,
	}
}

func toModelConfigResponses(configs []models.ModelConfig) []modelConfigResponse {
	responses := make([]modelConfigResponse, 0, len(configs))
	for _, config := range configs {
		responses = append(responses, toModelConfigResponse(config))
	}
	return responses
}

func normalizeModelConfig(config *models.ModelConfig) {
	if config.Priority < 0 {
		config.Priority = 0
	}
	if config.MaxConcurrency < 0 {
		config.MaxConcurrency = 0
	}
}

func validateModelConfig(config *models.ModelConfig) error {
	if strings.TrimSpace(config.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(config.ModelName) == "" {
		return errors.New("model_name is required")
	}
	if strings.TrimSpace(config.APIBaseURL) == "" {
		return errors.New("api_base_url is required")
	}
	if !strings.HasPrefix(config.APIBaseURL, "http://") && !strings.HasPrefix(config.APIBaseURL, "https://") {
		return errors.New("api_base_url must start with http:// or https://")
	}
	return nil
}

type modelConfigConnectionTestResult struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Input   map[string]any `json:"input"`
	Output  map[string]any `json:"output"`
}

func testModelConfigConnection(ctx context.Context, config models.ModelConfig) modelConfigConnectionTestResult {
	input := map[string]any{
		"method": "POST",
		"url":    buildProviderChatURL(config.APIBaseURL),
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}
	if strings.TrimSpace(config.APIKey) != "" {
		input["headers"].(map[string]string)["Authorization"] = "Bearer ***"
	}

	if strings.TrimSpace(config.APIBaseURL) == "" {
		return modelConfigConnectionTestResult{Success: false, Message: "api_base_url is required", Input: input, Output: map[string]any{"error": "api_base_url is required"}}
	}
	if strings.TrimSpace(config.ModelName) == "" {
		return modelConfigConnectionTestResult{Success: false, Message: "model_name is required", Input: input, Output: map[string]any{"error": "model_name is required"}}
	}

	requestPayload := map[string]any{
		"model": config.ModelName,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens":  1,
		"temperature": 0,
		"stream":      false,
	}
	input["body"] = requestPayload

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return modelConfigConnectionTestResult{Success: false, Message: err.Error(), Input: input, Output: map[string]any{"error": err.Error()}}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildProviderChatURL(config.APIBaseURL), bytes.NewReader(body))
	if err != nil {
		return modelConfigConnectionTestResult{Success: false, Message: err.Error(), Input: input, Output: map[string]any{"error": err.Error()}}
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(config.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	startedAt := time.Now()
	resp, err := http.DefaultClient.Do(req)
	duration := time.Since(startedAt)
	if err != nil {
		return modelConfigConnectionTestResult{
			Success: false,
			Message: err.Error(),
			Input:   input,
			Output: map[string]any{
				"duration_ms": duration.Milliseconds(),
				"error":       err.Error(),
			},
		}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyText := strings.TrimSpace(string(bodyBytes))
	output := map[string]any{
		"status_code": resp.StatusCode,
		"duration_ms": duration.Milliseconds(),
		"headers": map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		"body": bodyText,
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return modelConfigConnectionTestResult{Success: true, Message: "连接测试成功", Input: input, Output: output}
	}

	detail := bodyText
	if detail == "" {
		detail = http.StatusText(resp.StatusCode)
	}
	return modelConfigConnectionTestResult{
		Success: false,
		Message: fmt.Sprintf("连接测试失败: upstream returned %d %s", resp.StatusCode, detail),
		Input:   input,
		Output:  output,
	}
}

func modelConfigResult(result modelConfigConnectionTestResult) map[string]any {
	return map[string]any{
		"success": true,
		"data":    result,
	}
}

func (h *ModelConfigHandler) CreateModelConfig(w http.ResponseWriter, r *http.Request) {
	var config models.ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	normalizeModelConfig(&config)
	if err := validateModelConfig(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	if err := h.DB.Create(&config).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	InvalidateAllModelConfigCache()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toModelConfigResponse(config))
}

func (h *ModelConfigHandler) GetModelConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []models.ModelConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toModelConfigResponses(configs))
}

func (h *ModelConfigHandler) GetEnabledModelConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []models.ModelConfig
	if err := h.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toModelConfigResponses(configs))
}

func (h *ModelConfigHandler) GetModelConfig(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "model config id is required", http.StatusBadRequest)
		return
	}
	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "model config not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toModelConfigResponse(config))
}

func (h *ModelConfigHandler) ModifyModelConfig(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "model config id is required", http.StatusBadRequest)
		return
	}

	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "model config not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var input models.ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	normalizeModelConfig(&input)
	if err := validateModelConfig(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	config.Name = input.Name
	config.ModelName = input.ModelName
	config.APIBaseURL = input.APIBaseURL
	if strings.TrimSpace(input.APIKey) != "" {
		config.APIKey = input.APIKey
	}
	config.MaxTokens = input.MaxTokens
	config.Priority = input.Priority
	config.MaxConcurrency = input.MaxConcurrency
	config.Temperature = input.Temperature
	config.Description = input.Description
	config.Enabled = input.Enabled
	config.UpdatedAt = time.Now()

	updates := map[string]any{
		"name":            config.Name,
		"model_name":      config.ModelName,
		"api_base_url":    config.APIBaseURL,
		"max_tokens":      config.MaxTokens,
		"priority":        config.Priority,
		"max_concurrency": config.MaxConcurrency,
		"temperature":     config.Temperature,
		"description":     config.Description,
		"enabled":         config.Enabled,
		"updated_at":      config.UpdatedAt,
	}
	if strings.TrimSpace(input.APIKey) != "" {
		updates["api_key"] = config.APIKey
	}

	if err := h.DB.Model(&config).Updates(updates).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.DB.First(&config, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	InvalidateAllModelConfigCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toModelConfigResponse(config))
}

func (h *ModelConfigHandler) DeleteModelConfig(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "model config id is required", http.StatusBadRequest)
		return
	}

	result := h.DB.Delete(&models.ModelConfig{}, id)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}
	if result.RowsAffected == 0 {
		http.Error(w, "model config not found", http.StatusNotFound)
		return
	}

	InvalidateAllModelConfigCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func (h *ModelConfigHandler) TestModelConfig(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "model config id is required", http.StatusBadRequest)
		return
	}

	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "model config not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), modelConfigConnectionTestTimeout)
	defer cancel()
	result := testModelConfigConnection(ctx, config)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelConfigResult(result))
}

func (h *ModelConfigHandler) ResetModelConfigRuntime(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "model config id is required", http.StatusBadRequest)
		return
	}

	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "model config not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reset := getBackendRuntimeManager().resetConfigState(config, config.Name)
	InvalidateStatsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data": map[string]any{
			"reset":   reset,
			"message": map[bool]string{true: "调度状态已重置", false: "当前没有可重置的运行时状态"}[reset],
		},
	})
}

func (h *ModelConfigHandler) ResetAllModelConfigRuntime(w http.ResponseWriter, r *http.Request) {
	resetCount := getBackendRuntimeManager().resetAllState()
	InvalidateStatsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data": map[string]any{
			"reset_count": resetCount,
			"message":     "全部调度状态已重置",
		},
	})
}
