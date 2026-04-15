package models

import (
	"time"

	"gorm.io/gorm"
)

type RequestLog struct {
	ID                uint      `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ModelName         string    `gorm:"index;type:varchar(255)" json:"model_name"`
	BackendConfigID   uint      `gorm:"index" json:"backend_config_id"`
	BackendModelName  string    `gorm:"index;type:varchar(255)" json:"backend_model_name"`
	BackendAPIBaseURL string    `gorm:"type:varchar(500)" json:"backend_api_base_url"`
	Fingerprint       string    `gorm:"index;type:varchar(32)" json:"fingerprint"`
	ResponseTime      int64     `json:"response_time"`
	FirstTokenLatency int64     `json:"first_token_latency"`
	AvgTokenLatency   float64   `json:"avg_token_latency"`
	ActiveRequests    int       `json:"active_requests"`
	Request           string    `gorm:"type:longtext" json:"request"`
	Response          string    `gorm:"type:longtext" json:"response"`
	StreamResponse    []byte    `gorm:"type:longblob" json:"stream_response"`
}

func (r *RequestLog) BeforeCreate(tx *gorm.DB) error {
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	return nil
}

func (r *RequestLog) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	return nil
}

func (RequestLog) TableName() string {
	return "request_logs"
}
