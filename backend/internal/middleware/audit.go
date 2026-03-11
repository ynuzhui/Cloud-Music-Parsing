package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var sensitivePattern = regexp.MustCompile(`(?i)(password|pass|token|secret|cookie|authorization|music_u|smtp|redis)`)

func AuditLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		bodyPreview := ""

		if c.Request.Body != nil {
			rawBody, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(rawBody))
			if len(rawBody) > 0 {
				bodyPreview = sanitizeBody(rawBody)
				if len(bodyPreview) > 4096 {
					bodyPreview = bodyPreview[:4096]
				}
			}
		}

		c.Next()

		var userID *uint
		role := ""
		if claimsAny, ok := c.Get(ContextClaimsKey); ok {
			if claims, ok := claimsAny.(*security.Claims); ok {
				uid := claims.UserID
				userID = &uid
				role = strings.TrimSpace(claims.Role)
			}
		}

		logRow := model.AuditLog{
			UserID:      userID,
			Role:        role,
			Path:        c.FullPath(),
			Method:      c.Request.Method,
			IP:          c.ClientIP(),
			StatusCode:  c.Writer.Status(),
			LatencyMS:   time.Since(start).Milliseconds(),
			RequestBody: bodyPreview,
		}
		_ = db.Create(&logRow).Error
	}
}

func sanitizeBody(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return sensitivePattern.ReplaceAllString(trimmed, "***")
	}
	maskJSON(v)
	out, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func maskJSON(v any) {
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			if sensitivePattern.MatchString(k) {
				node[k] = "***"
				continue
			}
			maskJSON(child)
		}
	case []any:
		for _, child := range node {
			maskJSON(child)
		}
	}
}
