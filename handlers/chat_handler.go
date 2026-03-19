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
	DB           *gorm.DB
	asyncLogWriter *AsyncLogWriter
}

func NewChatHandler(db *gorm.DB) *ChatHandler {
	return &ChatHandler{
		DB:           db,
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

	cached, fingerprint := h.handleCache(w, body, modelName, isStream)
	if cached {
		log.WithField("elapsed", time.Since(startTime).Milliseconds()).Info("Request served from cache")
		return
	}

	reqLog := models.RequestLog{
		CreatedAt:   time.Now(),
		ModelName:   modelName,
		Request:     string(body),
		Fingerprint: fingerprint,
	}
	shouldLog := false
	defer func() {
		reqLog.ResponseTime = time.Since(startTime).Milliseconds()
		if shouldLog && reqLog.Response != "" {
			h.asyncLogWriter.Write(&reqLog)
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.WithField("status_code", resp.StatusCode).Warn("Provider returned non-2xx status code, skipping log")
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
		"model":      modelName,
		"is_stream":  isStream,
		"elapsed_ms": time.Since(startTime).Milliseconds(),
	}).Info("Chat completion request completed")
}
