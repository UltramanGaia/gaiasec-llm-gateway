package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"

	"gorm.io/gorm"
)

type ModelConfigHandler struct {
	DB *gorm.DB
}

func NewModelConfigHandler(db *gorm.DB) *ModelConfigHandler {
	return &ModelConfigHandler{
		DB: db,
	}
}

func (h *ModelConfigHandler) CreateModelConfig(w http.ResponseWriter, r *http.Request) {
	var config models.ModelConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
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
	json.NewEncoder(w).Encode(config)
}

func (h *ModelConfigHandler) GetModelConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []models.ModelConfig
	if err := h.DB.Find(&configs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (h *ModelConfigHandler) GetEnabledModelConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []models.ModelConfig
	if err := h.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
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
	json.NewEncoder(w).Encode(config)
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

	config.Name = input.Name
	config.ModelName = input.ModelName
	config.APIBaseURL = input.APIBaseURL
	config.APIKey = input.APIKey
	config.MaxTokens = input.MaxTokens
	config.Temperature = input.Temperature
	config.Description = input.Description
	config.Enabled = input.Enabled
	config.UpdatedAt = time.Now()

	updates := map[string]any{
		"name":         config.Name,
		"model_name":   config.ModelName,
		"api_base_url": config.APIBaseURL,
		"api_key":      config.APIKey,
		"max_tokens":   config.MaxTokens,
		"temperature":  config.Temperature,
		"description":  config.Description,
		"enabled":      config.Enabled,
		"updated_at":   config.UpdatedAt,
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
	json.NewEncoder(w).Encode(config)
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

	success := true
	if strings.TrimSpace(config.APIBaseURL) == "" || strings.TrimSpace(config.ModelName) == "" {
		success = false
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": success,
		"message": map[bool]string{true: "连接测试成功", false: "连接测试失败"}[success],
	})
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
		"reset":   reset,
		"message": map[bool]string{true: "调度状态已重置", false: "当前没有可重置的运行时状态"}[reset],
	})
}

func (h *ModelConfigHandler) ResetAllModelConfigRuntime(w http.ResponseWriter, r *http.Request) {
	resetCount := getBackendRuntimeManager().resetAllState()
	InvalidateStatsCache()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"resetCount": resetCount,
		"message":    "全部调度状态已重置",
	})
}
