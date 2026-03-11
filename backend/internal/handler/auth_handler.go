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
	emailCodeSvc   *service.EmailCodeService
}

func NewAuthHandler(
	db *gorm.DB,
	jwtMgr *security.JWTManager,
	settingSvc *service.SettingService,
	captchaSvc *service.CaptchaService,
	emailCodeSvc *service.EmailCodeService,
) *AuthHandler {
	return &AuthHandler{
		db:             db,
		jwtMgr:         jwtMgr,
		settingService: settingSvc,
		captchaService: captchaSvc,
		emailCodeSvc:   emailCodeSvc,
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
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !util.IsValidEmail(email) || req.Password == "" {
		util.Err(c, http.StatusBadRequest, "请输入有效邮箱和密码")
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
		util.Err(c, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	if !user.CheckPassword(req.Password) {
		util.Err(c, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != "active" {
		util.Err(c, http.StatusForbidden, "账号已被禁用")
		return
	}

	ttl := 24 * time.Hour
	if req.Remember {
		ttl = 7 * 24 * time.Hour
	}
	token, expiresAt, err := h.jwtMgr.IssueToken(user.ID, user.Username, user.Role, user.TokenVersion, ttl)
	if err != nil {
		util.Err(c, http.StatusInternalServerError, "生成登录凭证失败")
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

func (h *AuthHandler) SendRegisterEmailCode(c *gin.Context) {
	if !h.settingService.CanRegister() {
		util.Err(c, http.StatusForbidden, "当前未开放注册")
		return
	}
	if !h.settingService.RegisterEmailVerifyEnabled() {
		util.Err(c, http.StatusBadRequest, "系统未启用注册邮箱验证")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !util.IsValidEmail(email) {
		util.Err(c, http.StatusBadRequest, "邮箱格式无效")
		return
	}
	if h.emailCodeSvc == nil {
		util.Err(c, http.StatusInternalServerError, "验证码服务未初始化")
		return
	}
	if err := h.emailCodeSvc.SendRegisterCode(email); err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, gin.H{"sent": true})
}

func (h *AuthHandler) Register(c *gin.Context) {
	if !h.settingService.CanRegister() {
		util.Err(c, http.StatusForbidden, "当前未开放注册")
		return
	}
	var req struct {
		Username        string                  `json:"username"`
		Email           string                  `json:"email"`
		Password        string                  `json:"password"`
		ConfirmPassword string                  `json:"confirm_password"`
		EmailCode       string                  `json:"email_code"`
		Captcha         *service.CaptchaPayload `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)
	if !util.IsValidUsername(req.Username) {
		util.Err(c, http.StatusBadRequest, "用户名需以中文或英文开头，长度 2-32，可包含数字、下划线和短横线")
		return
	}
	if !util.IsValidEmail(req.Email) || len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "邮箱或密码格式不正确")
		return
	}
	if req.ConfirmPassword == "" {
		util.Err(c, http.StatusBadRequest, "请再次输入密码")
		return
	}
	if req.Password != req.ConfirmPassword {
		util.Err(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}
	if h.captchaService != nil {
		if err := h.captchaService.VerifyRegister(req.Captcha, strings.TrimSpace(c.ClientIP())); err != nil {
			util.Err(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	if h.settingService.RegisterEmailVerifyEnabled() {
		if strings.TrimSpace(req.EmailCode) == "" {
			util.Err(c, http.StatusBadRequest, "请输入邮箱验证码")
			return
		}
		if h.emailCodeSvc == nil {
			util.Err(c, http.StatusInternalServerError, "验证码服务未初始化")
			return
		}
		if err := h.emailCodeSvc.VerifyRegisterCode(req.Email, req.EmailCode); err != nil {
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
		if defaultGroup.Unlimited {
			user.DailyLimit = 0
			user.Concurrency = 0
		} else {
			user.DailyLimit = maxNonNegative(defaultGroup.DailyLimit)
			user.Concurrency = maxNonNegative(defaultGroup.Concurrency)
		}
	}
	if err := user.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "设置密码失败")
		return
	}
	if err := h.db.Create(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "用户名或邮箱已存在")
		return
	}
	util.OK(c, gin.H{"user_id": user.ID})
}

func maxNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
