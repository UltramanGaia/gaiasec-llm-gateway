package main

import (
	"os"
	"strings"

	"llm-gateway/config"

	log "github.com/sirupsen/logrus"
)

func initLogger(cfg *config.Config) {
	log.SetOutput(os.Stdout)

	level, err := log.ParseLevel(strings.ToLower(strings.TrimSpace(cfg.LogLevel)))
	if err != nil {
		log.WithError(err).WithField("configured_level", cfg.LogLevel).Warn("Invalid LOG_LEVEL, falling back to info")
		level = log.InfoLevel
	}
	log.SetLevel(level)

	switch strings.ToLower(strings.TrimSpace(cfg.LogFormat)) {
	case "", "text":
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			PadLevelText:    true,
			DisableQuote:    true,
		})
	case "json":
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			PadLevelText:    true,
			DisableQuote:    true,
		})
		log.WithField("configured_format", cfg.LogFormat).Warn("Invalid LOG_FORMAT, falling back to text")
	}
}
