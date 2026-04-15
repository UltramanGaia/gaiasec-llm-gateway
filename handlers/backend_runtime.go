package handlers

import (
	"math"
	"sort"
	"sync"
	"time"

	"llm-gateway/models"
)

const (
	routingEWMAAlpha             = 0.2
	defaultBackendLatencyScore   = 800.0
	defaultFirstTokenLatency     = 600.0
	activeRequestPenaltyPerConn  = 250.0
	failureRatePenaltyMultiplier = 1500.0
)

type backendObservation struct {
	Success           bool
	ResponseTimeMS    int64
	FirstTokenLatency int64
	AvgTokenLatency   float64
}

type backendRequestLease struct {
	manager               *backendRuntimeManager
	configID              uint
	publicModelName       string
	backendModelName      string
	backendAPIBaseURL     string
	activeRequestsOnStart int
	startTime             time.Time
	finished              bool
}

func (l *backendRequestLease) ActiveRequestsOnStart() int {
	if l == nil {
		return 0
	}
	return l.activeRequestsOnStart
}

func (l *backendRequestLease) Finish(observation backendObservation) {
	if l == nil || l.finished {
		return
	}
	l.finished = true
	l.manager.finishRequest(l, observation)
}

type backendRuntimeSnapshot struct {
	ConfigID             uint    `json:"config_id"`
	ModelName            string  `json:"model_name"`
	BackendModelName     string  `json:"backend_model_name"`
	BackendAPIBaseURL    string  `json:"backend_api_base_url"`
	ActiveRequests       int     `json:"active_requests"`
	TotalRequests        int64   `json:"total_requests"`
	SuccessCount         int64   `json:"success_count"`
	FailureCount         int64   `json:"failure_count"`
	SuccessRate          float64 `json:"success_rate"`
	EWMAResponseTime     float64 `json:"ewma_response_time"`
	EWMAFirstToken       float64 `json:"ewma_first_token_latency"`
	EWMAAvgTokenLatency  float64 `json:"ewma_avg_token_latency"`
	AdaptiveRoutingScore float64 `json:"adaptive_routing_score"`
	LastUpdatedAt        string  `json:"last_updated_at,omitempty"`
}

type backendRuntimeState struct {
	ConfigID            uint
	ModelName           string
	BackendModelName    string
	BackendAPIBaseURL   string
	ActiveRequests      int
	TotalRequests       int64
	SuccessCount        int64
	FailureCount        int64
	EWMAResponseTime    float64
	EWMAFirstToken      float64
	EWMAAvgTokenLatency float64
	LastUpdatedAt       time.Time
}

type rankedBackendConfig struct {
	config       models.ModelConfig
	score        float64
	hasTelemetry bool
}

type backendRuntimeManager struct {
	mu     sync.RWMutex
	states map[uint]*backendRuntimeState
}

var globalBackendRuntime = newBackendRuntimeManager()

func newBackendRuntimeManager() *backendRuntimeManager {
	return &backendRuntimeManager{
		states: make(map[uint]*backendRuntimeState),
	}
}

func getBackendRuntimeManager() *backendRuntimeManager {
	return globalBackendRuntime
}

func resetBackendRuntimeManagerForTests() {
	globalBackendRuntime = newBackendRuntimeManager()
}

func (m *backendRuntimeManager) resetConfigState(config models.ModelConfig, publicModelName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[config.ID]
	if state == nil {
		return false
	}

	state.ModelName = publicModelName
	state.BackendModelName = config.ModelName
	state.BackendAPIBaseURL = config.APIBaseURL
	state.TotalRequests = 0
	state.SuccessCount = 0
	state.FailureCount = 0
	state.EWMAResponseTime = 0
	state.EWMAFirstToken = 0
	state.EWMAAvgTokenLatency = 0
	state.LastUpdatedAt = time.Now()

	return true
}

func (m *backendRuntimeManager) resetAllState() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, state := range m.states {
		if state == nil {
			continue
		}
		state.TotalRequests = 0
		state.SuccessCount = 0
		state.FailureCount = 0
		state.EWMAResponseTime = 0
		state.EWMAFirstToken = 0
		state.EWMAAvgTokenLatency = 0
		state.LastUpdatedAt = time.Now()
		count++
	}

	return count
}

