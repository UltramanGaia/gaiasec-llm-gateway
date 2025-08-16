package models

import (
	"time"

	"gorm.io/gorm"
)

type Provider struct {
	gorm.Model
	Name      string `gorm:"unique;not null"`
	APIKey    string `gorm:"not null"`
	BaseURL   string `gorm:"not null"`
	CreatedAt time.Time
}
