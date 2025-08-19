package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/UltramanGaia/llm-gateway/handlers"
	"github.com/UltramanGaia/llm-gateway/models"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("llm.db"), &gorm.Config{})
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
var port int64

func init() {
	flag.StringVar(&host, "host", "0.0.0.0", "Host to listen on")
	flag.Int64Var(&port, "port", 8000, "Port to listen on")
	flag.Parse()
}

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// 初始化各个处理器
	authHandler := handlers.NewAuthHandler(db)
	chatHandler := handlers.NewChatHandler(db)
	providerHandler := handlers.NewProviderHandler(db)
	modelMappingHandler := handlers.NewModelMappingHandler(db)
	logHandler := handlers.NewLogHandler(db)

	// 登录路由不需要认证
	http.HandleFunc("/api/login", authHandler.Login)

	// API routes with authentication
	http.HandleFunc("/chat/completions", chatHandler.ChatCompletion)
	//http.HandleFunc("/chat/completions", handlers.JWTAuthMiddleware(chatHandler.ChatCompletion))

	// Provider API routes with authentication
	http.HandleFunc("/api/providers", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			providerHandler.GetProviders(w, r)
		} else if r.Method == "POST" {
			providerHandler.CreateProvider(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model Mapping API routes with authentication
	http.HandleFunc("/api/model-mappings", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			modelMappingHandler.GetModelMappings(w, r)
		} else if r.Method == "POST" {
			modelMappingHandler.CreateModelMapping(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Log API routes with authentication
	http.HandleFunc("/api/logs", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			logHandler.GetLogs(w, r)
		} else if r.Method == "DELETE" {
			logHandler.ClearLogs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Log detail API route with authentication
	http.HandleFunc("/api/logs/", handlers.JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
	http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./frontend/dist/assets"))))
	http.Handle("/favicon.ico", http.FileServer(http.Dir("./frontend/dist")))

	// 为SPA应用设置路由fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 如果请求的不是API路由，返回index.html以支持前端路由
		if !strings.HasPrefix(r.URL.Path, "/chat/") && !strings.HasPrefix(r.URL.Path, "/api/") {
			// 读取并提供index.html
			indexPath := "./frontend/dist/index.html"
			file, err := os.Open(indexPath)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// 设置内容类型
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// 复制文件内容到响应
			io.Copy(w, file)
		}
	})

	// 启动服务器
	address := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Server starting on %s\n", address)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
