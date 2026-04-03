package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llm-gateway/config"
	"llm-gateway/handlers"
	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

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

		duration := time.Since(startTime)

		log.Infof("Access: %s %s %d %d %s", r.Method, r.URL.Path, rw.statusCode, duration.Milliseconds(), r.UserAgent())
	})
}

func initDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
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

	err = db.AutoMigrate(
		&models.ModelConfig{},
		&models.RequestLog{},
		&models.Session{},
	)
	if err != nil {
		return nil, err
	}

	if err := removeLegacyModelConfigNameUniqueIndexes(db); err != nil {
		return nil, err
	}

	return db, nil
}

func removeLegacyModelConfigNameUniqueIndexes(db *gorm.DB) error {
	type uniqueIndexRow struct {
		IndexName string `gorm:"column:index_name"`
	}

	var indexes []uniqueIndexRow
	if err := db.Raw(`
		SELECT DISTINCT INDEX_NAME AS index_name
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name = ?
		  AND non_unique = 0
	`, (&models.ModelConfig{}).TableName(), "name").Scan(&indexes).Error; err != nil {
		return err
	}

	for _, index := range indexes {
		if index.IndexName == "" || index.IndexName == "PRIMARY" {
			continue
		}
		if err := db.Migrator().DropIndex(&models.ModelConfig{}, index.IndexName); err != nil {
			return err
		}
		log.WithField("index", index.IndexName).Info("Dropped legacy unique index for model config name")
	}

	return nil
}

var host string
var port int

func init() {
	flag.StringVar(&host, "host", "", "Host to listen on (overrides env variable)")
	flag.IntVar(&port, "port", 0, "Port to listen on (overrides env variable)")
	flag.Parse()
}

func main() {
	cfg := config.LoadConfig()

	db, err := initDB(cfg)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	models.StartLogCleanupTask(db, cfg.LogMaxCount, cfg.LogKeepCount, cfg.CleanupInterval)
	log.Infof("Log cleanup task started: maxCount=%d, keepCount=%d, interval=%v", cfg.LogMaxCount, cfg.LogKeepCount, cfg.CleanupInterval)

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

	mux := http.NewServeMux()

	handlers.RegisterSessionRoutes(mux, db)

	mux.HandleFunc("/chat/completions", chatHandler.ChatCompletion)
	mux.HandleFunc("/v1/chat/completions", chatHandler.ChatCompletion)
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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("ok"))
	})

	address := fmt.Sprintf("%s:%d", listenHost, listenPort)
	log.Printf("Server starting on %s\n", address)

	rateLimiter := NewRateLimiter(100, time.Minute)

	handler := rateLimitMiddleware(rateLimiter, accessLogMiddleware(mux))

	if err := http.ListenAndServe(address, handler); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
