package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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
	Total int64               `json:"total"`
	Logs  []models.RequestLog `json:"logs"`
}

// GetLogs 获取请求日志列表，可以根据查询参数过滤和分页
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

	// 添加日期范围过滤
	if startDate := r.URL.Query().Get("startDate"); startDate != "" {
		if t, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}

	if endDate := r.URL.Query().Get("endDate"); endDate != "" {
		if t, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	// 获取分页参数
	page := 1
	pageSize := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
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
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回带分页信息的响应
	response := LogResponse{
		Total: total,
		Logs:  logs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
