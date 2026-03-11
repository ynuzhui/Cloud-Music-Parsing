package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

type MailService struct {
	settingService *SettingService
}

func NewMailService(settingSvc *SettingService) *MailService {
	return &MailService{settingService: settingSvc}
}

func (s *MailService) SendText(to, subject, body string) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	if to == "" {
		return errors.New("收件人邮箱不能为空")
	}
	if subject == "" {
		subject = "系统通知"
	}

	settings, err := s.settingService.Load()
	if err != nil {
		return err
	}
	smtpCfg := settings.SMTP
	if strings.TrimSpace(smtpCfg.Host) == "" {
		return errors.New("请先配置 SMTP 服务")
	}
	if smtpCfg.Port <= 0 {
		return errors.New("SMTP 端口配置无效")
	}
	if strings.TrimSpace(smtpCfg.User) == "" {
		return errors.New("SMTP 发件账号不能为空")
	}

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		smtpCfg.User,
		to,
		subject,
		body,
	)

	var auth smtp.Auth
	if smtpCfg.User != "" {
		auth = smtp.PlainAuth("", smtpCfg.User, smtpCfg.Pass, smtpCfg.Host)
	}

	if smtpCfg.SSL {
		tlsConfig := &tls.Config{ServerName: smtpCfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("SMTP TLS 连接失败: %w", err)
		}
		client, err := smtp.NewClient(conn, smtpCfg.Host)
		if err != nil {
			return fmt.Errorf("SMTP 客户端创建失败: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
		if err := client.Mail(smtpCfg.User); err != nil {
			return fmt.Errorf("SMTP 发件人设置失败: %w", err)
		}
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("SMTP 收件人设置失败: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("SMTP DATA 阶段失败: %w", err)
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			_ = w.Close()
			return fmt.Errorf("SMTP 写入邮件内容失败: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("SMTP 提交邮件失败: %w", err)
		}
		if err := client.Quit(); err != nil {
			return fmt.Errorf("SMTP 连接关闭失败: %w", err)
		}
		return nil
	}

	if err := smtp.SendMail(addr, auth, smtpCfg.User, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("SMTP 发送失败: %w", err)
	}
	return nil
}
