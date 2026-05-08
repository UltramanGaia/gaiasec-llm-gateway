package handlers

import (
	"strconv"
	"sync"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	asyncLogQueueSize     = 10000
	asyncLogBatchSize     = 100
	asyncLogFlushInterval = 2 * time.Second
)

type LogWriteRequest struct {
	Log *models.RequestLog
}

type AsyncLogWriter struct {
	db       *gorm.DB
	logChan  chan *LogWriteRequest
	stopChan chan struct{}
	wg       sync.WaitGroup
}

var (
	asyncLogWriter     *AsyncLogWriter
	asyncLogWriterOnce sync.Once
)

func GetAsyncLogWriter(db *gorm.DB) *AsyncLogWriter {
	asyncLogWriterOnce.Do(func() {
		asyncLogWriter = &AsyncLogWriter{
			db:       db,
			logChan:  make(chan *LogWriteRequest, asyncLogQueueSize),
			stopChan: make(chan struct{}),
		}
		asyncLogWriter.start()
	})
	return asyncLogWriter
}

func (w *AsyncLogWriter) start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		batch := make([]*models.RequestLog, 0, asyncLogBatchSize)
		timer := time.NewTimer(asyncLogFlushInterval)
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerRunning := false

		flush := func() {
			if len(batch) == 0 {
				return
			}
			w.writeBatch(batch)
			clear(batch)
			batch = batch[:0]
		}

		for {
			select {
			case req := <-w.logChan:
				if req != nil && req.Log != nil {
					batch = append(batch, req.Log)
					if len(batch) >= asyncLogBatchSize {
						if timerRunning {
							if !timer.Stop() {
								select {
								case <-timer.C:
								default:
								}
							}
							timerRunning = false
						}
						flush()
					} else if !timerRunning {
						timer.Reset(asyncLogFlushInterval)
						timerRunning = true
					}
				}
			case <-timer.C:
				timerRunning = false
				flush()
			case <-w.stopChan:
				if timerRunning {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
				}
				for {
					select {
					case req := <-w.logChan:
						if req != nil && req.Log != nil {
							batch = append(batch, req.Log)
						}
					default:
						flush()
						return
					}
				}
			}
		}
	}()
}

func (w *AsyncLogWriter) writeBatch(logs []*models.RequestLog) {
	if len(logs) == 0 {
		return
	}

	for _, reqLog := range logs {
		if reqLog != nil {
			reqLog.PrepareInferenceMetadata()
		}
	}

	err := w.db.Create(&logs).Error
	if err == nil {
		w.resolveInferenceLinks(logs)
		return
	}

	log.WithError(err).WithField("batch_size", len(logs)).Warn("Batch log write failed, falling back to single inserts")

	for _, reqLog := range logs {
		if reqLog == nil {
			continue
		}
		if singleErr := w.db.Create(reqLog).Error; singleErr != nil {
			log.WithError(singleErr).Error("Single log write failed")
			continue
		}
		w.resolveInferenceLinks([]*models.RequestLog{reqLog})
	}
}

type inferenceCandidate struct {
	ID                   uint
	CreatedAt            time.Time
	InferredTraceKey     string
	InferredRequestKey   string
	InferredRootKey      string
	InferredMessageCount int
	RequestBytes         int
}

type resolvedInferenceLink struct {
	ParentID   uint
	TraceKey   string
	Reason     string
	Confidence float64
}

