package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go-music-aggregator/backend/internal/middleware"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct {
	db           *gorm.DB
	quotaService *service.QuotaService
}

func NewUserHandler(db *gorm.DB, quotaSvc *service.QuotaService) *UserHandler {
	return &UserHandler{
		db:           db,
		quotaService: quotaSvc,
	}
}

func (h *UserHandler) Me(c *gin.Context) {
	claims, ok := c.Get(middleware.ContextClaimsKey)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}
	authClaims, ok := claims.(*security.Claims)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "invalid auth context")
		return
	}

	var user model.User
	if err := h.db.First(&user, authClaims.UserID).Error; err != nil {
		util.Err(c, http.StatusUnauthorized, "user not found")
		return
	}
	groupName := ""
	if user.GroupID != nil {
		var group model.UserGroup
		if err := h.db.Select("id", "name").First(&group, *user.GroupID).Error; err == nil {
			groupName = group.Name
		}
	}
	util.OK(c, gin.H{
		"id":                user.ID,
		"username":          user.Username,
		"email":             user.Email,
		"role":              user.Role,
		"status":            user.Status,
		"group_id":          user.GroupID,
		"group_name":        groupName,
		"daily_limit":       user.DailyLimit,
		"concurrency_limit": user.Concurrency,
		"last_login_at":     user.LastLoginAt,
		"last_login_ip":     user.LastLoginIP,
		"created_at":        user.CreatedAt,
	})
}

func (h *UserHandler) QuotaToday(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}
	snapshot, err := h.quotaService.Today(userID, c.ClientIP())
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, snapshot)
}

func (h *UserHandler) UsageTrend(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}
	days := 7
	raw := strings.TrimSpace(c.Query("days"))
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 31 {
			days = n
		}
	}
	rows, err := h.quotaService.Trend(userID, days)
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{
		"timezone": util.BeijingTimezone,
		"days":     days,
		"items":    rows,
	})
}

func currentUserID(c *gin.Context) (uint, bool) {
	claimsAny, ok := c.Get(middleware.ContextClaimsKey)
	if !ok {
		return 0, false
	}
	claims, ok := claimsAny.(*security.Claims)
	if !ok {
		return 0, false
	}
	return claims.UserID, true
}
