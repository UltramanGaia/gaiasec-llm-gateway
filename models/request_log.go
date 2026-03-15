package models

import (
	"time"

	"llm-gateway/utils"

	"gorm.io/gorm"
)

type RequestLog struct {
	ID             string    `gorm:"primarykey;type:varchar(32)" json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	UserToken      string    `gorm:"index;type:varchar(255)" json:"userToken"`
	ModelName      string    `gorm:"index;type:varchar(255)" json:"modelName"`
	ProviderID     string    `gorm:"index;type:varchar(32)" json:"providerId"`
	Fingerprint    string    `gorm:"index;type:varchar(32)" json:"fingerprint"`
	ResponseTime   int64     `json:"responseTime"`
	Request        string    `gorm:"type:longtext" json:"request"`
	Response       string    `gorm:"type:longtext" json:"response"`
	StreamResponse string    `gorm:"type:longtext" json:"streamResponse"`
}

func (r *RequestLog) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = utils.GenerateID()
	}
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	return nil
}

func (r *RequestLog) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	return nil
}
