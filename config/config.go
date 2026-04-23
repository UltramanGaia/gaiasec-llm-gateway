package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	LogMaxCount     int64
	LogKeepCount    int64
	CleanupInterval time.Duration
	LogLevel        string
	LogFormat       string

	ServerHost        string
	ServerPort        int
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	DBHost           string
	DBPort           int
	DBUser           string
	DBPassword       string
	DBName           string
	DBDriver         string
	DBPath           string
	SessionServerKey string
}

var AppConfig *Config

func LoadConfig() *Config {
	cfg := &Config{
		LogMaxCount:       getEnvAsInt64("LOG_MAX_COUNT", 100000),
		LogKeepCount:      getEnvAsInt64("LOG_KEEP_COUNT", 50000),
		CleanupInterval:   getEnvAsDuration("CLEANUP_INTERVAL", 1*time.Hour),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		LogFormat:         getEnv("LOG_FORMAT", "text"),
		ServerHost:        getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:        getEnvAsInt("SERVER_PORT", 8090),
		ReadTimeout:       getEnvAsSeconds("SERVER_READ_TIMEOUT_SECONDS", 30*time.Second),
		ReadHeaderTimeout: getEnvAsSeconds("SERVER_READ_HEADER_TIMEOUT_SECONDS", 10*time.Second),
		WriteTimeout:      getEnvAsSeconds("SERVER_WRITE_TIMEOUT_SECONDS", 0),
		IdleTimeout:       getEnvAsSeconds("SERVER_IDLE_TIMEOUT_SECONDS", 120*time.Second),
		DBHost:            getEnv("DB_HOST", "gaiasec-mysql"),
		DBPort:            getEnvAsInt("DB_PORT", 3306),
		DBUser:            getEnv("DB_USER", "sa"),
		DBPassword:        getEnv("DB_PASSWORD", "qq123456"),
		DBName:            getEnv("DB_NAME", "gaiasec"),
		DBDriver:          getEnv("DB_DRIVER", "sqlite"),
		DBPath:            getEnv("DB_PATH", "./llm-gateway.db"),
		SessionServerKey:  getEnv("SESSION_SERVER_KEY", ""),
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

func getEnvAsSeconds(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
