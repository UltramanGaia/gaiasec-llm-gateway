package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-gateway/models"
	"llm-gateway/protocol"

	log "github.com/sirupsen/logrus"
)

type protocolRequestEnvelope struct {
	body      []byte
	rawBody   map[string]json.RawMessage
	modelName string
	isStream  bool
}

func (h *ChatHandler) handleProtocolRequest(w http.ResponseWriter, r *http.Request, inbound protocol.InboundProtocol) {
	startTime := time.Now()
	traceID := requestTraceIDFromHeaders(r.Header)
	ctx := withRequestTrace(r.Context(), traceID)
	r = r.WithContext(ctx)
	w.Header().Set("X-Trace-ID", traceID)

	endpoint := protocolEndpointName(inbound)
	loggerWithTrace(ctx).WithFields(log.Fields{
		"type":        "request",
		"endpoint":    endpoint,
		"remote_addr": r.RemoteAddr,
	}).Debug("Incoming request")

	if h.handleCORS(w, r) {
		loggerWithTrace(ctx).Debug("CORS preflight handled")
		return
	}

	envelope, err := h.parseProtocolRequest(r, inbound)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reqLog := models.RequestLog{
		CreatedAt: time.Now(),
		ModelName: envelope.modelName,
		Request:   string(envelope.body),
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
		if shouldLog && (reqLog.Response != "" || len(reqLog.StreamResponse) > 0) {
			h.asyncLogWriter.Write(&reqLog)
		}
		logContextState(ctx, log.Fields{
			"endpoint":       endpoint,
			"model":          envelope.modelName,
			"backend_model":  reqLog.BackendModelName,
			"backend_id":     reqLog.BackendConfigID,
			"is_stream":      envelope.isStream,
			"elapsed_ms":     reqLog.ResponseTime,
			"request_logged": shouldLog,
			"request_ok":     requestSucceeded,
		}, "Request context ended before handler exit")
	}()

	configs, err := h.getModelConfigs(envelope.modelName)
	if err != nil {
		loggerWithTrace(ctx).WithField("model", envelope.modelName).WithError(err).Error("Model lookup failed")
		http.Error(w, "Model not found: "+envelope.modelName, http.StatusNotFound)
		return
	}

	reqs := deriveCapabilityRequirements(inbound, envelope.rawBody)
	configs, err = filterConfigsByCapabilities(configs, reqs)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("model", envelope.modelName).Warn("Request rejected by capability validation")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, selectedConfig, lease, attempts, err := h.dispatchProtocolRequest(ctx, inbound, r.Header, envelope, configs)
	if err != nil {
		loggerWithTrace(ctx).WithError(err).WithFields(log.Fields{
			"model":    envelope.modelName,
			"attempts": attempts,
		}).Error("Protocol dispatch failed")
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
		loggerWithTrace(ctx).WithFields(log.Fields{
			"status_code":   resp.StatusCode,
			"model":         envelope.modelName,
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
		_, _ = w.Write(respBody)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if responseBodyWillBeRewritten(inbound, selectedConfig.UpstreamType, protocol.ResolveDispatchMode(inbound, selectedConfig.UpstreamType), envelope.isStream) {
		w.Header().Del("Content-Length")
		w.Header().Del("Content-Encoding")
	}

	w.WriteHeader(resp.StatusCode)
	if err := h.writeProtocolResponse(w, resp, &reqLog, selectedConfig, inbound, protocol.ResolveDispatchMode(inbound, selectedConfig.UpstreamType), envelope.isStream); err != nil {
		loggerWithTrace(ctx).WithError(err).WithField("endpoint", endpoint).Error("Protocol response handling failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if reqLog.Response != "" || len(reqLog.StreamResponse) > 0 {
		shouldLog = true
	}
	requestSucceeded = true
}

func responseBodyWillBeRewritten(inbound protocol.InboundProtocol, upstream models.UpstreamType, mode protocol.DispatchMode, isStream bool) bool {
	if isStream {
		return false
	}

	switch inbound {
	case protocol.InboundProtocolChat:
		return true
	case protocol.InboundProtocolResponses:
		return mode != protocol.DispatchPassthrough
	case protocol.InboundProtocolAnthropic:
		return upstream != models.UpstreamTypeAnthropicMessages
	default:
		return false
	}
}

func protocolEndpointName(inbound protocol.InboundProtocol) string {
	switch inbound {
	case protocol.InboundProtocolChat:
		return "chat.completions"
	case protocol.InboundProtocolResponses:
		return "responses"
	case protocol.InboundProtocolAnthropic:
		return "anthropic.messages"
	default:
		return string(inbound)
	}
}

func (h *ChatHandler) parseProtocolRequest(r *http.Request, inbound protocol.InboundProtocol) (protocolRequestEnvelope, error) {
	if inbound == protocol.InboundProtocolChat {
		body, rawBody, modelName, _, isStream, err := h.parseRequest(r)
		if err != nil {
			return protocolRequestEnvelope{}, err
		}
		return protocolRequestEnvelope{body: body, rawBody: rawBody, modelName: modelName, isStream: isStream}, nil
	}

	endpoint := protocolEndpointName(inbound)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logRequestReadFailure(r.Context(), r, endpoint, err)
		return protocolRequestEnvelope{}, err
	}

	var rawBody map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawBody); err != nil {
		logRequestParseFailure(r.Context(), r, endpoint, err)
		return protocolRequestEnvelope{}, err
	}

	var modelName string
	if rawModel, ok := rawBody["model"]; !ok || json.Unmarshal(rawModel, &modelName) != nil || modelName == "" {
		return protocolRequestEnvelope{}, errors.New("Model name is required")
	}

	var isStream bool
	if rawStream, ok := rawBody["stream"]; ok {
		_ = json.Unmarshal(rawStream, &isStream)
	}

	return protocolRequestEnvelope{body: body, rawBody: rawBody, modelName: modelName, isStream: isStream}, nil
}

func (h *ChatHandler) dispatchProtocolRequest(ctx context.Context, inbound protocol.InboundProtocol, headers http.Header, envelope protocolRequestEnvelope, configs []models.ModelConfig) (*http.Response, models.ModelConfig, *backendRequestLease, []providerAttempt, error) {
	attempts := make([]providerAttempt, 0, len(configs))
	var lastFailure *providerFailureResponse
	var lastErr error
	var unsupportedCount int

	for {
		orderedConfigs := buildAttemptOrder(envelope.modelName, configs)
		if len(orderedConfigs) == 0 {
			if err := waitForBackendCapacity(ctx, envelope.modelName); err != nil {
				return nil, models.ModelConfig{}, nil, attempts, err
			}
			continue
		}

		for _, config := range orderedConfigs {
			normalizedEnvelope := normalizeEnvelopeForConfig(envelope, config, inbound)
			mode := protocol.ResolveDispatchMode(inbound, config.UpstreamType)
			if mode == protocol.DispatchUnsupported {
				unsupportedCount++
				attempts = append(attempts, providerAttempt{
					ConfigID:     config.ID,
					BackendModel: config.ModelName,
					APIBaseURL:   config.APIBaseURL,
					Error:        fmt.Sprintf("inbound %s is not supported for upstream_type %s", inbound, config.UpstreamType),
				})
				continue
			}

			lease, ok := getBackendRuntimeManager().startRequest(config, envelope.modelName)
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
			attempt := providerAttempt{
				ConfigID:     config.ID,
				BackendModel: config.ModelName,
				APIBaseURL:   config.APIBaseURL,
				ActiveCount:  lease.ActiveRequestsOnStart(),
			}
			resp, retries, err := h.sendWithBadGatewayRetry(ctx, attempt, func() (*http.Response, error) {
				return h.sendProtocolRequest(ctx, inbound, headers, normalizedEnvelope, config)
			})
			attempt.RetryCount = retries
			if err != nil {
				attempt.Error = err.Error()
				attempt.ResponseTime = time.Since(attemptStart).Milliseconds()
				attempts = append(attempts, attempt)
				lease.Finish(backendObservation{Success: false, ResponseTimeMS: attempt.ResponseTime})
				lastErr = err
				continue
			}

			attempt.StatusCode = resp.StatusCode
			attempt.ResponseTime = time.Since(attemptStart).Milliseconds()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				attempts = append(attempts, attempt)
				return resp, config, lease, attempts, nil
			}

			respBody, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				attempt.Error = readErr.Error()
				attempts = append(attempts, attempt)
				lease.Finish(backendObservation{Success: false, ResponseTimeMS: attempt.ResponseTime})
				lastErr = readErr
				continue
			}

			attempts = append(attempts, attempt)
			lease.Finish(backendObservation{Success: false, ResponseTimeMS: attempt.ResponseTime})
			lastFailure = &providerFailureResponse{
				StatusCode: resp.StatusCode,
				Header:     resp.Header.Clone(),
				Body:       respBody,
				Config:     config,
			}
		}

		if lastFailure != nil {
			return cloneFailureResponse(lastFailure), lastFailure.Config, nil, attempts, nil
		}
		if lastErr != nil {
			return nil, models.ModelConfig{}, nil, attempts, lastErr
		}
		if unsupportedCount == len(configs) {
			return nil, models.ModelConfig{}, nil, attempts, fmt.Errorf("no backend for inbound %s is compatible with configured upstream types", inbound)
		}
		if err := waitForBackendCapacity(ctx, envelope.modelName); err != nil {
			return nil, models.ModelConfig{}, nil, attempts, err
		}
	}
}

