package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/cache"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db             *gorm.DB
	box            *security.SecretBox
	settingService *service.SettingService
	statsService   *service.StatsService
	parseService   *service.ParseService
}

func NewAdminHandler(
	db *gorm.DB,
	box *security.SecretBox,
	settingSvc *service.SettingService,
	statsSvc *service.StatsService,
	parseSvc *service.ParseService,
) *AdminHandler {
	return &AdminHandler{
		db:             db,
		box:            box,
		settingService: settingSvc,
		statsService:   statsSvc,
		parseService:   parseSvc,
	}
}

func (h *AdminHandler) Stats(c *gin.Context) {
	stats, err := h.statsService.Dashboard()
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, stats)
}

func (h *AdminHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.Load()
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, settings)
}

func (h *AdminHandler) SaveSettings(c *gin.Context) {
	var req service.SystemSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Redis.Enabled {
		redisCache := cache.NewRedisCache(req.Redis.Addr(), req.Redis.Pass, req.Redis.DB)
		if err := redisCache.Ping(c.Request.Context()); err != nil {
			_ = redisCache.Close()
			util.Err(c, http.StatusBadRequest, "redis connection failed: "+err.Error())
			return
		}
		_ = redisCache.Close()
	}

	if err := h.settingService.Save(req); err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.parseService.RefreshCacheBackend(c.Request.Context())
	util.OK(c, gin.H{"saved": true})
}

func (h *AdminHandler) AddCookie(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		Label    string `json:"label"`
		Value    string `json:"value"`
		Active   *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Provider = normalizeProvider(req.Provider)
	req.Label = strings.TrimSpace(req.Label)
	req.Value = strings.TrimSpace(req.Value)
	if req.Label == "" || req.Value == "" {
		util.Err(c, http.StatusBadRequest, "label and value are required")
		return
	}

	storeValue := req.Value
	if req.Provider == "netease" {
		musicU := h.parseService.ExtractMusicU(req.Value)
		if musicU == "" {
			util.Err(c, http.StatusBadRequest, "cookie must contain MUSIC_U")
			return
		}
		storeValue = "MUSIC_U=" + musicU
	}

	enc, err := h.box.Encrypt(storeValue)
	if err != nil {
		util.Err(c, http.StatusInternalServerError, "cookie encryption failed")
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	row := model.Cookie{
		Provider:       req.Provider,
		Label:          req.Label,
		ValueEncrypted: enc,
		Active:         active,
		Status:         service.CookieStatusUnknown,
	}
	if err := h.db.Create(&row).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	if req.Provider == "netease" {
		_, _ = h.verifyAndPersistCookie(c.Request.Context(), &row)
	}
	util.OK(c, gin.H{"id": row.ID})
}

func (h *AdminHandler) ListCookies(c *gin.Context) {
	var rows []model.Cookie
	if err := h.db.Order("id desc").Find(&rows).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		value := ""
		if plain, err := h.box.Decrypt(row.ValueEncrypted); err == nil {
			plain = strings.TrimSpace(plain)
			if row.Provider == "netease" {
				if musicU := h.parseService.ExtractMusicU(plain); musicU != "" {
					value = musicU
				} else {
					value = plain
				}
				value = strings.Join(strings.Fields(value), "")
				value = strings.Trim(strings.TrimSpace(value), "\"")
			} else {
				value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(plain, "\r", ""), "\n", ""))
			}
		}
		out = append(out, gin.H{
			"id":            row.ID,
			"provider":      row.Provider,
			"label":         row.Label,
			"value":         value,
			"active":        row.Active,
			"status":        row.Status,
			"nickname":      row.Nickname,
			"vip_type":      row.VipType,
			"vip_level":     row.VipLevel,
			"red_vip_level": row.RedVipLevel,
			"last_checked":  row.LastChecked,
			"call_count":    row.CallCount,
			"last_used_at":  row.LastUsedAt,
			"fail_count":    row.FailCount,
			"last_error":    row.LastError,
			"created_at":    row.CreatedAt,
			"updated_at":    row.UpdatedAt,
		})
	}
	util.OK(c, out)
}

