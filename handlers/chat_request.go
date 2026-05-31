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
	return h.sendOpenAIChatUpstreamRequest(ctx, headers, updatedBody, config, isStream)
}

func (h *ChatHandler) sendOpenAIChatUpstreamRequest(ctx context.Context, headers http.Header, requestBody []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	if shouldAggregateOpenAIChatToolCallResponse(requestBody, isStream) {
		resp, err := h.sendOpenAIChatUpstreamRequestAggregated(ctx, headers, requestBody, config)
		if err == nil {
			return resp, nil
		}
		loggerWithTrace(ctx).WithError(err).WithField("model", config.ModelName).Warn("OpenAI chat tool-call aggregation failed, falling back to standard non-stream request")
	}
	return h.executeOpenAIChatUpstreamRequest(ctx, headers, requestBody, config, isStream)
}

func (h *ChatHandler) sendOpenAIChatUpstreamRequestAggregated(ctx context.Context, headers http.Header, requestBody []byte, config models.ModelConfig) (*http.Response, error) {
	forcedBody, err := forceStreamRequestBody(requestBody)
	if err != nil {
		return nil, err
	}

	resp, err := h.executeOpenAIChatUpstreamRequest(ctx, headers, forcedBody, config, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	rawStream, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	streamLog, err := buildOpenAIStreamLogResponse(string(rawStream))
	if err != nil {
		return nil, err
	}

	return synthesizeJSONResponse(resp, []byte(streamLog.ResponseJSON)), nil
}

func (h *ChatHandler) executeOpenAIChatUpstreamRequest(ctx context.Context, headers http.Header, requestBody []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	providerURL := buildProviderChatURL(config.APIBaseURL)

	loggerWithTrace(ctx).WithFields(log.Fields{
		"url":         providerURL,
		"model":       config.ModelName,
		"is_stream":   isStream,
		"body_length": len(requestBody),
	}).Info("Dispatching provider request")

	req, err := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(requestBody))
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
	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
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

func shouldAggregateOpenAIChatToolCallResponse(requestBody []byte, isStream bool) bool {
	if isStream {
		return false
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		return false
	}

	toolsRaw, ok := payload["tools"]
	if !ok || len(toolsRaw) == 0 || string(toolsRaw) == "null" {
		return false
	}

	var tools []json.RawMessage
	return json.Unmarshal(toolsRaw, &tools) == nil && len(tools) > 0
}

func forceStreamRequestBody(requestBody []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		return nil, err
	}

	streamRaw, err := json.Marshal(true)
	if err != nil {
		return nil, err
	}
	payload["stream"] = streamRaw
	return json.Marshal(payload)
}

func synthesizeJSONResponse(resp *http.Response, body []byte) *http.Response {
	synth := new(http.Response)
	*synth = *resp
	synth.Header = resp.Header.Clone()
	synth.Header.Set("Content-Type", "application/json")
	synth.Header.Del("Content-Encoding")
	synth.Header.Del("Transfer-Encoding")
	synth.ContentLength = int64(len(body))
	synth.Body = io.NopCloser(bytes.NewReader(body))
	return synth
}
