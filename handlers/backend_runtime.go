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
	ConfigID             uint    `json:"configId"`
	ModelName            string  `json:"modelName"`
	BackendModelName     string  `json:"backendModelName"`
	BackendAPIBaseURL    string  `json:"backendApiBaseUrl"`
	ActiveRequests       int     `json:"activeRequests"`
	TotalRequests        int64   `json:"totalRequests"`
	SuccessCount         int64   `json:"successCount"`
	FailureCount         int64   `json:"failureCount"`
	SuccessRate          float64 `json:"successRate"`
	EWMAResponseTime     float64 `json:"ewmaResponseTime"`
	EWMAFirstToken       float64 `json:"ewmaFirstTokenLatency"`
	EWMAAvgTokenLatency  float64 `json:"ewmaAvgTokenLatency"`
	AdaptiveRoutingScore float64 `json:"adaptiveRoutingScore"`
	LastUpdatedAt        string  `json:"lastUpdatedAt,omitempty"`
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

func (m *backendRuntimeManager) startRequest(config models.ModelConfig, publicModelName string) *backendRequestLease {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.getOrCreateStateLocked(config, publicModelName)
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
	}
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
	if len(configs) <= 1 {
		return configs
	}

	m.mu.RLock()
	ranked := make([]rankedBackendConfig, 0, len(configs))
	for _, config := range configs {
		state := m.states[config.ID]
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

	if !anyRankedConfigHasTelemetry(ranked) {
		return buildRandomizedAttemptOrder(configs)
	}

	randOffset := nextRouteOffsetFn(len(configs))
	sort.SliceStable(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].score-ranked[j].score) > 0.001 {
			return ranked[i].score < ranked[j].score
		}

		leftTie := (int(ranked[i].config.ID) + len(configs) - randOffset) % len(configs)
		rightTie := (int(ranked[j].config.ID) + len(configs) - randOffset) % len(configs)
		return leftTie < rightTie
	})

	ordered := make([]models.ModelConfig, 0, len(ranked))
	for _, item := range ranked {
		ordered = append(ordered, item.config)
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
