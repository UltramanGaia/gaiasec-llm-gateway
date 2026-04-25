package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"llm-gateway/models"
)

func newModelConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelConfig{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	return db
}

func decodeObject(t *testing.T, recorder *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body %q: %v", recorder.Body.String(), err)
	}
	return payload
}

func TestGetModelConfigsDoesNotExposeAPIKey(t *testing.T) {
	db := newModelConfigTestDB(t)
	if err := db.Create(&models.ModelConfig{
		Name:       "openai",
		ModelName:  "gpt-test",
		APIBaseURL: "https://api.example.test",
		APIKey:     "secret-key",
		Enabled:    true,
	}).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	handler := NewModelConfigHandler(db)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/model-configs", nil)

	handler.GetModelConfigs(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-key") {
		t.Fatalf("response leaked api key: %s", recorder.Body.String())
	}

	var payload []map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body %q: %v", recorder.Body.String(), err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected one model config, got %d", len(payload))
	}
	if _, ok := payload[0]["api_key"]; ok {
		t.Fatalf("response must not contain api_key: %s", recorder.Body.String())
	}
	var apiKeySet bool
	if err := json.Unmarshal(payload[0]["api_key_set"], &apiKeySet); err != nil {
		t.Fatalf("expected api_key_set flag in response: %v", err)
	}
	if !apiKeySet {
		t.Fatalf("expected api_key_set to be true")
	}
}

func TestModifyModelConfigKeepsExistingAPIKeyWhenBlank(t *testing.T) {
	db := newModelConfigTestDB(t)
	config := models.ModelConfig{
		Name:       "openai",
		ModelName:  "gpt-test",
		APIBaseURL: "https://api.example.test",
		APIKey:     "secret-key",
		Enabled:    true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	body := []byte(`{
		"name": "openai-updated",
		"model_name": "gpt-test-2",
		"api_base_url": "https://api2.example.test",
		"api_key": "",
		"max_tokens": 4096,
		"priority": 2,
		"max_concurrency": 5,
		"temperature": 0.3,
		"description": "updated",
		"enabled": true
	}`)
	request := httptest.NewRequest(http.MethodPut, "/api/model-configs/1", bytes.NewReader(body))
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()

	NewModelConfigHandler(db).ModifyModelConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-key") {
		t.Fatalf("response leaked api key: %s", recorder.Body.String())
	}
	payload := decodeObject(t, recorder)
	if _, ok := payload["api_key"]; ok {
		t.Fatalf("response must not contain api_key: %s", recorder.Body.String())
	}

	var stored models.ModelConfig
	if err := db.First(&stored, config.ID).Error; err != nil {
		t.Fatalf("failed to reload model config: %v", err)
	}
	if stored.APIKey != "secret-key" {
		t.Fatalf("expected existing api key to be preserved, got %q", stored.APIKey)
	}
	if stored.Name != "openai-updated" {
		t.Fatalf("expected other fields to be updated, got name %q", stored.Name)
	}
}

func TestTestModelConfigReturnsResultInDataEnvelope(t *testing.T) {
	db := newModelConfigTestDB(t)
	var gotAuth string
	var gotModel string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions path, got %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read provider request: %v", err)
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode provider request %q: %v", string(body), err)
		}
		gotModel = payload.Model
		if len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.Messages[0].Content == "" {
			t.Fatalf("unexpected provider messages: %+v", payload.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:       "openai",
		ModelName:  "gpt-test",
		APIBaseURL: provider.URL,
		APIKey:     "secret-key",
		Enabled:    true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/model-configs/1/test", nil)
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()

	NewModelConfigHandler(db).TestModelConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeObject(t, recorder)
	var ok bool
	if err := json.Unmarshal(payload["success"], &ok); err != nil {
		t.Fatalf("expected top-level success flag: %v", err)
	}
	if !ok {
		t.Fatalf("expected top-level success to describe request handling")
	}
	var data struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Input   map[string]any `json:"input"`
		Output  map[string]any `json:"output"`
	}
	if err := json.Unmarshal(payload["data"], &data); err != nil {
		t.Fatalf("expected test result in data envelope: %v", err)
	}
	if !data.Success || data.Message != "连接测试成功" {
		t.Fatalf("unexpected test result: %+v", data)
	}
	if data.Input["url"] != provider.URL+"/chat/completions" {
		t.Fatalf("expected input url to describe provider call, got %+v", data.Input["url"])
	}
	headers, ok := data.Input["headers"].(map[string]any)
	if !ok || headers["Authorization"] != "Bearer ***" {
		t.Fatalf("expected masked authorization in input headers, got %+v", data.Input["headers"])
	}
	if data.Output["status_code"] != float64(http.StatusOK) {
		t.Fatalf("expected output status code 200, got %+v", data.Output["status_code"])
	}
	if gotAuth != "Bearer secret-key" {
		t.Fatalf("expected provider authorization header, got %q", gotAuth)
	}
	if gotModel != "gpt-test" {
		t.Fatalf("expected provider model gpt-test, got %q", gotModel)
	}
}

func TestTestModelConfigCanReportConnectionFailureWithoutRejectingRequest(t *testing.T) {
	db := newModelConfigTestDB(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid api key", http.StatusUnauthorized)
	}))
	defer provider.Close()

	config := models.ModelConfig{
		Name:       "openai",
		ModelName:  "gpt-test",
		APIBaseURL: provider.URL,
		Enabled:    true,
	}
	if err := db.Create(&config).Error; err != nil {
		t.Fatalf("failed to seed model config: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/model-configs/1/test", nil)
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()

	NewModelConfigHandler(db).TestModelConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeObject(t, recorder)
	var requestSuccess bool
	if err := json.Unmarshal(payload["success"], &requestSuccess); err != nil {
		t.Fatalf("expected top-level success flag: %v", err)
	}
	if !requestSuccess {
		t.Fatalf("connection failure should not be encoded as request failure")
	}
	var data struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload["data"], &data); err != nil {
		t.Fatalf("expected test result in data envelope: %v", err)
	}
	if data.Success || !strings.Contains(data.Message, "upstream returned 401") {
		t.Fatalf("unexpected test result: %+v", data)
	}
}
