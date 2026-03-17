package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read request body")
		return nil, nil, "", "", false, err
	}
	log.WithField("body_length", len(body)).Debug("Request body read successfully")

	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		log.WithError(err).WithField("body", string(body)).Error("Failed to parse request body as JSON")
		return nil, nil, "", "", false, err
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		log.Error("Model name is missing in request")
		return nil, nil, "", "", false, errors.New("Model name is required")
	}

	userToken := r.Header.Get("Authorization")

	isStream := false
	if streamValue, ok := requestBody["stream"].(bool); ok && streamValue {
		isStream = true
	}

	log.WithFields(log.Fields{
		"model":     modelName,
		"is_stream": isStream,
		"has_token": userToken != "",
	}).Info("Request parsed successfully")

	return body, requestBody, modelName, userToken, isStream, nil
}

func calculateFingerprint(request, modelName, userToken string) string {
	data := request + "|" + modelName + "|" + userToken
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// handleCache 检查缓存，如果命中则返回响应并返回true，否则返回false
func (h *ChatHandler) handleCache(w http.ResponseWriter, body []byte, modelName, userToken string, isStream bool) bool {
	fingerprint := calculateFingerprint(string(body), modelName, userToken)
	log.WithField("fingerprint", fingerprint).Debug("Checking cache for request")

	var existingLog models.RequestLog
	if err := h.DB.Where("fingerprint = ?", fingerprint).First(&existingLog).Error; err != nil {
		log.WithField("fingerprint", fingerprint).Info("Cache miss, will forward request to provider")
		return false
	}

	log.WithFields(log.Fields{
		"fingerprint": fingerprint,
		"is_stream":   isStream,
		"model":       modelName,
	}).Info("Cache hit, returning cached response")

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

// getModelConfig 获取模型配置信息
func (h *ChatHandler) getModelConfig(modelName string) (models.ModelConfig, error) {
	log.WithField("model", modelName).Debug("Looking up model config")

	var config models.ModelConfig
	if err := h.DB.Where("name = ? AND enabled = ?", modelName, true).First(&config).Error; err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model config not found or disabled")
		return models.ModelConfig{}, err
	}

	log.WithFields(log.Fields{
		"model":      modelName,
		"api_base":   config.APIBaseURL,
		"model_name": config.ModelName,
	}).Debug("Model config found")

	return config, nil
}

// sendProviderRequest 发送请求到提供商API
func (h *ChatHandler) sendProviderRequest(r *http.Request, requestBody map[string]interface{}, config models.ModelConfig, isStream bool) (*http.Response, error) {
	requestBody["model"] = config.ModelName
	updatedBody, err := json.Marshal(requestBody)
	if err != nil {
		log.WithError(err).Error("Failed to marshal request body for provider")
		return nil, err
	}

	reasoningVal, reasoningOk := requestBody["reasoning"].(map[string]interface{})
	if reasoningOk && reasoningVal["effort"] != nil {
		reasoningVal["effort"] = "none"
	}

	providerURL := config.APIBaseURL
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	providerURL += "chat/completions"

	log.WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(updatedBody),
	}).Info("Sending request to LLM provider")

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		log.WithError(err).WithField("url", providerURL).Error("Failed to create HTTP request for provider")
		return nil, err
	}

	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	startTime := time.Now()
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).WithField("url", providerURL).Error("Failed to send request to provider")
		return nil, err
	}

	elapsed := time.Since(startTime)
	log.WithFields(log.Fields{
		"url":           providerURL,
		"status_code":   resp.StatusCode,
		"response_time": elapsed.Milliseconds(),
	}).Info("Received response from LLM provider")

	return resp, nil
}

