package handler

import (
	"net/http"
	"time"

	"go-music-aggregator/backend/internal/middleware"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type InstallHandler struct {
	state       *middleware.InstallState
	service     *service.InstallService
	autoRestart bool
	RestartCh   chan struct{}
}

func NewInstallHandler(state *middleware.InstallState, svc *service.InstallService, autoRestart bool, restartCh chan struct{}) *InstallHandler {
	return &InstallHandler{
		state:       state,
		service:     svc,
		autoRestart: autoRestart,
		RestartCh:   restartCh,
	}
}

func (h *InstallHandler) TestConnection(c *gin.Context) {
	var req service.InstallTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if err := h.service.TestConnection(req); err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, gin.H{"connected": true})
}

func (h *InstallHandler) Complete(c *gin.Context) {
	if h.state.IsInstalled() {
		util.Err(c, http.StatusConflict, "系统已安装，禁止重复安装")
		return
	}

	var req service.InstallCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	result, err := h.service.Complete(req)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	restartScheduled := h.autoRestart && result.RestartSuggested
	if !restartScheduled {
		h.state.SetInstalled(true)
	}
	util.OK(c, gin.H{
		"installed":         true,
		"restart_scheduled": restartScheduled,
	})

	if restartScheduled && h.RestartCh != nil {
		go func() {
			time.Sleep(1200 * time.Millisecond)
			h.RestartCh <- struct{}{}
		}()
	}
}
