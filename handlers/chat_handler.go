package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/UltramanGaia/llm-gateway/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ChatHandler 处理聊天完成相关的请求
type ChatHandler struct {
	DB *gorm.DB
}

// NewChatHandler 创建ChatHandler的新实例
func NewChatHandler(db *gorm.DB) *ChatHandler {
	return &ChatHandler{
		DB: db,
	}
}

// ChatCompletion 处理聊天完成请求，根据模型名称路由到对应的LLM提供商
func (h *ChatHandler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	// Create request log entry
	reqLog := models.RequestLog{
		UserToken: r.Header.Get("Authorization"),
		CreatedAt: time.Now(),
	}
	defer func() {
		if err := h.DB.Create(&reqLog).Error; err != nil {
			log.Error("Failed to save request log: " + err.Error())
		}
	}()

	// Log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reqLog.Request = string(body)

	// Parse request to get model name
	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		http.Error(w, "Model name is required", http.StatusBadRequest)
		return
	}
	reqLog.ModelName = modelName

	// Find model mapping
	var mapping models.ModelMapping
	if err := h.DB.Where("alias = ?", modelName).First(&mapping).Error; err != nil {
		log.Error("Model not found: " + modelName)
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	// Get provider information
	var provider models.Provider
	if err := h.DB.First(&provider, mapping.ProviderID).Error; err != nil {
		log.Error("Provider not found for model: " + modelName)
		http.Error(w, "Provider configuration error", http.StatusInternalServerError)
		return
	}
	actualModelName := mapping.ModelName

	// Update request with actual model name
	requestBody["model"] = actualModelName
	updatedBody, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create new request to provider
	providerURL := provider.BaseURL
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	providerURL += "chat/completions"

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers and set API key
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the response
	reqLog.Response = string(respBody)

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Write response body
	w.Write(respBody)
}
