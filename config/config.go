package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	LogMaxCount         int64
	LogKeepCount        int64
	CleanupInterval     time.Duration
	DatabasePath        string
	ServerHost          string
	ServerPort          int
}

var AppConfig *Config

func LoadConfig() *Config {
	cfg := &Config{
		LogMaxCount:         getEnvAsInt64("LOG_MAX_COUNT", 100000),
		LogKeepCount:        getEnvAsInt64("LOG_KEEP_COUNT", 50000),
		CleanupInterval:     getEnvAsDuration("CLEANUP_INTERVAL", 1*time.Hour),
		DatabasePath:        getEnv("DATABASE_PATH", "llm.db"),
		ServerHost:          getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:          getEnvAsInt("SERVER_PORT", 8090),
	}
	
	AppConfig = cfg
	return cfg
}

func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if hours, err := strconv.Atoi(value); err == nil {
			return time.Duration(hours) * time.Hour
		}
	}
	return defaultValue
}
