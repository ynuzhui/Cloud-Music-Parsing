package service

import (
	"context"
	"log"
	"time"

	"go-music-aggregator/backend/internal/model"

	"gorm.io/gorm"
)

const (
	cleanupInterval          = 6 * time.Hour
	auditLogRetentionDays    = 90
	parseRecordRetentionDays = 90
	expiredCodeRetentionDays = 7
)

type CleanupJob struct {
	db *gorm.DB
}

func NewCleanupJob(db *gorm.DB) *CleanupJob {
	return &CleanupJob{db: db}
}

func (j *CleanupJob) Run(ctx context.Context) {
	j.runOnce()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.runOnce()
		}
	}
}

func (j *CleanupJob) runOnce() {
	now := time.Now()

	// Clean expired email verification codes
	cutoffCodes := now.AddDate(0, 0, -expiredCodeRetentionDays)
	if result := j.db.Where("expires_at < ? AND used_at IS NOT NULL", cutoffCodes).Delete(&model.EmailVerificationCode{}); result.Error != nil {
		log.Printf("[CLEANUP] delete used verification codes error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[CLEANUP] deleted %d used verification codes", result.RowsAffected)
	}
	// Also clean expired unused codes older than retention
	if result := j.db.Where("expires_at < ?", cutoffCodes).Delete(&model.EmailVerificationCode{}); result.Error != nil {
		log.Printf("[CLEANUP] delete expired verification codes error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[CLEANUP] deleted %d expired verification codes", result.RowsAffected)
	}

	// Clean old audit logs
	cutoffAudit := now.AddDate(0, 0, -auditLogRetentionDays)
	if result := j.db.Where("created_at < ?", cutoffAudit).Delete(&model.AuditLog{}); result.Error != nil {
		log.Printf("[CLEANUP] delete old audit logs error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[CLEANUP] deleted %d old audit logs", result.RowsAffected)
	}

	// Clean old parse records
	cutoffParse := now.AddDate(0, 0, -parseRecordRetentionDays)
	if result := j.db.Where("created_at < ?", cutoffParse).Delete(&model.ParseRecord{}); result.Error != nil {
		log.Printf("[CLEANUP] delete old parse records error: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[CLEANUP] deleted %d old parse records", result.RowsAffected)
	}
}
