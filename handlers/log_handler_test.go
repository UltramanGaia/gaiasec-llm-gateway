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

func TestBuildInferredTracesLinksByRequestPrefix(t *testing.T) {
	baseTime := time.Now()
	firstRequest := `{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"}]}`
	secondRequest := `{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"},{"role":"assistant","content":"ok"},{"role":"user","content":"continue"}]}`

	first, ok := buildInferredLogFeature(inferredTraceRow{
		ID:           1,
		CreatedAt:    baseTime,
		ModelName:    "auto",
		Request:      firstRequest,
		RequestBytes: len(firstRequest),
	})
	if !ok {
		t.Fatal("expected first request to produce features")
	}
	second, ok := buildInferredLogFeature(inferredTraceRow{
		ID:           2,
		CreatedAt:    baseTime.Add(time.Second),
		ModelName:    "auto",
		Request:      secondRequest,
		RequestBytes: len(secondRequest),
	})
	if !ok {
		t.Fatal("expected second request to produce features")
	}

	features := []inferredLogFeature{first, second}
	edges := inferRequestEdges(features, time.Minute)
	edge, ok := edges[2]
	if !ok {
		t.Fatal("expected second request to link to first")
	}
	if edge.ParentID != 1 || edge.Reason != "prefix:2" || edge.Confidence < 0.9 {
		t.Fatalf("unexpected edge: %+v", edge)
	}

	traces := buildInferredTraces(features, edges, 2, true)
	if len(traces) != 1 {
		t.Fatalf("expected one trace, got %d", len(traces))
	}
	if traces[0].StepCount != 2 || len(traces[0].Steps) != 2 {
		t.Fatalf("unexpected trace: %+v", traces[0])
	}
	if traces[0].Steps[1].ParentID != 1 {
		t.Fatalf("expected second step parent 1, got %d", traces[0].Steps[1].ParentID)
	}
}

func TestGetInferredTracesReturnsLinkedSteps(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	baseTime := time.Now()
	requests := []string{
		`{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"}]}`,
		`{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"},{"role":"assistant","content":"ok"},{"role":"user","content":"continue"}]}`,
	}
	created := make([]models.RequestLog, 0, len(requests))
	for index, body := range requests {
		entry := models.RequestLog{
			CreatedAt:        baseTime.Add(time.Duration(index) * time.Second),
			UpdatedAt:        baseTime.Add(time.Duration(index) * time.Second),
			ModelName:        "auto",
			BackendModelName: "backend",
			Request:          body,
			Response:         `{"ok":true}`,
			ResponseTime:     int64(100 + index),
		}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatalf("create request log: %v", err)
		}
		created = append(created, entry)
	}
	if err := db.Model(&models.RequestLog{}).Where("id = ?", created[1].ID).Updates(map[string]interface{}{
		"inferred_trace_key":    created[0].InferredTraceKey,
		"inferred_parent_id":    created[0].ID,
		"inferred_match_reason": "prefix:2",
		"inferred_confidence":   0.98,
	}).Error; err != nil {
		t.Fatalf("link request logs: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/request-logs/inferred-traces?include_steps=true", nil)
	NewLogHandler(db).GetInferredTraces(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response InferredTraceResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Traces) != 1 {
		t.Fatalf("expected one trace, total=%d len=%d", response.Total, len(response.Traces))
	}
	trace := response.Traces[0]
	if trace.StepCount != 2 || len(trace.Steps) != 2 {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	if trace.Steps[1].ParentID != trace.Steps[0].ID {
		t.Fatalf("expected linked steps, got parent=%d first=%d", trace.Steps[1].ParentID, trace.Steps[0].ID)
	}
}

func TestAsyncLogWriterMaterializesInferenceLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.RequestLog{}); err != nil {
		t.Fatalf("migrate request logs: %v", err)
	}

	writer := &AsyncLogWriter{db: db}
	first := &models.RequestLog{
		ModelName: "auto",
		Request:   `{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"}]}`,
		Response:  `{"ok":true}`,
	}
	second := &models.RequestLog{
		ModelName: "auto",
		Request:   `{"messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"start task"},{"role":"assistant","content":"ok"},{"role":"user","content":"continue"}]}`,
		Response:  `{"ok":true}`,
	}

	writer.writeBatch([]*models.RequestLog{first, second})

	var logs []models.RequestLog
	if err := db.Order("id ASC").Find(&logs).Error; err != nil {
		t.Fatalf("load logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected two logs, got %d", len(logs))
	}
	if logs[0].InferredTraceKey == "" || logs[0].InferredRequestKey == "" || logs[0].InferredPreview == "" {
		t.Fatalf("expected first log inference fields, got %+v", logs[0])
	}
	if logs[1].InferredParentID != logs[0].ID {
		t.Fatalf("expected second parent %d, got %d", logs[0].ID, logs[1].InferredParentID)
	}
	if logs[1].InferredTraceKey != logs[0].InferredTraceKey || logs[1].InferredMatchReason != "prefix:2" {
		t.Fatalf("unexpected second inference link: trace=%q reason=%q", logs[1].InferredTraceKey, logs[1].InferredMatchReason)
	}
}
