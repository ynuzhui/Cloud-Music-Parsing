package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go-music-aggregator/backend/internal/middleware"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/service"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type ParseHandler struct {
	parseService *service.ParseService
	quotaService *service.QuotaService
}

func NewParseHandler(parseService *service.ParseService, quotaService *service.QuotaService) *ParseHandler {
	return &ParseHandler{
		parseService: parseService,
		quotaService: quotaService,
	}
}

func (h *ParseHandler) ParseNetease(c *gin.Context) {
	userID := uint(0)
	if claimsAny, ok := c.Get(middleware.ContextClaimsKey); ok {
		if claims, ok := claimsAny.(*security.Claims); ok {
			userID = claims.UserID
		}
	}

	var req struct {
		URL     string `json:"url"`
		Quality string `json:"quality"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		util.Err(c, http.StatusBadRequest, "链接不能为空")
		return
	}

	if h.quotaService != nil {
		release, _, err := h.quotaService.AcquireParseQuota(userID, c.ClientIP())
		if err != nil {
			util.Err(c, http.StatusTooManyRequests, err.Error())
			return
		}
		defer release()
	}

	result, err := h.parseService.ParseNetease(c.Request.Context(), userID, c.ClientIP(), req.URL, req.Quality)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, result)
}

func (h *ParseHandler) Providers(c *gin.Context) {
	util.OK(c, gin.H{
		"providers": []gin.H{
			{
				"id":          "netease",
				"name":        "网易云音乐",
				"description": "解析 music.163.com 链接，使用 EAPI 加密流程",
			},
		},
	})
}

func (h *ParseHandler) SearchSong(c *gin.Context) {
	var req struct {
		Keyword string `json:"keyword"`
		Limit   int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		util.Err(c, http.StatusBadRequest, "关键词不能为空")
		return
	}
	results, err := h.parseService.SearchSong(c.Request.Context(), req.Keyword, req.Limit)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, results)
}

func (h *ParseHandler) PlaylistDetail(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		util.Err(c, http.StatusBadRequest, "歌单 ID 不能为空")
		return
	}
	info, err := h.parseService.FetchPlaylistTracks(c.Request.Context(), req.ID)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, info)
}

func (h *ParseHandler) GetLyric(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		util.Err(c, http.StatusBadRequest, "歌曲 ID 不能为空")
		return
	}
	lyric, err := h.parseService.FetchLyric(c.Request.Context(), req.ID)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	util.OK(c, lyric)
}

func (h *ParseHandler) DownloadLyric(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		util.Err(c, http.StatusBadRequest, "歌曲 ID 不能为空")
		return
	}

	fileName, body, err := h.parseService.BuildLyricDownload(c.Request.Context(), req.ID)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	setDownloadHeaders(c, fileName)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", body)
}

func (h *ParseHandler) DownloadCover(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		util.Err(c, http.StatusBadRequest, "歌曲 ID 不能为空")
		return
	}

	fileName, mime, body, err := h.parseService.BuildCoverDownload(c.Request.Context(), req.ID)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	setDownloadHeaders(c, fileName)
	c.Data(http.StatusOK, mime, body)
}

func setDownloadHeaders(c *gin.Context, fileName string) {
	escapedName := url.PathEscape(fileName)
	fallback := strings.ReplaceAll(strings.ReplaceAll(fileName, "\"", "_"), "\\", "_")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", fallback, escapedName))
	c.Header("Cache-Control", "no-store")
}
