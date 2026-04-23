package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"llm-gateway/models"

	"gorm.io/gorm"
)

type StatsHandler struct {
	DB *gorm.DB
}

func NewStatsHandler(db *gorm.DB) *StatsHandler {
	h := &StatsHandler{
		DB: db,
	}
	go h.startStatsCacheRefresh()
	return h
}

type StatsResponse struct {
	TotalRequests        int64   `json:"total_requests"`
	ActiveModels         int64   `json:"active_models"`
	ModelMappings        int64   `json:"model_mappings"`
	AvgResponseTime      float64 `json:"avg_response_time"`
	AvgFirstTokenLatency float64 `json:"avg_first_token_latency"`
	AvgTokenLatency      float64 `json:"avg_token_latency"`
	ActiveRequests       int     `json:"active_requests"`
}

type ModelStatsResponse struct {
	ModelName            string  `json:"model_name"`
	RequestCount         int64   `json:"request_count"`
	AvgResponseTime      float64 `json:"avg_response_time"`
	AvgFirstTokenLatency float64 `json:"avg_first_token_latency,omitempty"`
	AvgTokenLatency      float64 `json:"avg_token_latency,omitempty"`
	ActiveRequests       int     `json:"active_requests,omitempty"`
	SuccessRate          float64 `json:"success_rate,omitempty"`
	BackendConfigID      uint    `json:"backend_config_id,omitempty"`
	BackendModelName     string  `json:"backend_model_name,omitempty"`
	BackendAPIBaseURL    string  `json:"backend_api_base_url,omitempty"`
	AdaptiveRoutingScore float64 `json:"adaptive_routing_score,omitempty"`
}

type CachedStats struct {
	Stats         StatsResponse
	ProviderStats []ModelStatsResponse
	ModelStats    []ModelStatsResponse
	UpdatedAt     time.Time
}

var (
	globalCachedStats  CachedStats
	globalStatsCacheMu sync.RWMutex
	statsCacheTTL      = 5 * time.Minute
	statsTimeRange     = 1 * time.Hour // 统计时间范围：最近1小时
)

func InvalidateStatsCache() {
	globalStatsCacheMu.Lock()
	defer globalStatsCacheMu.Unlock()

	globalCachedStats = CachedStats{}
}

func (h *StatsHandler) startStatsCacheRefresh() {
	ticker := time.NewTicker(statsCacheTTL)
	defer ticker.Stop()
	for range ticker.C {
		h.refreshStatsCache()
	}
}

func (h *StatsHandler) refreshStatsCache() {
	// 计算时间范围
	since := time.Now().Add(-statsTimeRange)

	var totalRequests int64
	h.DB.Model(&models.RequestLog{}).
		Where("created_at >= ?", since).
		Count(&totalRequests)

	var activeModels int64
	h.DB.Model(&models.ModelConfig{}).Where("enabled = ?", true).Count(&activeModels)

	var modelMappings int64
	h.DB.Model(&models.ModelConfig{}).Count(&modelMappings)

	var avgResponseTime float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(response_time), 0)").
		Where("created_at >= ?", since).
		Scan(&avgResponseTime)

	var avgFirstTokenLatency float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(first_token_latency), 0)").
		Where("created_at >= ? AND first_token_latency > 0", since).
		Scan(&avgFirstTokenLatency)

	var avgTokenLatency float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(avg_token_latency), 0)").
		Where("created_at >= ? AND avg_token_latency > 0", since).
		Scan(&avgTokenLatency)

	if avgResponseTime == 0 {
		avgResponseTime = 200
	}

	stats := StatsResponse{
		TotalRequests:        totalRequests,
		ActiveModels:         activeModels,
		ModelMappings:        modelMappings,
		AvgResponseTime:      avgResponseTime,
		AvgFirstTokenLatency: avgFirstTokenLatency,
		AvgTokenLatency:      avgTokenLatency,
		ActiveRequests:       totalActiveRequests(),
	}

	providerStats := h.getProviderStatsFromDB()
	modelStats := h.getModelStatsFromDB()

	globalStatsCacheMu.Lock()
	globalCachedStats = CachedStats{
		Stats:         stats,
		ProviderStats: providerStats,
		ModelStats:    modelStats,
		UpdatedAt:     time.Now(),
	}
	globalStatsCacheMu.Unlock()
}

