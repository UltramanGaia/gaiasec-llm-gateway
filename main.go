package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"llm-gateway/config"
	"llm-gateway/handlers"
	"llm-gateway/models"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

//go:embed frontend/dist
var frontendFS embed.FS

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		rw := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     0,
		}

		next.ServeHTTP(rw, r)

		if rw.statusCode == 0 {
			rw.statusCode = http.StatusOK
		}
		if shouldSkipAccessLog(r.URL.Path, rw.statusCode) {
			return
		}

		duration := time.Since(startTime)
		message := accessLogMessage(r, rw.statusCode, duration)
		switch {
		case rw.statusCode >= http.StatusInternalServerError:
			log.Error(message)
		case rw.statusCode >= http.StatusBadRequest:
			log.Warn(message)
		default:
			log.Info(message)
		}
	})
}

func shouldSkipAccessLog(path string, status int) bool {
	if status >= http.StatusBadRequest {
		return false
	}

	switch path {
	case "/actuator/health", "/health", "/healthz":
		return true
	default:
		return false
	}
}

func accessLogMessage(r *http.Request, status int, duration time.Duration) string {
	message := fmt.Sprintf(
		"access %s %s status=%d duration=%dms",
		r.Method,
		r.URL.Path,
		status,
		duration.Milliseconds(),
	)
	if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
		message += fmt.Sprintf(" query=%q", rawQuery)
	}
	return message
}

func initDB(cfg *config.Config) (*gorm.DB, error) {
	gormLog := gormlogger.New(stdlog.New(os.Stdout, "\r\n", stdlog.LstdFlags), gormlogger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	})

	var dialector gorm.Dialector
	if cfg.DBDriver == "mysql" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
		dialector = mysql.Open(dsn)
	} else {
		dialector = sqlite.Open(cfg.DBPath)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := validateDatabaseSchema(db); err != nil {
		return nil, err
	}

	return db, nil
}

type schemaRequirement struct {
	model   any
	table   string
	columns []string
}

func validateDatabaseSchema(db *gorm.DB) error {
	requirements := []schemaRequirement{
		{
			model: &models.ModelConfig{},
			table: (&models.ModelConfig{}).TableName(),
			columns: []string{
				"id", "name", "model_name", "api_base_url", "api_key",
				"max_tokens", "priority", "max_concurrency", "temperature",
				"description", "created_at", "updated_at", "enabled",
			},
		},
		{
			model: &models.RequestLog{},
			table: (&models.RequestLog{}).TableName(),
			columns: []string{
				"id", "created_at", "updated_at", "model_name", "backend_config_id",
				"backend_model_name", "backend_api_base_url", "fingerprint",
				"response_time", "first_token_latency", "avg_token_latency",
				"active_requests", "request", "response", "stream_response",
				"request_bytes", "response_bytes", "stream_bytes",
			},
		},
		{
			model: &models.Session{},
			table: (&models.Session{}).TableName(),
			columns: []string{
				"id", "created_at", "updated_at", "project_id",
				"agent_id", "engine", "session_id", "events",
			},
		},
	}

	for _, requirement := range requirements {
		if !db.Migrator().HasTable(requirement.model) {
			return fmt.Errorf("required table %s is missing; apply schema.sql and migrations before starting llm-gateway", requirement.table)
		}
		for _, column := range requirement.columns {
			if !db.Migrator().HasColumn(requirement.model, column) {
				return fmt.Errorf("required column %s.%s is missing; apply schema.sql and migrations before starting llm-gateway", requirement.table, column)
			}
		}
	}

	return nil
}

func initDBWithRetry(cfg *config.Config) (*gorm.DB, error) {
	var lastErr error
	delay := 2 * time.Second

	for attempt := 1; attempt <= 12; attempt++ {
		db, err := initDB(cfg)
		if err == nil {
			if attempt > 1 {
				log.WithField("attempt", attempt).Info("Database initialization recovered")
			}
			return db, nil
		}

		lastErr = err
		if attempt == 12 {
			break
		}

		log.WithFields(log.Fields{
			"attempt":      attempt,
			"retry_in_sec": int(delay / time.Second),
		}).WithError(err).Warn("Database initialization failed, retrying")
		time.Sleep(delay)
		if delay < 10*time.Second {
			delay *= 2
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
		}
	}

	return nil, fmt.Errorf("initialize database after retries: %w", lastErr)
}

