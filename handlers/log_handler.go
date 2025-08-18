package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/UltramanGaia/llm-gateway/models"
	"gorm.io/gorm"
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

// GetLogs 获取请求日志列表，可以根据查询参数过滤
func (h *LogHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	var logs []models.RequestLog
	query := h.DB

	// Add filters based on query parameters
	if model := r.URL.Query().Get("model"); model != "" {
		query = query.Where("model_name = ?", model)
	}

	if userToken := r.URL.Query().Get("user_token"); userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if err := query.Find(&logs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