func (h *StatsHandler) getProviderStatsFromDB() []ModelStatsResponse {
	var providerStats []ModelStatsResponse

	// 计算时间范围
	since := time.Now().Add(-statsTimeRange)

	query := `
		SELECT
			rl.model_name,
			rl.backend_config_id,
			rl.backend_model_name,
			rl.backend_api_base_url,
			COUNT(*) as request_count,
			COALESCE(AVG(rl.response_time), 0) as avg_response_time,
			COALESCE(AVG(NULLIF(rl.first_token_latency, 0)), 0) as avg_first_token_latency,
			COALESCE(AVG(NULLIF(rl.avg_token_latency, 0)), 0) as avg_token_latency
		FROM request_logs rl
		WHERE rl.backend_config_id > 0
			AND rl.created_at >= ?
		GROUP BY rl.model_name, rl.backend_config_id, rl.backend_model_name, rl.backend_api_base_url
		ORDER BY request_count DESC
		LIMIT 100
	`

	rows, err := h.DB.Raw(query, since).Rows()
	if err != nil {
		return getDefaultProviderStats()
	}
	defer rows.Close()

	for rows.Next() {
		var stat ModelStatsResponse
		if err := rows.Scan(
			&stat.ModelName,
			&stat.BackendConfigID,
			&stat.BackendModelName,
			&stat.BackendAPIBaseURL,
			&stat.RequestCount,
			&stat.AvgResponseTime,
			&stat.AvgFirstTokenLatency,
			&stat.AvgTokenLatency,
		); err != nil {
			continue
		}
		enrichProviderStatWithRuntime(&stat)
		providerStats = append(providerStats, stat)
	}

	providerStats = mergeRuntimeProviderStats(providerStats)

	if len(providerStats) == 0 {
		return getDefaultProviderStats()
	}

	return providerStats
}

func (h *StatsHandler) getModelStatsFromDB() []ModelStatsResponse {
	var modelStats []ModelStatsResponse

	// 计算时间范围
	since := time.Now().Add(-statsTimeRange)

	query := `
		SELECT
			rl.model_name,
			COUNT(*) as request_count,
			COALESCE(AVG(rl.response_time), 0) as avg_response_time,
			COALESCE(AVG(NULLIF(rl.first_token_latency, 0)), 0) as avg_first_token_latency,
			COALESCE(AVG(NULLIF(rl.avg_token_latency, 0)), 0) as avg_token_latency
		FROM request_logs rl
		WHERE rl.created_at >= ?
		GROUP BY rl.model_name
		ORDER BY request_count DESC
		LIMIT 100
	`

	rows, err := h.DB.Raw(query, since).Rows()
	if err != nil {
		return getDefaultModelStats()
	}
	defer rows.Close()

	for rows.Next() {
		var stat ModelStatsResponse
		if err := rows.Scan(&stat.ModelName, &stat.RequestCount, &stat.AvgResponseTime, &stat.AvgFirstTokenLatency, &stat.AvgTokenLatency); err != nil {
			continue
		}
		modelStats = append(modelStats, stat)
	}

	if len(modelStats) == 0 {
		return getDefaultModelStats()
	}

	return modelStats
}

func getDefaultProviderStats() []ModelStatsResponse {
	return []ModelStatsResponse{}
}

func getDefaultModelStats() []ModelStatsResponse {
	return []ModelStatsResponse{}
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	globalStatsCacheMu.RLock()
	if time.Since(globalCachedStats.UpdatedAt) < statsCacheTTL {
		stats := globalCachedStats.Stats
		globalStatsCacheMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}
	globalStatsCacheMu.RUnlock()

	// 计算时间范围
	since := time.Now().Add(-statsTimeRange)

	var totalRequests int64
	h.DB.Model(&models.RequestLog{}).
		Where("created_at >= ?", since).
		Count(&totalRequests)

	var activeModels int64
	h.DB.Model(&models.ModelConfig{}).Where("enabled = ?", true).Count(&activeModels)

	var modelMappings int64
	h.DB.Model(&models.ModelConfig{}).Count(&modelMappings)

	var avgResponseTime float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(response_time), 0)").
		Where("created_at >= ?", since).
		Scan(&avgResponseTime)

	var avgFirstTokenLatency float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(first_token_latency), 0)").
		Where("created_at >= ? AND first_token_latency > 0", since).
		Scan(&avgFirstTokenLatency)

	var avgTokenLatency float64
	h.DB.Model(&models.RequestLog{}).
		Select("COALESCE(avg(avg_token_latency), 0)").
		Where("created_at >= ? AND avg_token_latency > 0", since).
		Scan(&avgTokenLatency)

	if avgResponseTime == 0 {
		avgResponseTime = 200
	}

	stats := StatsResponse{
		TotalRequests:        totalRequests,
		ActiveModels:         activeModels,
		ModelMappings:        modelMappings,
		AvgResponseTime:      avgResponseTime,
		AvgFirstTokenLatency: avgFirstTokenLatency,
		AvgTokenLatency:      avgTokenLatency,
		ActiveRequests:       totalActiveRequests(),
	}

	globalStatsCacheMu.Lock()
	globalCachedStats.Stats = stats
	globalCachedStats.UpdatedAt = time.Now()
	globalStatsCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *StatsHandler) GetProviderStats(w http.ResponseWriter, r *http.Request) {
	globalStatsCacheMu.RLock()
	if time.Since(globalCachedStats.UpdatedAt) < statsCacheTTL && len(globalCachedStats.ProviderStats) > 0 {
		providerStats := append([]ModelStatsResponse(nil), globalCachedStats.ProviderStats...)
		globalStatsCacheMu.RUnlock()
		providerStats = refreshProviderStatsRuntime(providerStats)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(providerStats)
		return
	}
	globalStatsCacheMu.RUnlock()

	providerStats := h.getProviderStatsFromDB()

	globalStatsCacheMu.Lock()
	globalCachedStats.ProviderStats = providerStats
	globalCachedStats.UpdatedAt = time.Now()
	globalStatsCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providerStats)
}

