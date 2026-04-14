package handlers

import (
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

	err := w.db.Create(&logs).Error
	if err == nil {
		return
	}

	log.WithError(err).WithField("batch_size", len(logs)).Warn("Failed to batch write logs, falling back to single inserts")

	for _, reqLog := range logs {
		if reqLog == nil {
			continue
		}
		if singleErr := w.db.Create(reqLog).Error; singleErr != nil {
			log.WithError(singleErr).Error("Failed to write log")
		}
	}
}

func (w *AsyncLogWriter) Write(reqLog *models.RequestLog) {
	select {
	case w.logChan <- &LogWriteRequest{Log: reqLog}:
	default:
		log.Warn("Log channel full, dropping log entry")
	}
}

func (w *AsyncLogWriter) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}
