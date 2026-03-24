package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

type Session struct {
	ID          uint        `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt   time.Time   `gorm:"index" json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	SessionID   string      `gorm:"uniqueIndex;type:varchar(36)" json:"sessionId"`
	Events      interface{} `gorm:"type:longtext" json:"events"`
	FinalOutput JSONMap     `gorm:"type:longtext" json:"finalOutput"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return nil
}

func (s *Session) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now()
	return nil
}

func (Session) TableName() string {
	return "sessions"
}
