package models

import (
	"time"

	"gorm.io/gorm"
)

// User 模型定义用户信息
type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"` // 存储加密后的密码
	CreatedAt time.Time
}

// JWTTokenResponse 定义JWT登录响应结构
type JWTTokenResponse struct {
	Token string `json:"token"`
}