func (h *ChatHandler) sendProtocolRequest(ctx context.Context, inbound protocol.InboundProtocol, headers http.Header, envelope protocolRequestEnvelope, config models.ModelConfig) (*http.Response, error) {
	switch inbound {
	case protocol.InboundProtocolChat:
		switch config.UpstreamType {
		case models.UpstreamTypeOpenAIChat:
			return h.sendProviderRequest(ctx, headers, envelope.rawBody, config, envelope.isStream)
		case models.UpstreamTypeOpenAIResponses:
			return h.sendChatToResponsesRequest(ctx, headers, envelope.body, config, envelope.isStream)
		case models.UpstreamTypeAnthropicMessages:
			return h.sendChatToAnthropicRequest(ctx, headers, envelope.body, config, envelope.isStream)
		}
	case protocol.InboundProtocolResponses:
		switch config.UpstreamType {
		case models.UpstreamTypeOpenAIChat:
			return h.sendResponsesRequest(ctx, headers, envelope.body, config, envelope.isStream)
		case models.UpstreamTypeOpenAIResponses:
			return h.sendResponsesPassthroughRequest(ctx, headers, envelope.rawBody, config, envelope.isStream)
		case models.UpstreamTypeAnthropicMessages:
			return h.sendResponsesToAnthropicRequest(ctx, headers, envelope.body, config, envelope.isStream)
		}
	case protocol.InboundProtocolAnthropic:
		switch config.UpstreamType {
		case models.UpstreamTypeAnthropicMessages:
			return h.sendAnthropicRequest(ctx, headers, envelope.body, config, envelope.isStream)
		case models.UpstreamTypeOpenAIChat:
			return h.sendAnthropicToChatRequest(ctx, headers, envelope.body, config, envelope.isStream)
		case models.UpstreamTypeOpenAIResponses:
			return h.sendAnthropicToResponsesRequest(ctx, headers, envelope.body, config, envelope.isStream)
		}
	}

	return nil, fmt.Errorf("unsupported route: inbound=%s upstream=%s", inbound, config.UpstreamType)
}

