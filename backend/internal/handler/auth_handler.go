package handler

import (
	"net/http"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db             *gorm.DB
	jwtMgr         *security.JWTManager
	settingService *service.SettingService
	captchaService *service.CaptchaService
}

func NewAuthHandler(
	db *gorm.DB,
	jwtMgr *security.JWTManager,
	settingSvc *service.SettingService,
	captchaSvc *service.CaptchaService,
) *AuthHandler {
	return &AuthHandler{
		db:             db,
		jwtMgr:         jwtMgr,
		settingService: settingSvc,
		captchaService: captchaSvc,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string                  `json:"email"`
		Password string                  `json:"password"`
		Remember bool                    `json:"remember"`
		Captcha  *service.CaptchaPayload `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !util.IsValidEmail(email) || req.Password == "" {
		util.Err(c, http.StatusBadRequest, "valid email and password are required")
		return
	}
	if h.captchaService != nil {
		if err := h.captchaService.VerifyLogin(req.Captcha, strings.TrimSpace(c.ClientIP())); err != nil {
			util.Err(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	var user model.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
		util.Err(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !user.CheckPassword(req.Password) {
		util.Err(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != "active" {
		util.Err(c, http.StatusForbidden, "account disabled")
		return
	}

	ttl := 24 * time.Hour
	if req.Remember {
		ttl = 7 * 24 * time.Hour
	}
	token, expiresAt, err := h.jwtMgr.IssueToken(user.ID, user.Username, user.Role, user.TokenVersion, ttl)
	if err != nil {
		util.Err(c, http.StatusInternalServerError, "failed to issue token")
		return
	}
	now := time.Now()
	_ = h.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"last_login_at": &now,
		"last_login_ip": strings.TrimSpace(c.ClientIP()),
	}).Error

	util.OK(c, gin.H{
		"token":      token,
		"expires_at": expiresAt,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	if !h.settingService.CanRegister() {
		util.Err(c, http.StatusForbidden, "register is disabled")
		return
	}
	var req struct {
		Username string                  `json:"username"`
		Email    string                  `json:"email"`
		Password string                  `json:"password"`
		Captcha  *service.CaptchaPayload `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if !util.IsValidUsername(req.Username) || !util.IsValidEmail(req.Email) || len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "invalid username, email or password")
		return
	}
	if h.captchaService != nil {
		if err := h.captchaService.VerifyRegister(req.Captcha, strings.TrimSpace(c.ClientIP())); err != nil {
			util.Err(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	user := model.User{
		Username: req.Username,
		Email:    req.Email,
		Role:     "user",
		Status:   "active",
	}
	var defaultGroup model.UserGroup
	if err := h.db.Where("is_default = ?", true).Order("id asc").First(&defaultGroup).Error; err == nil {
		groupID := defaultGroup.ID
		user.GroupID = &groupID
	}
	if err := user.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "failed to set password")
		return
	}
	if err := h.db.Create(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "username or email already exists")
		return
	}
	util.OK(c, gin.H{"user_id": user.ID})
}
