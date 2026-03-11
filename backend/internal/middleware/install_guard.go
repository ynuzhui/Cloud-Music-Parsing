package middleware

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

type InstallState struct {
	installed atomic.Bool
}

func NewInstallState(installed bool) *InstallState {
	s := &InstallState{}
	s.installed.Store(installed)
	return s
}

func (s *InstallState) IsInstalled() bool {
	return s.installed.Load()
}

func (s *InstallState) SetInstalled(v bool) {
	s.installed.Store(v)
}

func RequireInstalled(state *InstallState) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !state.IsInstalled() {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
				"code": http.StatusPreconditionRequired,
				"msg":  "系统尚未安装，请先完成 /install 流程",
			})
			return
		}
		c.Next()
	}
}
