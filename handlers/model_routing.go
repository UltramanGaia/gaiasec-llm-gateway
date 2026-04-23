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
	ResponseTime int64
	ActiveCount  int
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
	if configs, found := GetModelConfigsFromCache(modelName); found && len(configs) > 0 {
		log.WithFields(log.Fields{
			"model":        modelName,
			"config_count": len(configs),
		}).Debug("Model configs cache hit")
		return configs, nil
	}

	var configs []models.ModelConfig
	if err := h.DB.Where("name = ? AND enabled = ?", modelName, true).Order("id ASC").Find(&configs).Error; err != nil {
		log.WithError(err).WithField("model", modelName).Error("Model config query failed")
		return nil, err
	}

	valid := configs[:0]
	for _, c := range configs {
		if strings.TrimSpace(c.APIBaseURL) == "" || strings.TrimSpace(c.ModelName) == "" {
			log.WithFields(log.Fields{
				"model":        modelName,
				"config_id":    c.ID,
				"api_base_url": c.APIBaseURL,
				"model_name":   c.ModelName,
			}).Warn("Skipping model config with empty api_base_url or model_name")
			continue
		}
		valid = append(valid, c)
	}
	configs = valid

	if len(configs) == 0 {
		log.WithField("model", modelName).Warn("Model config not found")
		return nil, gorm.ErrRecordNotFound
	}

	SetModelConfigs(modelName, configs)

	log.WithFields(log.Fields{
		"model":        modelName,
		"config_count": len(configs),
	}).Info("Model configs loaded")

	return configs, nil
}

func buildProviderChatURL(apiBaseURL string) string {
	providerURL := strings.TrimSpace(apiBaseURL)
	if !strings.HasSuffix(providerURL, "/") {
		providerURL += "/"
	}
	return providerURL + "chat/completions"
}

func buildProviderRequestBody(requestBody map[string]json.RawMessage, config models.ModelConfig) ([]byte, error) {
	bodyCopy := make(map[string]json.RawMessage, len(requestBody)+1)
	for key, value := range requestBody {
		bodyCopy[key] = value
	}
	modelJSON, err := json.Marshal(config.ModelName)
	if err != nil {
		return nil, err
	}
	bodyCopy["model"] = modelJSON

	if reasoningRaw, ok := requestBody["reasoning"]; ok && len(reasoningRaw) > 0 {
		var reasoningCopy map[string]json.RawMessage
		if err := json.Unmarshal(reasoningRaw, &reasoningCopy); err == nil {
			if original, exists := reasoningCopy["effort"]; exists {
				reasoningCopy["effort"] = json.RawMessage(`"none"`)
				updatedReasoning, err := json.Marshal(reasoningCopy)
				if err != nil {
					return nil, err
				}
				bodyCopy["reasoning"] = updatedReasoning
				log.WithFields(log.Fields{
					"original_effort": string(original),
					"backend_model":   config.ModelName,
				}).Debug("reasoning.effort overridden to none for backend")
			}
		}
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

func buildAttemptOrder(modelName string, configs []models.ModelConfig) []models.ModelConfig {
	return getBackendRuntimeManager().buildAttemptOrder(modelName, configs)
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

func (h *ChatHandler) dispatchProviderRequest(headers http.Header, requestBody map[string]json.RawMessage, modelName string, configs []models.ModelConfig, isStream bool) (*http.Response, models.ModelConfig, *backendRequestLease, []providerAttempt, error) {
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
		resp, err := h.sendProviderRequest(headers, requestBody, config, isStream)
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
