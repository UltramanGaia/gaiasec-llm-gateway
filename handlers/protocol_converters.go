package handlers

import (
	"encoding/json"

	"llm-gateway/protocol"
)

func convertChatRequestToResponsesRequest(body []byte, modelName string) (ResponsesRequest, error) {
	ir, err := protocol.DecodeOpenAIChatRequest(body)
	if err != nil {
		return ResponsesRequest{}, err
	}
	encoded, err := protocol.EncodeResponsesRequest(ir, modelName)
	if err != nil {
		return ResponsesRequest{}, err
	}
	var req ResponsesRequest
	if err := json.Unmarshal(encoded, &req); err != nil {
		return ResponsesRequest{}, err
	}
	return req, nil
}

func convertChatRequestToAnthropicRequest(body []byte, modelName string) ([]byte, error) {
	ir, err := protocol.DecodeOpenAIChatRequest(body)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeAnthropicRequest(ir, modelName)
}

func convertAnthropicRequestToChatRequest(body []byte, modelName string) (map[string]any, error) {
	ir, err := protocol.DecodeAnthropicRequest(body)
	if err != nil {
		return nil, err
	}
	encoded, err := protocol.EncodeOpenAIChatRequest(ir, modelName)
	if err != nil {
		return nil, err
	}
	var chatReq map[string]any
	if err := json.Unmarshal(encoded, &chatReq); err != nil {
		return nil, err
	}
	return chatReq, nil
}

func convertResponsesRequestToAnthropicRequest(body []byte, modelName string) ([]byte, error) {
	ir, err := protocol.DecodeResponsesRequest(body)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeAnthropicRequest(ir, modelName)
}

func convertAnthropicRequestToResponsesRequest(body []byte, modelName string) (ResponsesRequest, error) {
	ir, err := protocol.DecodeAnthropicRequest(body)
	if err != nil {
		return ResponsesRequest{}, err
	}
	encoded, err := protocol.EncodeResponsesRequest(ir, modelName)
	if err != nil {
		return ResponsesRequest{}, err
	}
	var req ResponsesRequest
	if err := json.Unmarshal(encoded, &req); err != nil {
		return ResponsesRequest{}, err
	}
	return req, nil
}

func convertResponsesResponseToChatResponse(respBody []byte, requestedModel string) ([]byte, error) {
	ir, err := protocol.DecodeResponsesResponse(respBody)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeOpenAIChatResponse(ir, requestedModel)
}

func convertResponsesResponseToAnthropicResponse(respBody []byte, requestedModel string) ([]byte, error) {
	ir, err := protocol.DecodeResponsesResponse(respBody)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeAnthropicResponse(ir, requestedModel)
}

func convertAnthropicResponseToChatResponse(respBody []byte, requestedModel string) ([]byte, error) {
	ir, err := protocol.DecodeAnthropicResponse(respBody)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeOpenAIChatResponse(ir, requestedModel)
}

func convertChatResponseToAnthropicResponse(chatResp map[string]interface{}, requestedModel string) ([]byte, error) {
	body, err := json.Marshal(chatResp)
	if err != nil {
		return nil, err
	}
	ir, err := protocol.DecodeOpenAIChatResponse(body)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeAnthropicResponse(ir, requestedModel)
}

func convertAnthropicResponseToResponsesResponse(respBody []byte, requestedModel string) ([]byte, error) {
	ir, err := protocol.DecodeAnthropicResponse(respBody)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeResponsesResponse(ir, requestedModel)
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
