package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// NoCacheAPI 为所有 /api/ 路径的响应注入禁止缓存头，防止 CDN 缓存动态接口数据。
func NoCacheAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	}
}
