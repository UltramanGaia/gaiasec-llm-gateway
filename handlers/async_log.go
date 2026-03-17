package handlers

import (
	"sync"
	"time"

	"llm-gateway/models"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
			logChan:  make(chan *LogWriteRequest, 10000),
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
		batch := make([]*models.RequestLog, 0, 100)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case req := <-w.logChan:
				if req != nil && req.Log != nil {
					batch = append(batch, req.Log)
					if len(batch) >= 100 {
						w.writeBatch(batch)
						batch = batch[:0]
					}
				}
			case <-ticker.C:
				if len(batch) > 0 {
					w.writeBatch(batch)
					batch = batch[:0]
				}
			case <-w.stopChan:
				if len(batch) > 0 {
					w.writeBatch(batch)
				}
				return
			}
		}
	}()
}

func (w *AsyncLogWriter) writeBatch(logs []*models.RequestLog) {
	if len(logs) == 0 {
		return
	}
	if err := w.db.CreateInBatches(logs, 100).Error; err != nil {
		log.WithError(err).Error("Failed to write batch logs")
	} else {
		log.WithField("count", len(logs)).Debug("Batch logs written successfully")
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
