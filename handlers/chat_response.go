package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"
	"llm-gateway/protocol"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) handleStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	h.handleOpenAIStreamResponse(w, resp, reqLog, config)
}

func (h *ChatHandler) handleOpenAIStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	chunkCount := 0
	metrics := newStreamMetricsTracker()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				entry := loggerWithTrace(ctx).WithError(err).WithFields(log.Fields{
					"backend_model":       config.ModelName,
					"backend_config_id":   reqLog.BackendConfigID,
					"stream_chunks":       chunkCount,
					"first_token_latency": metrics.FirstTokenLatency(),
					"avg_token_latency":   metrics.AvgTokenLatency(),
					"termination_reason":  streamTerminationReason(err),
				})
				switch {
				case isExpectedStreamTermination(err):
					entry.Warn("OpenAI stream terminated before EOF")
				default:
					entry.Error("Stream read failed")
				}
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
	streamLog, err := buildOpenAIStreamLogResponse(rawStream.String())
	if err != nil {
		fields := log.Fields{
			"chunks":            chunkCount,
			"done_seen":         streamLog.DoneSeen,
			"data_events":       streamLog.DataEvents,
			"content_chunks":    streamLog.ContentChunks,
			"tool_call_chunks":  streamLog.ToolCallChunks,
			"reasoning_chunks":  streamLog.ReasoningChunks,
			"usage_chunks":      streamLog.UsageChunks,
			"first_token_ms":    reqLog.FirstTokenLatency,
			"avg_token_latency": reqLog.AvgTokenLatency,
		}
		if err == io.EOF {
			loggerWithTrace(ctx).WithFields(fields).Warn("Stream response had no loggable assistant payload, skipping log")
			return
		}
		loggerWithTrace(ctx).WithError(err).WithFields(fields).Warn("Stream log payload build failed")
		return
	}

	reqLog.Response = streamLog.ResponseJSON
	loggerWithTrace(ctx).WithFields(log.Fields{
		"chunks":           chunkCount,
		"content_length":   streamLog.ContentLength,
		"reasoning_length": streamLog.ReasoningLength,
		"tool_calls":       len(streamLog.ToolCalls),
		"backend_model":    config.ModelName,
	}).Info("Stream response completed")
}

type openAIStreamLogResult struct {
	ResponseJSON    string
	ContentLength   int
	ReasoningLength int
	RefusalLength   int
	AudioChunks     int
	AnnotationCount int
	DataEvents      int
	ContentChunks   int
	ReasoningChunks int
	RefusalChunks   int
	ToolCallChunks  int
	UsageChunks     int
	DoneSeen        bool
	ToolCalls       []openAIStreamToolCall
	Refusal         string
	Audio           map[string]interface{}
	Annotations     []interface{}
}

type openAIStreamToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

