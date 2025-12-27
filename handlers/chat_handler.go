package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"llm-gateway/models"
)

// ChatHandler 处理聊天完成相关的请求
type ChatHandler struct {
	DB *gorm.DB
}

// NewChatHandler 创建ChatHandler的新实例
func NewChatHandler(db *gorm.DB) *ChatHandler {
	return &ChatHandler{
		DB: db,
	}
}

// ChatCompletion 处理聊天完成请求，根据模型名称路由到对应的LLM提供商
func (h *ChatHandler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	// 1. 允许的跨域源（根据实际需求调整，* 表示允许所有源）
	origin := r.Header.Get("Origin")
	if origin != "" {
		// 生产环境建议指定具体域名，如：w.Header().Set("Access-Control-Allow-Origin", "https://example.com")
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	// 2. 允许的 HTTP 方法
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

	// 3. 允许的自定义请求头（包含前端可能发送的头，如 Authorization、Content-Type）
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

	// 4. 允许前端读取的自定义响应头（如需前端获取自定义头，需在此声明）
	w.Header().Set("Access-Control-Expose-Headers", "X-Custom-Header")

	// 5. 预检请求（OPTIONS）直接返回 204 No Content
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Create request log entry
	reqLog := models.RequestLog{
		UserToken: r.Header.Get("Authorization"),
		CreatedAt: time.Now(),
	}
	defer func() {
		if err := h.DB.Create(&reqLog).Error; err != nil {
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
	if err := h.DB.Where("alias = ?", modelName).First(&mapping).Error; err != nil {
		log.Error("Model not found: " + modelName)
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	// Get provider information
	var provider models.Provider
	if err := h.DB.First(&provider, mapping.ProviderID).Error; err != nil {
		log.Error("Provider not found for model: " + modelName)
		http.Error(w, "Provider configuration error", http.StatusInternalServerError)
		return
	}
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
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	// 检查是否需要流式响应
	isStream := false
	if streamValue, ok := requestBody["stream"].(bool); ok && streamValue {
		isStream = true
		req.Header.Set("Accept", "text/event-stream")
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// 处理流式响应
	if isStream {
		// 用于组合流式响应内容
		var fullResponse strings.Builder
		var mu sync.Mutex               // 保护并发访问
		var contentOnly strings.Builder // 仅用于拼接content内容

		// 定义响应结构体
		var streamResponse struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content string `json:"content"`
				}
				Logprobs     interface{} `json:"logprobs"`
				FinishReason string      `json:"finish_reason"`
			}
			Usage struct {
				PromptTokens       int `json:"prompt_tokens"`
				CompletionTokens   int `json:"completion_tokens"`
				TotalTokens        int `json:"total_tokens"`
				PromptTokensDetail struct {
					CachedTokens int `json:"cached_tokens"`
				}
				PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
				PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
			}
		}

		// 流式处理响应
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Error("Error reading stream: " + err.Error())
				}
				break
			}

			// 写入客户端
			w.Write([]byte(line))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// 组合完整响应内容用于日志
			mu.Lock()
			fullResponse.WriteString(line)
			mu.Unlock()

			// 解析JSON并提取content内容
			if strings.HasPrefix(strings.TrimSpace(line), "data:") {
				// 去掉"data: "前缀
				jsonStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
				if jsonStr != "" && jsonStr != "[DONE]" {
					// 解析JSON
					if err := json.Unmarshal([]byte(jsonStr), &streamResponse); err == nil {
						// 提取content内容
						if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.Content != "" {
							mu.Lock()
							contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
							mu.Unlock()
						}
					}
				}
			}
		}

		// 记录完整的响应和仅content的响应
		if len(streamResponse.Choices) > 0 {
			streamResponse.Choices[0].Delta.Content = contentOnly.String()
		}
		respData, _ := json.Marshal(streamResponse)

		reqLog.Response = string(respData)
	} else {
		// 非流式响应，保持原有处理方式
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		respBodyDecode, err := gzipDecode(respBody)
		if err != nil {
			// Log the response
			reqLog.Response = string(respBody)
		} else {
			reqLog.Response = string(respBodyDecode)
		}

		// Write response body
		w.Write(respBody)
	}
}

func gzipDecode(input []byte) ([]byte, error) {
	// 创建gzip.Reader
	reader, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// 读取解码后的数据
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