func (h *ChatHandler) writeProtocolResponse(w http.ResponseWriter, resp *http.Response, reqLog *models.RequestLog, config models.ModelConfig, inbound protocol.InboundProtocol, mode protocol.DispatchMode, isStream bool) error {
	switch inbound {
	case protocol.InboundProtocolChat:
		switch config.UpstreamType {
		case models.UpstreamTypeOpenAIResponses:
			if isStream {
				h.handleChatFromResponsesStreamResponse(w, resp, reqLog, config)
				return nil
			}
			return h.handleChatFromResponsesNonStreamResponse(w, resp, reqLog, config)
		case models.UpstreamTypeAnthropicMessages:
			if isStream {
				h.handleChatFromAnthropicStreamResponse(w, resp, reqLog, config)
				return nil
			}
			return h.handleChatFromAnthropicNonStreamResponse(w, resp, reqLog, config)
		}
		if isStream {
			h.handleStreamResponse(w, resp, reqLog, config)
			return nil
		}
		return h.handleNonStreamResponse(w, resp, reqLog, config)
	case protocol.InboundProtocolResponses:
		if mode == protocol.DispatchPassthrough {
			if isStream {
				h.passthroughStreamResponse(w, resp, reqLog)
				return nil
			}
			return h.passthroughNonStreamResponse(w, resp, reqLog)
		}
		if config.UpstreamType == models.UpstreamTypeAnthropicMessages {
			if isStream {
				h.handleResponsesFromAnthropicStreamResponse(w, resp, reqLog, config)
				return nil
			}
			return h.handleResponsesFromAnthropicNonStreamResponse(w, resp, reqLog, config)
		}
		if isStream {
			h.handleResponsesStreamResponse(w, resp, reqLog)
			return nil
		}
		return h.handleResponsesNonStreamResponse(w, resp, reqLog)
	case protocol.InboundProtocolAnthropic:
		switch config.UpstreamType {
		case models.UpstreamTypeOpenAIChat:
			if isStream {
				h.handleAnthropicFromChatStreamResponse(w, resp, reqLog, config)
				return nil
			}
			return h.handleAnthropicFromChatNonStreamResponse(w, resp, reqLog, config)
		case models.UpstreamTypeOpenAIResponses:
			if isStream {
				h.handleAnthropicFromResponsesStreamResponse(w, resp, reqLog, config)
				return nil
			}
			return h.handleAnthropicFromResponsesNonStreamResponse(w, resp, reqLog, config)
		}
		if isStream {
			h.passthroughStreamResponse(w, resp, reqLog)
			return nil
		}
		return h.passthroughNonStreamResponse(w, resp, reqLog)
	default:
		return fmt.Errorf("unsupported inbound protocol %s", inbound)
	}
}

