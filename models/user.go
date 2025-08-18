package models

import (
	"time"
)

// User 模型定义用户信息
type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Username  string    `gorm:"unique;not null" json:"username"`
	Password  string    `gorm:"not null" json:"password"` // 存储加密后的密码
}

// JWTTokenResponse 定义JWT登录响应结构
type JWTTokenResponse struct {
	Token string `json:"token"`
}
