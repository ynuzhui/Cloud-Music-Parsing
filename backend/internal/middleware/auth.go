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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "missing authorization header"})
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
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "login required"})
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "missing auth context"})
			return
		}
		authClaims, ok := claims.(*security.Claims)
		if !ok || (authClaims.Role != "admin" && authClaims.Role != "super_admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "admin required"})
			return
		}
		c.Next()
	}
}

func SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get(ContextClaimsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "missing auth context"})
			return
		}
		authClaims, ok := claims.(*security.Claims)
		if !ok || authClaims.Role != "super_admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "super admin required"})
			return
		}
		c.Next()
	}
}

func parseAndValidateClaims(authHeader string, jwtMgr *security.JWTManager, db *gorm.DB) (*security.Claims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, errors.New("invalid authorization format")
	}
	claims, err := jwtMgr.ParseToken(parts[1])
	if err != nil {
		return nil, errors.New("invalid token")
	}
	var user model.User
	if err := db.Select("id", "role", "status", "token_version").First(&user, claims.UserID).Error; err != nil {
		return nil, errors.New("invalid token")
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != "active" {
		return nil, errors.New("account disabled")
	}
	if user.TokenVersion != claims.TokenVer {
		return nil, errors.New("token expired")
	}
	claims.Role = user.Role
	return claims, nil
}
