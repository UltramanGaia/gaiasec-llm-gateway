package protocol

import (
	"fmt"
	"strings"

	"llm-gateway/models"
)

func ResolveInboundProtocol(path string) (InboundProtocol, error) {
	normalized := strings.TrimSpace(path)
	switch {
	case strings.HasSuffix(normalized, "/v1/chat/completions"), strings.HasSuffix(normalized, "/chat/completions"):
		return InboundProtocolChat, nil
	case strings.HasSuffix(normalized, "/v1/responses"), strings.HasSuffix(normalized, "/responses"):
		return InboundProtocolResponses, nil
	case strings.HasSuffix(normalized, "/v1/messages"), strings.HasSuffix(normalized, "/messages"):
		return InboundProtocolAnthropic, nil
	default:
		return "", fmt.Errorf("unsupported inbound path: %s", path)
	}
}

func ResolveDispatchMode(inbound InboundProtocol, upstream models.UpstreamType) DispatchMode {
	switch inbound {
	case InboundProtocolChat:
		switch upstream {
		case models.UpstreamTypeOpenAIChat:
			return DispatchPassthrough
		case models.UpstreamTypeOpenAIResponses, models.UpstreamTypeAnthropicMessages:
			return DispatchTransform
		}
	case InboundProtocolResponses:
		switch upstream {
		case models.UpstreamTypeOpenAIResponses:
			return DispatchPassthrough
		case models.UpstreamTypeOpenAIChat:
			return DispatchTransform
		case models.UpstreamTypeAnthropicMessages:
			return DispatchTransform
		}
	case InboundProtocolAnthropic:
		switch upstream {
		case models.UpstreamTypeAnthropicMessages:
			return DispatchPassthrough
		case models.UpstreamTypeOpenAIChat:
			return DispatchTransform
		case models.UpstreamTypeOpenAIResponses:
			return DispatchTransform
		}
	}

	return DispatchUnsupported
}
