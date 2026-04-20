package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"llm-gateway/config"

	log "github.com/sirupsen/logrus"
)

type humanTextFormatter struct{}

func (f *humanTextFormatter) Format(entry *log.Entry) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(entry.Time.Format("2006-01-02 15:04:05.000"))
	buf.WriteByte(' ')
	buf.WriteString(strings.ToUpper(entry.Level.String()))
	buf.WriteByte(' ')
	buf.WriteString(entry.Message)

	if len(entry.Data) > 0 {
		keys := make([]string, 0, len(entry.Data))
		for key := range entry.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			buf.WriteByte(' ')
			buf.WriteString(key)
			buf.WriteByte('=')
			buf.WriteString(fmt.Sprint(entry.Data[key]))
		}
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

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
		log.SetFormatter(&humanTextFormatter{})
	case "json":
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		log.SetFormatter(&humanTextFormatter{})
		log.WithField("configured_format", cfg.LogFormat).Warn("Invalid LOG_FORMAT, falling back to text")
	}
}
