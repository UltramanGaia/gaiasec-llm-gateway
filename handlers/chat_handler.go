package handlers

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"
	"llm-gateway/protocol"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ChatHandler struct {
	DB             *gorm.DB
	asyncLogWriter *AsyncLogWriter
}

func NewChatHandler(db *gorm.DB) *ChatHandler {
	return &ChatHandler{
		DB:             db,
		asyncLogWriter: GetAsyncLogWriter(db),
	}
}

func (h *ChatHandler) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	h.handleProtocolRequest(w, r, protocol.InboundProtocolChat)
}

func (h *ChatHandler) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	h.handleProtocolRequest(w, r, protocol.InboundProtocolAnthropic)
}

func (h *ChatHandler) sendAnthropicRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	providerURL := buildAnthropicMessagesURL(config.APIBaseURL)
	ir, err := protocol.DecodeAnthropicRequest(body)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("model", config.ModelName).Error("Anthropic request decode failed")
		return nil, err
	}
	requestBody, err := protocol.EncodeAnthropicRequest(ir, config.ModelName)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("model", config.ModelName).Error("Anthropic request body rewrite failed")
		return nil, err
	}

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(requestBody),
	}).Info("Dispatching Anthropic provider request")

	req, err := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(requestBody))
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Anthropic request creation failed")
		return nil, err
	}

	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	if headers != nil {
		if userAgent := strings.TrimSpace(headers.Get("User-Agent")); userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
	}

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	var client *http.Client
	if isStream {
		client = GetStreamHTTPClient()
	} else {
		client = GetHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Anthropic request failed")
		return nil, err
	}

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"status_code": resp.StatusCode,
	}).Info("Anthropic response received")

	return resp, nil
}

func buildAnthropicMessagesURL(apiBaseURL string) string {
	providerURL := strings.TrimSpace(apiBaseURL)
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	return providerURL + "messages"
}

func (h *ChatHandler) Responses(w http.ResponseWriter, r *http.Request) {
	h.handleProtocolRequest(w, r, protocol.InboundProtocolResponses)
}

func (h *ChatHandler) sendResponsesRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	ir, err := protocol.DecodeResponsesRequest(body)
	if err != nil {
		return nil, err
	}
	updatedBody, err := protocol.EncodeOpenAIChatRequest(ir, config.ModelName)
	if err != nil {
		return nil, err
	}

	providerURL := buildProviderChatURL(config.APIBaseURL)

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(updatedBody),
	}).Info("Dispatching Responses→Chat provider request")

	req, err := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Responses request creation failed")
		return nil, err
	}

	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if headers != nil {
		if userAgent := strings.TrimSpace(headers.Get("User-Agent")); userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
	}

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	var client *http.Client
	if isStream {
		client = GetStreamHTTPClient()
	} else {
		client = GetHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Responses request failed")
		return nil, err
	}

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"status_code": resp.StatusCode,
	}).Info("Responses→Chat response received")

	return resp, nil
}

func (h *ChatHandler) passthroughStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	var contentBuilder strings.Builder
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
					"backend_model":       reqLog.BackendModelName,
					"backend_config_id":   reqLog.BackendConfigID,
					"stream_chunks":       chunkCount,
					"first_token_latency": metrics.FirstTokenLatency(),
					"avg_token_latency":   metrics.AvgTokenLatency(),
					"termination_reason":  streamTerminationReason(err),
				})
				switch {
				case isExpectedStreamTermination(err):
					entry.Warn("Passthrough stream terminated before EOF")
				default:
					entry.Error("Passthrough stream read failed")
				}
			}
			break
		}

		chunkCount++
		if lineHasStreamingData(line) {
			metrics.Record(time.Now())
		}
		w.Write([]byte(line))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			contentBuilder.WriteString(line)
		}
	}

	loggerWithTrace(ctx).WithFields(log.Fields{
		"backend_model":       reqLog.BackendModelName,
		"backend_config_id":   reqLog.BackendConfigID,
		"stream_chunks":       chunkCount,
		"first_token_latency": metrics.FirstTokenLatency(),
		"avg_token_latency":   metrics.AvgTokenLatency(),
	}).Info("Passthrough stream completed")

	if contentBuilder.Len() > 0 {
		reqLog.Response = contentBuilder.String()
	}
	reqLog.FirstTokenLatency = metrics.FirstTokenLatency()
	reqLog.AvgTokenLatency = metrics.AvgTokenLatency()
}

func (h *ChatHandler) passthroughNonStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) error {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("Passthrough response read failed")
		return err
	}

	if len(respBody) == 0 {
		log.Warn("Passthrough response empty")
		return nil
	}

	w.Write(respBody)
	reqLog.Response = string(respBody)

	log.WithField("response_length", len(respBody)).Info("Passthrough non-stream completed")
	return nil
}

func lineHasStreamingData(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return false
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	return payload != "" && payload != "[DONE]"
}
