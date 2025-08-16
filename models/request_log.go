package models

import (
	"time"

	"gorm.io/gorm"
)

type RequestLog struct {
	gorm.Model
	UserToken string    `gorm:"index"`
	ModelName string    `gorm:"index"`
	Request   string    `gorm:"type:text"`
	Response  string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"index"`
}
