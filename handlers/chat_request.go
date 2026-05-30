package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
)

func (h *ChatHandler) parseRequest(r *http.Request) ([]byte, map[string]json.RawMessage, string, string, bool, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logRequestReadFailure(r.Context(), r, "chat.completions", err)
		return nil, nil, "", "", false, err
	}
	loggerWithTrace(r.Context()).WithField("body_length", len(body)).Debug("Request body loaded")

	var requestBody map[string]json.RawMessage
	if err := json.Unmarshal(body, &requestBody); err != nil {
		logRequestParseFailure(r.Context(), r, "chat.completions", err)
		return nil, nil, "", "", false, err
	}

	var modelName string
	if rawModel, ok := requestBody["model"]; !ok || json.Unmarshal(rawModel, &modelName) != nil || modelName == "" {
		loggerWithTrace(r.Context()).Error("Request missing model")
		return nil, nil, "", "", false, errors.New("Model name is required")
	}

	userToken := r.Header.Get("Authorization")

	var isStream bool
	if rawStream, ok := requestBody["stream"]; ok {
		if err := json.Unmarshal(rawStream, &isStream); err != nil {
			isStream = false
		}
	}

	loggerWithTrace(r.Context()).WithFields(log.Fields{
		"model":     modelName,
		"is_stream": isStream,
		"has_token": userToken != "",
	}).Debug("Request parsed")

	return body, requestBody, modelName, userToken, isStream, nil
}

func (h *ChatHandler) sendProviderRequest(ctx context.Context, headers http.Header, requestBody map[string]json.RawMessage, config models.ModelConfig, isStream bool) (*http.Response, error) {
	updatedBody, err := buildProviderRequestBody(requestBody, config)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).Error("Provider request build failed")
		return nil, err
	}

	providerURL := buildProviderChatURL(config.APIBaseURL)

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(updatedBody),
	}).Info("Dispatching provider request")

	req, err := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Provider request creation failed")
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

	startTime := time.Now()
	var client *http.Client
	if isStream {
		client = GetStreamHTTPClient()
	} else {
		client = GetHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("url", providerURL).Error("Provider request failed")
		return nil, err
	}

	elapsed := time.Since(startTime)
	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":           providerURL,
		"status_code":   resp.StatusCode,
		"response_time": elapsed.Milliseconds(),
	}).Info("Provider response received")

	return resp, nil
}
