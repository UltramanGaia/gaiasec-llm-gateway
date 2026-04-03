package handlers

import (
	"io"
	"net/http"
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
	log.WithField("remote_addr", r.RemoteAddr).Info("New chat completion request received")

	if h.handleCORS(w, r) {
		log.Debug("CORS preflight request handled")
		return
	}

	body, requestBody, modelName, _, isStream, err := h.parseRequest(r)
	if err != nil {
		log.WithError(err).Error("Failed to parse request")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqLog := models.RequestLog{
		CreatedAt: time.Now(),
		ModelName: modelName,
		Request:   string(body),
	}
	shouldLog := false
	defer func() {
		reqLog.ResponseTime = time.Since(startTime).Milliseconds()
		if shouldLog && reqLog.Response != "" {
			h.asyncLogWriter.Write(&reqLog)
		}
	}()

	configs, err := h.getModelConfigs(modelName)
	if err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model config not found")
		http.Error(w, "Model not found: "+modelName, http.StatusNotFound)
		return
	}

	resp, selectedConfig, attempts, err := h.dispatchProviderRequest(r.Header, requestBody, modelName, configs, isStream)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"model":    modelName,
			"attempts": attempts,
		}).Error("Failed to send request to provider")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithFields(log.Fields{
			"status_code":   resp.StatusCode,
			"model":         modelName,
			"backend_model": selectedConfig.ModelName,
			"backend_id":    selectedConfig.ID,
			"attempts":      attempts,
		}).Warn("Provider returned non-2xx status code after failover, skipping log")
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
		h.handleStreamResponse(w, resp, &reqLog)
	} else {
		if err := h.handleNonStreamResponse(w, resp, &reqLog); err != nil {
			log.WithError(err).Error("Failed to handle non-stream response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if reqLog.Response != "" {
		shouldLog = true
	}

	log.WithFields(log.Fields{
		"model":         modelName,
		"backend_model": selectedConfig.ModelName,
		"backend_id":    selectedConfig.ID,
		"is_stream":     isStream,
		"attempts":      attempts,
		"elapsed_ms":    time.Since(startTime).Milliseconds(),
	}).Info("Chat completion request completed")
}
