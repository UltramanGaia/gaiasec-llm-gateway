package handlers

import (
	"sync"

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

		for {
			select {
			case req := <-w.logChan:
				if req != nil && req.Log != nil {
					w.writeSingle(req.Log)
				}
			case <-w.stopChan:
				return
			}
		}
	}()
}

func (w *AsyncLogWriter) writeSingle(reqLog *models.RequestLog) {
	if err := w.db.Create(reqLog).Error; err != nil {
		log.WithError(err).Error("Failed to write log")
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
