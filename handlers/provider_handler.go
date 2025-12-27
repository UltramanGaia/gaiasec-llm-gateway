package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"
	"llm-gateway/models"
)

// ProviderHandler 处理Provider相关的API请求
type ProviderHandler struct {
	DB *gorm.DB
}

// NewProviderHandler 创建ProviderHandler的新实例
func NewProviderHandler(db *gorm.DB) *ProviderHandler {
	return &ProviderHandler{
		DB: db,
	}
}

// CreateProvider 创建新的Provider
func (h *ProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	// 使用临时结构体来避免ID类型不匹配问题
	type providerInput struct {
		Name    string `json:"name"`
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseURL"`
	}

	var input providerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 创建Provider结构体
	provider := models.Provider{
		Name:    input.Name,
		APIKey:  input.APIKey,
		BaseURL: input.BaseURL,
	}

	if err := h.DB.Create(&provider).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
}

// GetProviders 获取所有Provider列表
func (h *ProviderHandler) GetProviders(w http.ResponseWriter, r *http.Request) {
	var providers []models.Provider
	if err := h.DB.Find(&providers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(providers)
}

func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/providers/"):]
	var provider models.Provider
	if err := h.DB.First(&provider, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(provider)
}

func (h *ProviderHandler) ModifyProvider(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Path[len("/api/providers/"):]
	var provider models.Provider
	if err := h.DB.First(&provider, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用临时结构体来避免ID类型不匹配问题
	type providerInput struct {
		Name    string `json:"name"`
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseURL"`
	}

	var input providerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	provider.Name = input.Name
	provider.APIKey = input.APIKey
	provider.BaseURL = input.BaseURL
	provider.UpdatedAt = time.Now()

	if err := h.DB.Save(&provider).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(provider)
}
