package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
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

// handleCORS 处理跨域请求设置
func (h *ChatHandler) handleCORS(w http.ResponseWriter, r *http.Request) bool {
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
		return true
	}

	return false
}

// parseRequest 解析请求体，获取模型名称和流式标志
func (h *ChatHandler) parseRequest(r *http.Request) ([]byte, map[string]interface{}, string, string, bool, error) {
	// Log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, "", "", false, err
	}

	// Parse request to get model name
	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		return nil, nil, "", "", false, err
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		return nil, nil, "", "", false, errors.New("Model name is required")
	}

	userToken := r.Header.Get("Authorization")

	// 检查是否需要流式响应
	isStream := false
	if streamValue, ok := requestBody["stream"].(bool); ok && streamValue {
		isStream = true
	}

	return body, requestBody, modelName, userToken, isStream, nil
}

// handleCache 检查缓存，如果命中则返回响应并返回true，否则返回false
func (h *ChatHandler) handleCache(w http.ResponseWriter, body []byte, modelName, userToken string, isStream bool) bool {
	// 检查缓存：根据请求体、模型名称和用户令牌查询是否有相同请求
	var existingLog models.RequestLog
	if err := h.DB.Where("request = ? AND model_name = ? AND user_token = ?", string(body), modelName, userToken).First(&existingLog).Error; err != nil {
		// 缓存未命中，继续处理请求
		return false
	}

	// 找到缓存，根据请求类型返回响应
	if isStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		if existingLog.StreamResponse != "" {
			w.Write([]byte(existingLog.StreamResponse))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return true
		}

		var cachedResponse struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Index   int `json:"index"`
				Message struct {
					Content          string `json:"content"`
					Role             string `json:"role"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				Logprobs     interface{} `json:"logprobs"`
				FinishReason string      `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens       int `json:"prompt_tokens"`
				CompletionTokens   int `json:"completion_tokens"`
				TotalTokens        int `json:"total_tokens"`
				PromptTokensDetail struct {
					CachedTokens int `json:"cached_tokens"`
				}
				PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
				PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(existingLog.Response), &cachedResponse); err == nil {
			// 构建SSE响应
			var deltaResponse struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Choices []struct {
					Index int `json:"index"`
					Delta struct {
						Role             string `json:"role"`
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
					Logprobs     interface{} `json:"logprobs"`
					FinishReason string      `json:"finish_reason"`
				} `json:"choices"`
			}

			// 设置基础字段
			deltaResponse.ID = cachedResponse.ID
			deltaResponse.Object = cachedResponse.Object
			deltaResponse.Created = cachedResponse.Created
			deltaResponse.Model = cachedResponse.Model
			deltaResponse.Choices = make([]struct {
				Index int `json:"index"`
				Delta struct {
					Role             string `json:"role"`
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				Logprobs     interface{} `json:"logprobs"`
				FinishReason string      `json:"finish_reason"`
			}, len(cachedResponse.Choices))

			// 发送初始消息（role）
			for i, choice := range cachedResponse.Choices {
				deltaResponse.Choices[i].Index = choice.Index
				deltaResponse.Choices[i].Delta.Role = "assistant"
				deltaResponse.Choices[i].Delta.Content = ""

				jsonData, _ := json.Marshal(deltaResponse)
				w.Write([]byte("data: " + string(jsonData) + "\n\n"))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}

				// 获取content和reasoning_content
				content := choice.Message.Content
				if content == "" {
					content = choice.Delta.Content
				}
				reasoningContent := choice.Message.ReasoningContent
				if reasoningContent == "" {
					reasoningContent = choice.Delta.ReasoningContent
				}

				// 先发送reasoning_content内容（逐字符发送）
				for _, char := range reasoningContent {
					deltaResponse.Choices[i].Delta.Content = ""
					deltaResponse.Choices[i].Delta.ReasoningContent = string(char)
					jsonData, _ := json.Marshal(deltaResponse)
					w.Write([]byte("data: " + string(jsonData) + "\n\n"))
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}

				// 发送content内容（逐字符或逐词发送）
				for _, char := range content {
					deltaResponse.Choices[i].Delta.Content = string(char)
					deltaResponse.Choices[i].Delta.ReasoningContent = ""
					jsonData, _ := json.Marshal(deltaResponse)
					w.Write([]byte("data: " + string(jsonData) + "\n\n"))
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}

				// 发送finish事件
				deltaResponse.Choices[i].Delta.Content = ""
				deltaResponse.Choices[i].Delta.ReasoningContent = ""
				deltaResponse.Choices[i].FinishReason = "stop"
				jsonData, _ = json.Marshal(deltaResponse)
				w.Write([]byte("data: " + string(jsonData) + "\n\n"))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}

			// 发送结束标记
			w.Write([]byte("data: [DONE]\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		} else {
			// 解析失败，返回JSON响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(existingLog.Response))
		}
	} else {
		// 非流式请求：直接返回JSON响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(existingLog.Response))
	}

	return true
}

// getModelAndProvider 获取模型映射和提供商信息
func (h *ChatHandler) getModelAndProvider(modelName string) (string, models.Provider, error) {
	// Find model mapping
	var mapping models.ModelMapping
	if err := h.DB.Where("alias = ?", modelName).First(&mapping).Error; err != nil {
		return "", models.Provider{}, err
	}

	// Get provider information
	var provider models.Provider
	if err := h.DB.First(&provider, mapping.ProviderID).Error; err != nil {
		return "", models.Provider{}, err
	}

	return mapping.ModelName, provider, nil
}

// sendProviderRequest 发送请求到提供商API
func (h *ChatHandler) sendProviderRequest(r *http.Request, requestBody map[string]interface{}, actualModelName string, provider models.Provider, isStream bool) (*http.Response, error) {
	// Update request with actual model name
	requestBody["model"] = actualModelName
	updatedBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 安全处理嵌套map的判断和赋值
	// 第一步：获取"reasoning"的值，并断言为map[string]interface{}
	reasoningVal, reasoningOk := requestBody["reasoning"].(map[string]interface{})
	// 第二步：判断reasoning存在且是map类型，同时其下的"effort"非nil
	if reasoningOk && reasoningVal["effort"] != nil {
		reasoningVal["effort"] = "none"
	}

	// Create new request to provider
	providerURL := provider.BaseURL
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	providerURL += "chat/completions"

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		return nil, err
	}

	// Copy headers and set API key
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	// 检查是否需要流式响应
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// handleStreamResponse 处理流式响应
func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	// 用于组合流式响应内容
	var fullResponse strings.Builder
	var mu sync.Mutex                        // 保护并发访问
	var contentOnly strings.Builder          // 仅用于拼接content内容
	var reasoningContentOnly strings.Builder // 仅用于拼接reasoning_content内容

	// 定义响应结构体
	var streamResponse struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
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

	// 用于保存第一个事件的元数据
	var firstID, firstObject, firstModel string
	var firstCreated int64
	var hasMetadata bool

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
					// 保存第一个事件的元数据
					if !hasMetadata && streamResponse.ID != "" {
						firstID = streamResponse.ID
						firstObject = streamResponse.Object
						firstCreated = streamResponse.Created
						firstModel = streamResponse.Model
						hasMetadata = true
					}
					// 提取content内容
					if len(streamResponse.Choices) > 0 {
						if streamResponse.Choices[0].Delta.Content != "" {
							mu.Lock()
							contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
							mu.Unlock()
						}
						// 提取reasoning_content内容
						if streamResponse.Choices[0].Delta.ReasoningContent != "" {
							mu.Lock()
							reasoningContentOnly.WriteString(streamResponse.Choices[0].Delta.ReasoningContent)
							mu.Unlock()
						}
					}
				}
			}
		}
	}

	// 构建缓存响应（使用非流式格式，包含Message字段）
	cachedResp := struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
			} `json:"message"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		} `json:"choices"`
		Usage struct {
			PromptTokens       int `json:"prompt_tokens"`
			CompletionTokens   int `json:"completion_tokens"`
			TotalTokens        int `json:"total_tokens"`
			PromptTokensDetail struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_detail"`
			PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
		} `json:"usage"`
	}{
		ID:      firstID,
		Object:  firstObject,
		Created: firstCreated,
		Model:   firstModel,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role             string `json:"role"`
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
			} `json:"message"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		}{
			{
				Index: 0,
				Message: struct {
					Role             string `json:"role"`
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content,omitempty"`
				}{
					Role:             "assistant",
					Content:          contentOnly.String(),
					ReasoningContent: reasoningContentOnly.String(),
				},
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
	}

	cachedResp.Usage.PromptTokens = streamResponse.Usage.PromptTokens
	cachedResp.Usage.CompletionTokens = streamResponse.Usage.CompletionTokens
	cachedResp.Usage.TotalTokens = streamResponse.Usage.TotalTokens
	cachedResp.Usage.PromptTokensDetail.CachedTokens = streamResponse.Usage.PromptTokensDetail.CachedTokens
	cachedResp.Usage.PromptCacheHitTokens = streamResponse.Usage.PromptCacheHitTokens
	cachedResp.Usage.PromptCacheMissTokens = streamResponse.Usage.PromptCacheMissTokens

	respData, _ := json.Marshal(cachedResp)
	reqLog.Response = string(respData)
	reqLog.StreamResponse = fullResponse.String()
}

