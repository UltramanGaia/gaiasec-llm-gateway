package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) handleCache(w http.ResponseWriter, body []byte, modelName string, isStream bool) (bool, string) {
	fingerprint := calculateFingerprint(string(body), modelName)
	log.WithField("fingerprint", fingerprint).Debug("Checking cache for request")

	cache := GetCache()
	cacheKey := "fingerprint:" + fingerprint

	if cachedData, found := cache.Get(cacheKey); found {
		existingLog := cachedData.(*models.RequestLog)
		log.WithFields(log.Fields{
			"fingerprint": fingerprint,
			"is_stream":   isStream,
			"model":       modelName,
		}).Info("Cache hit in memory, returning cached response")

		if isStream {
			h.writeStreamCacheResponse(w, existingLog)
		} else {
			h.writeNonStreamCacheResponse(w, existingLog)
		}
		return true, fingerprint
	}

	var existingLog models.RequestLog
	if err := h.DB.Where("fingerprint = ?", fingerprint).First(&existingLog).Error; err != nil {
		log.WithField("fingerprint", fingerprint).Info("Cache miss, will forward request to provider")
		return false, fingerprint
	}

	cache.Set(cacheKey, &existingLog, 30*time.Minute)

	log.WithFields(log.Fields{
		"fingerprint": fingerprint,
		"is_stream":   isStream,
		"model":       modelName,
	}).Info("Cache hit in database, returning cached response")

	if isStream {
		h.writeStreamCacheResponse(w, &existingLog)
	} else {
		h.writeNonStreamCacheResponse(w, &existingLog)
	}

	return true, fingerprint
}

func (h *ChatHandler) writeStreamCacheResponse(w http.ResponseWriter, existingLog *models.RequestLog) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if len(existingLog.StreamResponse) > 0 {
		decompressedData, err := gzipDecode(existingLog.StreamResponse)
		if err != nil {
			log.WithError(err).Error("Failed to gzip decompress stream response")
			w.Write(existingLog.StreamResponse)
		} else {
			w.Write(decompressedData)
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
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
		h.writeCachedResponseAsSSE(w, &cachedResponse)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(existingLog.Response))
	}
}

func (h *ChatHandler) writeCachedResponseAsSSE(w http.ResponseWriter, cachedResponse interface{}) {
	type cachedRespType struct {
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
	}

	cr := cachedResponse.(*cachedRespType)

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

	deltaResponse.ID = cr.ID
	deltaResponse.Object = cr.Object
	deltaResponse.Created = cr.Created
	deltaResponse.Model = cr.Model
	deltaResponse.Choices = make([]struct {
		Index int `json:"index"`
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	}, len(cr.Choices))

	for i, choice := range cr.Choices {
		deltaResponse.Choices[i].Index = choice.Index
		deltaResponse.Choices[i].Delta.Role = "assistant"
		deltaResponse.Choices[i].Delta.Content = ""

		jsonData, _ := json.Marshal(deltaResponse)
		w.Write([]byte("data: " + string(jsonData) + "\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		content := choice.Message.Content
		if content == "" {
			content = choice.Delta.Content
		}
		reasoningContent := choice.Message.ReasoningContent
		if reasoningContent == "" {
			reasoningContent = choice.Delta.ReasoningContent
		}

		const chunkSize = 8
		for j := 0; j < len(reasoningContent); j += chunkSize {
			end := j + chunkSize
			if end > len(reasoningContent) {
				end = len(reasoningContent)
			}
			deltaResponse.Choices[i].Delta.Content = ""
			deltaResponse.Choices[i].Delta.ReasoningContent = reasoningContent[j:end]
			jsonData, _ = json.Marshal(deltaResponse)
			w.Write([]byte("data: " + string(jsonData) + "\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}

		for j := 0; j < len(content); j += chunkSize {
			end := j + chunkSize
			if end > len(content) {
				end = len(content)
			}
			deltaResponse.Choices[i].Delta.Content = content[j:end]
			deltaResponse.Choices[i].Delta.ReasoningContent = ""
			jsonData, _ = json.Marshal(deltaResponse)
			w.Write([]byte("data: " + string(jsonData) + "\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}

		deltaResponse.Choices[i].Delta.Content = ""
		deltaResponse.Choices[i].Delta.ReasoningContent = ""
		deltaResponse.Choices[i].FinishReason = "stop"
		jsonData, _ = json.Marshal(deltaResponse)
		w.Write([]byte("data: " + string(jsonData) + "\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.Write([]byte("data: [DONE]\n\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (h *ChatHandler) writeNonStreamCacheResponse(w http.ResponseWriter, existingLog *models.RequestLog) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(existingLog.Response))
}
