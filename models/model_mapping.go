package models

import "gorm.io/gorm"

type ModelMapping struct {
	gorm.Model
	Alias      string `gorm:"unique;not null"` // User-facing name
	ProviderID uint
	Provider   Provider `gorm:"foreignKey:ProviderID"`
	ModelName  string   `gorm:"not null"` // Actual provider model name
}
