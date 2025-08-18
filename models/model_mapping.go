package models

import "time"

type ModelMapping struct {
	ID         uint      `gorm:"primarykey" json:"id,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
	Alias      string    `gorm:"unique;not null" json:"alias"` // User-facing name
	ProviderID uint      `json:"providerID"`
	ModelName  string    `gorm:"not null" json:"modelName"` // Actual provider model name
}
