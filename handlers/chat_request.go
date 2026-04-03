package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) parseRequest(r *http.Request) ([]byte, map[string]interface{}, string, string, bool, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read request body")
		return nil, nil, "", "", false, err
	}
	log.WithField("body_length", len(body)).Debug("Request body read successfully")

	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		log.WithError(err).WithField("body", string(body)).Error("Failed to parse request body as JSON")
		return nil, nil, "", "", false, err
	}

	modelName, ok := requestBody["model"].(string)
	if !ok || modelName == "" {
		log.Error("Model name is missing in request")
		return nil, nil, "", "", false, errors.New("Model name is required")
	}

	userToken := r.Header.Get("Authorization")

	isStream := false
	if streamValue, ok := requestBody["stream"].(bool); ok && streamValue {
		isStream = true
	}

	log.WithFields(log.Fields{
		"model":     modelName,
		"is_stream": isStream,
		"has_token": userToken != "",
	}).Info("Request parsed successfully")

	return body, requestBody, modelName, userToken, isStream, nil
}

func (h *ChatHandler) sendProviderRequest(headers http.Header, requestBody map[string]interface{}, config models.ModelConfig, isStream bool) (*http.Response, error) {
	updatedBody, err := buildProviderRequestBody(requestBody, config)
	if err != nil {
		log.WithError(err).Error("Failed to marshal request body for provider")
		return nil, err
	}

	providerURL := buildProviderChatURL(config.APIBaseURL)

	log.WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(updatedBody),
	}).Info("Sending request to LLM provider")

	req, err := http.NewRequest("POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		log.WithError(err).WithField("url", providerURL).Error("Failed to create HTTP request for provider")
		return nil, err
	}

	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	startTime := time.Now()
	var client *http.Client
	if isStream {
		client = GetStreamHTTPClient()
	} else {
		client = GetHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).WithField("url", providerURL).Error("Failed to send request to provider")
		return nil, err
	}

	elapsed := time.Since(startTime)
	log.WithFields(log.Fields{
		"url":           providerURL,
		"status_code":   resp.StatusCode,
		"response_time": elapsed.Milliseconds(),
	}).Info("Received response from LLM provider")

	return resp, nil
}
