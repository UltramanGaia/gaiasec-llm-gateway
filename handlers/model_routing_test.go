package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/models"
)

func mustRawMessageMap(t *testing.T, payload interface{}) map[string]json.RawMessage {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal raw payload: %v", err)
	}

	return raw
}

func TestBuildProviderRequestBodyDoesNotMutateOriginalRequest(t *testing.T) {
	requestBody := mustRawMessageMap(t, map[string]interface{}{
		"model": "auto",
		"reasoning": map[string]interface{}{
			"effort": "high",
		},
	})

	body, err := buildProviderRequestBody(requestBody, models.ModelConfig{ModelName: "backend-a"})
	if err != nil {
		t.Fatalf("buildProviderRequestBody returned error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload["model"] != "backend-a" {
		t.Fatalf("expected backend model, got %v", payload["model"])
	}

	reasoning, ok := payload["reasoning"].(map[string]interface{})
	if !ok || reasoning["effort"] != "none" {
		t.Fatalf("expected reasoning.effort to be overwritten to none, got %#v", payload["reasoning"])
	}

	var originalModel string
	if err := json.Unmarshal(requestBody["model"], &originalModel); err != nil {
		t.Fatalf("failed to read original model: %v", err)
	}
	if originalModel != "auto" {
		t.Fatalf("original request model was mutated: %v", originalModel)
	}

	var originalReasoning map[string]interface{}
	if err := json.Unmarshal(requestBody["reasoning"], &originalReasoning); err != nil {
		t.Fatalf("failed to read original reasoning: %v", err)
	}
	if originalReasoning["effort"] != "high" {
		t.Fatalf("original reasoning payload was mutated: %#v", originalReasoning)
	}
}

func TestBuildAttemptOrderUsesRandomOffset(t *testing.T) {
	configs := []models.ModelConfig{
		{ID: 1, Name: "auto-random", ModelName: "backend-a"},
		{ID: 2, Name: "auto-random", ModelName: "backend-b"},
		{ID: 3, Name: "auto-random", ModelName: "backend-c"},
	}

	originalFn := nextRouteOffsetFn
	defer func() {
		nextRouteOffsetFn = originalFn
	}()

	nextRouteOffsetFn = func(count int) int {
		if count != 3 {
			t.Fatalf("unexpected config count: %d", count)
		}
		return 2
	}

	order := buildAttemptOrder(configs)

	if len(order) != 3 {
		t.Fatalf("unexpected order length: %d", len(order))
	}
	if order[0].ID != 3 || order[1].ID != 1 || order[2].ID != 2 {
		t.Fatalf("unexpected random-offset order: %#v", order)
	}
}

func TestDispatchProviderRequestFailsOverToNextBackend(t *testing.T) {
	originalFn := nextRouteOffsetFn
	defer func() {
		nextRouteOffsetFn = originalFn
	}()
	nextRouteOffsetFn = func(count int) int {
		if count != 2 {
			t.Fatalf("unexpected config count: %d", count)
		}
		return 0
	}

	var firstRequestModel string
	firstProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		firstRequestModel, _ = payload["model"].(string)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"backend-a unavailable"}`))
	}))
	defer firstProvider.Close()

	var secondRequestModel string
	secondProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key-b" {
			t.Fatalf("expected backend authorization header, got %q", got)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		secondRequestModel, _ = payload["model"].(string)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp-1","object":"chat.completion"}`))
	}))
	defer secondProvider.Close()

	handler := &ChatHandler{}
	configs := []models.ModelConfig{
		{ID: 1, Name: "auto-failover", ModelName: "backend-a", APIBaseURL: firstProvider.URL, APIKey: "key-a"},
		{ID: 2, Name: "auto-failover", ModelName: "backend-b", APIBaseURL: secondProvider.URL, APIKey: "key-b"},
	}

	requestBody := mustRawMessageMap(t, map[string]interface{}{
		"model": "auto-failover",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
	})

	resp, selectedConfig, attempts, err := handler.dispatchProviderRequest(http.Header{"X-Test": []string{"1"}}, requestBody, "auto-failover", configs, false)
	if err != nil {
		t.Fatalf("dispatchProviderRequest returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful failover response, got %d", resp.StatusCode)
	}
	if selectedConfig.ID != 2 {
		t.Fatalf("expected second backend to be selected, got %d", selectedConfig.ID)
	}
	if len(attempts) != 2 || attempts[0].StatusCode != http.StatusServiceUnavailable || attempts[1].StatusCode != http.StatusOK {
		t.Fatalf("unexpected attempts: %#v", attempts)
	}
	if firstRequestModel != "backend-a" {
		t.Fatalf("expected first backend request model to be rewritten, got %q", firstRequestModel)
	}
	if secondRequestModel != "backend-b" {
		t.Fatalf("expected second backend request model to be rewritten, got %q", secondRequestModel)
	}
	var originalModel string
	if err := json.Unmarshal(requestBody["model"], &originalModel); err != nil {
		t.Fatalf("failed to read original model: %v", err)
	}
	if originalModel != "auto-failover" {
		t.Fatalf("original request model was mutated: %v", originalModel)
	}
}