func (h *ChatHandler) sendResponsesPassthroughRequest(ctx context.Context, headers http.Header, requestBody map[string]json.RawMessage, config models.ModelConfig, isStream bool) (*http.Response, error) {
	bodyCopy, err := buildRequestBodyWithModel(requestBody, config.ModelName)
	if err != nil {
		return nil, err
	}
	updatedBody, err := json.Marshal(bodyCopy)
	if err != nil {
		return nil, err
	}

	providerURL := buildProviderResponsesURL(config.APIBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if headers != nil && strings.TrimSpace(headers.Get("User-Agent")) != "" {
		req.Header.Set("User-Agent", headers.Get("User-Agent"))
	}
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}

	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
	}
	return client.Do(req)
}

func (h *ChatHandler) sendChatToResponsesRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	responsesReq, err := convertChatRequestToResponsesRequest(body, config.ModelName)
	if err != nil {
		return nil, err
	}
	updatedBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, err
	}
	providerURL := buildProviderResponsesURL(config.APIBaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, providerURL, bytes.NewReader(updatedBody))
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if headers != nil && strings.TrimSpace(headers.Get("User-Agent")) != "" {
		req.Header.Set("User-Agent", headers.Get("User-Agent"))
	}
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
	}
	return client.Do(req)
}

func (h *ChatHandler) sendChatToAnthropicRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	requestBody, err := convertChatRequestToAnthropicRequest(body, config.ModelName)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildAnthropicMessagesURL(config.APIBaseURL), bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	if headers != nil && strings.TrimSpace(headers.Get("User-Agent")) != "" {
		req.Header.Set("User-Agent", headers.Get("User-Agent"))
	}
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
	}
	return client.Do(req)
}

func (h *ChatHandler) sendAnthropicToChatRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	chatReq, err := convertAnthropicRequestToChatRequest(body, config.ModelName)
	if err != nil {
		return nil, err
	}
	updatedBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, err
	}
	return h.sendOpenAIChatUpstreamRequest(ctx, headers, updatedBody, config, isStream)
}

func (h *ChatHandler) sendResponsesToAnthropicRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	requestBody, err := convertResponsesRequestToAnthropicRequest(body, config.ModelName)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildAnthropicMessagesURL(config.APIBaseURL), bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	if headers != nil && strings.TrimSpace(headers.Get("User-Agent")) != "" {
		req.Header.Set("User-Agent", headers.Get("User-Agent"))
	}
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
	}
	return client.Do(req)
}

func (h *ChatHandler) sendAnthropicToResponsesRequest(ctx context.Context, headers http.Header, body []byte, config models.ModelConfig, isStream bool) (*http.Response, error) {
	responsesReq, err := convertAnthropicRequestToResponsesRequest(body, config.ModelName)
	if err != nil {
		return nil, err
	}
	updatedBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildProviderResponsesURL(config.APIBaseURL), bytes.NewReader(updatedBody))
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	if headers != nil && strings.TrimSpace(headers.Get("User-Agent")) != "" {
		req.Header.Set("User-Agent", headers.Get("User-Agent"))
	}
	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := GetHTTPClient()
	if isStream {
		client = GetStreamHTTPClient()
	}
	return client.Do(req)
}
