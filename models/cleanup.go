package models

import (
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func CleanupLogsByCount(db *gorm.DB, maxCount, keepCount int64) (int64, error) {
	var totalCount int64
	if err := db.Model(&RequestLog{}).Count(&totalCount).Error; err != nil {
		return 0, err
	}

	if totalCount <= maxCount {
		return 0, nil
	}

	var thresholdID uint
	if err := db.Model(&RequestLog{}).
		Order("id DESC").
		Offset(int(keepCount)).
		Limit(1).
		Select("id").
		Scan(&thresholdID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
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
					log.Infof("Cleaned up %d log records (total exceeded %d, kept latest %d)", count, maxCount, keepCount)
				}
			}
		}
	}()

	count, err := CleanupLogsByCount(db, maxCount, keepCount)
	if err != nil {
		log.Errorf("Initial log cleanup failed: %v", err)
	} else if count > 0 {
		log.Infof("Initial cleanup: removed %d log records (total exceeded %d, kept latest %d)", count, maxCount, keepCount)
	}
}