func (h *AdminHandler) UpdateCookie(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		util.Err(c, http.StatusBadRequest, "invalid cookie id")
		return
	}

	var row model.Cookie
	if err := h.db.First(&row, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "cookie not found")
		return
	}

	var req struct {
		Label  *string `json:"label"`
		Value  *string `json:"value"`
		Active *bool   `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}

	valueChanged := false
	if req.Label != nil {
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			util.Err(c, http.StatusBadRequest, "label cannot be empty")
			return
		}
		row.Label = label
	}
	if req.Active != nil {
		row.Active = *req.Active
	}
	if req.Value != nil && strings.TrimSpace(*req.Value) != "" {
		valueChanged = true
		storeValue := strings.TrimSpace(*req.Value)
		if row.Provider == "netease" {
			musicU := h.parseService.ExtractMusicU(storeValue)
			if musicU == "" {
				util.Err(c, http.StatusBadRequest, "cookie must contain MUSIC_U")
				return
			}
			storeValue = "MUSIC_U=" + musicU
		}
		enc, encErr := h.box.Encrypt(storeValue)
		if encErr != nil {
			util.Err(c, http.StatusInternalServerError, "cookie encryption failed")
			return
		}
		row.ValueEncrypted = enc
		row.Status = service.CookieStatusUnknown
		row.LastError = ""
	}

	if err := h.db.Save(&row).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	if valueChanged && row.Provider == "netease" {
		_, _ = h.verifyAndPersistCookie(c.Request.Context(), &row)
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *AdminHandler) DeleteCookie(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		util.Err(c, http.StatusBadRequest, "invalid cookie id")
		return
	}
	if err := h.db.Delete(&model.Cookie{}, id).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{"deleted": true})
}

func (h *AdminHandler) VerifyCookie(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		util.Err(c, http.StatusBadRequest, "invalid cookie id")
		return
	}

	var row model.Cookie
	if err := h.db.First(&row, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "cookie not found")
		return
	}
	if row.Provider != "netease" {
		util.Err(c, http.StatusBadRequest, "only netease cookie verification is supported")
		return
	}

	result, verifyErr := h.verifyAndPersistCookie(c.Request.Context(), &row)
	if verifyErr != nil {
		util.Err(c, http.StatusInternalServerError, verifyErr.Error())
		return
	}
	util.OK(c, gin.H{
		"id":            row.ID,
		"status":        row.Status,
		"nickname":      row.Nickname,
		"vip_type":      row.VipType,
		"vip_level":     row.VipLevel,
		"red_vip_level": row.RedVipLevel,
		"last_checked":  row.LastChecked,
		"fail_count":    row.FailCount,
		"last_error":    row.LastError,
		"verify":        result,
	})
}

func (h *AdminHandler) VerifyAllCookies(c *gin.Context) {
	var rows []model.Cookie
	if err := h.db.Where("provider = ?", "netease").Order("id desc").Find(&rows).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	valid := 0
	invalid := 0
	for i := range rows {
		result, err := h.verifyAndPersistCookie(c.Request.Context(), &rows[i])
		if err != nil {
			continue
		}
		if result != nil && result.Valid {
			valid++
		} else {
			invalid++
		}
	}
	util.OK(c, gin.H{
		"total":   len(rows),
		"valid":   valid,
		"invalid": invalid,
	})
}

func (h *AdminHandler) AuditLogs(c *gin.Context) {
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	var rows []model.AuditLog
	if err := h.db.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, rows)
}

func (h *AdminHandler) SmtpTest(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.To) == "" {
		util.Err(c, http.StatusBadRequest, "recipient email is required")
		return
	}

	settings, err := h.settingService.Load()
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	smtpCfg := settings.SMTP
	if strings.TrimSpace(smtpCfg.Host) == "" {
		util.Err(c, http.StatusBadRequest, "smtp host is required")
		return
	}

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)
	subject := "Cloud Music Parsing SMTP test"
	body := "This is a test email from Cloud Music Parsing."
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		smtpCfg.User, req.To, subject, body)

	var auth smtp.Auth
	if smtpCfg.User != "" {
		auth = smtp.PlainAuth("", smtpCfg.User, smtpCfg.Pass, smtpCfg.Host)
	}

	if smtpCfg.SSL {
		tlsConfig := &tls.Config{ServerName: smtpCfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			util.Err(c, http.StatusBadRequest, "smtp tls connect failed: "+err.Error())
			return
		}
		client, err := smtp.NewClient(conn, smtpCfg.Host)
		if err != nil {
			util.Err(c, http.StatusBadRequest, "smtp client create failed: "+err.Error())
			return
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				util.Err(c, http.StatusBadRequest, "smtp auth failed: "+err.Error())
				return
			}
		}
		if err := client.Mail(smtpCfg.User); err != nil {
			util.Err(c, http.StatusBadRequest, "smtp MAIL FROM failed: "+err.Error())
			return
		}
		if err := client.Rcpt(req.To); err != nil {
			util.Err(c, http.StatusBadRequest, "smtp RCPT TO failed: "+err.Error())
			return
		}
		w, err := client.Data()
		if err != nil {
			util.Err(c, http.StatusBadRequest, "smtp DATA failed: "+err.Error())
			return
		}
		_, _ = w.Write([]byte(msg))
		_ = w.Close()
		_ = client.Quit()
	} else {
		if err := smtp.SendMail(addr, auth, smtpCfg.User, []string{req.To}, []byte(msg)); err != nil {
			util.Err(c, http.StatusBadRequest, "smtp send failed: "+err.Error())
			return
		}
	}

	util.OK(c, gin.H{"sent": true})
}

func normalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "netease"
	}
	return provider
}

func (h *AdminHandler) verifyAndPersistCookie(ctx context.Context, row *model.Cookie) (*service.CookieVerifyResult, error) {
	plain, err := h.box.Decrypt(row.ValueEncrypted)
	if err != nil {
		return nil, err
	}
	result, verifyErr := h.parseService.VerifyNeteaseCookie(ctx, plain)
	if verifyErr != nil {
		return nil, verifyErr
	}

	now := time.Now()
	updates := map[string]any{
		"last_checked": &now,
	}
	if result != nil && result.Valid {
		updates["status"] = service.CookieStatusValid
		updates["nickname"] = result.Nickname
		updates["vip_type"] = result.VipType
		updates["vip_level"] = result.VipLevel
		updates["red_vip_level"] = result.RedVipLevel
		updates["last_error"] = ""
		updates["fail_count"] = 0
	} else {
		updates["status"] = service.CookieStatusInvalid
		if result != nil {
			updates["last_error"] = strings.TrimSpace(result.Error)
		}
		updates["fail_count"] = row.FailCount + 1
	}

	if err := h.db.Model(&model.Cookie{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = h.db.First(row, row.ID).Error
	return result, nil
}
