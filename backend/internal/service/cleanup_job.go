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
	log.Printf("[任务:数据清理] 调度器已启动，执行间隔: %s", cleanupInterval)
	j.runOnce()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[任务:数据清理] 调度器已停止")
			return
		case <-ticker.C:
			j.runOnce()
		}
	}
}

func (j *CleanupJob) runOnce() {
	log.Printf("[任务:数据清理] 开始执行清理任务")
	now := time.Now()

	// Clean expired email verification codes
	cutoffCodes := now.AddDate(0, 0, -expiredCodeRetentionDays)
	if result := j.db.Where("expires_at < ? AND used_at IS NOT NULL", cutoffCodes).Delete(&model.EmailVerificationCode{}); result.Error != nil {
		log.Printf("[任务:数据清理] 删除已使用验证码失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[任务:数据清理] 已删除 %d 条已使用验证码", result.RowsAffected)
	}
	// Also clean expired unused codes older than retention
	if result := j.db.Where("expires_at < ?", cutoffCodes).Delete(&model.EmailVerificationCode{}); result.Error != nil {
		log.Printf("[任务:数据清理] 删除过期验证码失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[任务:数据清理] 已删除 %d 条过期验证码", result.RowsAffected)
	}

	// Clean old audit logs
	cutoffAudit := now.AddDate(0, 0, -auditLogRetentionDays)
	if result := j.db.Where("created_at < ?", cutoffAudit).Delete(&model.AuditLog{}); result.Error != nil {
		log.Printf("[任务:数据清理] 删除历史审计日志失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[任务:数据清理] 已删除 %d 条历史审计日志", result.RowsAffected)
	}

	// Clean old parse records
	cutoffParse := now.AddDate(0, 0, -parseRecordRetentionDays)
	if result := j.db.Where("created_at < ?", cutoffParse).Delete(&model.ParseRecord{}); result.Error != nil {
		log.Printf("[任务:数据清理] 删除历史解析记录失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[任务:数据清理] 已删除 %d 条历史解析记录", result.RowsAffected)
	}
	log.Printf("[任务:数据清理] 清理任务执行完成")
}
