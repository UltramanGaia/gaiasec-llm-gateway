package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func GenerateID() string {
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	return hex.EncodeToString([]byte(time.Now().Format("20060102150405"))) + hex.EncodeToString(randomBytes)
}
