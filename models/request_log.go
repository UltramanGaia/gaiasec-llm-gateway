package models

import "time"

type RequestLog struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	UserToken      string    `gorm:"index" json:"userToken"`
	ModelName      string    `gorm:"index" json:"modelName"`
	ProviderID     uint      `gorm:"index" json:"providerId"`
	ResponseTime   int64     `json:"responseTime"`
	Request        string    `gorm:"type:text" json:"request"`
	Response       string    `gorm:"type:text" json:"response"`
	StreamResponse string    `gorm:"type:text" json:"streamResponse"`
}
