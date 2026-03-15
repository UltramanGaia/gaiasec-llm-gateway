package models

import (
	"time"

	"llm-gateway/utils"

	"gorm.io/gorm"
)

type ModelMapping struct {
	ID         string    `gorm:"primarykey;type:varchar(32)" json:"id,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
	Alias      string    `gorm:"unique;not null;type:varchar(255)" json:"alias"`
	ProviderID string    `gorm:"type:varchar(32)" json:"providerID"`
	ModelName  string    `gorm:"not null;type:varchar(255)" json:"modelName"`
}

func (m *ModelMapping) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = utils.GenerateID()
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	return nil
}

func (m *ModelMapping) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}
