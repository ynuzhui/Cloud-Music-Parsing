package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

const (
	CookieStatusUnknown = "unknown"
	CookieStatusValid   = "valid"
	CookieStatusInvalid = "invalid"
	cookiePoolTTL       = 45 * time.Second
)

type activeCookieItem struct {
	ID     uint
	MusicU string
}

type CookieVerifyResult struct {
	Valid       bool   `json:"valid"`
	Status      string `json:"status"`
	Nickname    string `json:"nickname"`
	VipType     int    `json:"vip_type"`
	VipLevel    int    `json:"vip_level"`
	RedVipLevel int    `json:"red_vip_level"`
	Error       string `json:"error,omitempty"`
}

func (s *ParseService) ExtractMusicU(raw string) string {
	return extractMusicUFromRaw(raw)
}

func (s *ParseService) VerifyNeteaseCookie(ctx context.Context, rawCookie string) (*CookieVerifyResult, error) {
	musicU := extractMusicUFromRaw(rawCookie)
	if musicU == "" {
		return nil, errors.New("Cookie 未包含 MUSIC_U")
	}

	result := &CookieVerifyResult{
		Valid:  false,
		Status: CookieStatusInvalid,
	}

	accountResp := struct {
		Code    int `json:"code"`
		Account *struct {
			VipType     int `json:"vipType"`
			RedVipLevel int `json:"redVipLevel"`
		} `json:"account"`
		Profile *struct {
			Nickname    string `json:"nickname"`
			VipType     int    `json:"vipType"`
			RedVipLevel int    `json:"redVipLevel"`
		} `json:"profile"`
	}{}

	if err := s.callNeteaseAPI(ctx, http.MethodPost, "https://music.163.com/api/nuser/account/get", nil, "", musicU, &accountResp); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	if accountResp.Code != 200 || accountResp.Account == nil || accountResp.Profile == nil || strings.TrimSpace(accountResp.Profile.Nickname) == "" {
		result.Error = "cookie is not logged in or has expired"
		return result, nil
	}

	levelResp := struct {
		Code int `json:"code"`
		Data *struct {
			Level int `json:"level"`
		} `json:"data"`
	}{}
	vipLevel := 0
	if err := s.callNeteaseAPI(ctx, http.MethodPost, "https://music.163.com/api/user/level", nil, "", musicU, &levelResp); err == nil {
		if levelResp.Code == 200 && levelResp.Data != nil {
			vipLevel = levelResp.Data.Level
		}
	}

	vipType := accountResp.Profile.VipType
	if vipType == 0 {
		vipType = accountResp.Account.VipType
	}
	redVipLevel := accountResp.Account.RedVipLevel
	if redVipLevel == 0 {
		redVipLevel = accountResp.Profile.RedVipLevel
	}
	result.Valid = true
	result.Status = CookieStatusValid
	result.Nickname = strings.TrimSpace(accountResp.Profile.Nickname)
	result.VipType = vipType
	result.VipLevel = vipLevel
	result.RedVipLevel = redVipLevel
	result.Error = ""
	return result, nil
}

func (s *ParseService) callNeteaseAPI(ctx context.Context, method, endpoint string, query url.Values, formBody string, musicU string, out any) error {
	if query == nil {
		query = url.Values{}
	}
	target := endpoint
	encodedQuery := query.Encode()
	if encodedQuery != "" {
		target += "?" + encodedQuery
	}

	var body io.Reader
	if formBody != "" {
		body = strings.NewReader(formBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	setNeteasePublicHeaders(req)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if strings.TrimSpace(musicU) != "" {
		req.AddCookie(&http.Cookie{Name: "MUSIC_U", Value: strings.TrimSpace(musicU)})
	}

	client := s.buildHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("网易上游接口状态异常：%d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func setNeteasePublicHeaders(req *http.Request) {
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("User-Agent", util.RandomUserAgent())
	spoofIP := util.RandomIPv4()
	req.Header.Set("X-Forwarded-For", spoofIP)
	req.Header.Set("Client-IP", spoofIP)
	req.Header.Set("X-Real-IP", spoofIP)
}

func extractMusicUFromRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "=") {
		return raw
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '\n' || r == '\r'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx <= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(part[:idx]), "MUSIC_U") {
			return strings.Trim(strings.TrimSpace(part[idx+1:]), "\"")
		}
	}
	return ""
}

func (s *ParseService) pickActiveCookie() string {
	now := time.Now()
	s.cookieMu.Lock()
	defer s.cookieMu.Unlock()

	if len(s.cookiePool) == 0 || now.After(s.cookiePoolExp) {
		s.refreshActiveCookiePoolLocked(now)
	}
	if len(s.cookiePool) == 0 {
		return ""
	}

	idx := s.cookieCursor % len(s.cookiePool)
	picked := s.cookiePool[idx]
	s.cookieCursor = (idx + 1) % len(s.cookiePool)
	s.touchCookieUsage(picked.ID, now)
	return picked.MusicU
}

func (s *ParseService) InvalidateCookiePool() {
	s.cookieMu.Lock()
	s.cookiePool = nil
	s.cookieCursor = 0
	s.cookiePoolExp = time.Time{}
	s.cookieMu.Unlock()
}

func (s *ParseService) refreshActiveCookiePoolLocked(now time.Time) {
	rows := make([]model.Cookie, 0)
	if err := s.db.Where("provider = ? AND active = ? AND status = ?", "netease", true, CookieStatusValid).Order("id asc").Find(&rows).Error; err != nil || len(rows) == 0 {
		rows = rows[:0]
		if err := s.db.Where("provider = ? AND active = ?", "netease", true).Order("id asc").Find(&rows).Error; err != nil || len(rows) == 0 {
			s.cookiePool = nil
			s.cookieCursor = 0
			s.cookiePoolExp = now.Add(cookiePoolTTL)
			return
		}
	}

	items := make([]activeCookieItem, 0, len(rows))
	for _, row := range rows {
		plain, err := s.box.Decrypt(row.ValueEncrypted)
		if err != nil {
			continue
		}
		musicU := extractMusicUFromRaw(plain)
		if musicU == "" {
			musicU = strings.TrimSpace(plain)
		}
		musicU = strings.TrimSpace(musicU)
		if musicU == "" {
			continue
		}
		items = append(items, activeCookieItem{
			ID:     row.ID,
			MusicU: musicU,
		})
	}

	s.cookiePool = items
	if len(items) == 0 {
		s.cookieCursor = 0
	} else if s.cookieCursor >= len(items) {
		s.cookieCursor = 0
	}
	s.cookiePoolExp = now.Add(cookiePoolTTL)
}

func (s *ParseService) touchCookieUsage(id uint, at time.Time) {
	if id == 0 {
		return
	}
	_ = s.db.Model(&model.Cookie{}).Where("id = ?", id).Updates(map[string]any{
		"call_count":   gorm.Expr("call_count + 1"),
		"last_used_at": &at,
	}).Error
}