// handleStreamResponse 处理流式响应
func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	log.Info("Starting stream response handling")

	var fullResponse strings.Builder
	var mu sync.Mutex
	var contentOnly strings.Builder
	var reasoningContentOnly strings.Builder

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

	var firstID, firstObject, firstModel string
	var firstCreated int64
	var hasMetadata bool

	chunkCount := 0
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.WithError(err).Error("Error reading stream")
			}
			break
		}

		chunkCount++
		w.Write([]byte(line))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		mu.Lock()
		fullResponse.WriteString(line)
		mu.Unlock()

		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			jsonStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:"))
			if jsonStr != "" && jsonStr != "[DONE]" {
				if err := json.Unmarshal([]byte(jsonStr), &streamResponse); err == nil {
					if !hasMetadata && streamResponse.ID != "" {
						firstID = streamResponse.ID
						firstObject = streamResponse.Object
						firstCreated = streamResponse.Created
						firstModel = streamResponse.Model
						hasMetadata = true
					}
					if len(streamResponse.Choices) > 0 {
						if streamResponse.Choices[0].Delta.Content != "" {
							mu.Lock()
							contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
							mu.Unlock()
						}
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

	log.WithFields(log.Fields{
		"chunks":           chunkCount,
		"content_length":   contentOnly.Len(),
		"reasoning_length": reasoningContentOnly.Len(),
	}).Info("Stream response completed")

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
	log.Info("Starting non-stream response handling")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read response body from provider")
		return err
	}

	log.WithField("response_length", len(respBody)).Debug("Response body read from provider")

	respBodyDecode, err := gzipDecode(respBody)
	if err != nil {
		reqLog.Response = string(respBody)
		log.Debug("Response is not gzip encoded")
	} else {
		reqLog.Response = string(respBodyDecode)
		log.Debug("Response was gzip decoded")
	}

	w.Write(respBody)

	log.WithField("response_length", len(respBody)).Info("Non-stream response completed")
	return nil
}

// ChatCompletion 处理聊天完成请求，根据模型名称路由到对应的LLM提供商
func (h *ChatHandler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	log.WithField("remote_addr", r.RemoteAddr).Info("New chat completion request received")

	if h.handleCORS(w, r) {
		log.Debug("CORS preflight request handled")
		return
	}

	body, requestBody, modelName, userToken, isStream, err := h.parseRequest(r)
	if err != nil {
		log.WithError(err).Error("Failed to parse request")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.handleCache(w, body, modelName, userToken, isStream) {
		log.WithField("elapsed", time.Since(startTime).Milliseconds()).Info("Request served from cache")
		return
	}

	fingerprint := calculateFingerprint(string(body), modelName, userToken)
	reqLog := models.RequestLog{
		UserToken:   userToken,
		CreatedAt:   time.Now(),
		ModelName:   modelName,
		Request:     string(body),
		Fingerprint: fingerprint,
	}
	defer func() {
		if err := h.DB.Create(&reqLog).Error; err != nil {
			log.WithError(err).Error("Failed to save request log")
		} else {
			log.Debug("Request log saved successfully")
		}
	}()

	config, err := h.getModelConfig(modelName)
	if err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model config not found")
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	resp, err := h.sendProviderRequest(r, requestBody, config, isStream)
	if err != nil {
		log.WithError(err).Error("Failed to send request to provider")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if isStream {
		h.handleStreamResponse(w, resp, &reqLog)
	} else {
		if err := h.handleNonStreamResponse(w, resp, &reqLog); err != nil {
			log.WithError(err).Error("Failed to handle non-stream response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.WithFields(log.Fields{
		"model":      modelName,
		"is_stream":  isStream,
		"elapsed_ms": time.Since(startTime).Milliseconds(),
	}).Info("Chat completion request completed")
}

func gzipDecode(input []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (h *ChatHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	if h.handleCORS(w, r) {
		return
	}

	var configs []models.ModelConfig
	if err := h.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		http.Error(w, "Failed to get models", http.StatusInternalServerError)
		return
	}

	now := time.Now().Unix()
	modelsList := make([]Model, len(configs))
	for i, c := range configs {
		modelsList[i] = Model{
			ID:      c.Name,
			Object:  "model",
			Created: now,
			OwnedBy: "system",
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   modelsList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
