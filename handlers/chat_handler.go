package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"

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
	startTime := time.Now()
	log.WithFields(log.Fields{
		"type":        "request",
		"endpoint":    "chat.completions",
		"remote_addr": r.RemoteAddr,
	}).Debug("Incoming request")

	if h.handleCORS(w, r) {
		log.Debug("CORS preflight handled")
		return
	}

	body, requestBody, modelName, _, isStream, err := h.parseRequest(r)
	if err != nil {
		log.WithError(err).Error("Request parse failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqLog := models.RequestLog{
		CreatedAt: time.Now(),
		ModelName: modelName,
		Request:   string(body),
	}
	shouldLog := false
	var selectedLease *backendRequestLease
	requestSucceeded := false
	defer func() {
		reqLog.ResponseTime = time.Since(startTime).Milliseconds()
		if reqLog.FirstTokenLatency == 0 && reqLog.Response != "" {
			reqLog.FirstTokenLatency = reqLog.ResponseTime
		}
		if selectedLease != nil {
			selectedLease.Finish(backendObservation{
				Success:           requestSucceeded,
				ResponseTimeMS:    reqLog.ResponseTime,
				FirstTokenLatency: reqLog.FirstTokenLatency,
				AvgTokenLatency:   reqLog.AvgTokenLatency,
			})
		}
		if shouldLog && reqLog.Response != "" {
			h.asyncLogWriter.Write(&reqLog)
		}
	}()

	configs, err := h.getModelConfigs(modelName)
	if err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model lookup failed")
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	resp, selectedConfig, lease, attempts, err := h.dispatchProviderRequest(r.Header, requestBody, modelName, configs, isStream)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"model":    modelName,
			"attempts": attempts,
		}).Error("Provider dispatch failed")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	selectedLease = lease
	reqLog.BackendConfigID = selectedConfig.ID
	reqLog.BackendModelName = selectedConfig.ModelName
	reqLog.BackendAPIBaseURL = selectedConfig.APIBaseURL
	if lease != nil {
		reqLog.ActiveRequests = lease.ActiveRequestsOnStart()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithFields(log.Fields{
			"status_code":   resp.StatusCode,
			"model":         modelName,
			"backend_model": selectedConfig.ModelName,
			"backend_id":    selectedConfig.ID,
			"attempts":      attempts,
		}).Warn("Provider returned non-2xx, skipping request log")
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		w.Write(respBody)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if isStream {
		h.handleStreamResponse(w, resp, &reqLog, selectedConfig)
	} else {
		if err := h.handleNonStreamResponse(w, resp, &reqLog, selectedConfig); err != nil {
			log.WithError(err).Error("Non-stream response handling failed")
			return
		}
	}

	if reqLog.Response != "" || len(reqLog.StreamResponse) > 0 {
		shouldLog = true
	}
	requestSucceeded = true

	log.WithFields(log.Fields{
		"model":         modelName,
		"backend_model": selectedConfig.ModelName,
		"backend_id":    selectedConfig.ID,
		"is_stream":     isStream,
		"attempts":      attempts,
		"elapsed_ms":    time.Since(startTime).Milliseconds(),
	}).Info("Chat completion served")
}

