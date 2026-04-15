package models

import (
	"time"
)

type ModelConfig struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"not null;type:varchar(255);index" json:"name"`
	ModelName   string    `gorm:"not null;type:varchar(255)" json:"modelName"`
	APIBaseURL  string    `gorm:"not null;type:varchar(500)" json:"apiBaseUrl"`
	APIKey      string    `gorm:"not null;type:varchar(500)" json:"apiKey"`
	MaxTokens   int       `gorm:"default:8192" json:"maxTokens"`
	Temperature float64   `gorm:"default:0.7" json:"temperature"`
	Description string    `gorm:"type:varchar(500)" json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
}

func (ModelConfig) TableName() string {
	return "model_configs"
}