// handleNonStreamResponse 处理非流式响应
func (h *ChatHandler) handleNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) error {
	// 非流式响应，保持原有处理方式
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
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
	return nil
}

// ChatCompletion 处理聊天完成请求，根据模型名称路由到对应的LLM提供商
func (h *ChatHandler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	// 处理CORS
	if h.handleCORS(w, r) {
		return
	}

	body, requestBody, modelName, userToken, isStream, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(string(body))

	// 检查缓存
	if h.handleCache(w, body, modelName, userToken, isStream) {
		return
	}

	// 创建请求日志条目（仅当缓存未命中时）
	reqLog := models.RequestLog{
		UserToken: userToken,
		CreatedAt: time.Now(),
		ModelName: modelName,
		Request:   string(body),
	}
	defer func() {
		if err := h.DB.Create(&reqLog).Error; err != nil {
			log.Error("Failed to save request log: " + err.Error())
		}
	}()

	// 获取模型和提供商信息
	actualModelName, provider, err := h.getModelAndProvider(modelName)
	if err != nil {
		log.Error("Model or provider not found: " + modelName)
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	// 发送请求到提供商API
	resp, err := h.sendProviderRequest(r, requestBody, actualModelName, provider, isStream)
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

	// 处理响应
	if isStream {
		h.handleStreamResponse(w, resp, &reqLog)
	} else {
		if err := h.handleNonStreamResponse(w, resp, &reqLog); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
