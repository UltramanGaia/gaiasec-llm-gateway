package handlers

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	log.Info("Starting stream response handling")

	var fullResponse strings.Builder
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

		fullResponse.WriteString(line)

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
							contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
						}
						if streamResponse.Choices[0].Delta.ReasoningContent != "" {
							reasoningContentOnly.WriteString(streamResponse.Choices[0].Delta.ReasoningContent)
						}
					}
				} else {
					log.WithError(err).Error("Error unmarshalling stream")
				}
			} else {
				log.Info("ending")
			}
		}
	}

	log.WithFields(log.Fields{
		"chunks":           chunkCount,
		"content_length":   contentOnly.Len(),
		"reasoning_length": reasoningContentOnly.Len(),
	}).Info("Stream response completed")

	if contentOnly.Len() == 0 && reasoningContentOnly.Len() == 0 {
		log.Warn("Stream response has no content, skipping log")
		return
	}

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

	streamData := fullResponse.String()
	compressedData, err := gzipEncode([]byte(streamData))
	if err != nil {
		log.WithError(err).Error("Failed to gzip compress stream response")
		reqLog.StreamResponse = []byte(streamData)
	} else {
		reqLog.StreamResponse = compressedData
		log.WithFields(log.Fields{
			"original_size":   len(streamData),
			"compressed_size": len(compressedData),
			"ratio":           float64(len(compressedData)) / float64(len(streamData)) * 100,
		}).Debug("Stream response compressed")
	}
}

func (h *ChatHandler) handleNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) error {
	log.Info("Starting non-stream response handling")

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read response body from provider")
		return err
	}

	log.WithField("response_length", len(respBody)).Debug("Response body read from provider")

	if len(respBody) == 0 {
		log.Warn("Non-stream response is empty, skipping log")
		return nil
	}

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
