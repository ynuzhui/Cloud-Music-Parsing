package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

type CookieAutoVerifyJob struct {
	db             *gorm.DB
	box            *security.SecretBox
	parseService   *ParseService
	settingService *SettingService
	mailService    *MailService
}

func NewCookieAutoVerifyJob(
	db *gorm.DB,
	box *security.SecretBox,
	parseSvc *ParseService,
	settingSvc *SettingService,
	mailSvc *MailService,
) *CookieAutoVerifyJob {
	return &CookieAutoVerifyJob{
		db:             db,
		box:            box,
		parseService:   parseSvc,
		settingService: settingSvc,
		mailService:    mailSvc,
	}
}

func (j *CookieAutoVerifyJob) Run(ctx context.Context) {
	log.Printf("[任务:Cookie自动校验] 调度器已启动（北京时间 00:00 / 06:00 / 12:00 / 18:00）")
	for {
		now := util.NowBeijing()
		nextRun := nextCookieVerifyRun(now)
		wait := nextRun.Sub(now)
		if wait < time.Second {
			wait = time.Second
		}
		log.Printf("[任务:Cookie自动校验] 下一次执行时间: %s", nextRun.Format("2006-01-02 15:04:05"))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Printf("[任务:Cookie自动校验] 调度器已停止")
			return
		case <-timer.C:
			j.runOnce(ctx)
		}
	}
}

func nextCookieVerifyRun(now time.Time) time.Time {
	loc := util.BeijingLocation()
	local := now.In(loc)
	year, month, day := local.Date()
	targetHours := []int{0, 6, 12, 18}
	for _, hour := range targetHours {
		candidate := time.Date(year, month, day, hour, 0, 0, 0, loc)
		if candidate.After(local) {
			return candidate
		}
	}
	return time.Date(year, month, day+1, 0, 0, 0, 0, loc)
}

func (j *CookieAutoVerifyJob) runOnce(ctx context.Context) {
	log.Printf("[任务:Cookie自动校验] 开始执行自动校验")
	settings, err := j.settingService.Load()
	if err != nil {
		log.Printf("[任务:Cookie自动校验] 加载配置失败: %v", err)
		return
	}
	if !settings.Feature.CookieAutoVerify {
		log.Printf("[任务:Cookie自动校验] 功能未开启，本次跳过")
		return
	}
	if j.mailService == nil {
		log.Printf("[任务:Cookie自动校验] 邮件服务未初始化，跳过告警发送")
		return
	}

	var rows []model.Cookie
	if err := j.db.Where("provider = ?", "netease").Order("id DESC").Find(&rows).Error; err != nil {
		log.Printf("[任务:Cookie自动校验] 加载 Cookie 列表失败: %v", err)
		return
	}
	if len(rows) == 0 {
		log.Printf("[任务:Cookie自动校验] 未找到网易云 Cookie，本次跳过")
		return
	}

	total := len(rows)
	validCount := 0
	invalidCount := 0
	errorCount := 0
	errorDetails := make([]string, 0)

	for i := range rows {
		result, verifyErr := j.verifyAndPersistCookie(ctx, &rows[i])
		if verifyErr != nil {
			errorCount++
			errorDetails = append(errorDetails, fmt.Sprintf("ID=%d 校验异常：%v", rows[i].ID, verifyErr))
			continue
		}
		if result != nil && result.Valid {
			validCount++
		} else {
			invalidCount++
			if result != nil && strings.TrimSpace(result.Error) != "" {
				errorDetails = append(errorDetails, fmt.Sprintf("ID=%d 无效：%s", rows[i].ID, strings.TrimSpace(result.Error)))
			}
		}
	}

	if invalidCount == 0 && errorCount == 0 {
		log.Printf("[任务:Cookie自动校验] 执行完成：总数=%d，有效=%d，无效=%d，异常=%d", total, validCount, invalidCount, errorCount)
		return
	}

	superAdminEmail := j.getSuperAdminEmail()
	if superAdminEmail == "" {
		log.Printf("[任务:Cookie自动校验] 未找到超级管理员邮箱，跳过告警邮件")
		return
	}

	subject := "【云音解析】Cookie 自动校验告警"
	body := fmt.Sprintf(
		"北京时间：%s\n总数：%d\n有效：%d\n无效：%d\n异常：%d\n\n详情：\n%s\n\n如需接收告警邮件，请先完成 SMTP 配置。",
		util.NowBeijing().Format("2006-01-02 15:04:05"),
		total,
		validCount,
		invalidCount,
		errorCount,
		strings.Join(errorDetails, "\n"),
	)

	if err := j.mailService.SendText(superAdminEmail, subject, body); err != nil {
		log.Printf("[任务:Cookie自动校验] 发送告警邮件失败（请先配置 SMTP 服务）: %v", err)
	} else {
		log.Printf("[任务:Cookie自动校验] 告警邮件发送完成（收件人：%s）", superAdminEmail)
	}
	log.Printf("[任务:Cookie自动校验] 执行完成：总数=%d，有效=%d，无效=%d，异常=%d", total, validCount, invalidCount, errorCount)
}

func (j *CookieAutoVerifyJob) getSuperAdminEmail() string {
	var user model.User
	if err := j.db.Select("id", "email").First(&user, 1).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(user.Email)
}

func (j *CookieAutoVerifyJob) verifyAndPersistCookie(ctx context.Context, row *model.Cookie) (*CookieVerifyResult, error) {
	plain, err := j.box.Decrypt(row.ValueEncrypted)
	if err != nil {
		return nil, err
	}
	result, verifyErr := j.parseService.VerifyNeteaseCookie(ctx, plain)
	if verifyErr != nil {
		return nil, verifyErr
	}

	now := util.NowBeijing()
	updates := map[string]any{
		"last_checked": &now,
	}
	if result != nil && result.Valid {
		updates["status"] = CookieStatusValid
		updates["nickname"] = result.Nickname
		updates["vip_type"] = result.VipType
		updates["vip_level"] = result.VipLevel
		updates["red_vip_level"] = result.RedVipLevel
		updates["last_error"] = ""
		updates["fail_count"] = 0
	} else {
		updates["status"] = CookieStatusInvalid
		if result != nil {
			updates["last_error"] = strings.TrimSpace(result.Error)
		}
		updates["fail_count"] = row.FailCount + 1
	}

	if err := j.db.Model(&model.Cookie{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return result, nil
}