func (w *AsyncLogWriter) resolveInferenceLinks(logs []*models.RequestLog) {
	candidatesByRequestKey := make(map[string]inferenceCandidate)
	recentByRoot := make(map[string][]inferenceCandidate)

	var persisted []inferenceCandidate
	if err := w.db.
		Model(&models.RequestLog{}).
		Select(`
			id,
			created_at,
			inferred_trace_key,
			inferred_request_key,
			inferred_root_key,
			inferred_message_count,
			COALESCE(NULLIF(request_bytes, 0), LENGTH(request), 0) AS request_bytes
		`).
		Where("inferred_request_key <> ''").
		Order("created_at ASC, id ASC").
		Limit(3000).
		Scan(&persisted).Error; err != nil {
		log.WithError(err).Warn("Failed to load inference candidates for request log links")
		return
	}

	for _, candidate := range persisted {
		if candidate.InferredRequestKey != "" {
			candidatesByRequestKey[candidate.InferredRequestKey] = candidate
		}
		if candidate.InferredRootKey != "" {
			recentByRoot[candidate.InferredRootKey] = append(recentByRoot[candidate.InferredRootKey], candidate)
		}
	}

	for _, reqLog := range logs {
		if reqLog == nil || reqLog.ID == 0 || reqLog.InferredRequestKey == "" {
			continue
		}

		metadata, ok := models.BuildRequestTraceMetadata(reqLog.Request)
		if !ok {
			continue
		}

		link := findInferenceLink(reqLog, metadata, candidatesByRequestKey, recentByRoot)
		if link.ParentID != 0 {
			if err := w.db.Model(&models.RequestLog{}).
				Where("id = ?", reqLog.ID).
				Updates(map[string]interface{}{
					"inferred_parent_id":    link.ParentID,
					"inferred_trace_key":    link.TraceKey,
					"inferred_match_reason": link.Reason,
					"inferred_confidence":   link.Confidence,
				}).Error; err != nil {
				log.WithError(err).WithField("log_id", reqLog.ID).Warn("Failed to update inferred request log link")
			} else {
				reqLog.InferredParentID = link.ParentID
				reqLog.InferredTraceKey = link.TraceKey
				reqLog.InferredMatchReason = link.Reason
				reqLog.InferredConfidence = link.Confidence
			}
		}

		candidate := inferenceCandidate{
			ID:                   reqLog.ID,
			CreatedAt:            reqLog.CreatedAt,
			InferredTraceKey:     reqLog.InferredTraceKey,
			InferredRequestKey:   reqLog.InferredRequestKey,
			InferredRootKey:      reqLog.InferredRootKey,
			InferredMessageCount: reqLog.InferredMessageCount,
			RequestBytes:         reqLog.RequestBytes,
		}
		candidatesByRequestKey[candidate.InferredRequestKey] = candidate
		if candidate.InferredRootKey != "" {
			recentByRoot[candidate.InferredRootKey] = append(recentByRoot[candidate.InferredRootKey], candidate)
		}
	}
}

func findInferenceLink(reqLog *models.RequestLog, metadata models.RequestTraceMetadata, byRequestKey map[string]inferenceCandidate, recentByRoot map[string][]inferenceCandidate) resolvedInferenceLink {
	var best inferenceCandidate
	bestPrefixLen := -1
	for _, prefix := range metadata.PrefixKeys {
		candidate, ok := byRequestKey[prefix.Key]
		if !ok || !isEarlierInferenceCandidate(candidate, reqLog) {
			continue
		}
		if prefix.Length > bestPrefixLen {
			best = candidate
			bestPrefixLen = prefix.Length
		}
	}

	if best.ID != 0 {
		return resolvedInferenceLink{
			ParentID:   best.ID,
			TraceKey:   best.InferredTraceKey,
			Reason:     "prefix:" + strconv.Itoa(bestPrefixLen),
			Confidence: 0.98,
		}
	}

	const window = 20 * time.Minute
	for i := len(recentByRoot[metadata.RootKey]) - 1; i >= 0; i-- {
		candidate := recentByRoot[metadata.RootKey][i]
		if !isEarlierInferenceCandidate(candidate, reqLog) || reqLog.CreatedAt.Sub(candidate.CreatedAt) > window {
			continue
		}
		grew := candidate.RequestBytes < reqLog.RequestBytes || candidate.InferredMessageCount < metadata.MessageCount
		if !grew {
			continue
		}
		return resolvedInferenceLink{
			ParentID:   candidate.ID,
			TraceKey:   candidate.InferredTraceKey,
			Reason:     "root+time+len",
			Confidence: 0.72,
		}
	}

	return resolvedInferenceLink{}
}

func isEarlierInferenceCandidate(candidate inferenceCandidate, reqLog *models.RequestLog) bool {
	if candidate.ID == 0 || candidate.ID == reqLog.ID {
		return false
	}
	if candidate.CreatedAt.Before(reqLog.CreatedAt) {
		return true
	}
	return candidate.CreatedAt.Equal(reqLog.CreatedAt) && candidate.ID < reqLog.ID
}

func (w *AsyncLogWriter) Write(reqLog *models.RequestLog) {
	select {
	case w.logChan <- &LogWriteRequest{Log: reqLog}:
	default:
		log.Warn("Async log queue full, dropping entry")
	}
}

func (w *AsyncLogWriter) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}