func (h *StatsHandler) GetModelStats(w http.ResponseWriter, r *http.Request) {
	globalStatsCacheMu.RLock()
	if time.Since(globalCachedStats.UpdatedAt) < statsCacheTTL && len(globalCachedStats.ModelStats) > 0 {
		modelStats := globalCachedStats.ModelStats
		globalStatsCacheMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(modelStats)
		return
	}
	globalStatsCacheMu.RUnlock()

	modelStats := h.getModelStatsFromDB()

	globalStatsCacheMu.Lock()
	globalCachedStats.ModelStats = modelStats
	globalCachedStats.UpdatedAt = time.Now()
	globalStatsCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelStats)
}

func enrichProviderStatWithRuntime(stat *ModelStatsResponse) {
	if stat == nil || stat.BackendConfigID == 0 {
		return
	}

	for _, snapshot := range getBackendRuntimeManager().snapshots() {
		if snapshot.ConfigID != stat.BackendConfigID {
			continue
		}
		stat.ActiveRequests = snapshot.ActiveRequests
		stat.SuccessRate = snapshot.SuccessRate
		stat.AdaptiveRoutingScore = snapshot.AdaptiveRoutingScore
		if stat.BackendModelName == "" {
			stat.BackendModelName = snapshot.BackendModelName
		}
		if stat.BackendAPIBaseURL == "" {
			stat.BackendAPIBaseURL = snapshot.BackendAPIBaseURL
		}
		return
	}
}

func refreshProviderStatsRuntime(stats []ModelStatsResponse) []ModelStatsResponse {
	runtimeSnapshots := getBackendRuntimeManager().snapshots()
	for i := range stats {
		for _, snapshot := range runtimeSnapshots {
			if stats[i].BackendConfigID != snapshot.ConfigID {
				continue
			}
			stats[i].ActiveRequests = snapshot.ActiveRequests
			break
		}
	}
	return mergeRuntimeProviderStats(stats)
}

func mergeRuntimeProviderStats(stats []ModelStatsResponse) []ModelStatsResponse {
	seen := make(map[uint]struct{}, len(stats))
	for _, stat := range stats {
		if stat.BackendConfigID != 0 {
			seen[stat.BackendConfigID] = struct{}{}
		}
	}

	for _, snapshot := range getBackendRuntimeManager().snapshots() {
		if _, ok := seen[snapshot.ConfigID]; ok {
			continue
		}
		stats = append(stats, ModelStatsResponse{
			ModelName:            snapshot.ModelName,
			RequestCount:         snapshot.TotalRequests,
			AvgResponseTime:      snapshot.EWMAResponseTime,
			AvgFirstTokenLatency: snapshot.EWMAFirstToken,
			AvgTokenLatency:      snapshot.EWMAAvgTokenLatency,
			ActiveRequests:       snapshot.ActiveRequests,
			SuccessRate:          snapshot.SuccessRate,
			BackendConfigID:      snapshot.ConfigID,
			BackendModelName:     snapshot.BackendModelName,
			BackendAPIBaseURL:    snapshot.BackendAPIBaseURL,
			AdaptiveRoutingScore: snapshot.AdaptiveRoutingScore,
		})
	}

	return stats
}

func totalActiveRequests() int {
	total := 0
	for _, snapshot := range getBackendRuntimeManager().snapshots() {
		total += snapshot.ActiveRequests
	}
	return total
}

func handleError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
