package middleware

import (
	"net/http"
	"strings"

	"go-music-aggregator/backend/internal/security"

	"github.com/gin-gonic/gin"
)

const ContextClaimsKey = "auth_claims"

func JWTAuth(jwtMgr *security.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "missing authorization header"})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid authorization format"})
			return
		}
		claims, err := jwtMgr.ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid token"})
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
		if !ok || authClaims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "admin required"})
			return
		}
		c.Next()
	}
}
