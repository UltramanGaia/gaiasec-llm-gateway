package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type providerAttempt struct {
	ConfigID     uint
	BackendModel string
	APIBaseURL   string
	StatusCode   int
	Error        string
}

type providerFailureResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Config     models.ModelConfig
}

var (
	modelRouteMu      sync.Mutex
	modelRouteRand    = rand.New(rand.NewSource(time.Now().UnixNano()))
	nextRouteOffsetFn = defaultRandomRouteOffset
)

func (h *ChatHandler) getModelConfigs(modelName string) ([]models.ModelConfig, error) {
	log.WithField("model", modelName).Info("Looking up model configs")

	if configs, found := GetModelConfigsFromCache(modelName); found && len(configs) > 0 {
		log.WithFields(log.Fields{
			"model":        modelName,
			"config_count": len(configs),
		}).Info("Model configs found in cache")
		return configs, nil
	}

	var configs []models.ModelConfig
	if err := h.DB.Where("name = ? AND enabled = ?", modelName, true).Order("id ASC").Find(&configs).Error; err != nil {
		log.WithError(err).WithField("model", modelName).Error("Failed to query model configs")
		return nil, err
	}
	if len(configs) == 0 {
		log.WithField("model", modelName).Warn("Model config not found or disabled")
		return nil, gorm.ErrRecordNotFound
	}

	SetModelConfigs(modelName, configs)

	log.WithFields(log.Fields{
		"model":        modelName,
		"config_count": len(configs),
	}).Info("Model configs found and cached")

	return configs, nil
}

func buildProviderChatURL(apiBaseURL string) string {
	providerURL := strings.TrimSpace(apiBaseURL)
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	return providerURL + "chat/completions"
}

func buildProviderRequestBody(requestBody map[string]interface{}, config models.ModelConfig) ([]byte, error) {
	bodyCopy := make(map[string]interface{}, len(requestBody))
	for key, value := range requestBody {
		bodyCopy[key] = value
	}
	bodyCopy["model"] = config.ModelName

	if reasoningVal, ok := requestBody["reasoning"].(map[string]interface{}); ok {
		reasoningCopy := make(map[string]interface{}, len(reasoningVal))
		for key, value := range reasoningVal {
			reasoningCopy[key] = value
		}
		if reasoningCopy["effort"] != nil {
			reasoningCopy["effort"] = "none"
		}
		bodyCopy["reasoning"] = reasoningCopy
	}

	return json.Marshal(bodyCopy)
}

func defaultRandomRouteOffset(count int) int {
	if count <= 1 {
		return 0
	}

	modelRouteMu.Lock()
	defer modelRouteMu.Unlock()

	return modelRouteRand.Intn(count)
}

func buildAttemptOrder(configs []models.ModelConfig) []models.ModelConfig {
	if len(configs) <= 1 {
		return configs
	}

	offset := nextRouteOffsetFn(len(configs))
	ordered := make([]models.ModelConfig, 0, len(configs))
	for i := 0; i < len(configs); i++ {
		ordered = append(ordered, configs[(offset+i)%len(configs)])
	}
	return ordered
}

func cloneFailureResponse(failure *providerFailureResponse) *http.Response {
	return &http.Response{
		StatusCode:    failure.StatusCode,
		Status:        fmt.Sprintf("%d %s", failure.StatusCode, http.StatusText(failure.StatusCode)),
		Header:        failure.Header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(failure.Body)),
		ContentLength: int64(len(failure.Body)),
	}
}

func (h *ChatHandler) dispatchProviderRequest(headers http.Header, requestBody map[string]interface{}, modelName string, configs []models.ModelConfig, isStream bool) (*http.Response, models.ModelConfig, []providerAttempt, error) {
	orderedConfigs := buildAttemptOrder(configs)
	attempts := make([]providerAttempt, 0, len(orderedConfigs))

	var lastFailure *providerFailureResponse
	var lastErr error

	for _, config := range orderedConfigs {
		resp, err := h.sendProviderRequest(headers, requestBody, config, isStream)
		attempt := providerAttempt{
			ConfigID:     config.ID,
			BackendModel: config.ModelName,
			APIBaseURL:   config.APIBaseURL,
		}

		if err != nil {
			attempt.Error = err.Error()
			attempts = append(attempts, attempt)
			lastErr = err
			continue
		}

		attempt.StatusCode = resp.StatusCode
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			attempts = append(attempts, attempt)
			return resp, config, attempts, nil
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			attempt.Error = readErr.Error()
			attempts = append(attempts, attempt)
			lastErr = readErr
			continue
		}

		attempts = append(attempts, attempt)
		lastFailure = &providerFailureResponse{
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       body,
			Config:     config,
		}
	}

	if lastFailure != nil {
		return cloneFailureResponse(lastFailure), lastFailure.Config, attempts, nil
	}
	if lastErr != nil {
		return nil, models.ModelConfig{}, attempts, lastErr
	}
	return nil, models.ModelConfig{}, attempts, errors.New("no available backend model config")
}
