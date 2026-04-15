package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type JSONSlice []interface{}

func (js JSONSlice) Value() (driver.Value, error) {
	if js == nil {
		return nil, nil
	}
	return json.Marshal(js)
}

func (js *JSONSlice) Scan(value interface{}) error {
	if value == nil {
		*js = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, js)
}

type Session struct {
	ID        uint      `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ProjectID string    `gorm:"type:varchar(64)" json:"project_id"`
	AgentID   string    `gorm:"type:varchar(36)" json:"agent_id"`
	Engine    string    `gorm:"type:varchar(128)" json:"engine"`
	SessionID string    `gorm:"uniqueIndex;type:varchar(64)" json:"session_id"`
	Events    JSONSlice `gorm:"type:longtext" json:"events"`
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
