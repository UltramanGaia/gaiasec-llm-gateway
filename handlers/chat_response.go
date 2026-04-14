package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	log.Info("Starting stream response handling")
	h.handleOpenAIStreamResponse(w, resp, reqLog, config)
}

func (h *ChatHandler) handleOpenAIStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	log.Info("Starting OpenAI stream response handling")

	var rawStream bytes.Buffer
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
		rawStream.WriteString(line)
		w.Write([]byte(line))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	responseJSON, contentLength, reasoningLength, err := buildOpenAIStreamLogResponse(rawStream.String())
	if err != nil {
		if err == io.EOF {
			log.WithField("chunks", chunkCount).Warn("Stream response has no content, skipping log")
			return
		}
		log.WithError(err).WithField("chunks", chunkCount).Warn("Failed to build structured stream log response")
		return
	}

	reqLog.Response = responseJSON
	log.WithFields(log.Fields{
		"chunks":           chunkCount,
		"content_length":   contentLength,
		"reasoning_length": reasoningLength,
	}).Info("Stream response completed")
}

func buildOpenAIStreamLogResponse(rawStream string) (string, int, int, error) {
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
	}

	var firstID, firstObject, firstModel string
	var firstCreated int64
	var hasMetadata bool
	var sawContent bool

	scanner := bufio.NewScanner(strings.NewReader(rawStream))
	// Raise the scanner ceiling to tolerate long SSE data lines.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if jsonStr == "" || jsonStr == "[DONE]" {
			continue
		}

		if err := json.Unmarshal([]byte(jsonStr), &streamResponse); err != nil {
			log.WithError(err).Debug("Skipping malformed stream chunk while building log response")
			continue
		}

		if !hasMetadata && streamResponse.ID != "" {
			firstID = streamResponse.ID
			firstObject = streamResponse.Object
			firstCreated = streamResponse.Created
			firstModel = streamResponse.Model
			hasMetadata = true
		}

		if len(streamResponse.Choices) == 0 {
			continue
		}
		if streamResponse.Choices[0].Delta.Content != "" {
			contentOnly.WriteString(streamResponse.Choices[0].Delta.Content)
			sawContent = true
		}
		if streamResponse.Choices[0].Delta.ReasoningContent != "" {
			reasoningContentOnly.WriteString(streamResponse.Choices[0].Delta.ReasoningContent)
			sawContent = true
		}
	}

	if err := scanner.Err(); err != nil {
		return "", 0, 0, err
	}

	if !sawContent {
		return "", 0, 0, io.EOF
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

	respData, err := json.Marshal(cachedResp)
	if err != nil {
		return "", 0, 0, err
	}

	return string(respData), contentOnly.Len(), reasoningContentOnly.Len(), nil
}

func (h *ChatHandler) handleNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
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
