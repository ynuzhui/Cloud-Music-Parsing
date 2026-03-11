package handler

import (
	"net/http"

	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type PublicHandler struct {
	settingService *service.SettingService
}

func NewPublicHandler(settingSvc *service.SettingService) *PublicHandler {
	return &PublicHandler{settingService: settingSvc}
}

func (h *PublicHandler) Site(c *gin.Context) {
	settings, err := h.settingService.Load()
	if err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{
		"name":                settings.Site.Name,
		"keywords":            settings.Site.Keywords,
		"description":         settings.Site.Description,
		"icp_no":              settings.Site.ICPNo,
		"police_no":           settings.Site.PoliceNo,
		"parse_require_login": settings.Feature.ParseRequireLogin,
		"timezone":            "Asia/Shanghai",
	})
}
