package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/UltramanGaia/llm-gateway/models"
	log "github.com/sirupsen/logrus"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type App struct {
	DB *gorm.DB
}

// Provider API handlers
func (app *App) createProvider(w http.ResponseWriter, r *http.Request) {
	// 使用临时结构体来避免ID类型不匹配问题
	type providerInput struct {
		Name    string `json:"name"`
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseURL"`
	}

	var input providerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 创建Provider结构体
	provider := models.Provider{
		Name:    input.Name,
		APIKey:  input.APIKey,
		BaseURL: input.BaseURL,
	}

	if err := app.DB.Create(&provider).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
}

func (app *App) getProviders(w http.ResponseWriter, r *http.Request) {
	var providers []models.Provider
	if err := app.DB.Find(&providers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(providers)
}

// Model Mapping API handlers
func (app *App) createModelMapping(w http.ResponseWriter, r *http.Request) {
	var mapping models.ModelMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := app.DB.Create(&mapping).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(mapping)
}

func (app *App) getModelMappings(w http.ResponseWriter, r *http.Request) {
	var mappings []models.ModelMapping
	if err := app.DB.Find(&mappings).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(mappings)
}

// Credential API handlers
func (app *App) generateCredential(w http.ResponseWriter, r *http.Request) {
	// Generate a new API token
	token := generateRandomToken()

	// In a real implementation, you would store this token in a database
	// with associated permissions and expiration

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// Log API handlers
func (app *App) getLogs(w http.ResponseWriter, r *http.Request) {
	var logs []models.RequestLog
	query := app.DB

	// Add filters based on query parameters
	if model := r.URL.Query().Get("model"); model != "" {
		query = query.Where("model_name = ?", model)
	}

	if userToken := r.URL.Query().Get("user_token"); userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if err := query.Find(&logs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logs)
}

// Utility functions
func generateRandomToken() string {
	// In a real implementation, use a secure random generator
	// This is a placeholder implementation
	return fmt.Sprintf("token-%d", time.Now().UnixNano())
}

// Improved chat completion handler with model routing

// 检查文件是否存在
func isFileExists(filePath string) (bool, error) {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return !info.IsDir(), err
}

// JWT密钥
var jwtKey = []byte("your-secret-key") // 在生产环境中应使用环境变量

// 定义JWT声明结构
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

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

// 登录处理函数
func (app *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 查询用户
	var user models.User
	if err := app.DB.Where("username = ?", credentials.Username).First(&user).Error; err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(credentials.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// 创建JWT令牌
	expirationTime := time.Now().Add(24 * time.Hour) // 令牌有效期24小时
	claims := &Claims{
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回令牌
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.JWTTokenResponse{
		Token: tokenString,
	})
}

// JWT认证中间件
func JWTAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 跳过登录路由的认证
		if r.URL.Path == "/api/login" {
			next(w, r)
			return
		}

		// 从Authorization头获取令牌
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	// 提取令牌
	bearerToken := strings.Split(authHeader, " ")
	if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
		http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
		return
	}

	tokenString := bearerToken[1]

	// 解析令牌
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// 继续处理请求
	next(w, r)
	}
}

var host string
var port int64

func init() {
	flag.StringVar(&host, "host", "0.0.0.0", "Host to listen on")
	flag.Int64Var(&port, "port", 8000, "Port to listen on")
}

func (app *App) chatCompletion(w http.ResponseWriter, r *http.Request) {
	// Create request log entry
	reqLog := models.RequestLog{
		UserToken: r.Header.Get("Authorization"),
		CreatedAt: time.Now(),
	}
	defer func() {
		if err := app.DB.Create(&reqLog).Error; err != nil {
			log.Error("Failed to save request log: " + err.Error())
		}
	}()

	// Log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reqLog.Request = string(body)

	// Parse request to get model name
	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		http.Error(w, "Model name is required", http.StatusBadRequest)
		return
	}
	reqLog.ModelName = modelName

	// Find model mapping
	var mapping models.ModelMapping
	if err := app.DB.Where("alias = ?", modelName).Preload("Provider").First(&mapping).Error; err != nil {
		log.Error("Model not found: " + modelName)
		http.Error(w, "Model not found: " + modelName, http.StatusNotFound)
		return
	}

	// Get provider information
	provider := mapping.Provider
	actualModelName := mapping.ModelName

	// Update request with actual model name
	requestBody["model"] = actualModelName
	updatedBody, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create new request to provider
	providerURL := provider.BaseURL
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	providerURL += "chat/completions"

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers and set API key
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer " + provider.APIKey)

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the response
	reqLog.Response = string(respBody)

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Write response body
	w.Write(respBody)
}

func main() {
	flag.Parse()

	db, err := initDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Create an instance of the app structure
	app := &App{
		DB: db,
	}

	// 登录路由不需要认证
	http.HandleFunc("/api/login", app.login)

	// API routes with authentication
	http.HandleFunc("/chat/completions", JWTAuthMiddleware(app.chatCompletion))

	// Provider API routes with authentication
	http.HandleFunc("/api/providers", JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			app.getProviders(w, r)
		} else if r.Method == "POST" {
			app.createProvider(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model Mapping API routes with authentication
	http.HandleFunc("/api/model-mappings", JWTAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			app.getModelMappings(w, r)
		} else if r.Method == "POST" {
			app.createModelMapping(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Credential API routes with authentication
	http.HandleFunc("/api/credentials/generate", JWTAuthMiddleware(app.generateCredential))

	// Log API routes with authentication
	http.HandleFunc("/api/logs", JWTAuthMiddleware(app.getLogs))

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
