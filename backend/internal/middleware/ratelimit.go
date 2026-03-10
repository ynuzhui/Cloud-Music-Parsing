package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type limiterEntry struct {
	Count    int
	ExpireAt time.Time
}

type MemoryRateLimiter struct {
	mu     sync.Mutex
	data   map[string]limiterEntry
	limit  int
	window time.Duration
}

func NewMemoryRateLimiter(limit int, window time.Duration) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		data:   make(map[string]limiterEntry),
		limit:  limit,
		window: window,
	}
}

func (l *MemoryRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("%s|%s", ip, c.FullPath())
		now := time.Now()

		l.mu.Lock()
		entry := l.data[key]
		if now.After(entry.ExpireAt) {
			entry = limiterEntry{
				Count:    0,
				ExpireAt: now.Add(l.window),
			}
		}
		entry.Count++
		l.data[key] = entry
		l.cleanup(now)
		l.mu.Unlock()

		if entry.Count > l.limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": http.StatusTooManyRequests,
				"msg":  "too many requests",
			})
			return
		}
		c.Next()
	}
}

func (l *MemoryRateLimiter) cleanup(now time.Time) {
	if len(l.data) < 2048 {
		return
	}
	for k, v := range l.data {
		if now.After(v.ExpireAt) {
			delete(l.data, k)
		}
	}
}
