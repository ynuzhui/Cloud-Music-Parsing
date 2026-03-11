package middleware

import (
	"errors"
	"net/http"
	"strings"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const ContextClaimsKey = "auth_claims"

func JWTAuth(jwtMgr *security.JWTManager, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "缺少 Authorization 请求头"})
			return
		}
		claims, err := parseAndValidateClaims(authHeader, jwtMgr, db)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		c.Set(ContextClaimsKey, claims)
		c.Next()
	}
}

func OptionalJWTAuth(jwtMgr *security.JWTManager, db *gorm.DB, settingSvc *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		requireLogin := settingSvc.ParseRequireLogin()
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			if requireLogin {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "请先登录"})
				return
			}
			c.Next()
			return
		}
		claims, err := parseAndValidateClaims(authHeader, jwtMgr, db)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		c.Set(ContextClaimsKey, claims)
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get(ContextClaimsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "登录上下文缺失"})
			return
		}
		authClaims, ok := claims.(*security.Claims)
		if !ok || (authClaims.Role != "admin" && authClaims.Role != "super_admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get(ContextClaimsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "登录上下文缺失"})
			return
		}
		authClaims, ok := claims.(*security.Claims)
		if !ok || authClaims.Role != "super_admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "需要超级管理员权限"})
			return
		}
		c.Next()
	}
}

func parseAndValidateClaims(authHeader string, jwtMgr *security.JWTManager, db *gorm.DB) (*security.Claims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("Authorization 格式错误")
	}
	claims, err := jwtMgr.ParseToken(parts[1])
	if err != nil {
		return nil, errors.New("登录令牌无效")
	}
	var user model.User
	if err := db.Select("id", "role", "status", "token_version").First(&user, claims.UserID).Error; err != nil {
		return nil, errors.New("登录令牌无效")
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != "active" {
		return nil, errors.New("账号已被禁用")
	}
	if user.TokenVersion != claims.TokenVer {
		return nil, errors.New("登录状态已过期，请重新登录")
	}
	claims.Role = user.Role
	return claims, nil
}
