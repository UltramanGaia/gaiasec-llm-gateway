package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"
	"llm-gateway/models"
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

func (h *ModelConfigHandler) GetModelConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/model-mappings/"):]
	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (h *ModelConfigHandler) ModifyModelConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/model-mappings/"):]

	var config models.ModelConfig
	if err := h.DB.First(&config, id).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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

	if err := h.DB.Save(&config).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
