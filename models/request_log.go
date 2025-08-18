package models

import "time"

type RequestLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	UserToken string    `gorm:"index" json:"userToken"`
	ModelName string    `gorm:"index" json:"modelName"`
	Request   string    `gorm:"type:text" json:"request"`
	Response  string    `gorm:"type:text" json:"response"`
}
