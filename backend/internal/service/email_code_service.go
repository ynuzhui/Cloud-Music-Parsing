package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

const (
	emailCodeSceneRegister = "register"
	emailCodeTTL           = 10 * time.Minute
	emailCodeCooldown      = 60 * time.Second
)

type EmailCodeService struct {
	db          *gorm.DB
	mailService *MailService
}

func NewEmailCodeService(db *gorm.DB, mailSvc *MailService) *EmailCodeService {
	return &EmailCodeService{
		db:          db,
		mailService: mailSvc,
	}
}

func (s *EmailCodeService) SendRegisterCode(email string) error {
	email = strings.TrimSpace(email)
	if !util.IsValidEmail(email) {
		return errors.New("邮箱格式无效")
	}
	if s.mailService == nil {
		return errors.New("邮件服务未初始化")
	}

	now := util.NowBeijing()
	var latest model.EmailVerificationCode
	if err := s.db.
		Where("email = ? AND scene = ?", email, emailCodeSceneRegister).
		Order("id DESC").
		First(&latest).Error; err == nil {
		if now.Sub(latest.CreatedAt.In(util.BeijingLocation())) < emailCodeCooldown {
			return errors.New("发送过于频繁，请稍后再试")
		}
	}

	code, err := randomDigits(6)
	if err != nil {
		return fmt.Errorf("生成验证码失败：%w", err)
	}

	subject := "【云音解析】注册验证码"
	body := fmt.Sprintf("您的注册验证码为：%s\n有效期：10 分钟\n\n如非本人操作，请忽略本邮件。", code)
	if err := s.mailService.SendText(email, subject, body); err != nil {
		return err
	}

	record := model.EmailVerificationCode{
		Email:     email,
		Scene:     emailCodeSceneRegister,
		Code:      code,
		ExpiresAt: now.Add(emailCodeTTL),
	}
	return s.db.Create(&record).Error
}

func (s *EmailCodeService) VerifyRegisterCode(email, code string) error {
	email = strings.TrimSpace(email)
	code = strings.TrimSpace(code)
	if !util.IsValidEmail(email) {
		return errors.New("邮箱格式无效")
	}
	if len(code) != 6 {
		return errors.New("验证码格式无效")
	}

	now := util.NowBeijing()
	var latest model.EmailVerificationCode
	if err := s.db.
		Where("email = ? AND scene = ? AND used_at IS NULL", email, emailCodeSceneRegister).
		Order("id DESC").
		First(&latest).Error; err != nil {
		return errors.New("请先发送验证码")
	}
	if latest.ExpiresAt.Before(now) {
		return errors.New("验证码已过期，请重新发送")
	}
	if latest.Code != code {
		return errors.New("验证码错误")
	}

	usedAt := now
	return s.db.Model(&model.EmailVerificationCode{}).
		Where("id = ?", latest.ID).
		Update("used_at", &usedAt).Error
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid code length")
	}
	builder := strings.Builder{}
	builder.Grow(length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}