func (h *ChatHandler) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	log.WithFields(log.Fields{
		"type":        "request",
		"endpoint":    "anthropic.messages",
		"remote_addr": r.RemoteAddr,
	}).Debug("Incoming request")

	if h.handleCORS(w, r) {
		log.Debug("CORS preflight handled")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Request body read failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		log.WithError(err).Error("Request JSON parse failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		log.Error("Anthropic request missing model")
		http.Error(w, "Model name is required", http.StatusBadRequest)
		return
	}

	isStream := false
	if streamVal, ok := requestBody["stream"].(bool); ok && streamVal {
		isStream = true
	}

	reqLog := models.RequestLog{
		CreatedAt: time.Now(),
		ModelName: modelName,
		Request:   string(body),
	}
	shouldLog := false
	var selectedLease *backendRequestLease
	requestSucceeded := false
	defer func() {
		reqLog.ResponseTime = time.Since(startTime).Milliseconds()
		if reqLog.FirstTokenLatency == 0 && reqLog.Response != "" {
			reqLog.FirstTokenLatency = reqLog.ResponseTime
		}
		if selectedLease != nil {
			selectedLease.Finish(backendObservation{
				Success:           requestSucceeded,
				ResponseTimeMS:    reqLog.ResponseTime,
				FirstTokenLatency: reqLog.FirstTokenLatency,
				AvgTokenLatency:   reqLog.AvgTokenLatency,
			})
		}
		if shouldLog && reqLog.Response != "" {
			h.asyncLogWriter.Write(&reqLog)
		}
	}()

	configs, err := h.getModelConfigs(modelName)
	if err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model lookup failed")
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	resp, selectedConfig, lease, attempts, err := h.dispatchAnthropicRequest(r.Header, body, modelName, configs, isStream)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"model":    modelName,
			"attempts": attempts,
		}).Error("Anthropic provider dispatch failed")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	selectedLease = lease
	reqLog.BackendConfigID = selectedConfig.ID
	reqLog.BackendModelName = selectedConfig.ModelName
	reqLog.BackendAPIBaseURL = selectedConfig.APIBaseURL
	if lease != nil {
		reqLog.ActiveRequests = lease.ActiveRequestsOnStart()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithFields(log.Fields{
			"status_code":   resp.StatusCode,
			"model":         modelName,
			"backend_model": selectedConfig.ModelName,
			"attempts":      attempts,
		}).Warn("Provider returned non-2xx, skipping request log")
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		respBody, _ := io.ReadAll(resp.Body)
		w.Write(respBody)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if isStream {
		h.passthroughStreamResponse(w, resp, &reqLog)
	} else {
		if err := h.passthroughNonStreamResponse(w, resp, &reqLog); err != nil {
			log.WithError(err).Error("Anthropic non-stream response handling failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if reqLog.Response != "" {
		shouldLog = true
	}
	requestSucceeded = true

	log.WithFields(log.Fields{
		"model":         modelName,
		"backend_model": selectedConfig.ModelName,
		"backend_id":    selectedConfig.ID,
		"is_stream":     isStream,
		"attempts":      attempts,
		"elapsed_ms":    time.Since(startTime).Milliseconds(),
	}).Info("Anthropic request served")
}

func (h *ChatHandler) dispatchAnthropicRequest(headers http.Header, body []byte, modelName string, configs []models.ModelConfig, isStream bool) (*http.Response, models.ModelConfig, *backendRequestLease, []providerAttempt, error) {
	orderedConfigs := buildAttemptOrder(modelName, configs)
	attempts := make([]providerAttempt, 0, len(orderedConfigs))

	var lastFailure *providerFailureResponse
	var lastErr error

	for _, config := range orderedConfigs {
		lease, ok := getBackendRuntimeManager().startRequest(config, modelName)
		if !ok {
			attempts = append(attempts, providerAttempt{
				ConfigID:     config.ID,
				BackendModel: config.ModelName,
				APIBaseURL:   config.APIBaseURL,
				Error:        "max concurrency reached",
			})
			continue
		}
		attemptStart := time.Now()
		resp, err := h.sendAnthropicRequest(headers, body, config, isStream)
		attempt := providerAttempt{
			ConfigID:     config.ID,
			BackendModel: config.ModelName,
			APIBaseURL:   config.APIBaseURL,
			ActiveCount:  lease.ActiveRequestsOnStart(),
		}

		if err != nil {
			attempt.Error = err.Error()
			attempt.ResponseTime = time.Since(attemptStart).Milliseconds()
			attempts = append(attempts, attempt)
			lease.Finish(backendObservation{
				Success:        false,
				ResponseTimeMS: attempt.ResponseTime,
			})
			lastErr = err
			continue
		}

		attempt.StatusCode = resp.StatusCode
		attempt.ResponseTime = time.Since(attemptStart).Milliseconds()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			attempts = append(attempts, attempt)
			return resp, config, lease, attempts, nil
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			attempt.Error = readErr.Error()
			attempts = append(attempts, attempt)
			lease.Finish(backendObservation{
				Success:        false,
				ResponseTimeMS: attempt.ResponseTime,
			})
			lastErr = readErr
			continue
		}

		attempts = append(attempts, attempt)
		lease.Finish(backendObservation{
			Success:        false,
			ResponseTimeMS: attempt.ResponseTime,
		})
		lastFailure = &providerFailureResponse{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       body,
			Config:     config,
		}
	}

	if lastFailure != nil {
		return cloneFailureResponse(lastFailure), lastFailure.Config, nil, attempts, nil
	}
	if lastErr != nil {
		return nil, models.ModelConfig{}, nil, attempts, lastErr
	}
	return nil, models.ModelConfig{}, nil, attempts, errors.New("no available backend model config")
}

func (h *ChatHandler) sendAnthropicRequest(headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	providerURL := buildAnthropicMessagesURL(config.APIBaseURL)

	log.WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(body),
	}).Info("Dispatching Anthropic provider request")

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(body))
	if err != nil {
		log.WithError(err).WithField("url", providerURL).Error("Anthropic request creation failed")
		return nil, err
	}

	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
		log.WithError(err).WithField("url", providerURL).Error("Anthropic request failed")
		return nil, err
	}

	log.WithFields(log.Fields{
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

func (h *ChatHandler) passthroughStreamResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog) {
	var contentBuilder strings.Builder
	chunkCount := 0
	metrics := newStreamMetricsTracker()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.WithError(err).Error("Passthrough stream read failed")
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

	log.WithField("chunks", chunkCount).Info("Passthrough stream completed")

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
