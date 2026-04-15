package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm-gateway/models"
)

func TestGetProviderStatsRefreshesRuntimeWhenCacheIsFresh(t *testing.T) {
	resetBackendRuntimeManagerForTests()
	InvalidateStatsCache()
	t.Cleanup(func() {
		resetBackendRuntimeManagerForTests()
		InvalidateStatsCache()
	})

	config := models.ModelConfig{
		ID:             1,
		Name:           "auto",
		ModelName:      "backend-a",
		APIBaseURL:     "https://example.test/v1",
		MaxConcurrency: 2,
	}
	globalStatsCacheMu.Lock()
	globalCachedStats = CachedStats{
		ProviderStats: []ModelStatsResponse{
			{
				ModelName:            "auto",
				BackendConfigID:      config.ID,
				ActiveRequests:       0,
				SuccessRate:          0.25,
				AdaptiveRoutingScore: 999,
			},
		},
		UpdatedAt: time.Now(),
	}
	globalStatsCacheMu.Unlock()

	lease, ok := getBackendRuntimeManager().startRequest(config, "auto")
	if !ok {
		t.Fatal("expected request lease to start")
	}

	stats := readProviderStats(t)
	if len(stats) != 1 {
		t.Fatalf("expected one provider stat, got %d", len(stats))
	}
	if stats[0].ActiveRequests != 1 {
		t.Fatalf("expected active_requests to be refreshed to 1, got %d", stats[0].ActiveRequests)
	}
	if stats[0].SuccessRate != 0.25 {
		t.Fatalf("expected cached success_rate to be preserved, got %f", stats[0].SuccessRate)
	}
	if stats[0].AdaptiveRoutingScore != 999 {
		t.Fatalf("expected cached adaptive_routing_score to be preserved, got %f", stats[0].AdaptiveRoutingScore)
	}

	lease.Finish(backendObservation{Success: true, ResponseTimeMS: 100})

	stats = readProviderStats(t)
	if len(stats) != 1 {
		t.Fatalf("expected one provider stat after finish, got %d", len(stats))
	}
	if stats[0].ActiveRequests != 0 {
		t.Fatalf("expected active_requests to be refreshed to 0, got %d", stats[0].ActiveRequests)
	}
	if stats[0].SuccessRate != 0.25 {
		t.Fatalf("expected cached success_rate to remain preserved after finish, got %f", stats[0].SuccessRate)
	}
	if stats[0].AdaptiveRoutingScore != 999 {
		t.Fatalf("expected cached adaptive_routing_score to remain preserved after finish, got %f", stats[0].AdaptiveRoutingScore)
	}
}

func readProviderStats(t *testing.T) []ModelStatsResponse {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/stats/providers", nil)
	(&StatsHandler{}).GetProviderStats(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var stats []ModelStatsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&stats); err != nil {
		t.Fatalf("decode provider stats: %v", err)
	}
	return stats
}
