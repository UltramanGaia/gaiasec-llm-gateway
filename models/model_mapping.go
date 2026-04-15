package models

import (
	"time"
)

type ModelConfig struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Name           string    `gorm:"not null;type:varchar(255);index" json:"name"`
	ModelName      string    `gorm:"not null;type:varchar(255)" json:"model_name"`
	APIBaseURL     string    `gorm:"not null;type:varchar(500)" json:"api_base_url"`
	APIKey         string    `gorm:"not null;type:varchar(500)" json:"api_key"`
	MaxTokens      int       `gorm:"default:8192" json:"max_tokens"`
	Priority       int       `gorm:"default:0;index" json:"priority"`
	MaxConcurrency int       `gorm:"default:0" json:"max_concurrency"`
	Temperature    float64   `gorm:"default:0.7" json:"temperature"`
	Description    string    `gorm:"type:varchar(500)" json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
}

func (ModelConfig) TableName() string {
	return "model_configs"
}