var host string
var port int

func init() {
	flag.StringVar(&host, "host", "", "Host to listen on (overrides env variable)")
	flag.IntVar(&port, "port", 0, "Port to listen on (overrides env variable)")
}

func main() {
	flag.Parse()

	cfg := config.LoadConfig()
	initLogger(cfg)

	db, err := initDBWithRetry(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	models.StartLogCleanupTask(db, cfg.LogMaxCount, cfg.LogKeepCount, cfg.CleanupInterval)
	log.WithFields(log.Fields{
		"max_count":  cfg.LogMaxCount,
		"keep_count": cfg.LogKeepCount,
		"interval":   cfg.CleanupInterval,
	}).Info("Request log cleanup task started")

	listenHost := cfg.ServerHost
	listenPort := cfg.ServerPort

	if host != "" {
		listenHost = host
	}
	if port != 0 {
		listenPort = port
	}

	chatHandler := handlers.NewChatHandler(db)
	modelConfigHandler := handlers.NewModelConfigHandler(db)
	logHandler := handlers.NewLogHandler(db)
	statsHandler := handlers.NewStatsHandler(db)

	frontendHandler, err := handlers.NewFrontendHandler(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatal("Failed to load frontend:", err)
	}

	mux := http.NewServeMux()

	handlers.RegisterSessionRoutes(mux, db)

	mux.HandleFunc("/actuator/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/chat/completions", chatHandler.ChatCompletion)
	mux.HandleFunc("/v1/chat/completions", chatHandler.ChatCompletion)
	mux.HandleFunc("/v1/messages", chatHandler.AnthropicMessages)
	mux.HandleFunc("/v1/models", chatHandler.ListModels)

	mux.HandleFunc("/api/model-mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			modelConfigHandler.GetModelConfig(w, r)
		} else if r.Method == "POST" || r.Method == "PUT" {
			modelConfigHandler.ModifyModelConfig(w, r)
		} else if r.Method == "DELETE" {
			modelConfigHandler.DeleteModelConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-mappings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			modelConfigHandler.GetModelConfigs(w, r)
		} else if r.Method == "POST" {
			modelConfigHandler.CreateModelConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/enabled", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			modelConfigHandler.GetEnabledModelConfigs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/{id}/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			modelConfigHandler.TestModelConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/{id}/clone", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			modelConfigHandler.CloneModelConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/{id}/reset-runtime", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			modelConfigHandler.ResetModelConfigRuntime(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			modelConfigHandler.GetModelConfig(w, r)
		case http.MethodPut:
			modelConfigHandler.ModifyModelConfig(w, r)
		case http.MethodDelete:
			modelConfigHandler.DeleteModelConfig(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			modelConfigHandler.GetModelConfigs(w, r)
		case http.MethodPost:
			modelConfigHandler.CreateModelConfig(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/model-configs/reset-runtime", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			modelConfigHandler.ResetAllModelConfigRuntime(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/stats/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetProviderStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/stats/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetModelStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/request-logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logHandler.GetLogs(w, r)
		} else if r.Method == "DELETE" {
			logHandler.ClearLogs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/request-logs/", func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) >= 4 {
			id := pathParts[3]
			if id != "" {
				if len(pathParts) >= 5 && pathParts[4] == "replay" {
					if r.Method == "POST" {
						r.URL.RawQuery = "id=" + id
						logHandler.ReplayLog(w, r)
						return
					}
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if r.Method == "GET" {
					r.URL.RawQuery = "id=" + id
					logHandler.GetLogDetail(w, r)
					return
				}
			}
		}
		if r.Method == "GET" {
			http.Error(w, "Log ID is required", http.StatusBadRequest)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.Error(w, "API endpoint not found", http.StatusNotFound)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})

	address := fmt.Sprintf("%s:%d", listenHost, listenPort)
	log.WithFields(log.Fields{
		"address":        address,
		"db_driver":      cfg.DBDriver,
		"log_level":      cfg.LogLevel,
		"log_format":     cfg.LogFormat,
		"read_timeout":   cfg.ReadTimeout,
		"write_timeout":  cfg.WriteTimeout,
		"idle_timeout":   cfg.IdleTimeout,
		"header_timeout": cfg.ReadHeaderTimeout,
	}).Info("Server starting")

	handler := accessLogMiddleware(mux)

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("Server failed to start:", err)
	}
}
