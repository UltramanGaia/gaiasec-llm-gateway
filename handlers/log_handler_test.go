package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"llm-gateway/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLogQueryValueSupportsSnakeCase(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/request-logs?backend_config_id=7", nil)

	value := queryValue(request, "backend_config_id")

	if value != "7" {
		t.Fatalf("expected snake_case value, got %q", value)
	}
}

func TestGetLogsReturnsSummaryWithoutLargeBodies(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	requestBody := `{"messages":[{"role":"user","content":"` + strings.Repeat("hello ", 50) + `"}]}`
	if err := db.Create(&models.RequestLog{
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		ModelName:         "gpt-test",
		BackendConfigID:   7,
		BackendModelName:  "backend-test",
		BackendAPIBaseURL: "http://provider",
		Request:           requestBody,
		Response:          strings.Repeat("response", 1000),
		StreamResponse:    []byte(strings.Repeat("stream", 1000)),
		ResponseTime:      123,
	}).Error; err != nil {
		t.Fatalf("create request log: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/request-logs?page_size=500", nil)
	NewLogHandler(db).GetLogs(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response LogResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Logs) != 1 {
		t.Fatalf("expected one log, total=%d len=%d", response.Total, len(response.Logs))
	}

	log := response.Logs[0]
	if log.RequestPreview == "" || len(log.RequestPreview) > 123 {
		t.Fatalf("unexpected request preview %q", log.RequestPreview)
	}
	if log.ResponseBytes == 0 || log.StreamBytes == 0 {
		t.Fatalf("expected body sizes, got response=%d stream=%d", log.ResponseBytes, log.StreamBytes)
	}
}
