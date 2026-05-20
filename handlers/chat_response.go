package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	h.handleOpenAIStreamResponse(w, resp, reqLog, config)
}

func (h *ChatHandler) handleOpenAIStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	chunkCount := 0
	metrics := newStreamMetricsTracker()
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.WithError(err).Error("Stream read failed")
			}
			break
		}

		chunkCount++
		if lineHasOpenAIStreamToken(line) {
			metrics.Record(time.Now())
		}
		rawStream.WriteString(line)
		w.Write([]byte(line))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
	responseJSON, contentLength, reasoningLength, err := buildOpenAIStreamLogResponse(rawStream.String())
	if err != nil {
		if err == io.EOF {
			log.WithField("chunks", chunkCount).Warn("Stream response empty, skipping log")
			return
		}
		log.WithError(err).WithField("chunks", chunkCount).Warn("Stream log payload build failed")
		return
	}

	reqLog.Response = responseJSON
	log.WithFields(log.Fields{
		"chunks":           chunkCount,
		"content_length":   contentLength,
		"reasoning_length": reasoningLength,
		"backend_model":    config.ModelName,
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
	var lastFinishReason string

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
			log.WithError(err).Debug("Skipping malformed stream chunk")
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
		if streamResponse.Choices[0].FinishReason != "" {
			lastFinishReason = streamResponse.Choices[0].FinishReason
		}
	}

	if err := scanner.Err(); err != nil {
		return "", 0, 0, err
	}

	if !sawContent {
		return "", 0, 0, io.EOF
	}

	finishReason := lastFinishReason
	if finishReason == "" {
		finishReason = "stop"
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
				FinishReason: finishReason,
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
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Provider response read failed")
		return err
	}

	log.WithField("response_length", len(respBody)).Debug("Provider response body loaded")

	if len(respBody) == 0 {
		log.Warn("Non-stream response empty, skipping log")
		return nil
	}

	respBodyDecode, err := gzipDecode(respBody)
	if err != nil {
		reqLog.Response = string(respBody)
		log.Debug("Provider response not gzip encoded")
	} else {
		reqLog.Response = string(respBodyDecode)
		log.Debug("Provider response gzip decoded")
	}

	w.Write(respBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}

	log.WithFields(log.Fields{
		"response_length": len(respBody),
		"backend_model":   config.ModelName,
	}).Info("Non-stream response completed")
	return nil
}

type streamMetricsTracker struct {
	startTime     time.Time
	firstTokenAt  time.Time
	lastTokenAt   time.Time
	intervalSum   time.Duration
	intervalCount int
}

func newStreamMetricsTracker() *streamMetricsTracker {
	return &streamMetricsTracker{
		startTime: time.Now(),
	}
}

func (t *streamMetricsTracker) Record(at time.Time) {
	if t == nil {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	if t.firstTokenAt.IsZero() {
		t.firstTokenAt = at
		t.lastTokenAt = at
		return
	}
	t.intervalSum += at.Sub(t.lastTokenAt)
	t.intervalCount++
	t.lastTokenAt = at
}

func (t *streamMetricsTracker) FirstTokenLatency() int64 {
	if t == nil || t.firstTokenAt.IsZero() {
		return 0
	}
	return t.firstTokenAt.Sub(t.startTime).Milliseconds()
}

func (t *streamMetricsTracker) AvgTokenLatency() float64 {
	if t == nil || t.intervalCount == 0 {
		return 0
	}
	return float64(t.intervalSum.Milliseconds()) / float64(t.intervalCount)
}

func lineHasOpenAIStreamToken(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return false
	}

	jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if jsonStr == "" || jsonStr == "[DONE]" {
		return false
	}

	var streamResponse struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &streamResponse); err != nil {
		return false
	}
	if len(streamResponse.Choices) == 0 {
		return false
	}

	return streamResponse.Choices[0].Delta.Content != "" || streamResponse.Choices[0].Delta.ReasoningContent != ""
}

func (h *ChatHandler) handleResponsesNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Responses provider response read failed")
		return err
	}

	if len(respBody) == 0 {
		log.Warn("Responses non-stream response empty")
		return nil
	}

	respBodyDecode, err := gzipDecode(respBody)
	if err != nil {
		reqLog.Response = string(respBody)
	} else {
		reqLog.Response = string(respBodyDecode)
	}

	var chatResp map[string]interface{}
	if err := json.Unmarshal(respBodyDecode, &chatResp); err == nil {
		responsesResp := convertChatResponseToResponses(chatResp)
		responsesBody, err := json.Marshal(responsesResp)
		if err == nil {
			w.Write(responsesBody)
			reqLog.Response = string(responsesBody)
			if reqLog.FirstTokenLatency == 0 {
				reqLog.FirstTokenLatency = reqLog.ResponseTime
			}
			log.WithFields(log.Fields{
				"response_length": len(responsesBody),
			}).Info("Responses non-stream response converted")
			return nil
		}
	}

	w.Write(respBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	log.WithField("response_length", len(respBody)).Info("Responses non-stream response passthrough")
	return nil
}