func (m *backendRuntimeManager) startRequest(config models.ModelConfig, publicModelName string) (*backendRequestLease, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.getOrCreateStateLocked(config, publicModelName)
	if config.MaxConcurrency > 0 && state.ActiveRequests >= config.MaxConcurrency {
		return nil, false
	}
	state.ActiveRequests++
	state.LastUpdatedAt = time.Now()

	return &backendRequestLease{
		manager:               m,
		configID:              config.ID,
		publicModelName:       publicModelName,
		backendModelName:      config.ModelName,
		backendAPIBaseURL:     config.APIBaseURL,
		activeRequestsOnStart: state.ActiveRequests,
		startTime:             time.Now(),
	}, true
}

func (m *backendRuntimeManager) finishRequest(lease *backendRequestLease, observation backendObservation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[lease.configID]
	if state == nil {
		state = &backendRuntimeState{
			ConfigID:          lease.configID,
			ModelName:         lease.publicModelName,
			BackendModelName:  lease.backendModelName,
			BackendAPIBaseURL: lease.backendAPIBaseURL,
		}
		m.states[lease.configID] = state
	}

	if state.ActiveRequests > 0 {
		state.ActiveRequests--
	}

	state.ModelName = lease.publicModelName
	state.BackendModelName = lease.backendModelName
	state.BackendAPIBaseURL = lease.backendAPIBaseURL
	state.TotalRequests++
	state.LastUpdatedAt = time.Now()

	if observation.Success {
		state.SuccessCount++
	} else {
		state.FailureCount++
	}

	if observation.ResponseTimeMS > 0 {
		state.EWMAResponseTime = ewma(state.EWMAResponseTime, float64(observation.ResponseTimeMS))
	}
	if observation.FirstTokenLatency > 0 {
		state.EWMAFirstToken = ewma(state.EWMAFirstToken, float64(observation.FirstTokenLatency))
	}
	if observation.AvgTokenLatency > 0 {
		state.EWMAAvgTokenLatency = ewma(state.EWMAAvgTokenLatency, observation.AvgTokenLatency)
	}
}

func (m *backendRuntimeManager) getOrCreateStateLocked(config models.ModelConfig, publicModelName string) *backendRuntimeState {
	state := m.states[config.ID]
	if state == nil {
		state = &backendRuntimeState{
			ConfigID:          config.ID,
			ModelName:         publicModelName,
			BackendModelName:  config.ModelName,
			BackendAPIBaseURL: config.APIBaseURL,
		}
		m.states[config.ID] = state
	}
	return state
}

func (m *backendRuntimeManager) scoreForConfig(config models.ModelConfig) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := m.states[config.ID]
	if state == nil {
		return defaultBackendLatencyScore
	}
	return computeAdaptiveScore(state)
}

func (m *backendRuntimeManager) buildAttemptOrder(modelName string, configs []models.ModelConfig) []models.ModelConfig {
	m.mu.RLock()
	ranked := make([]rankedBackendConfig, 0, len(configs))
	for _, config := range configs {
		state := m.states[config.ID]
		if config.MaxConcurrency > 0 && state != nil && state.ActiveRequests >= config.MaxConcurrency {
			continue
		}
		score := defaultBackendLatencyScore
		hasTelemetry := false
		if state != nil {
			score = computeAdaptiveScore(state)
			hasTelemetry = state.TotalRequests > 0 || state.ActiveRequests > 0
		}
		ranked = append(ranked, rankedBackendConfig{
			config:       config,
			score:        score,
			hasTelemetry: hasTelemetry,
		})
	}
	m.mu.RUnlock()

	if len(ranked) <= 1 {
		ordered := make([]models.ModelConfig, 0, len(ranked))
		for _, item := range ranked {
			ordered = append(ordered, item.config)
		}
		return ordered
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].config.Priority != ranked[j].config.Priority {
			return ranked[i].config.Priority < ranked[j].config.Priority
		}
		return ranked[i].config.ID < ranked[j].config.ID
	})

	ordered := make([]models.ModelConfig, 0, len(ranked))
	for start := 0; start < len(ranked); {
		end := start + 1
		for end < len(ranked) && ranked[end].config.Priority == ranked[start].config.Priority {
			end++
		}

		group := ranked[start:end]
		if !anyRankedConfigHasTelemetry(group) {
			available := make([]models.ModelConfig, 0, len(group))
			for _, item := range group {
				available = append(available, item.config)
			}
			ordered = append(ordered, buildRandomizedAttemptOrder(available)...)
			start = end
			continue
		}

		randOffset := nextRouteOffsetFn(len(group))
		sort.SliceStable(group, func(i, j int) bool {
			if math.Abs(group[i].score-group[j].score) > 0.001 {
				return group[i].score < group[j].score
			}

			leftTie := (i + len(group) - randOffset) % len(group)
			rightTie := (j + len(group) - randOffset) % len(group)
			return leftTie < rightTie
		})

		for _, item := range group {
			ordered = append(ordered, item.config)
		}
		start = end
	}

	return ordered
}

