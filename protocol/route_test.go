package protocol

import (
	"testing"

	"llm-gateway/models"
)

func TestResolveInboundProtocol(t *testing.T) {
	tests := []struct {
		path     string
		expected InboundProtocol
	}{
		{path: "/v1/chat/completions", expected: InboundProtocolChat},
		{path: "/chat/completions", expected: InboundProtocolChat},
		{path: "/v1/responses", expected: InboundProtocolResponses},
		{path: "/responses", expected: InboundProtocolResponses},
		{path: "/v1/messages", expected: InboundProtocolAnthropic},
		{path: "/messages", expected: InboundProtocolAnthropic},
	}

	for _, tc := range tests {
		got, err := ResolveInboundProtocol(tc.path)
		if err != nil {
			t.Fatalf("ResolveInboundProtocol(%q) returned error: %v", tc.path, err)
		}
		if got != tc.expected {
			t.Fatalf("ResolveInboundProtocol(%q) = %q, want %q", tc.path, got, tc.expected)
		}
	}
}

func TestResolveDispatchMode(t *testing.T) {
	tests := []struct {
		inbound  InboundProtocol
		upstream models.UpstreamType
		expected DispatchMode
	}{
		{InboundProtocolChat, models.UpstreamTypeOpenAIChat, DispatchPassthrough},
		{InboundProtocolChat, models.UpstreamTypeOpenAIResponses, DispatchTransform},
		{InboundProtocolChat, models.UpstreamTypeAnthropicMessages, DispatchTransform},
		{InboundProtocolResponses, models.UpstreamTypeOpenAIChat, DispatchTransform},
		{InboundProtocolResponses, models.UpstreamTypeOpenAIResponses, DispatchPassthrough},
		{InboundProtocolResponses, models.UpstreamTypeAnthropicMessages, DispatchTransform},
		{InboundProtocolAnthropic, models.UpstreamTypeAnthropicMessages, DispatchPassthrough},
		{InboundProtocolAnthropic, models.UpstreamTypeOpenAIChat, DispatchTransform},
		{InboundProtocolAnthropic, models.UpstreamTypeOpenAIResponses, DispatchTransform},
	}

	for _, tc := range tests {
		if got := ResolveDispatchMode(tc.inbound, tc.upstream); got != tc.expected {
			t.Fatalf("ResolveDispatchMode(%q, %q) = %q, want %q", tc.inbound, tc.upstream, got, tc.expected)
		}
	}
}
