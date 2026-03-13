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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// responseWriterWrapper 用于包装 http.ResponseWriter 以捕获响应状态码
// 用于实现 access log 记录

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader 重写 WriteHeader 方法以捕获状态码

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write 重写 Write 方法以确保默认状态码为 200

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// accessLogMiddleware 实现 access log 中间件
// 记录所有 HTTP 请求的访问日志，包括请求方法、URL、状态码、响应时间等

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 记录请求开始时间
		startTime := time.Now()

		// 创建响应包装器以捕获状态码
		rw := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     0,
		}

		// 处理请求
		next.ServeHTTP(rw, r)

		// 计算响应时间
		duration := time.Since(startTime)

		// 记录 access log
		log.Infof("Access: %s %s %d %d %s", r.Method, r.URL.Path, rw.statusCode, duration.Milliseconds(), r.UserAgent())
	})
}

func initDB(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto migrate models
	err = db.AutoMigrate(
		&models.Provider{},
		&models.ModelMapping{},
		&models.RequestLog{},
		&models.User{},
	)
	if err != nil {
		return nil, err
	}

	// 创建默认管理员用户
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		password, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			Username: "admin",
			Password: string(password),
		}
		db.Create(&admin)
		log.Info("Created default admin user: username=admin, password=admin123")
	}

	return db, nil
}

var host string
var port int

func init() {
	flag.StringVar(&host, "host", "", "Host to listen on (overrides env variable)")
	flag.IntVar(&port, "port", 0, "Port to listen on (overrides env variable)")
	flag.Parse()
}

// main函数入口
func main() {
	cfg := config.LoadConfig()

	db, err := initDB(cfg.DatabasePath)
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

	authHandler := handlers.NewAuthHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	providerHandler := handlers.NewProviderHandler(db)
	modelMappingHandler := handlers.NewModelMappingHandler(db)
	logHandler := handlers.NewLogHandler(db)
	statsHandler := handlers.NewStatsHandler(db)

	// 创建一个新的ServeMux
	mux := http.NewServeMux()

	// 登录路由不需要认证
	mux.HandleFunc("/api/login", authHandler.Login)

	// API routes with authentication
	mux.HandleFunc("/chat/completions", chatHandler.ChatCompletion)
	mux.HandleFunc("/v1/chat/completions", chatHandler.ChatCompletion)

	// Provider API routes with authentication
	mux.HandleFunc("/api/providers", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			providerHandler.GetProviders(w, r)
		} else if r.Method == "POST" {
			providerHandler.CreateProvider(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/providers/{id}", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			providerHandler.GetProvider(w, r)
		} else if r.Method == "POST" {
			providerHandler.ModifyProvider(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model Mapping API routes with authentication
	mux.HandleFunc("/api/model-mappings/{id}", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			modelMappingHandler.GetModelMapping(w, r)
		} else if r.Method == "POST" {
			modelMappingHandler.ModifyModelMapping(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/model-mappings", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			modelMappingHandler.GetModelMappings(w, r)
		} else if r.Method == "POST" {
			modelMappingHandler.CreateModelMapping(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Stats API routes with authentication
	mux.HandleFunc("/api/stats", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Provider stats API route with authentication
	mux.HandleFunc("/api/stats/providers", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetProviderStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model stats API route with authentication
	mux.HandleFunc("/api/stats/models", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			statsHandler.GetModelStats(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Log API routes with authentication
	mux.HandleFunc("/api/logs", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logHandler.GetLogs(w, r)
		} else if r.Method == "DELETE" {
			logHandler.ClearLogs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Log detail API route with authentication
	mux.HandleFunc("/api/logs/", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			pathParts := strings.Split(r.URL.Path, "/")
			if len(pathParts) >= 4 {
				id := pathParts[3]
				if id != "" {
					r.URL.RawQuery = "id=" + id
					logHandler.GetLogDetail(w, r)
					return
				}
			}
			http.Error(w, "Log ID is required", http.StatusBadRequest)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Frontend static files
	// 使用文件服务器提供静态文件
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./frontend/dist/assets"))))
	mux.Handle("/favicon.ico", http.FileServer(http.Dir("./frontend/dist")))

	// Catch-all handler for non-API routes - serve frontend index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.Error(w, "API endpoint not found", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, "./frontend/dist/index.html")
	})

	address := fmt.Sprintf("%s:%d", listenHost, listenPort)
	log.Printf("Server starting on %s\n", address)
	if err := http.ListenAndServe(address, accessLogMiddleware(mux)); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
