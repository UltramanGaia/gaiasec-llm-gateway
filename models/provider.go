package models

import (
	"time"

	"llm-gateway/utils"

	"gorm.io/gorm"
)

type Provider struct {
	ID        string    `gorm:"primarykey;type:varchar(32)" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Name      string    `gorm:"unique;not null;type:varchar(255)" json:"name"`
	APIKey    string    `gorm:"not null;type:varchar(500)" json:"apiKey"`
	BaseURL   string    `gorm:"not null;type:varchar(500)" json:"baseUrl"`
}

func (p *Provider) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = utils.GenerateID()
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Provider) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	return nil
}
