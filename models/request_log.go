package models

import (
	"time"

	"gorm.io/gorm"
)

type RequestLog struct {
	ID             uint      `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt      time.Time `gorm:"index" json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ModelName      string    `gorm:"index;type:varchar(255)" json:"modelName"`
	Fingerprint    string    `gorm:"index;type:varchar(32)" json:"fingerprint"`
	ResponseTime   int64     `json:"responseTime"`
	Request        string    `gorm:"type:longtext" json:"request"`
	Response       string    `gorm:"type:longtext" json:"response"`
	StreamResponse []byte    `gorm:"type:longblob" json:"streamResponse"`
}

func (r *RequestLog) BeforeCreate(tx *gorm.DB) error {
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	return nil
}

func (r *RequestLog) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	return nil
}

func (RequestLog) TableName() string {
	return "request_logs"
}