func buildOpenAIStreamLogResponse(rawStream string) (openAIStreamLogResult, error) {
	result := openAIStreamLogResult{}
	var contentOnly strings.Builder
	var reasoningContentOnly strings.Builder
	var refusalOnly strings.Builder
	var audioPayload map[string]interface{}
	var annotationsPayload []interface{}

	var firstID, firstObject, firstModel string
	var firstCreated int64
	var hasMetadata bool
	var sawLoggablePayload bool
	var lastFinishReason string
	var lastUsage struct {
		PromptTokens          int
		CompletionTokens      int
		TotalTokens           int
		CachedTokens          int
		PromptCacheHitTokens  int
		PromptCacheMissTokens int
	}
	toolCallsByIndex := make(map[int]*openAIStreamToolCall)

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
		if jsonStr == "" {
			continue
		}
		if jsonStr == "[DONE]" {
			result.DoneSeen = true
			continue
		}
		result.DataEvents++
		var chatChunk map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &chatChunk); err != nil {
			log.WithError(err).Debug("Skipping malformed stream chunk")
			continue
		}
		irEvents := protocol.IRStreamEventsFromChatChunk(chatChunk)

		if !hasMetadata && stringValue(chatChunk["id"]) != "" {
			firstID = stringValue(chatChunk["id"])
			firstObject = stringValue(chatChunk["object"])
			firstCreated = numberValueToInt64(chatChunk["created"])
			firstModel = stringValue(chatChunk["model"])
			hasMetadata = true
		}

		for _, event := range irEvents {
			switch event.Type {
			case "output_text.delta":
				contentOnly.WriteString(firstNonEmptyString(event.Text, event.Delta))
				result.ContentChunks++
				sawLoggablePayload = true
			case "reasoning.delta":
				reasoningContentOnly.WriteString(firstNonEmptyString(event.Text, event.Delta))
				result.ReasoningChunks++
				sawLoggablePayload = true
			case "refusal.delta":
				refusalOnly.WriteString(firstNonEmptyString(event.Refusal, event.Delta))
				result.RefusalChunks++
				sawLoggablePayload = true
			case "audio.delta":
				if len(event.Audio) > 0 {
					_ = json.Unmarshal(event.Audio, &audioPayload)
					result.AudioChunks++
					sawLoggablePayload = true
				}
			case "annotation.added":
				if len(event.Annotations) > 0 {
					_ = json.Unmarshal(event.Annotations, &annotationsPayload)
					result.AnnotationCount = len(annotationsPayload)
					sawLoggablePayload = true
				}
			case "tool_call.delta":
				result.ToolCallChunks++
				sawLoggablePayload = true
				toolCall := toolCallsByIndex[event.Index]
				if toolCall == nil {
					toolCall = &openAIStreamToolCall{Index: event.Index}
					toolCallsByIndex[event.Index] = toolCall
				}
				if event.ItemID != "" {
					toolCall.ID = event.ItemID
				}
				if toolType, ok := event.ProviderExtensions["type"].(string); ok && toolType != "" {
					toolCall.Type = toolType
				}
				if name, ok := event.ProviderExtensions["name"].(string); ok && name != "" {
					toolCall.Function.Name = name
				}
				if event.Arguments != "" {
					toolCall.Function.Arguments += event.Arguments
				}
			case "response.completed":
				if event.FinishReason != "" {
					lastFinishReason = event.FinishReason
				}
			case "usage":
				if len(event.Usage) > 0 {
					var usage map[string]interface{}
					if err := json.Unmarshal(event.Usage, &usage); err == nil {
						result.UsageChunks++
						lastUsage.PromptTokens = numberValueToInt(usage["prompt_tokens"])
						lastUsage.CompletionTokens = numberValueToInt(usage["completion_tokens"])
						lastUsage.TotalTokens = numberValueToInt(usage["total_tokens"])
						if details, ok := usage["prompt_tokens_detail"].(map[string]interface{}); ok {
							lastUsage.CachedTokens = numberValueToInt(details["cached_tokens"])
						}
						lastUsage.PromptCacheHitTokens = numberValueToInt(usage["prompt_cache_hit_tokens"])
						lastUsage.PromptCacheMissTokens = numberValueToInt(usage["prompt_cache_miss_tokens"])
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return result, err
	}

	if len(toolCallsByIndex) > 0 {
		toolCalls := make([]openAIStreamToolCall, 0, len(toolCallsByIndex))
		for idx := 0; idx < len(toolCallsByIndex)+8; idx++ {
			toolCall := toolCallsByIndex[idx]
			if toolCall != nil {
				toolCalls = append(toolCalls, *toolCall)
				delete(toolCallsByIndex, idx)
			}
		}
		if len(toolCallsByIndex) > 0 {
			for _, toolCall := range toolCallsByIndex {
				toolCalls = append(toolCalls, *toolCall)
			}
		}
		result.ToolCalls = toolCalls
	}

	if !sawLoggablePayload {
		return result, io.EOF
	}

	finishReason := lastFinishReason
	if finishReason == "" {
		if len(result.ToolCalls) > 0 {
			finishReason = "tool_calls"
		} else {
			finishReason = "stop"
		}
	}

	cachedResp := struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role             string                 `json:"role"`
				Content          string                 `json:"content"`
				ReasoningContent string                 `json:"reasoning_content,omitempty"`
				Refusal          string                 `json:"refusal,omitempty"`
				Audio            map[string]interface{} `json:"audio,omitempty"`
				Annotations      []interface{}          `json:"annotations,omitempty"`
				ToolCalls        []openAIStreamToolCall `json:"tool_calls,omitempty"`
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
				Role             string                 `json:"role"`
				Content          string                 `json:"content"`
				ReasoningContent string                 `json:"reasoning_content,omitempty"`
				Refusal          string                 `json:"refusal,omitempty"`
				Audio            map[string]interface{} `json:"audio,omitempty"`
				Annotations      []interface{}          `json:"annotations,omitempty"`
				ToolCalls        []openAIStreamToolCall `json:"tool_calls,omitempty"`
			} `json:"message"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		}{
			{
				Index: 0,
				Message: struct {
					Role             string                 `json:"role"`
					Content          string                 `json:"content"`
					ReasoningContent string                 `json:"reasoning_content,omitempty"`
					Refusal          string                 `json:"refusal,omitempty"`
					Audio            map[string]interface{} `json:"audio,omitempty"`
					Annotations      []interface{}          `json:"annotations,omitempty"`
					ToolCalls        []openAIStreamToolCall `json:"tool_calls,omitempty"`
				}{
					Role:             "assistant",
					Content:          contentOnly.String(),
					ReasoningContent: reasoningContentOnly.String(),
					Refusal:          refusalOnly.String(),
					Audio:            audioPayload,
					Annotations:      annotationsPayload,
					ToolCalls:        result.ToolCalls,
				},
				FinishReason: finishReason,
				Logprobs:     nil,
			},
		},
	}

	cachedResp.Usage.PromptTokens = lastUsage.PromptTokens
	cachedResp.Usage.CompletionTokens = lastUsage.CompletionTokens
	cachedResp.Usage.TotalTokens = lastUsage.TotalTokens
	cachedResp.Usage.PromptTokensDetail.CachedTokens = lastUsage.CachedTokens
	cachedResp.Usage.PromptCacheHitTokens = lastUsage.PromptCacheHitTokens
	cachedResp.Usage.PromptCacheMissTokens = lastUsage.PromptCacheMissTokens

	respData, err := json.Marshal(cachedResp)
	if err != nil {
		return result, err
	}

	result.ResponseJSON = string(respData)
	result.ContentLength = contentOnly.Len()
	result.ReasoningLength = reasoningContentOnly.Len()
	result.RefusalLength = refusalOnly.Len()
	result.Refusal = refusalOnly.String()
	result.Audio = audioPayload
	result.Annotations = annotationsPayload
	return result, nil
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

	respBodyDecode, decodeErr := gzipDecode(respBody)
	if decodeErr != nil {
		respBodyDecode = respBody
		reqLog.Response = string(respBody)
		log.Debug("Provider response not gzip encoded")
	} else {
		reqLog.Response = string(respBodyDecode)
		log.Debug("Provider response gzip decoded")
	}

	if ir, err := protocol.DecodeOpenAIChatResponse(respBodyDecode); err == nil {
		if normalizedBody, err := protocol.EncodeOpenAIChatResponse(ir, ir.Model); err == nil {
			_, _ = w.Write(normalizedBody)
			reqLog.Response = string(normalizedBody)
			if reqLog.FirstTokenLatency == 0 {
				reqLog.FirstTokenLatency = reqLog.ResponseTime
			}

			log.WithFields(log.Fields{
				"response_length": len(normalizedBody),
				"backend_model":   config.ModelName,
			}).Info("Non-stream response normalized")
			return nil
		}
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

	var chatChunk map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &chatChunk); err != nil {
		return false
	}
	events := protocol.IRStreamEventsFromChatChunk(chatChunk)
	if len(events) == 0 {
		return false
	}
	for _, event := range events {
		switch event.Type {
		case "output_text.delta", "reasoning.delta", "refusal.delta", "audio.delta", "annotation.added", "tool_call.delta":
			return true
		}
	}
	return false
}

func numberValueToInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func numberValueToInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
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
		respBodyDecode = respBody
		reqLog.Response = string(respBody)
	} else {
		reqLog.Response = string(respBodyDecode)
	}

	if ir, err := protocol.DecodeOpenAIChatResponse(respBodyDecode); err == nil {
		if responsesBody, err := protocol.EncodeResponsesResponse(ir, ir.Model); err == nil {
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

func (h *ChatHandler) handleChatFromResponsesNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Responses provider response read failed")
		return err
	}
	if len(respBody) == 0 {
		return nil
	}

	decodedBody, err := gzipDecode(respBody)
	if err != nil {
		decodedBody = respBody
	}
	chatBody, err := convertResponsesResponseToChatResponse(decodedBody, config.ModelName)
	if err != nil {
		w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	_, _ = w.Write(chatBody)
	reqLog.Response = string(chatBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	return nil
}

func (h *ChatHandler) handleChatFromAnthropicNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Anthropic provider response read failed")
		return err
	}
	if len(respBody) == 0 {
		return nil
	}

	decodedBody, err := gzipDecode(respBody)
	if err != nil {
		decodedBody = respBody
	}
	chatBody, err := convertAnthropicResponseToChatResponse(decodedBody, config.ModelName)
	if err != nil {
		_, _ = w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	_, _ = w.Write(chatBody)
	reqLog.Response = string(chatBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	return nil
}

func (h *ChatHandler) handleAnthropicFromChatNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Chat provider response read failed")
		return err
	}
	if len(respBody) == 0 {
		return nil
	}

	decodedBody, err := gzipDecode(respBody)
	if err != nil {
		decodedBody = respBody
	}

	var chatResp map[string]interface{}
	if err := json.Unmarshal(decodedBody, &chatResp); err != nil {
		_, _ = w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	anthropicBody, err := convertChatResponseToAnthropicResponse(chatResp, config.ModelName)
	if err != nil {
		_, _ = w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	_, _ = w.Write(anthropicBody)
	reqLog.Response = string(anthropicBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	return nil
}

func (h *ChatHandler) handleResponsesFromAnthropicNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Anthropic provider response read failed")
		return err
	}
	if len(respBody) == 0 {
		return nil
	}

	decodedBody, err := gzipDecode(respBody)
	if err != nil {
		decodedBody = respBody
	}
	responsesBody, err := convertAnthropicResponseToResponsesResponse(decodedBody, config.ModelName)
	if err != nil {
		_, _ = w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	_, _ = w.Write(responsesBody)
	reqLog.Response = string(responsesBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	return nil
}

func (h *ChatHandler) handleAnthropicFromResponsesNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Responses provider response read failed")
		return err
	}
	if len(respBody) == 0 {
		return nil
	}

	decodedBody, err := gzipDecode(respBody)
	if err != nil {
		decodedBody = respBody
	}
	anthropicBody, err := convertResponsesResponseToAnthropicResponse(decodedBody, config.ModelName)
	if err != nil {
		_, _ = w.Write(respBody)
		reqLog.Response = string(decodedBody)
		return nil
	}

	_, _ = w.Write(anthropicBody)
	reqLog.Response = string(anthropicBody)
	if reqLog.FirstTokenLatency == 0 {
		reqLog.FirstTokenLatency = reqLog.ResponseTime
	}
	return nil
}

func (h *ChatHandler) handleChatFromAnthropicStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	var outStream bytes.Buffer
	metrics := newStreamMetricsTracker()
	state := protocol.NewAnthropicInboundState()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			if err != io.EOF {
				loggerWithTrace(ctx).WithError(err).Warn("Anthropic stream read failed")
			}
			break
		}
		rawStream.WriteString(protocol.FormatSSEFrame(frame))
		chunks, done := protocol.ChatChunksFromAnthropicFrame(frame, state)
		for _, chunk := range chunks {
			if lineHasOpenAIStreamToken("data: " + mustJSON(chunk)) {
				metrics.Record(time.Now())
			}
			line := "data: " + mustJSON(chunk) + "\n\n"
			outStream.WriteString(line)
			_, _ = w.Write([]byte(line))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if done {
			outStream.WriteString("data: [DONE]\n\n")
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			break
		}
	}
	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.Response = outStream.String()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func (h *ChatHandler) handleResponsesFromAnthropicStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	var outStream bytes.Buffer
	metrics := newStreamMetricsTracker()
	state := protocol.NewAnthropicInboundState()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)
	var seqNum int64
	var sawCompleted bool

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			if err != io.EOF {
				loggerWithTrace(ctx).WithError(err).Warn("Anthropic stream read failed")
			}
			break
		}
		rawStream.WriteString(protocol.FormatSSEFrame(frame))
		events, done, responseID, model := protocol.ResponsesEventsFromAnthropicFrame(frame, state, &seqNum)
		for _, event := range events {
			if event.Event == "response.completed" {
				sawCompleted = true
			}
			if responsesEventHasOutputToken(event) {
				metrics.Record(time.Now())
			}
			line := protocol.FormatResponsesStreamEvent(event)
			outStream.WriteString(line)
			_, _ = w.Write([]byte(line))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if done {
			if !sawCompleted {
				h.writeResponsesStreamDone(w, responseID, model, "", seqNum)
				outStream.WriteString(protocol.FormatResponsesStreamEvent(protocol.BuildResponsesDoneEvent(responseID, model, "", seqNum+1)))
				outStream.WriteString(protocol.FormatResponsesStreamEvent(protocol.BuildResponsesTerminalDoneEvent(seqNum + 2)))
				outStream.WriteString("data: [DONE]\n\n")
			} else {
				terminal := protocol.BuildResponsesTerminalDoneEvent(seqNum + 1)
				w.Write([]byte(protocol.FormatResponsesStreamEvent(terminal)))
				w.Write([]byte("data: [DONE]\n\n"))
				outStream.WriteString(protocol.FormatResponsesStreamEvent(terminal))
				outStream.WriteString("data: [DONE]\n\n")
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
			break
		}
	}
	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.Response = outStream.String()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func responsesEventHasOutputToken(event protocol.ResponsesStreamEvent) bool {
	switch event.Event {
	case "response.output_text.delta",
		"response.reasoning.delta",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.annotation.added",
		"response.refusal.done",
		"response.audio.done":
		return true
	case "response.output_item.added", "response.output_item.done":
		data, _ := event.Data.(map[string]interface{})
		item, _ := data["item"].(map[string]interface{})
		if len(item) == 0 {
			return false
		}
		switch item["type"] {
		case "function_call", "custom_tool_call", "mcp_call", "web_search_call", "file_search_call", "image_generation_call", "computer_call", "code_interpreter_call", "local_shell_call", "shell_call", "apply_patch_call":
			return strings.TrimSpace(stringValue(item["arguments"])) != "" || strings.TrimSpace(stringValue(item["input"])) != ""
		case "reasoning":
			return strings.TrimSpace(responseItemSummaryText(item)) != ""
		default:
			return false
		}
	default:
		return false
	}
}

func responseItemSummaryText(item map[string]interface{}) string {
	summary, _ := item["summary"].([]interface{})
	for _, raw := range summary {
		part, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if text := stringValue(part["text"]); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func (h *ChatHandler) handleChatFromResponsesStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	var outStream bytes.Buffer
	metrics := newStreamMetricsTracker()
	state := protocol.NewResponsesStreamState()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			if err != io.EOF {
				loggerWithTrace(ctx).WithError(err).Warn("Responses stream read failed")
			}
			break
		}
		if frame.Event != "" {
			rawStream.WriteString("event: " + frame.Event + "\n")
		}
		rawStream.WriteString("data: " + frame.Data + "\n\n")
		chunks, done := protocol.ChatChunksFromResponsesFrame(frame, state)
		for _, chunk := range chunks {
			if lineHasOpenAIStreamToken("data: " + mustJSON(chunk)) {
				metrics.Record(time.Now())
			}
			line := "data: " + mustJSON(chunk) + "\n\n"
			outStream.WriteString(line)
			_, _ = w.Write([]byte(line))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if done {
			outStream.WriteString("data: [DONE]\n\n")
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			break
		}
	}
	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.Response = outStream.String()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func (h *ChatHandler) handleAnthropicFromChatStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	var outStream bytes.Buffer
	metrics := newStreamMetricsTracker()
	state := protocol.NewAnthropicOutboundState()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				loggerWithTrace(ctx).WithError(err).Warn("Chat stream read failed")
			}
			break
		}
		rawStream.WriteString(line)
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "" {
			continue
		}
		if dataStr == "[DONE]" {
			for _, frame := range protocol.FlushAnthropicFrames(state, true) {
				text := protocol.FormatSSEFrame(frame)
				outStream.WriteString(text)
				_, _ = w.Write([]byte(text))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
			break
		}
		var chatChunk map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &chatChunk); err != nil {
			continue
		}
		for _, frame := range protocol.AnthropicFramesFromChatChunk(chatChunk, state) {
			if anthropicFrameHasOutputToken(frame) {
				metrics.Record(time.Now())
			}
			text := protocol.FormatSSEFrame(frame)
			outStream.WriteString(text)
			_, _ = w.Write([]byte(text))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.Response = outStream.String()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func (h *ChatHandler) handleAnthropicFromResponsesStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig) {
	var rawStream bytes.Buffer
	var outStream bytes.Buffer
	metrics := newStreamMetricsTracker()
	state := protocol.NewAnthropicOutboundState()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	reader := bufio.NewReader(resp.Body)

	w.Header().Set("Content-Type", "text/event-stream")
	for {
		frame, err := protocol.ReadSSEFrame(reader)
		if err != nil {
			if err != io.EOF {
				loggerWithTrace(ctx).WithError(err).Warn("Responses stream read failed")
			}
			break
		}
		rawStream.WriteString(protocol.FormatSSEFrame(protocol.SSEFrame{Event: frame.Event, Data: frame.Data}))
		for _, anthropicFrame := range protocol.AnthropicEventsFromResponsesFrame(frame, state) {
			if anthropicFrameHasOutputToken(anthropicFrame) {
				metrics.Record(time.Now())
			}
			text := protocol.FormatSSEFrame(anthropicFrame)
			outStream.WriteString(text)
			_, _ = w.Write([]byte(text))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
	reqLog.StreamResponse = append(reqLog.StreamResponse[:0], rawStream.Bytes()...)
	reqLog.Response = outStream.String()
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func anthropicFrameHasOutputToken(frame protocol.SSEFrame) bool {
	for _, event := range protocol.IRStreamEventsFromAnthropicFrame(frame) {
		switch event.Type {
		case "output_text.delta", "reasoning.delta", "refusal.delta", "audio.delta", "annotation.added", "tool_call.start", "tool_call.delta":
			return true
		case "reasoning.start":
			if strings.TrimSpace(event.Text) != "" {
				return true
			}
		}
	}
	if frame.Event == "content_block_delta" {
		return true
	}
	return false
}

func (h *ChatHandler) handleResponsesStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	var rawStream bytes.Buffer
	chunkCount := 0
	metrics := newStreamMetricsTracker()
	ctx := context.Background()
	if resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		loggerWithTrace(ctx).Warn("Response writer does not support streaming")
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
	state := protocol.NewChatToResponsesStreamState()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				entry := loggerWithTrace(ctx).WithError(err).WithFields(log.Fields{
					"backend_model":       reqLog.BackendModelName,
					"backend_config_id":   reqLog.BackendConfigID,
					"stream_chunks":       chunkCount,
					"first_token_latency": metrics.FirstTokenLatency(),
					"avg_token_latency":   metrics.AvgTokenLatency(),
					"termination_reason":  streamTerminationReason(err),
				})
				switch {
				case isExpectedStreamTermination(err):
					entry.Warn("Responses stream terminated before EOF")
				default:
					entry.Error("Responses stream read failed")
				}
			}
			break
		}

		rawStream.WriteString(line)

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if dataStr == "" || dataStr == "[DONE]" {
				if dataStr == "[DONE]" {
					h.writeResponsesStreamDone(w, responseID, model, state.MessageText[0], seqNum)
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
				responsesEvents := protocol.ConvertChatChunkToResponsesEventsStateful(chatChunk, seqNum, state)
				if responsesEventsHasOutputToken(responsesEvents) {
					metrics.Record(time.Now())
				}
				for _, event := range responsesEvents {
					eventLine := protocol.FormatResponsesStreamEvent(event)
					w.Write([]byte(eventLine))
					if flusher != nil {
						flusher.Flush()
					}
				}
			} else if usage, ok := chatChunk["usage"].(map[string]interface{}); ok && usage != nil {
				h.writeResponsesStreamCompleted(w, responseID, model, usage, state.MessageText[0], seqNum)
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

	loggerWithTrace(ctx).WithFields(log.Fields{
		"backend_model":       reqLog.BackendModelName,
		"backend_config_id":   reqLog.BackendConfigID,
		"stream_chunks":       chunkCount,
		"first_token_latency": metrics.FirstTokenLatency(),
		"avg_token_latency":   metrics.AvgTokenLatency(),
	}).Info("Responses stream response completed")
}

func responsesEventsHasOutputToken(events []protocol.ResponsesStreamEvent) bool {
	for _, event := range events {
		if responsesEventHasOutputToken(event) {
			return true
		}
	}
	return false
}

func convertChatStreamChunkToResponsesEvents(chatChunk map[string]interface{}, seqNum int64) []protocol.ResponsesStreamEvent {
	return protocol.ConvertChatChunkToResponsesEvents(chatChunk, seqNum)
}

func (h *ChatHandler) writeResponsesStreamCompleted(w http.ResponseWriter, id, model string, usage map[string]interface{}, outputText string, seqNum int64) {
	seqNum++
	inputTokens := 0
	outputTokens := 0
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		inputTokens = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		outputTokens = int(ct)
	}

	event := protocol.BuildResponsesCompletedEvent(id, model, map[string]interface{}{
		"prompt_tokens":     inputTokens,
		"completion_tokens": outputTokens,
	}, outputText, seqNum)
	w.Write([]byte(protocol.FormatResponsesStreamEvent(event)))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (h *ChatHandler) writeResponsesStreamDone(w http.ResponseWriter, id, model, outputText string, seqNum int64) {
	seqNum++
	event := protocol.BuildResponsesDoneEvent(id, model, outputText, seqNum)
	w.Write([]byte(protocol.FormatResponsesStreamEvent(event)))
	w.Write([]byte(protocol.FormatResponsesStreamEvent(protocol.BuildResponsesTerminalDoneEvent(seqNum + 1))))
	w.Write([]byte("data: [DONE]\n\n"))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
