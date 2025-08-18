package models

import "time"

type Provider struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Name      string    `gorm:"unique;not null" json:"name"`
	APIKey    string    `gorm:"not null" json:"apiKey"`
	BaseURL   string    `gorm:"not null" json:"baseUrl"`
}
