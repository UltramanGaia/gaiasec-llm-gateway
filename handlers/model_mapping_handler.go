package handlers

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/UltramanGaia/llm-gateway/models"
	"gorm.io/gorm"
)

// ModelMappingHandler 处理ModelMapping相关的API请求
type ModelMappingHandler struct {
	DB *gorm.DB
}

// NewModelMappingHandler 创建ModelMappingHandler的新实例
func NewModelMappingHandler(db *gorm.DB) *ModelMappingHandler {
	return &ModelMappingHandler{
		DB: db,
	}
}

// CreateModelMapping 创建新的ModelMapping
func (h *ModelMappingHandler) CreateModelMapping(w http.ResponseWriter, r *http.Request) {
	var mapping models.ModelMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		log.Errorf("Error decoding request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.DB.Create(&mapping).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(mapping)
}

// GetModelMappings 获取所有ModelMapping列表
func (h *ModelMappingHandler) GetModelMappings(w http.ResponseWriter, r *http.Request) {
	var mappings []models.ModelMapping
	if err := h.DB.Find(&mappings).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mappings)
}