func anyRankedConfigHasTelemetry(configs []rankedBackendConfig) bool {
	for _, config := range configs {
		if config.hasTelemetry {
			return true
		}
	}
	return false
}

func buildRandomizedAttemptOrder(configs []models.ModelConfig) []models.ModelConfig {
	offset := nextRouteOffsetFn(len(configs))
	ordered := make([]models.ModelConfig, 0, len(configs))
	for i := 0; i < len(configs); i++ {
		ordered = append(ordered, configs[(offset+i)%len(configs)])
	}
	return ordered
}

func computeAdaptiveScore(state *backendRuntimeState) float64 {
	responseScore := state.EWMAResponseTime
	if responseScore <= 0 {
		responseScore = defaultBackendLatencyScore
	}

	firstTokenScore := state.EWMAFirstToken
	if firstTokenScore <= 0 {
		firstTokenScore = defaultFirstTokenLatency
	}

	avgTokenScore := state.EWMAAvgTokenLatency
	if avgTokenScore < 0 {
		avgTokenScore = 0
	}

	failureRate := 0.0
	if state.TotalRequests > 0 {
		failureRate = float64(state.FailureCount) / float64(state.TotalRequests)
	}

	return responseScore +
		(0.35 * firstTokenScore) +
		(0.15 * avgTokenScore) +
		(float64(state.ActiveRequests) * activeRequestPenaltyPerConn) +
		(failureRate * failureRatePenaltyMultiplier)
}

func ewma(previous, current float64) float64 {
	if previous <= 0 {
		return current
	}
	return (routingEWMAAlpha * current) + ((1 - routingEWMAAlpha) * previous)
}

func (m *backendRuntimeManager) snapshots() []backendRuntimeSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]backendRuntimeSnapshot, 0, len(m.states))
	for _, state := range m.states {
		successRate := 0.0
		if state.TotalRequests > 0 {
			successRate = float64(state.SuccessCount) / float64(state.TotalRequests)
		}

		snapshot := backendRuntimeSnapshot{
			ConfigID:             state.ConfigID,
			ModelName:            state.ModelName,
			BackendModelName:     state.BackendModelName,
			BackendAPIBaseURL:    state.BackendAPIBaseURL,
			ActiveRequests:       state.ActiveRequests,
			TotalRequests:        state.TotalRequests,
			SuccessCount:         state.SuccessCount,
			FailureCount:         state.FailureCount,
			SuccessRate:          successRate,
			EWMAResponseTime:     state.EWMAResponseTime,
			EWMAFirstToken:       state.EWMAFirstToken,
			EWMAAvgTokenLatency:  state.EWMAAvgTokenLatency,
			AdaptiveRoutingScore: computeAdaptiveScore(state),
		}
		if !state.LastUpdatedAt.IsZero() {
			snapshot.LastUpdatedAt = state.LastUpdatedAt.UTC().Format(time.RFC3339)
		}
		snapshots = append(snapshots, snapshot)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].AdaptiveRoutingScore == snapshots[j].AdaptiveRoutingScore {
			return snapshots[i].ConfigID < snapshots[j].ConfigID
		}
		return snapshots[i].AdaptiveRoutingScore < snapshots[j].AdaptiveRoutingScore
	})

	return snapshots
}
