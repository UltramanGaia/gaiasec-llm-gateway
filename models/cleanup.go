package models

import (
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func CleanupLogsByCount(db *gorm.DB, maxCount, keepCount int64) (int64, error) {
	var thresholdID uint
	err := db.Raw(`
		SELECT id FROM request_logs 
		ORDER BY id DESC 
		LIMIT 1 OFFSET ?
	`, keepCount).Scan(&thresholdID).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}

	if thresholdID == 0 {
		return 0, nil
	}

	result := db.Where("id < ?", thresholdID).Delete(&RequestLog{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

func StartLogCleanupTask(db *gorm.DB, maxCount, keepCount int64, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				count, err := CleanupLogsByCount(db, maxCount, keepCount)
				if err != nil {
					log.Errorf("Failed to cleanup logs: %v", err)
				} else if count > 0 {
					log.Infof("Cleaned up %d log records (kept latest %d)", count, keepCount)
				}
			}
		}
	}()

	count, err := CleanupLogsByCount(db, maxCount, keepCount)
	if err != nil {
		log.Errorf("Initial log cleanup failed: %v", err)
	} else if count > 0 {
		log.Infof("Initial cleanup: removed %d log records (kept latest %d)", count, keepCount)
	}
}