func (h *ChatHandler) handleResponsesStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	var rawStream bytes.Buffer
	chunkCount := 0
	metrics := newStreamMetricsTracker()

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Warn("Response writer does not support streaming")
		h.passthroughStreamResponse(w, resp, reqLog)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	reader := bufio.NewReader(resp.Body)
	var seqNum int64
	var responseID string
	var model string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.WithError(err).Error("Responses stream read failed")
			}
			break
		}

		rawStream.WriteString(line)

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if dataStr == "" || dataStr == "[DONE]" {
				if dataStr == "[DONE]" {
					h.writeResponsesStreamDone(w, responseID, model, seqNum)
				}
				continue
			}

			seqNum++
			var chatChunk map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &chatChunk); err != nil {
				w.Write([]byte(line))
				if flusher != nil {
					flusher.Flush()
				}
				continue
			}

			if id, ok := chatChunk["id"].(string); ok && id != "" {
				responseID = id
			}
			if m, ok := chatChunk["model"].(string); ok && m != "" {
				model = m
			}

			if choices, ok := chatChunk["choices"].([]interface{}); ok && len(choices) > 0 {
				chunkCount++
				metrics.Record(time.Now())
				responsesEvents := convertChatStreamChunkToResponsesEvents(chatChunk, seqNum)
				for _, event := range responsesEvents {
					eventLine := formatResponsesStreamEvent(event)
					w.Write([]byte(eventLine))
					if flusher != nil {
						flusher.Flush()
					}
				}
			} else if usage, ok := chatChunk["usage"].(map[string]interface{}); ok && usage != nil {
				h.writeResponsesStreamCompleted(w, responseID, model, usage, seqNum)
			}
		} else if trimmed != "" {
			w.Write([]byte(line))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}

	reqLog.StreamResponse = rawStream.Bytes()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()

	log.WithField("chunks", chunkCount).Info("Responses stream response completed")
}

func convertChatStreamChunkToResponsesEvents(chatChunk map[string]interface{}, seqNum int64) []ResponsesStreamEvent {
	events := make([]ResponsesStreamEvent, 0)

	if choices, ok := chatChunk["choices"].([]interface{}); ok {
		for _, ch := range choices {
			choice, ok := ch.(map[string]interface{})
			if !ok {
				continue
			}
			idx := 0
			if i, ok := choice["index"].(float64); ok {
				idx = int(i)
			}

			delta, ok := choice["delta"].(map[string]interface{})
			if !ok {
				continue
			}

			if role, ok := delta["role"].(string); ok && role == "assistant" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.added",
					Data: map[string]interface{}{
						"type":            "response.output_item.added",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"type":   "message",
							"status": "in_progress",
							"role":   "assistant",
						},
					},
				})
			}

			if content, ok := delta["content"].(string); ok && content != "" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_text.delta",
					Data: map[string]interface{}{
						"type":            "response.output_text.delta",
						"sequence_number": seqNum,
						"output_index":    idx,
						"content_index":   0,
						"delta":           content,
					},
				})
			}

			if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
				for _, tc := range toolCalls {
					toolCall, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					tcID, _ := toolCall["id"].(string)
					fn, _ := toolCall["function"].(map[string]interface{})
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)

					if tcID != "" && name != "" {
						events = append(events, ResponsesStreamEvent{
							Event: "response.output_item.added",
							Data: map[string]interface{}{
								"type":            "response.output_item.added",
								"sequence_number": seqNum,
								"output_index":    idx + 100,
								"item": map[string]interface{}{
									"type":    "function_call",
									"id":      tcID,
									"call_id": tcID,
									"name":    name,
									"status":  "in_progress",
								},
							},
						})
					}

					if args != "" {
						events = append(events, ResponsesStreamEvent{
							Event: "response.function_call_arguments.delta",
							Data: map[string]interface{}{
								"type":            "response.function_call_arguments.delta",
								"sequence_number": seqNum,
								"output_index":    idx + 100,
								"delta":           args,
							},
						})
					}
				}
			}

			if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
				events = append(events, ResponsesStreamEvent{
					Event: "response.output_item.done",
					Data: map[string]interface{}{
						"type":            "response.output_item.done",
						"sequence_number": seqNum,
						"output_index":    idx,
						"item": map[string]interface{}{
							"type":   "message",
							"status": "completed",
							"role":   "assistant",
						},
					},
				})
			}
		}
	}

	return events
}

func (h *ChatHandler) writeResponsesStreamCompleted(w http.ResponseWriter, id, model string, usage map[string]interface{}, seqNum int64) {
	seqNum++
	inputTokens := 0
	outputTokens := 0
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		inputTokens = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		outputTokens = int(ct)
	}

	event := ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response": map[string]interface{}{
				"id":     id,
				"object": "response",
				"status": "completed",
				"model":  model,
				"usage": map[string]interface{}{
					"input_tokens":  inputTokens,
					"output_tokens": outputTokens,
					"total_tokens":  inputTokens + outputTokens,
				},
			},
		},
	}
	w.Write([]byte(formatResponsesStreamEvent(event)))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (h *ChatHandler) writeResponsesStreamDone(w http.ResponseWriter, id, model string, seqNum int64) {
	seqNum++
	event := ResponsesStreamEvent{
		Event: "response.completed",
		Data: map[string]interface{}{
			"type":            "response.completed",
			"sequence_number": seqNum,
			"response": map[string]interface{}{
				"id":     id,
				"object": "response",
				"status": "completed",
				"model":  model,
			},
		},
	}
	w.Write([]byte(formatResponsesStreamEvent(event)))
	w.Write([]byte("data: [DONE]\n\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func formatResponsesStreamEvent(event ResponsesStreamEvent) string {
	data, _ := json.Marshal(event.Data)
	return "event: " + event.Event + "\ndata: " + string(data) + "\n\n"
}

type ResponsesStreamEvent struct {
	Event string
	Data  interface{}
}
